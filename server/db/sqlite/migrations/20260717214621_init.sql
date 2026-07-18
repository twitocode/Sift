-- +goose Up
CREATE TABLE
  pages (
    url TEXT PRIMARY KEY,
    title TEXT,
    description TEXT,
    text TEXT,
    status_code INTEGER,
    crawled_at DATETIME,
    content_hash TEXT
  );

-- many to many
CREATE TABLE
  links (
    from_url TEXT,
    to_url TEXT,
    FOREIGN KEY (from_url) REFERENCES pages (url)
  );

-- +goose Down
DROP TABLE links;

DROP TABLE pages;