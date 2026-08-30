package main

import (
	"context"
	"database/sql"
	"fmt"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/twitocode/sift/internal/app"
	"github.com/twitocode/sift/internal/common"
	"github.com/twitocode/sift/internal/indexer"
	"github.com/twitocode/sift/internal/progress"
	"github.com/twitocode/sift/internal/ranker"
	"github.com/twitocode/sift/internal/store"
	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"

	pgxvec "github.com/pgvector/pgvector-go/pgx"
	_ "modernc.org/sqlite"
)

func run(ctx context.Context, getenv func(string) string) error {
	ctx, cancel := signal.NotifyContext(ctx, os.Interrupt, syscall.SIGTERM)
	defer cancel()

	cfg := common.NewConfig(getenv)
	logger, _ := common.NewLogger(getenv, zapcore.DebugLevel)

	defer func() { _ = logger.Sync() }()

	pool, err := setupPostgres(ctx, cfg)
	if err != nil {
		return err
	}

	defer pool.Close()

	sqliteDb, err := sql.Open("sqlite", cfg.SQLitePath()+"?_pragma=busy_timeout(5000)")
	if err != nil {
		log.Fatal("Sqlite connection error", zap.Error(err))
	}

	defer sqliteDb.Close()

	pageStore := store.NewPageStore(sqliteDb, logger)
	indexerStore := store.NewIndexerStore(sqliteDb, logger)
	terms, indexStats, err := getIndexTerms(logger, cfg, pageStore, indexerStore)
	if err != nil {
		return err
	}

	ranker := ranker.NewRanker(logger, cfg, terms, indexStats, indexerStore, pageStore)

	ranker.LoadDocuments(context.Background())

	services := app.NewServices(cfg, pool, logger, ranker)
	handler := app.NewServer(cfg, services, logger)

	srv := &http.Server{
		Addr:         ":" + cfg.Port,
		Handler:      handler,
		ReadTimeout:  15 * time.Second,
		WriteTimeout: 15 * time.Second,
		IdleTimeout:  30 * time.Second,
	}

	errCh := make(chan error, 1)
	go func() {
		logger.Info(fmt.Sprintf("starting server on :%s", cfg.Port))
		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			errCh <- err
		}
	}()

	select {
	case <-ctx.Done():
		logger.Info("shutting down server")
	case err := <-errCh:
		return err
	}

	shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer shutdownCancel()
	return srv.Shutdown(shutdownCtx)
}

func getIndexTerms(logger *zap.Logger, cfg *common.Config, pageStore *store.PageStore, indexerStore *store.IndexerStore) (map[string]indexer.TermData, *common.IndexStats, error) {

	in := indexer.NewIndexer(logger, cfg, pageStore, indexerStore)
	done := make(chan error, 1)
	go func() {
		_, _, err := in.Get(context.Background())
		done <- err
		close(done)
	}()

	if err := progress.Run("index", in.Snapshot, done); err != nil {
		logger.Error("progress ui", zap.Error(err))
	}

	terms, stats, err := in.Get(context.Background())
	if err != nil {
		return nil, nil, fmt.Errorf("index: %w", err)
	}
	return terms, stats, nil
}

func setupPostgres(ctx context.Context, cfg *common.Config) (*pgxpool.Pool, error) {
	poolConfig, err := pgxpool.ParseConfig(cfg.DatabaseURL)
	if err != nil {
		return nil, fmt.Errorf("db pool config: %w", err)
	}

	poolConfig.AfterConnect = func(ctx context.Context, conn *pgx.Conn) error {
		if _, err := conn.Exec(ctx, "CREATE EXTENSION IF NOT EXISTS vector"); err != nil {
			return err
		}
		return pgxvec.RegisterTypes(ctx, conn)
	}
	pool, err := pgxpool.NewWithConfig(ctx, poolConfig)
	if err != nil {
		return nil, fmt.Errorf("db pool: %w", err)
	}

	if err := pool.Ping(ctx); err != nil {
		return nil, fmt.Errorf("db ping: %w", err)
	}
	return pool, nil
}

func main() {
	if err := run(context.Background(), os.Getenv); err != nil {
		log.Fatal(err)
	}
}
