-- +goose Up
ALTER TABLE pages ADD has_been_crawled INTEGER DEFAULT(TRUE);

-- +goose Down
ALTER TABLE pages DROP has_been_crawled;