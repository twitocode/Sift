package ranker

import (
	"context"
	"math"
	"slices"
	"strings"
	"time"

	"github.com/patrickmn/go-cache"
	"github.com/twitocode/sift/internal/common"
	"github.com/twitocode/sift/internal/indexer"
	"github.com/twitocode/sift/internal/metrics"
	"github.com/twitocode/sift/internal/store"
	"go.uber.org/zap"
	"golang.org/x/exp/mmap"
)

type Ranker struct {
	log *zap.Logger
	cfg *common.Config

	docs  map[uint32]uint32
	terms map[string]indexer.TermData

	indexerStore *store.IndexerStore
	pageStore    *store.PageStore

	indexMeta    *common.IndexStats
	indexMetrics *metrics.IndexerMetrics

	pagesCache    *cache.Cache
	postingReader *mmap.ReaderAt
}

func NewRanker(log *zap.Logger, cfg *common.Config, terms map[string]indexer.TermData, indexerStore *store.IndexerStore, pageStore *store.PageStore, indexMetrics *metrics.IndexerMetrics) *Ranker {
	return &Ranker{
		log:           log,
		cfg:           cfg,
		terms:         terms,
		indexerStore:  indexerStore,
		pageStore:     pageStore,
		pagesCache:    cache.New(5*time.Minute, 10*time.Minute),
		postingReader: indexer.CreateMMapReader(),
		indexMetrics:  indexMetrics,
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

func (r *Ranker) Query(ctx context.Context, query string) QueryResult {
	startTime := time.Now()
	query = strings.ToLower(query)
	tokens := indexer.Tokenize(query)
	scores := make(map[uint32]float64)

	//TODO: need a better way to handle cases like 'gItHuB' that dont match tokens without blowing up the index

	candidatesHeap := NewBestCandidateHeap(50)
	tokenStats := make(map[string]TokenStats)

	var averagePostingScanDuration float64
	pagesQueried := make(map[uint32]struct{})

	for i, token := range tokens {
		data, ok := r.terms[token]

		if !ok {
			continue
		}

		postingStartTime := time.Now()
		postings := indexer.LoadIndexSection(r.postingReader, data.ByteOffset, data.Count)

		for _, posting := range postings {
			score, ok := scores[posting.PageID]
			if !ok {
				scores[posting.PageID] = 0
			}

			tokenCount := r.docs[posting.PageID]

			score += CalculateBM25(len(postings), tokens, tokenCount, posting.BodyFrequency, r.indexMeta)
			score += math.Pow(4.5, float64(posting.TitleFrequency))
			score += math.Pow(6.5, float64(posting.DomainFrequency))

			scores[posting.PageID] = score

			if _, ok := pagesQueried[posting.PageID]; !ok {
				pagesQueried[posting.PageID] = struct{}{}
			}
		}

		scanTime := time.Since(postingStartTime).Microseconds()

		if stats, ok := tokenStats[token]; !ok {
			tokenStats[token] = TokenStats{
				PostingsCount: data.Count,
				ScanTime:      scanTime,
			}
		} else {
			stats.ScanTime = scanTime
			tokenStats[token] = stats
		}
		if i > 0 {
			averagePostingScanDuration += float64(scanTime) / float64(i)
		} else {
			averagePostingScanDuration = float64(scanTime)
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
			desc = common.TruncateString(result.Description, 40)
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
			Score:   scores[uint32(result.ID)],
      TitleTokens: len(indexer.Tokenize(result.Title)),
      BodyTokens: len(indexer.Tokenize(result.Text)),
		}
	}

	out := QueryResult{
		Results:                     searchResults,
		Count:                       len(searchResults),
		TimeElapsed:                 time.Since(startTime).Milliseconds(),
		TokenStats:                  tokenStats,
		AveragePostingsScanDuration: averagePostingScanDuration,
		PossibleResultsQueried:      len(pagesQueried),
		IndexerMetrics:              ToSimpleIndexerMetrics(r.indexMetrics),
	}

	return out
}
