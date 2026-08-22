-- name: DeleteAllPages :exec
DELETE FROM pages;

-- name: SetPageInfo :exec
INSERT INTO
  pages (
    final_url,
    request_url,
    title,
    og_title,
    favicon,
    text,
    description,
    status_code,
    crawled_at,
    has_been_crawled,
    content_hash,
    found_canonical
  )
VALUES
  (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?);

-- name: FindPageByURL :one
SELECT
  EXISTS (
    SELECT
      1
    FROM
      pages
    WHERE
      final_url = ?
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
  final_url = ?;

-- name: GetPageInfoByID :one
SELECT
  id,
  request_url,
  title,
  description,
  status_code,
  crawled_at,
  content_hash,
  has_been_crawled,
  duplicate_of,
  found_canonical,
  final_url,
  resolved_canonical,
  favicon,
  og_title
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

-- name: GetPaginatedPageBatch :many
SELECT
  id,
  title,
  text
FROM
  pages
WHERE
  has_been_crawled = TRUE
  AND id > ?
LIMIT
  ?;

-- name: GetTotalCrawledPageCount :one
SELECT
  COUNT(id)
FROM
  pages
WHERE
  has_been_crawled = TRUE;

-- name: FindCanonicDuplicatesPages :many
SELECT
  *
FROM
  pages
WHERE
  has_been_crawled = TRUE
  AND resolved_canonical = FALSE
  AND found_canonical IS NOT NULL
  AND text IS NOT NULL
  AND found_canonical <> final_url
  AND content_hash IS NOT NULL
  AND content_hash <> 0
  AND length (trim(text)) > 0
  AND duplicate_of IS NULL;

-- name: FindPossibleDuplicatePages :many
SELECT
  *
FROM
  pages
WHERE
  has_been_crawled = TRUE
  AND resolved_canonical = FALSE
  AND found_canonical IS NULL
  AND text IS NOT NULL
  AND content_hash IS NOT NULL
  AND content_hash <> 0
  AND length (trim(text)) > 0
  AND duplicate_of IS NULL
ORDER BY
  content_hash ASC;

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
  duplicate_of = ?,
  resolved_canonical = TRUE
WHERE
  id IN (sqlc.slice ('ids'));