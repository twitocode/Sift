package main

import (
	"context"
	"database/sql"
	"os"

	"github.com/twitocode/sift/internal/common"
	"github.com/twitocode/sift/internal/crawler"
	"github.com/twitocode/sift/internal/crawler/dedup"
	"github.com/twitocode/sift/internal/indexer"
	"github.com/twitocode/sift/internal/metrics"
	"github.com/twitocode/sift/internal/progress"
	"github.com/twitocode/sift/internal/ranker"
	"github.com/twitocode/sift/internal/store"
	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"

	_ "modernc.org/sqlite"
)

func main() {
	log, logLevel := common.NewLogger(os.Getenv, zap.InfoLevel)
	cfg := common.NewConfig(os.Getenv)

	sqliteDb, err := sql.Open("sqlite", cfg.SQLitePath())

	if err != nil {
		log.Fatal("Sqlite connection error", zap.Error(err))
		return
	}
	log.Info("Connected to Sqlite")

	logLevel.SetLevel(zapcore.Level(6))
	pageStore := store.NewPageStore(sqliteDb, log)
	indexerStore := store.NewIndexerStore(sqliteDb, log)

	dbCtx := context.Background()
	pageStore.BeforeCrawl(dbCtx)
	indexerStore.BeforeIndexing(dbCtx)

	engine := crawler.NewEngine(log, pageStore, cfg)
	in := indexer.NewIndexer(log, cfg, pageStore, indexerStore)
	done := make(chan error, 1)
	type indexResult struct {
		terms   map[string]indexer.TermData
		metrics *metrics.IndexerMetrics
	}
	indexed := make(chan indexResult, 1)
	go func() {
		engine.Start()

		deduplicator := dedup.NewDeduplicator(pageStore, log)
		deduplicator.Start(context.Background())

		terms, indexMetrics, err := in.Get()
		indexed <- indexResult{terms: terms, metrics: indexMetrics}
		done <- err
		close(done)
	}()

	_ = progress.Run("crawl + index", func() progress.Snapshot {
		crawl := engine.Snapshot()
		index := in.Snapshot()
		crawl.Index = index.Index
		return crawl
	}, done)

	engine.PrintSummary()
	in.PrintSummary()

	indexedResult := <-indexed
	ranker := ranker.NewRanker(log, cfg, indexedResult.terms, indexerStore, pageStore, indexedResult.metrics)
	ranker.LoadDocuments(context.Background())
	ranker.LoadIndexMeta(context.Background())

	queries := []string{
		"How does Generative Artificial Intelligence work?",
		"President donald j trump ",
		"Mental health resources for students",
		"gItHuB access tokens",
	}

	for _, query := range queries {
		ranker.Query(context.Background(), query)
	}
}
