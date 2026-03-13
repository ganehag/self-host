-- name: GetTsDataRange :many
SELECT	ts_uuid,
	value,
	ts
FROM tsdata
WHERE ts_uuid = ANY(sqlc.arg(ts_uuids)::uuid[])
AND ts BETWEEN sqlc.arg(start) AND sqlc.arg(stop);

-- name: GetTsDataRangeLimited :many
WITH ranked AS (
	SELECT
		ts_uuid,
		value,
		ts,
		ROW_NUMBER() OVER (PARTITION BY ts_uuid ORDER BY ts ASC) AS series_row_num,
		ROW_NUMBER() OVER (ORDER BY ts_uuid ASC, ts ASC) AS total_row_num
	FROM tsdata
	WHERE ts_uuid = ANY(sqlc.arg(ts_uuids)::uuid[])
	AND ts BETWEEN sqlc.arg(start) AND sqlc.arg(stop)
	AND (sqlc.arg(ge_null)::boolean = true OR tsdata.value >= sqlc.arg(ge))
	AND (sqlc.arg(le_null)::boolean = true OR tsdata.value <= sqlc.arg(le))
)
SELECT
	ts_uuid,
	value,
	ts,
	series_row_num,
	total_row_num
FROM ranked
WHERE (sqlc.arg(max_points_per_series)::BIGINT <= 0 OR series_row_num <= sqlc.arg(max_points_per_series)::BIGINT + 1)
AND (sqlc.arg(max_total_points)::BIGINT <= 0 OR total_row_num <= sqlc.arg(max_total_points)::BIGINT + 1)
ORDER BY ts_uuid ASC, ts ASC;

-- name: GetTsHourlyRollupRange :many
SELECT
	ts_uuid,
	bucket_ts,
	sample_count,
	sample_sum,
	sample_min,
	sample_max
FROM tsdata_hourly_rollups
WHERE ts_uuid = ANY(sqlc.arg(ts_uuids)::uuid[])
AND bucket_ts BETWEEN sqlc.arg(start) AND sqlc.arg(stop)
ORDER BY ts_uuid ASC, bucket_ts ASC;

-- name: GetTsDailyRollupRange :many
SELECT
	ts_uuid,
	bucket_ts,
	sample_count,
	sample_sum,
	sample_min,
	sample_max
FROM tsdata_daily_rollups
WHERE ts_uuid = ANY(sqlc.arg(ts_uuids)::uuid[])
AND bucket_ts BETWEEN sqlc.arg(start) AND sqlc.arg(stop)
ORDER BY ts_uuid ASC, bucket_ts ASC;

-- name: GetTsDataRangeAgg :many
WITH tsdata_trunc AS (
	SELECT
       	ts_uuid,
       	value,
	CASE
		WHEN sqlc.arg(truncate)::text = 'minute5' THEN
		  (date_trunc('hour', ts) + date_part('minute', ts)::int / 5 * interval '5 min') AT time zone sqlc.arg(timezone)::text
		WHEN sqlc.arg(truncate)::text = 'minute10' THEN
		  (date_trunc('hour', ts) + date_part('minute', ts)::int / 10 * interval '10 min') AT time zone sqlc.arg(timezone)::text
		WHEN sqlc.arg(truncate)::text = 'minute15' THEN
		  (date_trunc('hour', ts) + date_part('minute', ts)::int / 15 * interval '15 min') AT time zone sqlc.arg(timezone)::text
		WHEN sqlc.arg(truncate)::text = 'minute20' THEN
		  (date_trunc('hour', ts) + date_part('minute', ts)::int / 20 * interval '20 min') AT time zone sqlc.arg(timezone)::text
		WHEN sqlc.arg(truncate)::text = 'minute30' THEN
		  (date_trunc('hour', ts) + date_part('minute', ts)::int / 30 * interval '30 min') AT time zone sqlc.arg(timezone)::text
		ELSE
		  date_trunc(sqlc.arg(truncate)::text, ts AT time zone sqlc.arg(timezone)::text) AT time zone sqlc.arg(timezone)::text
	END AS ts
	FROM tsdata
	WHERE ts_uuid = ANY(sqlc.arg(ts_uuids)::uuid[])
	AND ts BETWEEN sqlc.arg(start) AND sqlc.arg(stop)
)
SELECT
        ts_uuid::uuid,
	(CASE
		WHEN sqlc.arg(aggregate)::text = 'avg'::text THEN AVG(value)
		WHEN sqlc.arg(aggregate)::text = 'min'::text THEN MIN(value)
		WHEN sqlc.arg(aggregate)::text = 'max'::text THEN MAX(value)
		WHEN sqlc.arg(aggregate)::text = 'count'::text THEN COUNT(value)
		WHEN sqlc.arg(aggregate)::text = 'sum'::text THEN SUM(value)
	END)::DOUBLE PRECISION AS value,
        ts::timestamptz
FROM tsdata_trunc
GROUP BY ts_uuid, ts
ORDER BY ts_uuid ASC, ts ASC;

-- name: CreateTsData :execrows
INSERT INTO tsdata(ts_uuid, value, ts, created_by)
VALUES (
	sqlc.arg(ts_uuid),
	sqlc.arg(value),
	sqlc.arg(ts),
	sqlc.arg(created_by)
);

-- name: DeleteAllTsData :execrows
DELETE FROM tsdata
WHERE ts_uuid = ANY(sqlc.arg(ts_uuid));

-- name: DeleteTsDataRange :execrows
DELETE FROM tsdata
WHERE ts_uuid = ANY(sqlc.arg(ts_uuids)::uuid[])
AND ts BETWEEN sqlc.arg(start) AND sqlc.arg(stop)
AND (sqlc.arg(ge_null)::boolean = true OR tsdata.value >= sqlc.arg(ge))
AND (sqlc.arg(le_null)::boolean = true OR tsdata.value <= sqlc.arg(le))
;
