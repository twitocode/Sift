package main

import (
	"context"
	"database/sql"
	"os"

	"github.com/twitocode/sift/internal/common"
	"github.com/twitocode/sift/internal/crawler"
	"github.com/twitocode/sift/internal/crawler/dedup"
	"github.com/twitocode/sift/internal/progress"
	"github.com/twitocode/sift/internal/store"
	"go.uber.org/zap"

	_ "modernc.org/sqlite"
)

func main() {
	log, _ := common.NewLogger(os.Getenv, zap.InfoLevel)

	cfg := common.NewConfig(os.Getenv)

	sqliteDb, err := sql.Open("sqlite", cfg.SQLitePath())

	if err != nil {
		log.Fatal("Sqlite connection error", zap.Error(err))
	}
	log.Info("Connected to Sqlite")

	store := store.NewPageStore(sqliteDb, log)
	store.BeforeCrawl(context.Background())

	engine := crawler.NewEngine(log, store, cfg)
	done := make(chan error, 1)
	go func() {
		engine.Start()
		close(done)
	}()

	_ = progress.Run("crawl", engine.Snapshot, done)
	engine.PrintSummary()

	deduplicator := dedup.NewDeduplicator(store, log)
	deduplicator.Start(context.Background())
}
