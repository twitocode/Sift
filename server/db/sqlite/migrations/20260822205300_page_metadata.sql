-- +goose Up
ALTER TABLE pages ADD COLUMN favicon TEXT;
ALTER TABLE pages ADD COLUMN og_title TEXT;

-- +goose Down
ALTER TABLE pages DROP COLUMN og_title;
ALTER TABLE pages DROP COLUMN favicon;
