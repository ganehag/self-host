-- name: CreateDatasetUpload :one
INSERT INTO dataset_uploads (upload_id, dataset_uuid, created_by, storage_backend, storage_bucket, storage_key, backend_upload_id)
VALUES (
	sqlc.arg(upload_id),
	sqlc.arg(dataset_uuid),
	sqlc.arg(created_by),
	sqlc.arg(storage_backend),
	NULLIF(sqlc.arg(storage_bucket)::text, ''),
	NULLIF(sqlc.arg(storage_key)::text, ''),
	NULLIF(sqlc.arg(backend_upload_id)::text, '')
)
RETURNING *;

-- name: FindDatasetUploadByID :one
SELECT *
FROM dataset_uploads
WHERE upload_id = sqlc.arg(upload_id)
LIMIT 1;

-- name: DeleteDatasetUploadByID :execrows
DELETE FROM dataset_uploads
WHERE upload_id = sqlc.arg(upload_id);

-- name: UpsertDatasetUploadPart :execrows
INSERT INTO dataset_upload_parts (upload_id, part_number, size, checksum_md5, etag)
VALUES (
	sqlc.arg(upload_id),
	sqlc.arg(part_number),
	sqlc.arg(size),
	sqlc.arg(checksum_md5),
	NULLIF(sqlc.arg(etag)::text, '')
)
ON CONFLICT (upload_id, part_number) DO UPDATE
SET
	size = EXCLUDED.size,
	checksum_md5 = EXCLUDED.checksum_md5,
	etag = EXCLUDED.etag,
	created = NOW();

-- name: FindDatasetUploadParts :many
SELECT *
FROM dataset_upload_parts
WHERE upload_id = sqlc.arg(upload_id)
ORDER BY part_number ASC;
