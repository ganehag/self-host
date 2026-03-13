BEGIN;

CREATE OR REPLACE FUNCTION user_has_access(uid UUID, act policy_action, res TEXT) RETURNS boolean AS $$
	SELECT
		EXISTS (
			SELECT 1
			FROM user_groups
			INNER JOIN group_policies ON group_policies.group_uuid = user_groups.group_uuid
			WHERE user_groups.user_uuid = uid
			AND group_policies.action = act
			AND group_policies.effect = 'allow'
			AND res LIKE group_policies.resource
		)
		AND NOT EXISTS (
			SELECT 1
			FROM user_groups
			INNER JOIN group_policies ON group_policies.group_uuid = user_groups.group_uuid
			WHERE user_groups.user_uuid = uid
			AND group_policies.action = act
			AND group_policies.effect = 'deny'
			AND res LIKE group_policies.resource
		);
$$ LANGUAGE sql;

COMMIT;
