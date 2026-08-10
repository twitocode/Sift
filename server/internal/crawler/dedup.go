package crawler

import (
	"context"
	"fmt"
	"slices"
	"strings"

	"github.com/twitocode/sift/internal/common"
	"go.uber.org/zap"
)

type Deduplicator struct {
	store *PageStore
	log   *zap.Logger
	index *SimHashIndex
}

type CanonicalInfo struct {
	page    *Page
	similar []*Page
}

func NewDeduplicator(store *PageStore, log *zap.Logger) *Deduplicator {
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

	canonicals := make(map[URL]*CanonicalInfo)
	unfiltered := make([]*Page, 0)

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

		if page.FinalURL == page.FoundCanonical || page.FinalURL == page.FoundCanonical+"/" || page.FinalURL == page.FoundCanonical+".html" || page.FinalURL == page.FoundCanonical+"/index.html" {
			canonicals[page.FoundCanonical] = &CanonicalInfo{
				page:    page,
				similar: make([]*Page, 0),
			}
			page.ResolvedCanonical = true
			continue //TODO: HANDLE LATER
		}

		unfiltered = append(unfiltered, page)
	}

	d.ReconcileConflicts(canonicals, unfiltered)

	for _, v := range canonicals {
		d.store.BatchAssignCanonical(ctx, int(v.page.ID), common.Map(v.similar, func(e *Page, _ int) (int64, bool) {
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
		parent := set.Find(i)

		if parent == i {
			clusters[parent] = []int{parent}
		} else {
			if cluster, ok := clusters[parent]; ok {
				clusters[parent] = append(cluster, i)
			} else {
				clusters[parent] = []int{parent, i}
			}
		}
	}

	for parent, cluster := range clusters {
		if len(cluster) == 1 {
			continue
		}

		d.ReconcileClusterConflicts(cluster)
	}
}

func PageIndex(pages []*Page, page *Page) int {
	i, _ := slices.BinarySearchFunc(pages, page, func(e *Page, t *Page) int {
		if e.ID == t.ID {
			return 0
		} else if t.ID < e.ID {
			return 1
		}

		return -1
	})
	return i
}

func (d *Deduplicator) ReconcileConflicts(canonicals map[URL]*CanonicalInfo, pages []*Page) {
	for _, page := range pages {
		foundInfo, ok := canonicals[page.FoundCanonical]

		if ok {
			fmt.Println("Found canonical")
			page.ResolvedCanonical = true
			page.DuplicateOf = foundInfo.page.ID
			foundInfo.similar = append(foundInfo.similar, page)
		}
	}
}

func (d *Deduplicator) ReconcileClusterConflicts(cluster []int, pages []*Page) {

}

func findByFingerprint(pages []*Page, f uint64) *Page {
	i := slices.IndexFunc(pages, func(page *Page) bool {
		return page.ContentHash == f
	})

	if i == -1 {
		return nil
	}

	return pages[i]
}

func findByUrl(pages []*Page, u URL) *Page {
	i := slices.IndexFunc(pages, func(page *Page) bool {
		return page.FinalURL == u
	})

	if i == -1 {
		return nil
	}

	return pages[i]
}
