package indexer

import (
	"context"
	"sync"
	"time"

	"github.com/twitocode/sift/internal/common"
	"github.com/twitocode/sift/internal/metrics"
	"github.com/twitocode/sift/internal/progress"
	"github.com/twitocode/sift/internal/store"
	"go.uber.org/zap"
)

type Indexer struct {
	log          *zap.Logger
	pageStore    *store.PageStore
	indexerStore *store.IndexerStore
	metrics      *metrics.IndexerMetrics
	cfg          *common.Config

	index   *common.SafeMap[string, []common.Posting]
	elapsed time.Duration
}

func NewIndexer(log *zap.Logger, cfg *common.Config, pageStore *store.PageStore, indexerStore *store.IndexerStore) *Indexer {
	return &Indexer{
		log:          log,
		pageStore:    pageStore,
		indexerStore: indexerStore,
		cfg:          cfg,
		metrics:      metrics.NewIndexerMetrics(log),
		index:        common.NewSafeMap[string, []common.Posting](),
	}
}

func (in *Indexer) Generate() map[string][]common.Posting {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	in.log.Info("Started Indexing")
	var storeWg sync.WaitGroup

	storeWg.Add(1)
	//in.indexerStore.BeforeIndexing(ctx)
	go in.indexerStore.RunTimer(ctx, &storeWg, in.metrics)
	//TODO: make indexer run on a ticker (or cron job)
	//TODO: add a vector store - calculate embeddings alongside invert index

	start := time.Now()
	indexStats := common.IndexStats{}

	pageSearchIndex := 0
	totalPageCount, err := in.pageStore.GetTotalCrawledPageCount(ctx)
	if err != nil {
		in.log.Error("Could not get page count from db", zap.Error(err))
		in.shutdown(cancel)
		storeWg.Wait()
		return in.index.ToMap()
	}

	batchSize := totalPageCount / 10
	if batchSize < 1 {
		batchSize = 1
	}

	workerCount := 100
	workerChan := make(chan []*common.Page, workerCount)
	var workerWg sync.WaitGroup

	in.metrics.DocumentsTotal.Store(totalPageCount)
	in.metrics.BatchSize.Store(batchSize)

	for {
		pages, err := in.pageStore.GetPaginatedPageBatch(ctx, int64(pageSearchIndex), uint16(batchSize))
		if err != nil {
			in.log.Error("Could not get page batch from db", zap.Int("index", pageSearchIndex), zap.Error(err))
			break
		}
		if len(pages) == 0 {
			break
		}

		chunks := common.SplitIntoNChunks(pages, workerCount)
		workerWg.Add(len(chunks))
		in.spawnWorkers(ctx, len(chunks), &workerInfo{
			indexStats: &indexStats,
			wg:         &workerWg,
			workerChan: workerChan,
		})

		for _, chunk := range chunks {
			workerChan <- chunk
		}

		in.metrics.DocumentsRead.Add(int64(len(pages)))
		in.metrics.BatchesRead.Add(1)
		in.metrics.CurrentBatch.Store(in.metrics.BatchesRead.Load())
		workerWg.Wait()

		pageSearchIndex = int(pages[len(pages)-1].ID)
	}

	close(workerChan)
	if indexStats.DocumentCount > 0 {
		indexStats.AverageDocLength = float64(indexStats.TotalTokenCount) / float64(indexStats.DocumentCount)
	}
	in.indexerStore.AddIndexMetadata(ctx, &indexStats)
	in.shutdown(cancel)
	storeWg.Wait()

	in.elapsed = time.Since(start)
	return in.index.ToMap()
}

func (in *Indexer) PrintSummary() {
	in.metrics.PrintSummary(in.elapsed)
}

func (in *Indexer) Snapshot() progress.Snapshot {
	return progress.Snapshot{
		Index: progress.IndexSnapshot{
			DocumentsTotal:   in.metrics.DocumentsTotal.Load(),
			DocumentsRead:    in.metrics.DocumentsRead.Load(),
			DocumentsIndexed: in.metrics.DocumentsIndexed.Load(),
			DocumentsStored:  in.metrics.DocumentsStored.Load(),
			BatchesRead:      in.metrics.BatchesRead.Load(),
			CurrentBatch:     in.metrics.CurrentBatch.Load(),
			BatchSize:        in.metrics.BatchSize.Load(),
			TotalTokens:      in.metrics.TotalTokens.Load(),
			UniqueTerms:      in.metrics.UniqueTerms.Load(),
			Flushes:          in.metrics.Flushes.Load(),
		},
	}
}

func (in *Indexer) Index(ctx context.Context, page *common.Page) *common.DocumentStats {
	textTokens := Tokenize(page.Text)
	titleTokens := Tokenize(page.Title)

	in.metrics.BodyTokens.Add(int64(len(textTokens)))
	in.metrics.TitleTokens.Add(int64(len(titleTokens)))

	postingMap := make(map[string]common.Posting)
	document := &common.DocumentStats{
		TokenCount: uint32(len(textTokens) + len(textTokens)),
		PageID:     page.ID,
	}

	for _, token := range textTokens {
		if entry, ok := postingMap[token]; !ok {
			postingMap[token] = common.Posting{
				Frequency:    1,
				PageID:       uint32(page.ID),
				MatchesTitle: false,
			}
			in.metrics.TotalPostings.Add(1)
		} else {
			entry.Frequency += 1
			postingMap[token] = entry
		}
	}

	for _, token := range titleTokens {
		if entry, ok := postingMap[token]; !ok {
			postingMap[token] = common.Posting{
				Frequency:    1,
				PageID:       uint32(page.ID),
				MatchesTitle: true,
			}
			in.metrics.TotalPostings.Add(1)
		} else {
			entry.Frequency += 1
			entry.MatchesTitle = true
			postingMap[token] = entry
		}
		in.metrics.TitlePostings.Add(1)
	}

	for token, posting := range postingMap {
		postings, ok := in.index.Get(token)
		if !ok {
			in.index.Set(token, []common.Posting{posting})
			continue
		}

		postings = append(postings, posting)
		in.index.Set(token, postings)
	}

	return document
}

type workerInfo struct {
	indexStats *common.IndexStats
	wg         *sync.WaitGroup
	workerChan <-chan []*common.Page
}

func (in *Indexer) spawnWorkers(ctx context.Context, count int, info *workerInfo) {
	for i := range count {
		go func() {
			defer info.wg.Done()

		Outer:
			for {
				select {
				case <-ctx.Done():
					if ctx.Err() != nil {
						in.log.Info("Stopped worker", zap.Int("worker", i))
						return
					}
				case pages, ok := <-info.workerChan:
					if !ok {
						return
					}

					for _, page := range pages {
						document := in.Index(ctx, page)

						info.indexStats.Lock()
						info.indexStats.DocumentCount++
						info.indexStats.TotalTokenCount += uint64(document.TokenCount)
						info.indexStats.Unlock()

						in.indexerStore.Add(ctx, document)
						in.metrics.DocumentsIndexed.Add(1)
						in.metrics.TotalTokens.Store(int64(info.indexStats.TotalTokenCount))
						in.metrics.UniqueTerms.Store(int64(in.index.Length()))
					}

					break Outer
				}
			}
		}()
	}
}

func (in Indexer) shutdown(cancel context.CancelFunc) {
	in.indexerStore.Shutdown()
	cancel()
}
