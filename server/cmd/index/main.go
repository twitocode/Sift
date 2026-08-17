package main

import (
	"database/sql"
	"os"

	"github.com/twitocode/sift/internal/common"
	"github.com/twitocode/sift/internal/indexer"
	"github.com/twitocode/sift/internal/progress"
	"github.com/twitocode/sift/internal/store"
	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"

	_ "modernc.org/sqlite"
)

func main() {
	log, logLevel := common.NewLogger(os.Getenv, zap.DebugLevel)
	logLevel.SetLevel(zapcore.Level(6))

	cfg := common.NewConfig(os.Getenv)

	sqliteDb, err := sql.Open("sqlite", cfg.SQLitePath())

	if err != nil {
		log.Fatal("Sqlite connection error", zap.Error(err))
	}
	log.Info("Connected to Sqlite")

	pageStore := store.NewPageStore(sqliteDb, log)
	indexStore := store.NewIndexerStore(sqliteDb, log)
	in := indexer.NewIndexer(log, cfg, pageStore, indexStore)
	done := make(chan error, 1)
	go func() {
		_, err := in.Get()
		done <- err
		close(done)
	}()

	if err := progress.Run("index", in.Snapshot, done); err != nil {
		log.Error("progress ui", zap.Error(err))
	}
}
