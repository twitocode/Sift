-- +goose Up
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
  document_count,
  total_token_count,
  average_doc_length
)
SELECT
  document_count,
  total_token_count,
  average_doc_length
FROM index_metadata_old;

DROP TABLE index_metadata_old;

-- +goose Down
ALTER TABLE index_metadata RENAME TO index_metadata_new;

CREATE TABLE
  index_metadata (
    document_count INTEGER NOT NULL,
    total_token_count INTEGER NOT NULL,
    average_doc_length INTEGER NOT NULL
  );

INSERT INTO index_metadata (
  document_count,
  total_token_count,
  average_doc_length
)
SELECT
  document_count,
  total_token_count,
  average_doc_length
FROM index_metadata_new;

DROP TABLE index_metadata_new;
