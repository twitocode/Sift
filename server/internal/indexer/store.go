package indexer

import (
	"context"
	"database/sql"
	"sync"
	"time"

	"github.com/twitocode/sift/internal/db"
	"go.uber.org/zap"
)

type IndexerStore struct {
	sqliteDb   *sql.DB
	queries    *db.Queries
	log        *zap.Logger
	bufferChan chan *DocumentStats
	buffer     []*DocumentStats
	bufferSize int
	metrics    *IndexerMetrics

	mu sync.Mutex
}

func NewIndexerStore(sqliteDb *sql.DB, log *zap.Logger) *IndexerStore {
	bufferSize := 7500

	return &IndexerStore{
		queries:    db.New(sqliteDb),
		sqliteDb:   sqliteDb,
		log:        log,
		bufferSize: bufferSize,
		bufferChan: make(chan *DocumentStats, bufferSize*2),
		buffer:     make([]*DocumentStats, 0),
	}
}

func (is *IndexerStore) RunTimer(ctx context.Context, wg *sync.WaitGroup, metrics *IndexerMetrics) {
	is.metrics = metrics

	ticker := time.NewTicker(time.Second * 4)
	defer ticker.Stop()

	runFlush := func() {
		newCtx, cancel := context.WithTimeout(context.Background(), time.Second*10)
		defer cancel()
		is.flush(newCtx)
	}

	drain := func() {
		for doc := range is.bufferChan {
			is.buffer = append(is.buffer, doc)
		}
	}

	is.log.Info("Started Indexer Batcher")
	for {
		select {
		case <-ctx.Done():
			drain()
			runFlush()
			wg.Done()
			return
		case page, ok := <-is.bufferChan:
			if ctx.Err() != nil {
				drain()
				runFlush()
				wg.Done()
				return
			}
			if !ok {
				drain()
				runFlush()
				wg.Done()
				return
			}

			is.buffer = append(is.buffer, page)
			if len(is.buffer) >= is.bufferSize {
				runFlush()
			}

		case <-ticker.C:
			is.log.Debug("Flushed Indexer buffer")
			runFlush()
		}
	}
}

func (is *IndexerStore) flush(ctx context.Context) {
	is.metrics.DocumentsStored.Add(int64(len(is.buffer)))
	is.metrics.Flushes.Add(1)

	is.BatchAddDocumentMetadata(ctx, is.buffer)
	is.buffer = is.buffer[:0]
}

// doing a direct insert for now
func (is *IndexerStore) Add(ctx context.Context, sm *DocumentStats) error {
	select {
	case <-ctx.Done():
		return nil
	case is.bufferChan <- sm:
		return nil
	}
}
func (is *IndexerStore) BeforeIndexing(ctx context.Context) {
	is.log.Info("Attempting to delete previous document stats")
	err := is.queries.DeleteAllDocuments(ctx)
	if err != nil {
		is.log.Fatal("Sqlite delete all error", zap.Error(err))
		is.metrics.StoreErrors.Add(1)
		return
	}
	is.log.Info("Successfully cleaned up indexing documents")
}

func (is *IndexerStore) AddIndexMetadata(ctx context.Context, data IndexStats) {
	err := is.queries.AddIndexMeta(ctx, db.AddIndexMetaParams{
		DocumentCount:    int64(data.DocumentCount),
		TotalTokenCount:  int64(data.TotalTokenCount),
		AverageDocLength: int64(data.AverageDocLength),
	})

	if err != nil {
		is.log.Error("Sqlite insert error", zap.Error(err))
		is.metrics.StoreErrors.Add(1)
	}
}

func (is *IndexerStore) BatchAddDocumentMetadata(ctx context.Context, meta []*DocumentStats) {
	tx, err := is.sqliteDb.Begin()

	if err != nil {
		is.log.Error("Sqlite transaction error", zap.Error(err))
		is.metrics.StoreErrors.Add(1)
		tx.Rollback()
		return
	}

	qtx := is.queries.WithTx(tx)

	for _, data := range meta {
		err := qtx.AddDocumentMeta(ctx, db.AddDocumentMetaParams{
			TokenCount: int64(data.TokenCount),
			PageID:     data.PageID,
		})

		if err != nil {
			is.log.Error("Sqlite insert in transaction error", zap.Error(err))
			return
		}
	}

	err = tx.Commit()
	if err != nil {
		is.log.Error(
			"Sqlite write error",
			zap.Error(err),
		)
		tx.Rollback()
		return
	}

}

func (is *IndexerStore) LoadLatestIndexMetadata(ctx context.Context) *IndexStats {
	data, err := is.queries.GetLatestIndexMeta(ctx)

	if err != nil {
		is.log.Error("Sqlite select error", zap.Error(err))
		is.metrics.StoreErrors.Add(1)
		return nil
	}

	return &IndexStats{
		DocumentCount:    uint64(data.DocumentCount),
		TotalTokenCount:  uint64(data.TotalTokenCount),
		AverageDocLength: float64(data.AverageDocLength),
	}
}

func (is *IndexerStore) LoadAllDocuments(ctx context.Context) []DocumentStats {
	res, err := is.queries.GetAllDocumentMeta(ctx)

	if err != nil {
		is.log.Error("Sqlite select error", zap.Error(err))
		is.metrics.StoreErrors.Add(1)
		return nil
	}

	out := make([]DocumentStats, 0)

	for _, data := range res {
		stats := DocumentStats{
			TokenCount: uint32(data.TokenCount),
			ID:         data.ID,
			PageID:     data.PageID,
		}

		out = append(out, stats)
	}

	return out
}

func (is *IndexerStore) Shutdown() {
	close(is.bufferChan)
}

func BoolToInt64(b bool) int64 {
	if b {
		return 1
	}

	return 0
}
