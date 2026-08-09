-- +goose Up
ALTER TABLE pages
ADD COLUMN duplicate_of INTEGER REFERENCES pages(id);
ALTER TABLE pages 
ADD COLUMN found_canonical TEXT DEFAULT(NULL);

-- +goose Down
ALTER TABLE pages DROP COLUMN duplicate_of;
ALTER TABLE pages DROP COLUMN found_canonical;
