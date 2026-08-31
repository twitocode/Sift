-- +goose Up
ALTER TABLE index_metadata ADD COLUMN documents_read INTEGER NOT NULL DEFAULT 0;
ALTER TABLE index_metadata ADD COLUMN documents_indexed INTEGER NOT NULL DEFAULT 0;
ALTER TABLE index_metadata ADD COLUMN body_tokens INTEGER NOT NULL DEFAULT 0;
ALTER TABLE index_metadata ADD COLUMN title_tokens INTEGER NOT NULL DEFAULT 0;
ALTER TABLE index_metadata ADD COLUMN unique_terms INTEGER NOT NULL DEFAULT 0;
ALTER TABLE index_metadata ADD COLUMN total_postings INTEGER NOT NULL DEFAULT 0;
ALTER TABLE index_metadata ADD COLUMN title_postings INTEGER NOT NULL DEFAULT 0;
ALTER TABLE index_metadata ADD COLUMN time_elapsed INTEGER NOT NULL DEFAULT 0;

-- +goose Down
ALTER TABLE index_metadata RENAME TO index_metadata_old;

CREATE TABLE
  index_metadata (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    document_count INTEGER NOT NULL,
    total_token_count INTEGER NOT NULL,
    average_doc_length INTEGER NOT NULL,
    created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP
  );

INSERT INTO index_metadata (
  id,
  document_count,
  total_token_count,
  average_doc_length,
  created_at
)
SELECT
  id,
  document_count,
  total_token_count,
  average_doc_length,
  created_at
FROM index_metadata_old;

DROP TABLE index_metadata_old;
