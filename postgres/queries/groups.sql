-- name: ExistsGroup :one
SELECT COUNT(*) AS count
FROM groups
WHERE groups.uuid = sqlc.arg(uuid);

-- name: CreateGroup :one
INSERT INTO groups(name)
VALUES(sqlc.arg(name))
RETURNING *;

-- name: FindGroups :many
WITH usr AS (
	SELECT user_tokens.user_uuid AS uuid
	FROM user_tokens
	WHERE user_tokens.token_hash = sha256(sqlc.arg(token))
	LIMIT 1
), permitted_groups AS (
	SELECT *
	FROM groups
	WHERE EXISTS (
		SELECT 1
		FROM usr
		INNER JOIN user_groups ON user_groups.user_uuid = usr.uuid
		INNER JOIN group_policies ON group_policies.group_uuid = user_groups.group_uuid
		WHERE group_policies.action = 'read'
		AND group_policies.effect = 'allow'
		AND ('groups/'||groups.uuid) LIKE group_policies.resource
	)
	AND NOT EXISTS (
		SELECT 1
		FROM usr
		INNER JOIN user_groups ON user_groups.user_uuid = usr.uuid
		INNER JOIN group_policies ON group_policies.group_uuid = user_groups.group_uuid
		WHERE group_policies.action = 'read'
		AND group_policies.effect = 'deny'
		AND ('groups/'||groups.uuid) LIKE group_policies.resource
	)
	ORDER BY name
	LIMIT sqlc.arg(arg_limit)::BIGINT
	OFFSET sqlc.arg(arg_offset)::BIGINT
)
SELECT *
FROM permitted_groups;

-- name: FindGroupByUuid :one
SELECT * FROM groups
WHERE uuid = sqlc.arg(uuid)
LIMIT 1;

-- name: FindGroupsByUser :many
SELECT groups.*
FROM groups, user_groups
WHERE groups.uuid = user_groups.group_uuid
AND user_groups.user_uuid = sqlc.arg(uuid);

-- name: DeleteGroup :execrows
DELETE FROM groups
WHERE groups.uuid = sqlc.arg(uuid);

-- name: SetGroupNameByUUID :execrows
UPDATE groups
SET name = sqlc.arg(name)
WHERE groups.uuid = sqlc.arg(uuid);
