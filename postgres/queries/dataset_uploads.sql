-- name: CreateDatasetUpload :one
INSERT INTO dataset_uploads (upload_id, dataset_uuid, created_by)
VALUES (
	sqlc.arg(upload_id),
	sqlc.arg(dataset_uuid),
	sqlc.arg(created_by)
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
INSERT INTO dataset_upload_parts (upload_id, part_number, size, checksum_md5)
VALUES (
	sqlc.arg(upload_id),
	sqlc.arg(part_number),
	sqlc.arg(size),
	sqlc.arg(checksum_md5)
)
ON CONFLICT (upload_id, part_number) DO UPDATE
SET
	size = EXCLUDED.size,
	checksum_md5 = EXCLUDED.checksum_md5,
	created = NOW();

-- name: FindDatasetUploadParts :many
SELECT *
FROM dataset_upload_parts
WHERE upload_id = sqlc.arg(upload_id)
ORDER BY part_number ASC;
