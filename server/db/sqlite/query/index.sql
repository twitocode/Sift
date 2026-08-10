-- name: AddIndexMeta :exec
INSERT INTO
  index_metadata (
    document_count,
    total_token_count,
    average_doc_length
  )
VALUES
  (?, ?, ?);

-- name: AddDocumentMeta :exec
INSERT INTO
  documents (token_count, page_id)
VALUES
  (?, ?);

-- name: GetLatestIndexMeta :one
SELECT
  *
FROM
  index_metadata
ORDER BY
  id DESC
LIMIT
  1;

-- name: GetDocumentMeta :one
SELECT
  *
FROM
  documents
WHERE
  id = ?;

-- name: GetAllDocumentMeta :many
SELECT
  *
FROM
  documents;