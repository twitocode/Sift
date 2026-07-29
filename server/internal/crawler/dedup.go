package crawler

import (
	"context"
	"fmt"
	"slices"
	"strings"

	"go.uber.org/zap"
)

type Deduplicator struct {
	store *PageStore
	log   *zap.Logger
	index *SimHashIndex
}

func NewDeduplicator(store *PageStore, log *zap.Logger) *Deduplicator {
	return &Deduplicator{
		store: store,
		index: NewSimHashIndex(3, false),
		log:   log,
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
		fingerprint := CreateSimhashFingerprint(page.Text)

		trimmedText := strings.TrimSpace(page.Text)
		if len(trimmedText) == 0 {
			d.log.Debug("Page has not text", zap.String("url", page.URL.String()))
			continue
		}
    
		if yes, other := d.index.TryInsert(fingerprint); !yes {
			otherPage := findByFingerprint(pages, other)
			d.log.Debug("Found duplicate", zap.String("current", string(page.URL)), zap.String("other", otherPage.URL.String()))
			removedCount++
		}

		if i%250 == 0 {
			d.log.Debug(fmt.Sprintf("Processed %d pages", i))
		}
	}
	d.log.Info("Finished Deduplicator", zap.Int("removed", removedCount))
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
