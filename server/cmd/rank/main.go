package main

import (
	"context"
	"database/sql"
	"os"

	"github.com/twitocode/sift/internal/common"
	"github.com/twitocode/sift/internal/indexer"
	"github.com/twitocode/sift/internal/progress"
	"github.com/twitocode/sift/internal/ranker"
	"github.com/twitocode/sift/internal/store"
	"go.uber.org/zap"

	_ "modernc.org/sqlite"
)

func main() {
	log := common.NewLogger(os.Getenv, zap.InfoLevel)
	cfg := common.NewConfig(os.Getenv)

	sqliteDb, err := sql.Open("sqlite", common.SQLitePath())

	if err != nil {
		log.Fatal("Sqlite connection error", zap.Error(err))
	}
	log.Info("Connected to Sqlite")

	pageStore := store.NewPageStore(sqliteDb, log)
	indexerStore := store.NewIndexerStore(sqliteDb, log)

	in := indexer.NewIndexer(log, cfg, pageStore, indexerStore)
	type indexResult struct {
		index *common.SafeMap[string, []common.Posting]
	}
	result := make(chan indexResult, 1)
	done := make(chan error, 1)
	go func() {
		result <- indexResult{index: in.Generate()}
		close(done)
	}()

	if err := progress.Run("index", in.Snapshot, done); err != nil {
		log.Error("progress ui", zap.Error(err))
	}

	index := (<-result).index
	ctx := context.Background()
	ranker := ranker.NewRanker(log, cfg, index, indexerStore, pageStore)
	ranker.LoadDocuments(ctx)
	ranker.LoadIndexMeta(ctx)

	// ranker.Query(ctx, "How does Generative Artificial Intelligence work?")
	ranker.Query(ctx, "President donald j trump ")
}
