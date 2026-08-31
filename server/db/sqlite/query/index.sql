-- name: DeleteAllDocuments :exec
DELETE FROM documents;

-- name: DeleteAllIndexMeta :exec
DELETE FROM index_metadata;

-- name: AddIndexMeta :exec
INSERT INTO
  index_metadata (
    document_count,
    total_token_count,
    average_doc_length,
    documents_read,
    documents_indexed,
    body_tokens,
    title_tokens,
    unique_terms,
    total_postings,
    title_postings,
    time_elapsed
  )
VALUES
  (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?);

-- name: AddDocumentMeta :exec
INSERT INTO
  documents (token_count, page_id)
VALUES
  (?, ?);

-- name: GetLatestIndexMeta :one
SELECT
  document_count,
  total_token_count,
  average_doc_length,
  documents_read,
  documents_indexed,
  body_tokens,
  title_tokens,
  unique_terms,
  total_postings,
  title_postings,
  time_elapsed
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

-- name: GetDocumentMetaByPageID :one
SELECT
  *
FROM
  documents
WHERE
  page_id = ?;

-- name: GetAllDocumentMeta :many
SELECT
  *
FROM
  documents;