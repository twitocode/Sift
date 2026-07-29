package main

import (
	"context"
	"database/sql"
	"os"

	"github.com/twitocode/sift/internal/app"
	"github.com/twitocode/sift/internal/crawler"
	"go.uber.org/zap"

	_ "modernc.org/sqlite"
)

func main() {
	log := app.NewLogger(os.Getenv, zap.DebugLevel)
	app.NewConfig(os.Getenv)

	sqliteDb, err := sql.Open("sqlite", "../../db/sqlite/sift.db")

	if err != nil {
		log.Fatal("Sqlite connection error", zap.Error(err))
	}
	log.Info("Connected to Sqlite")

	store := crawler.NewPageStore(sqliteDb, log)
	crawler.NewEngine(log, store)

  deduplicator := crawler.NewDeduplicator(store, log)
  deduplicator.Start(context.Background())
}
