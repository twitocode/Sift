package indexer

import (
	"context"
	"slices"
	"sync"
	"time"

	"github.com/twitocode/sift/internal/common"
	"github.com/twitocode/sift/internal/crawler"
	"go.uber.org/zap"
)

type Indexer struct {
	log          *zap.Logger
	pageStore    *crawler.PageStore
	indexerStore *IndexerStore
	metrics      *IndexerMetrics

	index *common.SafeMap[string, []Posting]
}

func NewIndexer(log *zap.Logger, pageStore *crawler.PageStore, indexerStore *IndexerStore) *Indexer {
	return &Indexer{
		log:          log,
		pageStore:    pageStore,
		indexerStore: indexerStore,
		metrics:      NewIndexerMetrics(log),
		index:        common.NewSafeMap[string, []Posting](),
	}
}

func (in *Indexer) Start() {
	ctx, cancel := context.WithCancel(context.Background())

	in.log.Info("Started Indexing")
	var wg sync.WaitGroup

	wg.Add(1)
	//in.indexerStore.BeforeIndexing(ctx)
	go in.indexerStore.RunTimer(ctx, &wg, in.metrics)
	//TODO: make indexer run on a ticker (or cron job)
	//TODO: add a vector store - calculate embeddings alongside invert index
	pages, err := in.pageStore.GetAll(ctx)

	if err != nil {
		in.log.Fatal("Aborted Indexing", zap.Error(err))
		return
	}

	start := time.Now()
	in.metrics.DocumentsRead.Add(int64(len(pages)))

	indexStats := IndexStats{}

	for _, page := range pages {
		document := in.Index(ctx, page)
		indexStats.DocumentCount++
		indexStats.TotalTokenCount += uint64(document.TokenCount)
		in.indexerStore.Add(ctx, document)
		in.metrics.DocumentsIndexed.Add(1)
	}

	in.indexerStore.AddIndexMetadata(ctx, indexStats)
	in.shutdown(cancel)
	wg.Wait()

	indexStats.AverageDocLength = float64(indexStats.TotalTokenCount) / float64(indexStats.DocumentCount)

	elapsed := time.Since(start)
	in.metrics.TotalTokens.Add(int64(indexStats.TotalTokenCount))
	in.metrics.UniqueTerms.Add(int64(len(in.index.Keys())))

	in.metrics.PrintSummary(elapsed)
}

func (in *Indexer) Index(ctx context.Context, page *crawler.Page) *DocumentStats {
	textTokens := Tokenize(page.Text)
	titleTokens := Tokenize(page.Title)

	titlesStartIndex := len(textTokens)
	allTokens := slices.Concat(textTokens, titleTokens)

	in.metrics.BodyTokens.Add(int64(len(textTokens)))
	in.metrics.TitleTokens.Add(int64(len(titleTokens)))

	postingMap := make(map[string]Posting)
	document := &DocumentStats{
		TokenCount: uint32(len(allTokens)),
		PageID:     page.ID,
	}

	for i, token := range allTokens {
		if entry, ok := postingMap[token]; !ok {
			postingMap[token] = Posting{
				Frequency:    1,
				DocID:        page.ID,
				MatchesTitle: i >= titlesStartIndex,
			}
			in.metrics.TotalPostings.Add(1)
		} else {
			entry.Frequency += 1
			if !entry.MatchesTitle {
				entry.MatchesTitle = i >= titlesStartIndex
			}

			postingMap[token] = entry
		}

		if i >= titlesStartIndex {
			in.metrics.TitlePostings.Add(1)

		}
	}

	for token, posting := range postingMap {
		postings, ok := in.index.Get(token)
		if !ok {
			in.index.Set(token, []Posting{posting})
			continue
		}

		postings = append(postings, posting)
		in.index.Set(token, postings)
	}

	return document
}

func (in Indexer) shutdown(cancel context.CancelFunc) {
	in.indexerStore.Shutdown()
	cancel()
}
