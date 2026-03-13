-- name: ExistsDataset :one
SELECT COUNT(*) AS count
FROM datasets
WHERE datasets.uuid = sqlc.arg(uuid);

-- name: CreateDataset :one
WITH ds AS (
	INSERT INTO datasets (name, format, content, checksum, size, belongs_to, created_by, updated_by, tags)
	VALUES(
		sqlc.arg(name)::text,
		sqlc.arg(format)::text,
		sqlc.arg(content)::bytea,
		sha256(sqlc.arg(content)::bytea),
		length(sqlc.arg(content))::integer,
		NULLIF(sqlc.arg(belongs_to)::uuid, '00000000-0000-0000-0000-000000000000'::uuid),
		sqlc.arg(created_by)::uuid,
		sqlc.arg(created_by)::uuid,
		sqlc.arg(tags)
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
SELECT format, content, encode(checksum, 'hex') AS checksum
FROM datasets
WHERE datasets.uuid = sqlc.arg(uuid)
LIMIT 1;

-- name: SetDatasetNameByUUID :execrows
UPDATE datasets
SET name = sqlc.arg(name)
WHERE datasets.uuid = sqlc.arg(uuid);

-- name: SetDatasetFormatByUUID :execrows
UPDATE datasets
SET format = sqlc.arg(format)
WHERE datasets.uuid = sqlc.arg(uuid);

-- name: SetDatasetContentByUUID :execrows
UPDATE datasets
SET content = sqlc.arg(content)::bytea,
    checksum = sha256(sqlc.arg(content)::bytea)
WHERE datasets.uuid = sqlc.arg(uuid);

-- name: SetDatasetThingByUUID :execrows
UPDATE datasets
SET belongs_to = sqlc.arg(thing_uuid)
WHERE datasets.uuid = sqlc.arg(uuid);

-- name: SetDatasetTags :execrows
UPDATE datasets
SET tags = sqlc.arg(tags)
WHERE datasets.uuid = sqlc.arg(uuid);

-- name: DeleteDataset :execrows
DELETE FROM datasets
WHERE datasets.uuid = sqlc.arg(uuid);
