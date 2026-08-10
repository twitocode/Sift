package indexer

import (
	"context"
	"database/sql"
	"sync"

	"github.com/twitocode/sift/internal/db"
	"go.uber.org/zap"
)

type IndexerStore struct {
	sqliteDb *sql.DB
	queries  *db.Queries
	log      *zap.Logger

	mu sync.Mutex
}

func NewIndexerStore(sqliteDb *sql.DB, log *zap.Logger) *IndexerStore {
	return &IndexerStore{
		queries:  db.New(sqliteDb),
		sqliteDb: sqliteDb,
		log:      log,
	}
}

func (ps *IndexerStore) AddIndexMetadata(ctx context.Context, data IndexStats) {
	err := ps.queries.AddIndexMeta(ctx, db.AddIndexMetaParams{
		DocumentCount:    int64(data.DocumentCount),
		TotalTokenCount:  int64(data.TotalTokenCount),
		AverageDocLength: int64(data.AverageDocLength),
	})

	if err != nil {
		ps.log.Error("Sqlite insert error", zap.Error(err))
	}
}

func (ps *IndexerStore) BatchAddDocumentMetadata(ctx context.Context, meta map[int64]DocumentStats) {
	tx, err := ps.sqliteDb.Begin()

	if err != nil {
		ps.log.Error("Sqlite transaction error", zap.Error(err))
		tx.Rollback()
		return
	}

	for pageId, data := range meta {
		err := ps.queries.AddDocumentMeta(ctx, db.AddDocumentMetaParams{
			TokenCount: int64(data.TokenCount),
			PageID:     pageId,
		})

		if err != nil {
			ps.log.Error("Sqlite insert in transaction error", zap.Error(err))
			tx.Rollback()
			return
		}
	}

	tx.Commit()
}

func (ps *IndexerStore) LoadLatestIndexMetadata(ctx context.Context) *IndexStats {
	data, err := ps.queries.GetLatestIndexMeta(ctx)

	if err != nil {
		ps.log.Error("Sqlite select error", zap.Error(err))
	}

	return &IndexStats{
		DocumentCount:    uint64(data.DocumentCount),
		TotalTokenCount:  uint64(data.TotalTokenCount),
		AverageDocLength: float64(data.AverageDocLength),
	}
}

func (ps *IndexerStore) LoadAllDocuments(ctx context.Context) map[int64]DocumentStats {
	res, err := ps.queries.GetAllDocumentMeta(ctx)

	if err != nil {
		ps.log.Error("Sqlite select error", zap.Error(err))
	}

	out := make(map[int64]DocumentStats)

	for _, data := range res {
		stats := DocumentStats{
			TokenCount: uint32(data.TokenCount),
			ID:         data.ID,
		}

		out[data.PageID] = stats
	}

	return out
}

func (ps *IndexerStore) Shutdown() {

}

func BoolToInt64(b bool) int64 {
	if b {
		return 1
	}

	return 0
}
