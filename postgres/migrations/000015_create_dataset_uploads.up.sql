BEGIN;

CREATE TABLE dataset_uploads (
	upload_id TEXT PRIMARY KEY,
	dataset_uuid UUID NOT NULL REFERENCES datasets(uuid) ON DELETE CASCADE,
	created_by UUID REFERENCES users(uuid) ON DELETE SET NULL,
	created TIMESTAMPTZ DEFAULT NOW() NOT NULL
);

CREATE INDEX dataset_uploads_dataset_uuid_idx ON dataset_uploads(dataset_uuid);

CREATE TABLE dataset_upload_parts (
	upload_id TEXT NOT NULL REFERENCES dataset_uploads(upload_id) ON DELETE CASCADE,
	part_number INTEGER NOT NULL CHECK (part_number > 0),
	size INTEGER NOT NULL CHECK (size >= 0),
	checksum_md5 TEXT NOT NULL,
	created TIMESTAMPTZ DEFAULT NOW() NOT NULL,
	PRIMARY KEY (upload_id, part_number)
);

CREATE INDEX dataset_upload_parts_upload_id_idx ON dataset_upload_parts(upload_id);

COMMIT;
