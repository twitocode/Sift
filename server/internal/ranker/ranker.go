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

	docs         map[int64]common.DocumentStats
	indexerStore *store.IndexerStore
	pageStore    *store.PageStore
	index        *common.SafeMap[string, []common.Posting]
	indexMeta    *common.IndexStats
}

func NewRanker(log *zap.Logger, cfg *common.Config, index *common.SafeMap[string, []common.Posting], indexerStore *store.IndexerStore, pageStore *store.PageStore) *Ranker {
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
	docs := common.ToMap(res, func(e common.DocumentStats) (int64, common.DocumentStats) {
		return e.PageID, e
	})

	r.docs = docs
}

func (r *Ranker) LoadIndexMeta(ctx context.Context) {
	meta := r.indexerStore.LoadLatestIndexMetadata(ctx)
	r.indexMeta = meta
}

func sortPagesByScore(pages []*common.Page, scores map[int64]float64) {
	slices.SortFunc(pages, func(a *common.Page, b *common.Page) int {
		switch {
		case scores[a.ID] > scores[b.ID]:
			return -1
		case scores[a.ID] < scores[b.ID]:
			return 1
		default:
			return 0
		}
	})

}

func (r *Ranker) Query(ctx context.Context, query string) []*common.Page {
	tokens := indexer.Tokenize(query)
	scores := make(map[int64]float64)

	for _, token := range tokens {
		postings, ok := r.index.Get(token)

		if !ok {
			continue
		}

		for _, posting := range postings {
			score, ok := scores[posting.PageID]
			if !ok {
				scores[posting.PageID] = 0
			}

			doc := r.docs[posting.PageID]
			score += CalculateBM25(len(postings), tokens, uint64(doc.TokenCount), posting.Frequency, *r.indexMeta)
			if posting.MatchesTitle {
				score += 10
			}
			scores[posting.PageID] = score
		}
	}

	results := make([]*common.Page, 0, len(scores))
	for id := range scores {
		pageInfo, err := r.pageStore.GetByID(ctx, id)
		if err != nil {
			continue
		}

		results = append(results, pageInfo)
	}

	sortPagesByScore(results, scores)

	fmt.Printf("Results:\n")
	for i, page := range results {
		if i == 50 {
			break
		}
		fmt.Printf("%.2f: %s\n", scores[page.ID], page.Title)
	}

	return results
}
