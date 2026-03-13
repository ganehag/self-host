BEGIN;

ALTER TABLE datasets
ADD COLUMN storage_backend TEXT NOT NULL DEFAULT 'inline',
ADD COLUMN storage_bucket TEXT,
ADD COLUMN storage_key TEXT;

ALTER TABLE datasets
ADD CONSTRAINT datasets_storage_backend_check
CHECK (storage_backend IN ('inline', 's3'));

ALTER TABLE datasets
ADD CONSTRAINT datasets_storage_ref_check
CHECK (
	(storage_backend = 'inline' AND storage_bucket IS NULL AND storage_key IS NULL)
	OR
	(storage_backend = 's3' AND storage_bucket IS NOT NULL AND storage_key IS NOT NULL)
);

CREATE INDEX datasets_storage_backend_idx ON datasets(storage_backend);

ALTER TABLE dataset_uploads
ADD COLUMN storage_backend TEXT NOT NULL DEFAULT 'inline',
ADD COLUMN storage_bucket TEXT,
ADD COLUMN storage_key TEXT,
ADD COLUMN backend_upload_id TEXT;

ALTER TABLE dataset_uploads
ADD CONSTRAINT dataset_uploads_storage_backend_check
CHECK (storage_backend IN ('inline', 's3'));

ALTER TABLE dataset_upload_parts
ADD COLUMN etag TEXT;

COMMIT;
