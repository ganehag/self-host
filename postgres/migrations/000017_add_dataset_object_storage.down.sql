BEGIN;

ALTER TABLE dataset_upload_parts
DROP COLUMN etag;

ALTER TABLE dataset_uploads
DROP CONSTRAINT dataset_uploads_storage_backend_check,
DROP COLUMN backend_upload_id,
DROP COLUMN storage_key,
DROP COLUMN storage_bucket,
DROP COLUMN storage_backend;

DROP INDEX IF EXISTS datasets_storage_backend_idx;

ALTER TABLE datasets
DROP CONSTRAINT datasets_storage_ref_check,
DROP CONSTRAINT datasets_storage_backend_check,
DROP COLUMN storage_key,
DROP COLUMN storage_bucket,
DROP COLUMN storage_backend;

COMMIT;
