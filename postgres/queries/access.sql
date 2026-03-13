-- name: CheckUserTokenHasAccess :one
WITH usr AS (
	SELECT user_tokens.user_uuid AS uuid
	FROM user_tokens
	WHERE user_tokens.token_hash = sha256(sqlc.arg(token))
	LIMIT 1
)
SELECT COALESCE((
	SELECT
		EXISTS (
			SELECT 1
			FROM user_groups
			INNER JOIN group_policies ON group_policies.group_uuid = user_groups.group_uuid
			WHERE user_groups.user_uuid = usr.uuid
			AND group_policies.action = sqlc.arg(action)::policy_action
			AND group_policies.effect = 'allow'
			AND sqlc.arg(resource)::TEXT LIKE group_policies.resource
		)
		AND NOT EXISTS (
			SELECT 1
			FROM user_groups
			INNER JOIN group_policies ON group_policies.group_uuid = user_groups.group_uuid
			WHERE user_groups.user_uuid = usr.uuid
			AND group_policies.action = sqlc.arg(action)::policy_action
			AND group_policies.effect = 'deny'
			AND sqlc.arg(resource)::TEXT LIKE group_policies.resource
		)
	FROM usr
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
	COALESCE(bool_and(
		EXISTS (
			SELECT 1
			FROM user_groups
			INNER JOIN group_policies ON group_policies.group_uuid = user_groups.group_uuid
			WHERE user_groups.user_uuid = usr_r.uuid
			AND group_policies.action = usr_r.action
			AND group_policies.effect = 'allow'
			AND usr_r.resource LIKE group_policies.resource
		)
		AND NOT EXISTS (
			SELECT 1
			FROM user_groups
			INNER JOIN group_policies ON group_policies.group_uuid = user_groups.group_uuid
			WHERE user_groups.user_uuid = usr_r.uuid
			AND group_policies.action = usr_r.action
			AND group_policies.effect = 'deny'
			AND usr_r.resource LIKE group_policies.resource
		)
	), false)::boolean AS access
FROM usr_r;
