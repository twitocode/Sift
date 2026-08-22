package ranker

import (
	"context"
	"fmt"
	"slices"
	"strings"
	"time"

	"github.com/patrickmn/go-cache"
	"github.com/twitocode/sift/internal/common"
	"github.com/twitocode/sift/internal/indexer"
	"github.com/twitocode/sift/internal/store"
	"go.uber.org/zap"
	"golang.org/x/exp/mmap"
)

type Ranker struct {
	log *zap.Logger
	cfg *common.Config

	docs         map[uint32]uint32
	indexerStore *store.IndexerStore
	pageStore    *store.PageStore
	terms        map[string]indexer.TermData
	indexMeta    *common.IndexStats

	pagesCache    *cache.Cache
	postingReader *mmap.ReaderAt
}

func NewRanker(log *zap.Logger, cfg *common.Config, terms map[string]indexer.TermData, indexerStore *store.IndexerStore, pageStore *store.PageStore) *Ranker {
	return &Ranker{
		log:           log,
		cfg:           cfg,
		terms:         terms,
		indexerStore:  indexerStore,
		pageStore:     pageStore,
		pagesCache:    cache.New(5*time.Minute, 10*time.Minute),
		postingReader: indexer.CreateMMapReader(),
	}
}

func (r *Ranker) LoadDocuments(ctx context.Context) {
	res := r.indexerStore.LoadAllDocuments(ctx)
	docs := common.ToMap(res, func(e common.DocumentStats) (uint32, uint32) {
		return uint32(e.PageID), e.TokenCount
	})

	r.docs = docs
	r.log.Info("Loaded all documents", zap.Int("count", len(docs)))
}

func (r *Ranker) LoadIndexMeta(ctx context.Context) {
	meta := r.indexerStore.LoadLatestIndexMetadata(ctx)
	r.indexMeta = meta
	r.log.Info("Loaded Recent index meta")
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

func (r *Ranker) Query(ctx context.Context, query string) []common.SearchResult {
	query = strings.ToLower(query)
	tokens := indexer.Tokenize(query)
	scores := make(map[uint32]float64)

	//TODO: need a better way to handle cases like 'gItHuB' that dont match tokens without blowing up the index

	candidatesHeap := NewBestCandidateHeap(50)

	for _, token := range tokens {
		data, ok := r.terms[token]

		if !ok {
			continue
		}

		postings := indexer.LoadIndexSection(r.postingReader, data.ByteOffset, data.Count)

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

			if posting.MatchesDomain {
				score += 50
			}

			scores[posting.PageID] = score
		}
	}

	for id, score := range scores {
		candidatesHeap.Add(&idScore{
			id:    int32(id),
			score: score,
		})
	}

	results := make([]*common.Page, 0)
	for _, v := range candidatesHeap.values {
		pageInfo, ok := r.pagesCache.Get(string(v.id))

		if !ok {
			pageInfo, err := r.pageStore.GetByID(ctx, int64(v.id))

			if err != nil {
				continue
			}
			r.pagesCache.Set(string(v.id), pageInfo, cache.DefaultExpiration)
			results = append(results, pageInfo)
		} else {
			results = append(results, pageInfo.(*common.Page))
		}
	}

	sortPagesByScore(results, scores)
	//logResults(query, results, scores)

	searchResults := make([]common.SearchResult, len(results))
	for i, result := range results {
		desc := result.Description
		if len(desc) > 300 {
			desc = truncateString(result.Description, 40)
			if desc[len(desc)-1] == '.' {
				desc += ".."
			} else {
				desc += "..."
			}
		}

		searchResults[i] = common.SearchResult{
			Title:   result.Title,
			OGTitle: result.OGTitle,
			Favicon: result.Favicon.String(),
			Desc:    desc,
			Url:     result.FinalURL.String(),
		}
	}
	return searchResults
}

func logResults(query string, results []*common.Page, scores map[uint32]float64) {
	fmt.Printf("\nQuery: %s\n", query)
	fmt.Printf("Results:\n\n")
	for i, page := range results {
		if i == 10 {
			break
		}
		fmt.Printf("%.2f: %s\n", scores[uint32(page.ID)], page.Title)
	}
}

func truncateString(str string, n int) string {
	words := strings.Fields(str)

	if len(words) <= n {
		return str
	}

	return strings.Join(words[:n], " ")
}
