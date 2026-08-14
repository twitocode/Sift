package dedup

import (
	"context"
	"slices"
	"strings"

	"github.com/twitocode/sift/internal/common"
	"github.com/twitocode/sift/internal/store"
	"go.uber.org/zap"
)

type Deduplicator struct {
	store *store.PageStore
	log   *zap.Logger
	index *SimHashIndex
}

type CanonicalInfo struct {
	page    *common.Page
	similar []*common.Page
}

func NewDeduplicator(store *store.PageStore, log *zap.Logger) *Deduplicator {
	return &Deduplicator{
		store: store,
		index: NewSimHashIndex(3),
		log:   log,
	}
}

func (d *Deduplicator) Start(ctx context.Context) {
	d.log.Info("Started Deduplicator")
	d.HandleCanonicDuplicates(ctx)
	d.HandleRandomDuplicates(ctx)
	d.log.Info("Finished Deduplicator")
}

func (d *Deduplicator) HandleCanonicDuplicates(ctx context.Context) {
	pages, err := d.store.FindCanonicDuplicatesPages(ctx)

	canonicals := make(map[common.URL]*CanonicalInfo)
	unfiltered := make([]*common.Page, 0)

	if err != nil {
		d.log.Fatal("Could not get pages", zap.Error(err))
		return
	}

	for _, page := range pages {
		trimmedText := strings.TrimSpace(page.Text)
		if len(trimmedText) == 0 || page.ContentHash == 0 {
			d.log.Debug("Page has no text", zap.String("url", page.FinalURL.String()))
			continue
		}

		if IsProbablyCanonical(page) {
			canonicals[page.FoundCanonical] = &CanonicalInfo{
				page:    page,
				similar: make([]*common.Page, 0),
			}
			page.ResolvedCanonical = true
			continue //TODO: HANDLE LATER
		}

		unfiltered = append(unfiltered, page)
	}

	d.ReconcileConflicts(canonicals, unfiltered)

	for _, v := range canonicals {
		d.store.BatchAssignCanonical(ctx, v.page.ID, common.Map(v.similar, func(e *common.Page, _ int) (int64, bool) {
			return e.ID, true
		}))
	}
}

func (d *Deduplicator) HandleRandomDuplicates(ctx context.Context) {
	pages, err := d.store.FindPossibleDuplicatePages(ctx)

	if err != nil {
		d.log.Fatal("Could not get pages", zap.Error(err))
		return
	}

	set := NewDisjointSet(len(pages))

	for i, page := range pages {
		similar := d.index.FindSimilar(page.ContentHash)

		for _, candidate := range similar {
			if AreFingerprintsSimilar(page.ContentHash, candidate.fingerprint, 3) {
				set.Union(i, candidate.index)
			}
		}
		d.index.DryInsert(page.ContentHash, i)
	}

	clusters := make(map[int][]int)

	for i, _ := range pages {
		root := set.Find(i)
		clusters[root] = append(clusters[root], i)
	}

	for _, cluster := range clusters {
		if len(cluster) == 1 {
			continue
		}

		d.ReconcileClusterConflicts(ctx, cluster, pages)
	}
}

func PageIndex(pages []*common.Page, page *common.Page) int {
	i, _ := slices.BinarySearchFunc(pages, page, func(e *common.Page, t *common.Page) int {
		if e.ID == t.ID {
			return 0
		} else if t.ID < e.ID {
			return 1
		}

		return -1
	})
	return i
}

func (d *Deduplicator) ReconcileConflicts(canonicals map[common.URL]*CanonicalInfo, pages []*common.Page) {
	for _, page := range pages {
		foundInfo, ok := canonicals[page.FoundCanonical]

		if ok {
			page.ResolvedCanonical = true
			page.DuplicateOf = foundInfo.page.ID
			foundInfo.similar = append(foundInfo.similar, page)
		}
	}
}

func (d *Deduplicator) ReconcileClusterConflicts(ctx context.Context, cluster []int, pages []*common.Page) {
	ranks := make([]int, len(cluster))

	//find canonicals
	for i, pageIndex := range cluster {
		ranks[i] = 0

		page := pages[pageIndex]
		if IsProbablyCanonical(page) {
			ranks[i] += 50
		}

		//other pages find this one to be the canonical
		for j, otherPageIndex := range cluster {
			other := pages[otherPageIndex]
			if j == i {
				continue
			}

			if IsCanonicalOwner(page, other) {
				ranks[i] += 100
			}
		}

		if common.IsSuccessCode(page.StatusCode) {
			ranks[i] += 20
		}

		if strings.Join(strings.Fields(page.Text), "") != "" {
			ranks[i] += 5
		}

		if page.Title != "" {
			ranks[i] += 10
		}
	}

	highestRank := 0
	candidates := make([]int, 0)

	for i, rank := range ranks {
		if rank == highestRank {
			candidates = append(candidates, cluster[i])
		} else if rank > highestRank {
			candidates = []int{cluster[i]}
			highestRank = rank
		}
	}

	var electedPage *common.Page
	if len(candidates) == 1 {
		electedPage = pages[candidates[0]]
	}

	//choose lowest db id
	lowestId := pages[candidates[0]].ID
	pageIndex := candidates[0]

	for _, candidate := range candidates {
		page := pages[candidate]
		if lowestId > page.ID {
			lowestId = page.ID
			pageIndex = candidate
		}
	}

	electedPage = pages[pageIndex]
	duplicates := common.Map(cluster, func(e int, _ int) (int64, bool) {
		if pageIndex == e {
			return 0, false
		}

		page := pages[e]
		return page.ID, true
	})

	d.store.BatchAssignCanonical(ctx, electedPage.ID, duplicates)
}

func findByFingerprint(pages []*common.Page, f uint64) *common.Page {
	i := slices.IndexFunc(pages, func(page *common.Page) bool {
		return page.ContentHash == f
	})

	if i == -1 {
		return nil
	}

	return pages[i]
}

func findByUrl(pages []*common.Page, u common.URL) *common.Page {
	i := slices.IndexFunc(pages, func(page *common.Page) bool {
		return page.FinalURL == u
	})

	if i == -1 {
		return nil
	}

	return pages[i]
}

func IsProbablyCanonical(page *common.Page) bool {
	return page.FinalURL == page.FoundCanonical || page.FinalURL == page.FoundCanonical+"/" || page.FinalURL == page.FoundCanonical+".html" || page.FinalURL == page.FoundCanonical+"/index.html"
}
func IsCanonicalOwner(page *common.Page, other *common.Page) bool {
	return other.FoundCanonical == page.FinalURL || other.FoundCanonical == page.FinalURL+"/" || other.FoundCanonical == page.FinalURL+".html" || other.FoundCanonical == page.FinalURL+"/index.html"
}
