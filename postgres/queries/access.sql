-- name: CheckUserTokenHasAccess :one
WITH usr AS (
	SELECT user_tokens.user_uuid AS uuid
	FROM user_tokens
	WHERE user_tokens.token_hash = sha256(sqlc.arg(token))
	LIMIT 1
)
SELECT COALESCE((
	SELECT match.effect = 'allow'
	FROM usr
	LEFT JOIN LATERAL (
		SELECT group_policies.effect
		FROM user_groups
		INNER JOIN group_policies ON group_policies.group_uuid = user_groups.group_uuid
		WHERE user_groups.user_uuid = usr.uuid
		AND group_policies.action = sqlc.arg(action)::policy_action
		AND sqlc.arg(resource)::TEXT LIKE group_policies.resource
		ORDER BY group_policies.priority ASC,
			CASE WHEN group_policies.effect = 'deny' THEN 0 ELSE 1 END ASC
		LIMIT 1
	) AS match ON true
	LIMIT 1
), false)::boolean AS access;

-- name: CheckUserTokenHasAccessMany :one
WITH usr AS (
	SELECT user_tokens.user_uuid AS uuid
	FROM user_tokens
	WHERE user_tokens.token_hash = sha256(sqlc.arg(token))
	LIMIT 1
), usr_r AS (
	SELECT
		usr.uuid,
		sqlc.arg(action)::policy_action AS action,
		unnest((SELECT sqlc.arg(resources)::TEXT[]))::TEXT AS resource
	FROM usr
	)
SELECT
	COALESCE(bool_and(match.effect = 'allow'), false)::boolean AS access
FROM usr_r
LEFT JOIN LATERAL (
	SELECT group_policies.effect
	FROM user_groups
	INNER JOIN group_policies ON group_policies.group_uuid = user_groups.group_uuid
	WHERE user_groups.user_uuid = usr_r.uuid
	AND group_policies.action = usr_r.action
	AND usr_r.resource LIKE group_policies.resource
	ORDER BY group_policies.priority ASC,
		CASE WHEN group_policies.effect = 'deny' THEN 0 ELSE 1 END ASC
	LIMIT 1
) AS match ON true;

-- name: ExplainUserTokenAccessMany :many
WITH usr AS (
	SELECT user_tokens.user_uuid AS uuid
	FROM user_tokens
	WHERE user_tokens.token_hash = sha256(sqlc.arg(token))
	LIMIT 1
), usr_r AS (
	SELECT
		usr.uuid,
		sqlc.arg(action)::policy_action AS action,
		unnest((SELECT sqlc.arg(resources)::TEXT[]))::TEXT AS resource
	FROM usr
)
SELECT
	usr_r.resource,
	(match.uuid IS NOT NULL)::boolean AS matched,
	COALESCE(match.uuid, '00000000-0000-0000-0000-000000000000'::uuid) AS policy_uuid,
	COALESCE(match.group_uuid, '00000000-0000-0000-0000-000000000000'::uuid) AS group_uuid,
	COALESCE(match.priority, -1)::integer AS priority,
	COALESCE(match.effect, 'deny'::policy_effect) AS effect,
	COALESCE(match.action, usr_r.action) AS action,
	COALESCE(match.resource, '') AS policy_resource,
	COALESCE(match.effect = 'allow', false)::boolean AS access
FROM usr_r
LEFT JOIN LATERAL (
	SELECT
		group_policies.uuid,
		group_policies.group_uuid,
		group_policies.priority,
		group_policies.effect,
		group_policies.action,
		group_policies.resource
	FROM user_groups
	INNER JOIN group_policies ON group_policies.group_uuid = user_groups.group_uuid
	WHERE user_groups.user_uuid = usr_r.uuid
	AND group_policies.action = usr_r.action
	AND usr_r.resource LIKE group_policies.resource
	ORDER BY group_policies.priority ASC,
		CASE WHEN group_policies.effect = 'deny' THEN 0 ELSE 1 END ASC
	LIMIT 1
) AS match ON true;
