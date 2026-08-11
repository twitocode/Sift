package main

import (
	"context"
	"database/sql"
	"os"

	"github.com/twitocode/sift/internal/common"
	"github.com/twitocode/sift/internal/crawler"
	"github.com/twitocode/sift/internal/indexer"
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

	pageStore := crawler.NewPageStore(sqliteDb, log)
	indexerStore := indexer.NewIndexerStore(sqliteDb, log)

  dbCtx := context.Background()
	pageStore.BeforeCrawl(dbCtx)
  indexerStore.BeforeIndexing(dbCtx)
	crawler.NewEngine(log, pageStore, cfg).Start()

	deduplicator := crawler.NewDeduplicator(pageStore, log)
	deduplicator.Start(context.Background())

	indexer.NewIndexer(log, pageStore, indexerStore).Start()
}
