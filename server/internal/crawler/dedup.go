package crawler

import (
	"context"
	"fmt"
	"slices"
	"strings"

	"go.uber.org/zap"
)

type Deduplicator struct {
	store     *PageStore
	log       *zap.Logger
	index     *SimHashIndex
	conflicts map[URL][]*Page
}

func NewDeduplicator(store *PageStore, log *zap.Logger) *Deduplicator {
	return &Deduplicator{
		store:     store,
		index:     NewSimHashIndex(3, false),
		log:       log,
		conflicts: make(map[URL][]*Page),
	}
}

func (d *Deduplicator) Start(ctx context.Context) {
	d.log.Info("Started Deduplicator")
	pages, err := d.store.GetAll(ctx)
	removedCount := 0

	if err != nil {
		d.log.Fatal("Could not get pages", zap.Error(err))
		return
	}

	for i, page := range pages {
		trimmedText := strings.TrimSpace(page.Text)
		if len(trimmedText) == 0 || page.ContentHash == 0 {
			d.log.Debug("Page has not text", zap.String("url", page.URL.String()))
			continue
		}

		if ok, other := d.index.TryInsert(page.ContentHash); !ok {
			otherPage := findByFingerprint(pages, other)

			_, pageConflictsExists := d.conflicts[page.URL]
			_, otherPageConflictsExists := d.conflicts[otherPage.URL]

			if pageConflictsExists && !otherPageConflictsExists {
				d.conflicts[page.URL] = append(d.conflicts[page.URL], otherPage)
			} else if !pageConflictsExists && otherPageConflictsExists {
				d.conflicts[otherPage.URL] = append(d.conflicts[otherPage.URL], page)
			} else {
				d.conflicts[page.URL] = []*Page{otherPage}
			}

			d.log.Debug("Found duplicate", zap.String("current", string(page.URL)), zap.String("other", otherPage.URL.String()))
			removedCount++
		}

		if i%250 == 0 {
			d.log.Debug(fmt.Sprintf("Processed %d pages", i))
		}
	}

	d.ReconcileConflicts(pages)
	d.log.Info("Finished Deduplicator", zap.Int("removed", removedCount))
}

// elects a canonical page
func (d *Deduplicator) ReconcileConflicts(pages []*Page) {
	//TODO: properly reconcile conflicts
	// for main, others := range d.conflicts {
	// 	page := findByUrl(pages, main)
	// 	// if page == nil {
	// 	//   //for some reason
	// 	//   for _, other := range others {
	// 	//     _, otherPageConflictsExists := d.conflicts[other.URL]

	// 	//     if !otherPageConflictsExists {
	// 	//       d.conflicts[other.URL] = others[1:]
	// 	//       others =
	// 	//     }
	// 	//   }
	// 	// }

	// 	var earliestDiscovery time.Time = page.CrawledAt

	// 	for _, otherPage := range others {
	// 		if otherPage.CrawledAt.Before(earliestDiscovery) {
	// 			earliestDiscovery = otherPage.CrawledAt
	// 		}
	// 	}

	// 	earliestPages := common.Map(others, func(e *Page) bool {
	//     //within a day of each other
	// 		if earliestDiscovery.Sub(e.CrawledAt) < time.Hour*24 {
	// 			return true
	// 		}

	// 		return false
	// 	})

	//   for _, page := range earliestPages {

	//   }
	// }
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
		return page.URL == u
	})

	if i == -1 {
		return nil
	}

	return pages[i]
}
