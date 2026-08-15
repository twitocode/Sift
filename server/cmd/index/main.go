package main

import (
	"database/sql"
	"os"

	"github.com/twitocode/sift/internal/common"
	"github.com/twitocode/sift/internal/indexer"
	"github.com/twitocode/sift/internal/progress"
	"github.com/twitocode/sift/internal/store"
	"go.uber.org/zap"

	_ "modernc.org/sqlite"
)

func main() {
	log := common.NewLogger(os.Getenv, zap.DebugLevel)
	cfg := common.NewConfig(os.Getenv)

	sqliteDb, err := sql.Open("sqlite", common.SQLitePath())

	if err != nil {
		log.Fatal("Sqlite connection error", zap.Error(err))
	}
	log.Info("Connected to Sqlite")

	pageStore := store.NewPageStore(sqliteDb, log)
	indexStore := store.NewIndexerStore(sqliteDb, log)
	in := indexer.NewIndexer(log, cfg, pageStore, indexStore)
	done := make(chan error, 1)
	go func() {
		in.Generate()
		close(done)
	}()

	if err := progress.Run("index", in.Snapshot, done); err != nil {
		log.Error("progress ui", zap.Error(err))
	}
}
