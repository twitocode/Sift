-- name: SetPageInfo :exec
INSERT
OR REPLACE INTO pages (
  url,
  title,
  text,
  description,
  status_code,
  crawled_at,
  content_hash
)
VALUES
  (?, ?, ?, ?, ?, ?, ?);

-- name: FindPage :one
SELECT
  EXISTS (
    SELECT
      1
    FROM
      pages
    WHERE
      url = ?
  );

-- name: GetPageInfo :one
SELECT
  *
FROM
  pages
WHERE
  url = ?;