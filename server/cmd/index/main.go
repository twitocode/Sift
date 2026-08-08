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
	log := common.NewLogger(os.Getenv, zap.DebugLevel)
	common.NewConfig(os.Getenv)

	sqliteDb, err := sql.Open("sqlite", common.SQLitePath())

	if err != nil {
		log.Fatal("Sqlite connection error", zap.Error(err))
	}
	log.Info("Connected to Sqlite")

	store := crawler.NewPageStore(sqliteDb, log)
	indexer.NewIndexer(log, store).Start(context.Background())
}
