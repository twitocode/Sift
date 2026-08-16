package ranker

import (
	"context"
	"fmt"
	"slices"

	"github.com/twitocode/sift/internal/common"
	"github.com/twitocode/sift/internal/indexer"
	"github.com/twitocode/sift/internal/store"
	"go.uber.org/zap"
)

type Ranker struct {
	log *zap.Logger
	cfg *common.Config

	docs         map[uint32]uint32
	indexerStore *store.IndexerStore
	pageStore    *store.PageStore
	index        map[string][]common.Posting
	indexMeta    *common.IndexStats
}

func NewRanker(log *zap.Logger, cfg *common.Config, index map[string][]common.Posting, indexerStore *store.IndexerStore, pageStore *store.PageStore) *Ranker {
	return &Ranker{
		log:          log,
		cfg:          cfg,
		index:        index,
		indexerStore: indexerStore,
		pageStore:    pageStore,
	}
}

func (r *Ranker) LoadDocuments(ctx context.Context) {
	res := r.indexerStore.LoadAllDocuments(ctx)
	docs := common.ToMap(res, func(e common.DocumentStats) (uint32, uint32) {
		return uint32(e.PageID), e.TokenCount
	})

	r.docs = docs
}

func (r *Ranker) LoadIndexMeta(ctx context.Context) {
	meta := r.indexerStore.LoadLatestIndexMetadata(ctx)
	r.indexMeta = meta
}

func sortPagesByScore(pages []*common.Page, scores map[uint32]float64) {
	slices.SortFunc(pages, func(a *common.Page, b *common.Page) int {
		switch {
		case scores[uint32(a.ID)] > scores[uint32(b.ID)]:
			return -1
		case scores[uint32(a.ID)] < scores[uint32(b.ID)]:
			return 1
		default:
			return 0
		}
	})

}

func (r *Ranker) Query(ctx context.Context, query string) []*common.Page {
	tokens := indexer.Tokenize(query)
	scores := make(map[uint32]float64)

	for _, token := range tokens {
		postings, ok := r.index[token]

		if !ok {
			continue
		}

		for _, posting := range postings {
			score, ok := scores[posting.PageID]
			if !ok {
				scores[posting.PageID] = 0
			}

			tokenCount := r.docs[posting.PageID]
			score += CalculateBM25(len(postings), tokens, tokenCount, posting.Frequency, r.indexMeta)
			if posting.MatchesTitle {
				score += 10
			}
			scores[posting.PageID] = score
		}
	}

	results := make([]*common.Page, 0, len(scores))
	for id := range scores {
		pageInfo, err := r.pageStore.GetByID(ctx, int64(id))
		if err != nil {
			continue
		}

		results = append(results, pageInfo)
	}

	sortPagesByScore(results, scores)

	fmt.Printf("Query: %s\n", query)
	fmt.Printf("Results:\n\n")
	for i, page := range results {
		if i == 10 {
			break
		}
		fmt.Printf("%.2f: %s\n", scores[uint32(page.ID)], page.Title)
	}

	return results
}
