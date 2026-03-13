BEGIN;

CREATE OR REPLACE FUNCTION user_has_access(uid UUID, act policy_action, res TEXT) RETURNS boolean AS $$
	WITH policies AS (
	        SELECT group_policies.effect, group_policies.priority, group_policies.resource
        	FROM group_policies, user_groups
	        WHERE user_groups.group_uuid = group_policies.group_uuid
	        AND user_groups.user_uuid = $1
        	AND action = $2::policy_action
	), c AS (
		SELECT res AS resource
	), has_access AS (
		SELECT *
		FROM c
		WHERE c.resource LIKE ANY((SELECT resource FROM policies WHERE effect = 'allow'))
		EXCEPT
		SELECT *
		FROM c
		WHERE c.resource LIKE ANY((SELECT resource FROM policies WHERE effect = 'deny'))
	)
	SELECT COUNT(*) > 0 FROM has_access;
$$ LANGUAGE sql;

COMMIT;
