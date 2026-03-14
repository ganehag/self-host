-- name: ExistsPolicy :one
SELECT COUNT(*) AS count
FROM group_policies
WHERE group_policies.uuid = sqlc.arg(uuid);

-- name: CreatePolicy :one
INSERT INTO group_policies(group_uuid, priority, effect, action, resource)
VALUES (
	sqlc.arg(group_uuid),
	sqlc.arg(priority),
	sqlc.arg(effect),
	sqlc.arg(action),
	sqlc.arg(resource)
)
RETURNING *;

-- name: FindPolicies :many
WITH usr AS (
	SELECT user_tokens.user_uuid AS uuid
	FROM user_tokens
	WHERE user_tokens.token_hash = sha256(sqlc.arg(token))
	LIMIT 1
), f_group_policies AS (
	SELECT * FROM group_policies
	WHERE
		sqlc.arg(group_uuids)::uuid[] IS NULL
	OR
		group_policies.group_uuid = ANY(sqlc.arg(group_uuids)::uuid[])
)
SELECT f_group_policies.*
FROM usr
INNER JOIN f_group_policies ON true
LEFT JOIN LATERAL (
	SELECT group_policies.effect
	FROM user_groups
	INNER JOIN group_policies ON group_policies.group_uuid = user_groups.group_uuid
	WHERE user_groups.user_uuid = usr.uuid
	AND group_policies.action = 'read'
	AND ('policies/' || f_group_policies.uuid) LIKE group_policies.resource
	ORDER BY group_policies.priority ASC,
		CASE WHEN group_policies.effect = 'deny' THEN 0 ELSE 1 END ASC
	LIMIT 1
) AS match ON true
WHERE COALESCE(match.effect = 'allow', false)
ORDER BY
	f_group_policies.resource DESC,
	f_group_policies.effect ASC,
	f_group_policies.action DESC,
	f_group_policies.priority ASC
LIMIT sqlc.arg(arg_limit)::BIGINT
OFFSET sqlc.arg(arg_offset)::BIGINT;

-- name: FindPolicyByUUID :one
SELECT *
FROM group_policies
WHERE group_policies.uuid = sqlc.arg(uuid);

-- name: SetPolicyGroup :execrows
UPDATE group_policies
SET group_uuid = sqlc.arg(group_uuid)
WHERE group_policies.uuid = sqlc.arg(uuid);

-- name: SetPolicyPriority :execrows
UPDATE group_policies
SET priority = sqlc.arg(priority)
WHERE group_policies.uuid = sqlc.arg(uuid);

-- name: SetPolicyEffect :execrows
UPDATE group_policies
SET effect = sqlc.arg(effect)
WHERE group_policies.uuid = sqlc.arg(uuid);

-- name: SetPolicyAction :execrows
UPDATE group_policies
SET action = sqlc.arg(action)
WHERE group_policies.uuid = sqlc.arg(uuid);

-- name: SetPolicyResource :execrows
UPDATE group_policies
SET resource = sqlc.arg(resource)
WHERE group_policies.uuid = sqlc.arg(uuid);

-- name: DeletePolicyByUUID :execrows
DELETE
FROM group_policies
WHERE group_policies.uuid = sqlc.arg(uuid);
