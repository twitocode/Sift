package store

import (
	"context"
	"database/sql"
	"testing"

	"github.com/twitocode/sift/internal/common"
	"go.uber.org/zap"
	_ "modernc.org/sqlite"
)

func TestGetPaginatedPageBatchIncludesFinalURL(t *testing.T) {
	sqliteDB, err := sql.Open("sqlite", ":memory:")
	if err != nil {
		t.Fatalf("open sqlite: %v", err)
	}
	t.Cleanup(func() {
		_ = sqliteDB.Close()
	})

	_, err = sqliteDB.Exec(`
		CREATE TABLE pages (
			id INTEGER PRIMARY KEY,
			title TEXT,
			text TEXT,
			final_url TEXT NOT NULL,
			has_been_crawled INTEGER NOT NULL
		);
		INSERT INTO pages (id, title, text, final_url, has_been_crawled)
		VALUES (42, 'GitHub', 'Source hosting', 'https://github.com/openai/codex', 1);
	`)
	if err != nil {
		t.Fatalf("prepare pages: %v", err)
	}

	pageStore := NewPageStore(sqliteDB, zap.NewNop())
	pages, err := pageStore.GetPaginatedPageBatch(context.Background(), 0, 10)
	if err != nil {
		t.Fatalf("GetPaginatedPageBatch() error = %v", err)
	}
	if len(pages) != 1 {
		t.Fatalf("GetPaginatedPageBatch() returned %d pages, want 1", len(pages))
	}
	if pages[0].FinalURL != common.URL("https://github.com/openai/codex") {
		t.Fatalf("FinalURL = %q, want page final URL", pages[0].FinalURL)
	}
}
