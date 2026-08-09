package main

import (
	"context"
	"database/sql"
	"os"

	"github.com/twitocode/sift/internal/common"
	"github.com/twitocode/sift/internal/crawler"
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

	store := crawler.NewPageStore(sqliteDb, log)
	store.BeforeCrawl(context.Background())

	crawler.NewEngine(log, store, cfg).Start()

	deduplicator := crawler.NewDeduplicator(store, log)
	deduplicator.Start(context.Background())
}
