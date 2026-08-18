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
	log, _ := common.NewLogger(os.Getenv, zap.InfoLevel)
	cfg := common.NewConfig(os.Getenv)

	sqliteDb, err := sql.Open("sqlite", cfg.SQLitePath())

	if err != nil {
		log.Fatal("Sqlite connection error", zap.Error(err))
	}
	log.Info("Connected to Sqlite")

	pageStore := store.NewPageStore(sqliteDb, log)
	indexerStore := store.NewIndexerStore(sqliteDb, log)

	in := indexer.NewIndexer(log, cfg, pageStore, indexerStore)
	type indexResult struct {
		terms map[string]indexer.TermData
	}
	result := make(chan indexResult, 1)
	done := make(chan error, 1)
	go func() {
		terms, err := in.Get()
		result <- indexResult{terms: terms}
		done <- err
		close(done)
	}()

	if err := progress.Run("index", in.Snapshot, done); err != nil {
		log.Error("progress ui", zap.Error(err))
	}
	//in.PrintSummary()

	terms := (<-result).terms
	ctx := context.Background()
	ranker := ranker.NewRanker(log, cfg, terms, indexerStore, pageStore)
	ranker.LoadDocuments(ctx)
	ranker.LoadIndexMeta(ctx)

	queries := []string{
		"How does Generative Artificial Intelligence work?",
		"President donald j trump ",
		"Mental health resources for students",
		"gItHuB access tokens",
		"google account",
		"how to deal with stomach pain",
		"acid reflux",
		"python programming tutorial",
	}

	for _, query := range queries {
		ranker.Query(ctx, query)
	}
}
