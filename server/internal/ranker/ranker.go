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

	indexMeta *common.IndexStats

	pagesCache    *cache.Cache
	postingReader *mmap.ReaderAt
}

func NewRanker(log *zap.Logger, cfg *common.Config, terms map[string]indexer.TermData, indexMeta *common.IndexStats, indexerStore *store.IndexerStore, pageStore *store.PageStore) *Ranker {
	return &Ranker{
		log:           log,
		cfg:           cfg,
		terms:         terms,
		indexMeta:     indexMeta,
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

func (r *Ranker) Query(ctx context.Context, query string) QueryResult {
	startTime := time.Now()
	query = strings.ToLower(query)

  //new system keeps track of original token from a query and the stemmed-normalized version
  //It also keeps track of coverage. Page with 10 tokens is ranked lower than a page with 1 github, 1 mcp, and 1 repository token.
  
	originalTokens := strings.Fields(query)
	tokens := queryLookupTokens(query)

	scores := make(map[uint32]float64)
	matchedOriginals := make(map[uint32]map[string]struct{})

	candidatesHeap := NewBestCandidateHeap(50)
	tokenStats := make(map[string]TokenStats)
	pagesQueried := make(map[uint32]struct{})

	var totalPostingScanDuration int64
	var postingScanCount int

	for _, token := range tokens {
		data, ok := r.terms[token]

		if !ok {
			continue
		}

		postingStartTime := time.Now()
		postings := indexer.LoadIndexSection(r.postingReader, data.ByteOffset, data.Count)
		idf := ComputeIDF(r.indexMeta, float64(len(postings)))
		covered := originalsCoveredByToken(token, originalTokens)

		for _, posting := range postings {
			score := scores[posting.PageID]
			tokenCount := r.docs[posting.PageID]

			score += CalculateBM25(len(postings), tokens, tokenCount, posting.BodyFrequency, r.indexMeta)

			score += math.Pow(4.5, float64(posting.TitleFrequency))
			score += domainMatchBoost(posting.DomainFrequency, idf)
			score += urlMatchBoost(posting.URLFrequency, idf)

			if token == query || strings.Contains(token, query) {
				score += 100
			}
			if slices.Contains(originalTokens, token) {
				score += idf * 2
			}
			scores[posting.PageID] = score

			if len(covered) > 0 {
				seen := matchedOriginals[posting.PageID]
				if seen == nil {
					seen = make(map[string]struct{})
					matchedOriginals[posting.PageID] = seen
				}
				for _, original := range covered {
					seen[original] = struct{}{}
				}
			}

			pagesQueried[posting.PageID] = struct{}{}
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
		totalPostingScanDuration += scanTime
		postingScanCount++
	}

	if len(originalTokens) > 0 {
		for id, score := range scores {
			scores[id] = score * coverageMultiplier(len(matchedOriginals[id]), len(originalTokens))
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

	searchResults := make([]common.SearchResult, 0)
	duplicates := collectDuplicateURLs(results)
	for _, result := range results {
		if result.DuplicateOf > -1 {
			continue
		}

		desc := result.Description
		if len(desc) > 300 {
			desc = common.TruncateString(result.Description, 40)
			if desc[len(desc)-1] == '.' {
				desc += ".."
			} else {
				desc += "..."
			}
		}

		d := duplicates[result.ID]
		if d == nil {
			d = []string{}
		}

		searchResults = append(searchResults, common.SearchResult{
			Title:       result.Title,
			OGTitle:     result.OGTitle,
			Favicon:     result.Favicon.String(),
			Desc:        desc,
			Url:         result.FinalURL.String(),
			OriginalUrl: result.RequestedURL.String(),
			Duplicates:  d,
			Score:       scores[uint32(result.ID)],
			TitleTokens: len(indexer.Tokenize(result.Title)),
			BodyTokens:  len(indexer.Tokenize(result.Text)),
		})
	}

	out := QueryResult{
		Results:                     searchResults,
		Count:                       len(searchResults),
		TimeElapsed:                 time.Since(startTime).Milliseconds(),
		TokenStats:                  tokenStats,
		AveragePostingsScanDuration: averagePostingScanDuration(totalPostingScanDuration, postingScanCount),
		PossibleResultsQueried:      len(pagesQueried),
		IndexerMetrics:              ToSimpleIndexerMetrics(r.indexMeta),
	}

	return out
}
  
func averagePostingScanDuration(totalDuration int64, scanCount int) float64 {
	if scanCount == 0 {
		return 0
	}
	return float64(totalDuration) / float64(scanCount)
}

func urlMatchBoost(frequency uint32, idf float64) float64 {
	return idf * float64(frequency) * 0.24
}

func domainMatchBoost(frequency uint32, idf float64) float64 {
	return idf * float64(frequency) * 1.2
}

func coverageMultiplier(matched, total int) float64 {
	if total <= 0 {
		return 1
	}
	if matched <= 0 {
		return 0
	}
	if matched > total {
		matched = total
	}
	ratio := float64(matched) / float64(total)
	return ratio * ratio
}
