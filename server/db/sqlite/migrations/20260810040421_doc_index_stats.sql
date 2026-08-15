-- +goose Up
CREATE TABLE
  documents (
    id INTEGER PRIMARY KEY,
    token_count INTEGER NOT NULL DEFAULT(0),
    page_id INTEGER NOT NULL,
    FOREIGN KEY (page_id) REFERENCES pages (id)
  );

CREATE TABLE
  index_metadata (
    document_count INTEGER NOT NULL,
    total_token_count INTEGER NOT NULL,
    average_doc_length INTEGER NOT NULL
  );

-- +goose Down
DROP TABLE documents;
DROP TABLE index_metadata;