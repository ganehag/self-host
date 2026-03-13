-- name: ExistsThing :one
SELECT COUNT(*) AS count
FROM things
WHERE things.uuid = sqlc.arg(uuid);

-- name: CreateThing :one
WITH t AS (
	INSERT INTO things (
		name, type, created_by, tags
	) VALUES (
		sqlc.arg(name),
		sqlc.arg(type),
		sqlc.arg(created_by),
		sqlc.arg(tags)
	)
	RETURNING *
), grp AS (
	SELECT groups.uuid
	FROM groups, user_groups
	WHERE user_groups.group_uuid = groups.uuid
	AND user_groups.user_uuid = (SELECT created_by FROM t)
	AND groups.uuid = (
		SELECT users.uuid
		FROM users
		WHERE users.name = groups.name
	)
	LIMIT 1
), grp_policies AS (
	INSERT INTO group_policies(group_uuid, priority, effect, action, resource)
	VALUES (
		(SELECT uuid FROM grp), 0, 'allow', 'create','things/'||(SELECT uuid FROM t)||'/%'
	), (
		(SELECT uuid FROM grp), 0, 'allow', 'read','things/'||(SELECT uuid FROM t)||'/%'
	), (
		(SELECT uuid FROM grp), 0, 'allow', 'update','things/'||(SELECT uuid FROM t)||'/%'
	), (
		(SELECT uuid FROM grp), 0, 'allow', 'delete','things/'||(SELECT uuid FROM t)||'/%'
	)
)
SELECT *
FROM t LIMIT 1;

-- name: FindThingByUUID :one
SELECT *
FROM things
WHERE things.uuid = sqlc.arg(uuid)
LIMIT 1;

-- name: FindThings :many
WITH usr AS (
	SELECT user_tokens.user_uuid AS uuid
	FROM user_tokens
	WHERE user_tokens.token_hash = sha256(sqlc.arg(token))
	LIMIT 1
), permitted_things AS (
	SELECT *
	FROM things
	WHERE EXISTS (
		SELECT 1
		FROM usr
		INNER JOIN user_groups ON user_groups.user_uuid = usr.uuid
		INNER JOIN group_policies ON group_policies.group_uuid = user_groups.group_uuid
		WHERE group_policies.action = 'read'
		AND group_policies.effect = 'allow'
		AND ('things/'||things.uuid) LIKE group_policies.resource
	)
	AND NOT EXISTS (
		SELECT 1
		FROM usr
		INNER JOIN user_groups ON user_groups.user_uuid = usr.uuid
		INNER JOIN group_policies ON group_policies.group_uuid = user_groups.group_uuid
		WHERE group_policies.action = 'read'
		AND group_policies.effect = 'deny'
		AND ('things/'||things.uuid) LIKE group_policies.resource
	)
	ORDER BY name
	LIMIT sqlc.arg(arg_limit)::BIGINT
	OFFSET sqlc.arg(arg_offset)::BIGINT
)
SELECT *
FROM permitted_things;

-- name: FindThingsByTags :many
WITH usr AS (
	SELECT user_tokens.user_uuid AS uuid
	FROM user_tokens
	WHERE user_tokens.token_hash = sha256(sqlc.arg(token))
	LIMIT 1
), permitted_things AS (
	SELECT *
	FROM things
	WHERE sqlc.arg(tags) && things.tags
	AND EXISTS (
		SELECT 1
		FROM usr
		INNER JOIN user_groups ON user_groups.user_uuid = usr.uuid
		INNER JOIN group_policies ON group_policies.group_uuid = user_groups.group_uuid
		WHERE group_policies.action = 'read'
		AND group_policies.effect = 'allow'
		AND ('things/'||things.uuid) LIKE group_policies.resource
	)
	AND NOT EXISTS (
		SELECT 1
		FROM usr
		INNER JOIN user_groups ON user_groups.user_uuid = usr.uuid
		INNER JOIN group_policies ON group_policies.group_uuid = user_groups.group_uuid
		WHERE group_policies.action = 'read'
		AND group_policies.effect = 'deny'
		AND ('things/'||things.uuid) LIKE group_policies.resource
	)
	ORDER BY name
	LIMIT sqlc.arg(arg_limit)::BIGINT
	OFFSET sqlc.arg(arg_offset)::BIGINT
)
SELECT *
FROM permitted_things;

-- name: SetThingNameByUUID :execrows
UPDATE things
SET name = sqlc.arg(name)
WHERE things.uuid = sqlc.arg(uuid);

-- name: SetThingTypeByUUID :execrows
UPDATE things
SET type = sqlc.arg(type)
WHERE things.uuid = sqlc.arg(uuid);

-- name: SetThingStateByUUID :execrows
UPDATE things
SET state = sqlc.arg(state)
WHERE things.uuid = sqlc.arg(uuid);

-- name: SetThingTags :execrows
UPDATE things
SET tags = sqlc.arg(tags)
WHERE things.uuid = sqlc.arg(uuid);

-- name: DeleteThing :execrows
DELETE FROM things
WHERE things.uuid = sqlc.arg(uuid);
