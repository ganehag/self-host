BEGIN;

CREATE OR REPLACE FUNCTION user_has_access(uid UUID, act policy_action, res TEXT) RETURNS boolean AS $$
	SELECT COALESCE((
		SELECT group_policies.effect = 'allow'
		FROM user_groups
		INNER JOIN group_policies ON group_policies.group_uuid = user_groups.group_uuid
		WHERE user_groups.user_uuid = uid
		AND group_policies.action = act
		AND res LIKE group_policies.resource
		ORDER BY group_policies.priority ASC,
			CASE WHEN group_policies.effect = 'deny' THEN 0 ELSE 1 END ASC
		LIMIT 1
	), false);
$$ LANGUAGE sql;

COMMIT;
