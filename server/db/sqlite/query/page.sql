-- name: DeleteAll :exec
DELETE FROM pages;

-- name: SetPageInfo :exec
INSERT
OR REPLACE INTO pages (
  url,
  title,
  text,
  description,
  status_code,
  crawled_at,
  has_been_crawled,
  content_hash,
  found_canonical
)
VALUES
  (?, ?, ?, ?, ?, ?, ?, ?, ?);

-- name: FindPageByURL :one
SELECT
  EXISTS (
    SELECT
      1
    FROM
      pages
    WHERE
      url = ?
  );

-- name: FindPageByID :one
SELECT
  EXISTS (
    SELECT
      1
    FROM
      pages
    WHERE
      id = ?
  );

-- name: GetPageInfoByURL :one
SELECT
  *
FROM
  pages
WHERE
  url = ?;

-- name: GetPageInfoByID :one
SELECT
  *
FROM
  pages
WHERE
  id = ?;

-- name: GetAllPages :many
SELECT
  *
FROM
  pages
WHERE
  has_been_crawled = TRUE;

-- name: AssignCanonical :exec
UPDATE pages
SET
  duplicate_of = ?
WHERE
  id = ?
LIMIT
  1;

-- name: BatchAssignCanonical :exec
UPDATE pages
SET
  duplicate_of = ?
WHERE
  id IN (sqlc.slice ('ids'));