-- name: ExistsDataset :one
SELECT COUNT(*) AS count
FROM datasets
WHERE datasets.uuid = sqlc.arg(uuid);

-- name: CreateDataset :one
WITH ds AS (
	INSERT INTO datasets (uuid, name, format, content, checksum, size, belongs_to, created_by, updated_by, tags, storage_backend, storage_bucket, storage_key)
	VALUES(
		sqlc.arg(uuid)::uuid,
		sqlc.arg(name)::text,
		sqlc.arg(format)::text,
		sqlc.arg(content)::bytea,
		decode(sqlc.arg(checksum)::text, 'hex'),
		sqlc.arg(size)::integer,
		NULLIF(sqlc.arg(belongs_to)::uuid, '00000000-0000-0000-0000-000000000000'::uuid),
		sqlc.arg(created_by)::uuid,
		sqlc.arg(created_by)::uuid,
		sqlc.arg(tags),
		sqlc.arg(storage_backend)::text,
		NULLIF(sqlc.arg(storage_bucket)::text, ''),
		NULLIF(sqlc.arg(storage_key)::text, '')
	)
	RETURNING
		uuid,
		name,
		format,
		encode(checksum, 'hex') AS checksum,
		size,
		belongs_to,
		created,
		updated,
		created_by,
		updated_by,
		tags
), grp AS (
	SELECT groups.uuid
	FROM groups, user_groups
	WHERE user_groups.group_uuid = groups.uuid
	AND user_groups.user_uuid = (SELECT created_by FROM ds)
	AND groups.uuid = (
		SELECT users.uuid
		FROM users
		WHERE users.name = groups.name
	)
	LIMIT 1
), grp_policies AS (
	INSERT INTO group_policies(group_uuid, priority, effect, action, resource)
	VALUES (
		(SELECT uuid FROM grp), 0, 'allow', 'create','datasets/'||(SELECT uuid FROM ds)||'/%'
	), (
		(SELECT uuid FROM grp), 0, 'allow', 'read','datasets/'||(SELECT uuid FROM ds)||'/%'
	), (
		(SELECT uuid FROM grp), 0, 'allow', 'update','datasets/'||(SELECT uuid FROM ds)||'/%'
	), (
		(SELECT uuid FROM grp), 0, 'allow', 'delete','datasets/'||(SELECT uuid FROM ds)||'/%'
	)
)
SELECT *
FROM ds LIMIT 1;

-- name: FindDatasets :many
WITH usr AS (
	SELECT user_tokens.user_uuid AS uuid
	FROM user_tokens
	WHERE user_tokens.token_hash = sha256(sqlc.arg(token))
	LIMIT 1
), permitted_datasets AS (
	SELECT
		uuid,
		name,
		format,
		encode(checksum, 'hex') AS checksum,
		size,
		belongs_to,
		created,
		updated,
		created_by,
		updated_by,
		tags
	FROM datasets
	WHERE EXISTS (
		SELECT 1
		FROM usr
		INNER JOIN user_groups ON user_groups.user_uuid = usr.uuid
		INNER JOIN group_policies ON group_policies.group_uuid = user_groups.group_uuid
		WHERE group_policies.action = 'read'
		AND group_policies.effect = 'allow'
		AND ('datasets/'||datasets.uuid) LIKE group_policies.resource
	)
	AND NOT EXISTS (
		SELECT 1
		FROM usr
		INNER JOIN user_groups ON user_groups.user_uuid = usr.uuid
		INNER JOIN group_policies ON group_policies.group_uuid = user_groups.group_uuid
		WHERE group_policies.action = 'read'
		AND group_policies.effect = 'deny'
		AND ('datasets/'||datasets.uuid) LIKE group_policies.resource
	)
	ORDER BY name
	LIMIT sqlc.arg(arg_limit)::BIGINT
	OFFSET sqlc.arg(arg_offset)::BIGINT
)
SELECT
	*
FROM permitted_datasets;

-- name: FindDatasetsByTags :many
WITH usr AS (
	SELECT user_tokens.user_uuid AS uuid
	FROM user_tokens
	WHERE user_tokens.token_hash = sha256(sqlc.arg(token))
	LIMIT 1
), permitted_datasets AS (
	SELECT
		uuid,
		name,
		format,
		encode(checksum, 'hex') AS checksum,
		size,
		belongs_to,
		created,
		updated,
		created_by,
		updated_by,
		tags
	FROM datasets
	WHERE sqlc.arg(tags) && datasets.tags
	AND EXISTS (
		SELECT 1
		FROM usr
		INNER JOIN user_groups ON user_groups.user_uuid = usr.uuid
		INNER JOIN group_policies ON group_policies.group_uuid = user_groups.group_uuid
		WHERE group_policies.action = 'read'
		AND group_policies.effect = 'allow'
		AND ('datasets/'||datasets.uuid) LIKE group_policies.resource
	)
	AND NOT EXISTS (
		SELECT 1
		FROM usr
		INNER JOIN user_groups ON user_groups.user_uuid = usr.uuid
		INNER JOIN group_policies ON group_policies.group_uuid = user_groups.group_uuid
		WHERE group_policies.action = 'read'
		AND group_policies.effect = 'deny'
		AND ('datasets/'||datasets.uuid) LIKE group_policies.resource
	)
	ORDER BY name
	LIMIT sqlc.arg(arg_limit)::BIGINT
	OFFSET sqlc.arg(arg_offset)::BIGINT
)
SELECT
	*
FROM permitted_datasets;

-- name: FindDatasetByUUID :one
SELECT
	uuid,
	name,
	format,
	encode(checksum, 'hex') AS checksum,
	size,
	belongs_to,
	created,
	updated,
	created_by,
	updated_by,
	tags
FROM datasets
WHERE datasets.uuid = sqlc.arg(uuid)
LIMIT 1;

-- name: FindDatasetByThing :many
SELECT
	uuid,
	name,
	format,
	encode(checksum, 'hex') AS checksum,
	size,
	belongs_to,
	created,
	updated,
	created_by,
	updated_by,
	tags
FROM datasets
WHERE datasets.belongs_to = sqlc.arg(thing_uuid)
ORDER BY name
;

-- name: GetDatasetContentByUUID :one
SELECT format, content, encode(checksum, 'hex') AS checksum, size, storage_backend, storage_bucket, storage_key
FROM datasets
WHERE datasets.uuid = sqlc.arg(uuid)
LIMIT 1;

-- name: GetDatasetObjectRefByUUID :one
SELECT format, encode(checksum, 'hex') AS checksum, size, storage_backend, storage_bucket, storage_key
FROM datasets
WHERE datasets.uuid = sqlc.arg(uuid)
LIMIT 1;

-- name: UpdateDatasetByUUID :execrows
UPDATE datasets
SET
	name = CASE
		WHEN sqlc.arg(set_name)::boolean THEN sqlc.arg(name)
		ELSE name
	END,
	format = CASE
		WHEN sqlc.arg(set_format)::boolean THEN sqlc.arg(format)
		ELSE format
	END,
	content = CASE
		WHEN sqlc.arg(set_content)::boolean THEN sqlc.arg(content)::bytea
		ELSE content
	END,
	checksum = CASE
		WHEN sqlc.arg(set_content)::boolean THEN decode(sqlc.arg(checksum)::text, 'hex')
		ELSE checksum
	END,
	size = CASE
		WHEN sqlc.arg(set_content)::boolean THEN sqlc.arg(size)::integer
		ELSE size
	END,
	storage_backend = CASE
		WHEN sqlc.arg(set_content)::boolean THEN sqlc.arg(storage_backend)::text
		ELSE storage_backend
	END,
	storage_bucket = CASE
		WHEN sqlc.arg(set_content)::boolean THEN NULLIF(sqlc.arg(storage_bucket)::text, '')
		ELSE storage_bucket
	END,
	storage_key = CASE
		WHEN sqlc.arg(set_content)::boolean THEN NULLIF(sqlc.arg(storage_key)::text, '')
		ELSE storage_key
	END,
	updated = CASE
		WHEN sqlc.arg(set_name)::boolean
			OR sqlc.arg(set_format)::boolean
			OR sqlc.arg(set_content)::boolean
			OR sqlc.arg(set_thing_uuid)::boolean
			OR sqlc.arg(set_tags)::boolean
		THEN NOW()
		ELSE updated
	END,
	belongs_to = CASE
		WHEN sqlc.arg(set_thing_uuid)::boolean THEN sqlc.arg(thing_uuid)
		ELSE belongs_to
	END,
	tags = CASE
		WHEN sqlc.arg(set_tags)::boolean THEN sqlc.arg(tags)
		ELSE tags
	END
WHERE datasets.uuid = sqlc.arg(uuid);

-- name: DeleteDataset :execrows
DELETE FROM datasets
WHERE datasets.uuid = sqlc.arg(uuid);
