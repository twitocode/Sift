-- +goose Up
PRAGMA foreign_keys = OFF;

ALTER TABLE pages
RENAME COLUMN url TO request_url;

CREATE TABLE
  pages_new (
    id INTEGER PRIMARY KEY,
    request_url TEXT NOT NULL UNIQUE,
    title TEXT,
    description TEXT,
    text TEXT,
    status_code INTEGER,
    crawled_at DATETIME,
    content_hash INTEGER,
    has_been_crawled INTEGER DEFAULT (TRUE),
    duplicate_of INTEGER REFERENCES pages(id),
    found_canonical TEXT DEFAULT(NULL),
    final_url TEXT NOT NULL,
    resolved_canonical INTEGER DEFAULT(FALSE)
  );

INSERT INTO
  pages_new (
    id,
    request_url,
    title,
    description,
    text,
    status_code,
    crawled_at,
    content_hash,
    has_been_crawled,
    duplicate_of,
    found_canonical,
    final_url,
    resolved_canonical
  )
SELECT
  id,
  request_url,
  title,
  description,
  text,
  status_code,
  crawled_at,
  content_hash,
  has_been_crawled,
  duplicate_of,
  found_canonical,
  request_url,
  FALSE
FROM
  pages;

DROP TABLE pages;

ALTER TABLE pages_new
RENAME TO pages;

PRAGMA foreign_keys = ON;

-- +goose Down
ALTER TABLE pages
RENAME COLUMN request_url TO url;

ALTER TABLE pages
DROP COLUMN final_url;

ALTER TABLE pages
DROP COLUMN resolved_canonical;