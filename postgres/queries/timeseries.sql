-- name: ExistsTimeseries :one
SELECT COUNT(*) AS count
FROM timeseries
WHERE timeseries.uuid = sqlc.arg(uuid);

-- name: CountExistingTimeseries :one
SELECT COUNT(*) AS count
FROM timeseries
WHERE timeseries.uuid = ANY(sqlc.arg(uuids)::uuid[]);

-- name: CreateTimeseries :one
WITH t AS (
	INSERT INTO timeseries(
		thing_uuid,
		name,
		si_unit,
		lower_bound,
		upper_bound,
		created_by,
		tags
	) VALUES (
		NULLIF(sqlc.arg(thing_uuid)::uuid, '00000000-0000-0000-0000-000000000000'::uuid),
		sqlc.arg(name),
		sqlc.arg(si_unit),
		sqlc.arg(lower_bound),
		sqlc.arg(upper_bound),
		sqlc.arg(created_by),
		sqlc.arg(tags)
	) RETURNING *
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
		(SELECT uuid FROM grp), 0, 'allow', 'create','timeseries/'||(SELECT uuid FROM t)||'/%'
	), (
		(SELECT uuid FROM grp), 0, 'allow', 'read','timeseries/'||(SELECT uuid FROM t)||'/%'
	), (
		(SELECT uuid FROM grp), 0, 'allow', 'update','timeseries/'||(SELECT uuid FROM t)||'/%'
	), (
		(SELECT uuid FROM grp), 0, 'allow', 'delete','timeseries/'||(SELECT uuid FROM t)||'/%'
	)
)
SELECT *
FROM t LIMIT 1;

-- name: GetTimeseriesByUUID :one
SELECT * FROM timeseries
WHERE uuid = sqlc.arg(uuid)
LIMIT 1;

-- name: GetUnitFromTimeseries :one
SELECT si_unit FROM timeseries
WHERE uuid = sqlc.arg(uuid)
LIMIT 1;

-- name: FindTimeseries :many
WITH usr AS (
	SELECT user_tokens.user_uuid AS uuid
	FROM user_tokens
	WHERE user_tokens.token_hash = sha256(sqlc.arg(token))
	LIMIT 1
), permitted_timeseries AS (
	SELECT *
	FROM timeseries
	WHERE EXISTS (
		SELECT 1
		FROM usr
		INNER JOIN user_groups ON user_groups.user_uuid = usr.uuid
		INNER JOIN group_policies ON group_policies.group_uuid = user_groups.group_uuid
		WHERE group_policies.action = 'read'
		AND group_policies.effect = 'allow'
		AND ('timeseries/'||timeseries.uuid) LIKE group_policies.resource
	)
	AND NOT EXISTS (
		SELECT 1
		FROM usr
		INNER JOIN user_groups ON user_groups.user_uuid = usr.uuid
		INNER JOIN group_policies ON group_policies.group_uuid = user_groups.group_uuid
		WHERE group_policies.action = 'read'
		AND group_policies.effect = 'deny'
		AND ('timeseries/'||timeseries.uuid) LIKE group_policies.resource
	)
	ORDER BY name
	LIMIT sqlc.arg(arg_limit)::BIGINT
	OFFSET sqlc.arg(arg_offset)::BIGINT
)
SELECT *
FROM permitted_timeseries;

-- name: FindTimeseriesByTags :many
WITH usr AS (
	SELECT user_tokens.user_uuid AS uuid
	FROM user_tokens
	WHERE user_tokens.token_hash = sha256(sqlc.arg(token))
	LIMIT 1
), permitted_timeseries AS (
	SELECT *
	FROM timeseries
	WHERE sqlc.arg(tags) && timeseries.tags
	AND EXISTS (
		SELECT 1
		FROM usr
		INNER JOIN user_groups ON user_groups.user_uuid = usr.uuid
		INNER JOIN group_policies ON group_policies.group_uuid = user_groups.group_uuid
		WHERE group_policies.action = 'read'
		AND group_policies.effect = 'allow'
		AND ('timeseries/'||timeseries.uuid) LIKE group_policies.resource
	)
	AND NOT EXISTS (
		SELECT 1
		FROM usr
		INNER JOIN user_groups ON user_groups.user_uuid = usr.uuid
		INNER JOIN group_policies ON group_policies.group_uuid = user_groups.group_uuid
		WHERE group_policies.action = 'read'
		AND group_policies.effect = 'deny'
		AND ('timeseries/'||timeseries.uuid) LIKE group_policies.resource
	)
	ORDER BY name
	LIMIT sqlc.arg(arg_limit)::BIGINT
	OFFSET sqlc.arg(arg_offset)::BIGINT
)
SELECT *
FROM permitted_timeseries;

-- name: FindTimeseriesByThing :many
SELECT * FROM timeseries
WHERE sqlc.arg(thing_uuid) = timeseries.thing_uuid
ORDER BY name
;

-- name: FindTimeseriesByUUID :one
SELECT * FROM timeseries
WHERE sqlc.arg(ts_uuid) = timeseries.uuid
LIMIT 1;

-- name: SetTimeseriesThing :execrows
UPDATE timeseries
SET thing_uuid = sqlc.arg(thing_uuid)
WHERE timeseries.uuid = sqlc.arg(uuid);

-- name: SetTimeseriesName :execrows
UPDATE timeseries
SET name = sqlc.arg(name)
WHERE timeseries.uuid = sqlc.arg(uuid);

-- name: SetTimeseriesSiUnit :execrows
UPDATE timeseries
SET si_unit = sqlc.arg(si_unit)
WHERE timeseries.uuid = sqlc.arg(uuid);

-- name: SetTimeseriesLowerBound :execrows
UPDATE timeseries
SET lower_bound = sqlc.arg(lower_bound)
WHERE timeseries.uuid = sqlc.arg(uuid);

-- name: SetTimeseriesUpperBound :execrows
UPDATE timeseries
SET upper_bound = sqlc.arg(upper_bound)
WHERE timeseries.uuid = sqlc.arg(uuid);

-- name: SetTimeseriesTags :execrows
UPDATE timeseries
SET tags = sqlc.arg(tags)
WHERE timeseries.uuid = sqlc.arg(uuid);

-- name: DeleteTimeseries :execrows
DELETE FROM timeseries
WHERE uuid = sqlc.arg(uuid);
