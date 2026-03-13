BEGIN;

CREATE TABLE tsdata_daily_rollups (
  ts_uuid UUID REFERENCES timeseries(uuid) ON DELETE CASCADE NOT NULL,
  bucket_ts TIMESTAMPTZ NOT NULL,
  sample_count BIGINT NOT NULL,
  sample_sum DOUBLE PRECISION NOT NULL,
  sample_min DOUBLE PRECISION NOT NULL,
  sample_max DOUBLE PRECISION NOT NULL,

  PRIMARY KEY (ts_uuid, bucket_ts)
);

CREATE INDEX tsdata_daily_rollups_bucket_ts_idx ON tsdata_daily_rollups(bucket_ts);

CREATE OR REPLACE FUNCTION refresh_tsdata_daily_rollups_for_affected_hourly_from_old()
RETURNS trigger AS
$$
BEGIN
  DELETE FROM tsdata_daily_rollups rollup
  USING (
    SELECT DISTINCT ts_uuid, date_trunc('day', bucket_ts) AS bucket_ts
    FROM old_rows
  ) buckets
  WHERE rollup.ts_uuid = buckets.ts_uuid
  AND rollup.bucket_ts = buckets.bucket_ts;

  INSERT INTO tsdata_daily_rollups(ts_uuid, bucket_ts, sample_count, sample_sum, sample_min, sample_max)
  SELECT
    buckets.ts_uuid,
    buckets.bucket_ts,
    SUM(hourly.sample_count) AS sample_count,
    SUM(hourly.sample_sum) AS sample_sum,
    MIN(hourly.sample_min) AS sample_min,
    MAX(hourly.sample_max) AS sample_max
  FROM (
    SELECT DISTINCT ts_uuid, date_trunc('day', bucket_ts) AS bucket_ts
    FROM old_rows
  ) buckets
  INNER JOIN tsdata_hourly_rollups hourly
    ON hourly.ts_uuid = buckets.ts_uuid
   AND hourly.bucket_ts >= buckets.bucket_ts
   AND hourly.bucket_ts < buckets.bucket_ts + interval '1 day'
  GROUP BY buckets.ts_uuid, buckets.bucket_ts;

  RETURN NULL;
END;
$$ LANGUAGE plpgsql;

CREATE OR REPLACE FUNCTION refresh_tsdata_daily_rollups_for_affected_hourly()
RETURNS trigger AS
$$
BEGIN
  DELETE FROM tsdata_daily_rollups rollup
  USING (
    SELECT DISTINCT ts_uuid, date_trunc('day', bucket_ts) AS bucket_ts
    FROM (
      SELECT ts_uuid, bucket_ts FROM old_rows
      UNION
      SELECT ts_uuid, bucket_ts FROM new_rows
    ) affected
  ) buckets
  WHERE rollup.ts_uuid = buckets.ts_uuid
  AND rollup.bucket_ts = buckets.bucket_ts;

  INSERT INTO tsdata_daily_rollups(ts_uuid, bucket_ts, sample_count, sample_sum, sample_min, sample_max)
  SELECT
    buckets.ts_uuid,
    buckets.bucket_ts,
    SUM(hourly.sample_count) AS sample_count,
    SUM(hourly.sample_sum) AS sample_sum,
    MIN(hourly.sample_min) AS sample_min,
    MAX(hourly.sample_max) AS sample_max
  FROM (
    SELECT DISTINCT ts_uuid, date_trunc('day', bucket_ts) AS bucket_ts
    FROM (
      SELECT ts_uuid, bucket_ts FROM old_rows
      UNION
      SELECT ts_uuid, bucket_ts FROM new_rows
    ) affected
  ) buckets
  INNER JOIN tsdata_hourly_rollups hourly
    ON hourly.ts_uuid = buckets.ts_uuid
   AND hourly.bucket_ts >= buckets.bucket_ts
   AND hourly.bucket_ts < buckets.bucket_ts + interval '1 day'
  GROUP BY buckets.ts_uuid, buckets.bucket_ts;

  RETURN NULL;
END;
$$ LANGUAGE plpgsql;

CREATE OR REPLACE FUNCTION upsert_tsdata_daily_rollups_from_hourly_new()
RETURNS trigger AS
$$
BEGIN
  INSERT INTO tsdata_daily_rollups(ts_uuid, bucket_ts, sample_count, sample_sum, sample_min, sample_max)
  SELECT
    ts_uuid,
    date_trunc('day', bucket_ts) AS bucket_ts,
    SUM(sample_count) AS sample_count,
    SUM(sample_sum) AS sample_sum,
    MIN(sample_min) AS sample_min,
    MAX(sample_max) AS sample_max
  FROM new_rows
  GROUP BY ts_uuid, date_trunc('day', bucket_ts)
  ON CONFLICT (ts_uuid, bucket_ts)
  DO UPDATE SET
    sample_count = tsdata_daily_rollups.sample_count + EXCLUDED.sample_count,
    sample_sum = tsdata_daily_rollups.sample_sum + EXCLUDED.sample_sum,
    sample_min = LEAST(tsdata_daily_rollups.sample_min, EXCLUDED.sample_min),
    sample_max = GREATEST(tsdata_daily_rollups.sample_max, EXCLUDED.sample_max);

  RETURN NULL;
END;
$$ LANGUAGE plpgsql;

CREATE TRIGGER tsdata_daily_rollups_after_hourly_insert
AFTER INSERT ON tsdata_hourly_rollups
REFERENCING NEW TABLE AS new_rows
FOR EACH STATEMENT
EXECUTE FUNCTION upsert_tsdata_daily_rollups_from_hourly_new();

CREATE TRIGGER tsdata_daily_rollups_after_hourly_delete
AFTER DELETE ON tsdata_hourly_rollups
REFERENCING OLD TABLE AS old_rows
FOR EACH STATEMENT
EXECUTE FUNCTION refresh_tsdata_daily_rollups_for_affected_hourly_from_old();

CREATE TRIGGER tsdata_daily_rollups_after_hourly_update
AFTER UPDATE ON tsdata_hourly_rollups
REFERENCING OLD TABLE AS old_rows NEW TABLE AS new_rows
FOR EACH STATEMENT
EXECUTE FUNCTION refresh_tsdata_daily_rollups_for_affected_hourly();

INSERT INTO tsdata_daily_rollups(ts_uuid, bucket_ts, sample_count, sample_sum, sample_min, sample_max)
SELECT
  ts_uuid,
  date_trunc('day', bucket_ts) AS bucket_ts,
  SUM(sample_count) AS sample_count,
  SUM(sample_sum) AS sample_sum,
  MIN(sample_min) AS sample_min,
  MAX(sample_max) AS sample_max
FROM tsdata_hourly_rollups
GROUP BY ts_uuid, date_trunc('day', bucket_ts);

COMMIT;
