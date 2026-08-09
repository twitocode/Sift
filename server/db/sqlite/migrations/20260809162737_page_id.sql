-- +goose Up
PRAGMA foreign_keys = OFF;

CREATE TABLE
  pages_new (
    id INTEGER PRIMARY KEY,
    url TEXT NOT NULL UNIQUE,
    title TEXT,
    description TEXT,
    text TEXT,
    status_code INTEGER,
    crawled_at DATETIME,
    content_hash INTEGER,
    has_been_crawled INTEGER DEFAULT (TRUE)
  );

INSERT INTO
  pages_new (
    url,
    title,
    description,
    text,
    status_code,
    crawled_at,
    content_hash,
    has_been_crawled
  )
SELECT
  url,
  title,
  description,
  text,
  status_code,
  crawled_at,
  CASE
    WHEN content_hash IS NULL
    OR TRIM(content_hash) = '' THEN NULL
    ELSE CAST(content_hash AS INTEGER)
  END,
  has_been_crawled
FROM
  pages;

DROP TABLE pages;

ALTER TABLE pages_new
RENAME TO pages;


PRAGMA foreign_keys = ON;

-- +goose Down
PRAGMA foreign_keys = OFF;

CREATE TABLE
  pages_old (
    url TEXT PRIMARY KEY,
    title TEXT,
    description TEXT,
    text TEXT,
    status_code INTEGER,
    crawled_at DATETIME,
    content_hash INTEGER,
    has_been_crawled INTEGER DEFAULT (TRUE)
  );

INSERT INTO
  pages_old (
    url,
    title,
    description,
    text,
    status_code,
    crawled_at,
    content_hash,
    has_been_crawled
  )
SELECT
  url,
  title,
  description,
  text,
  status_code,
  crawled_at,
  content_hash,
  has_been_crawled
FROM
  pages;

DROP TABLE pages;

ALTER TABLE pages_old
RENAME TO pages;

PRAGMA foreign_keys = ON;