BEGIN;

CREATE TABLE tsdata_hourly_rollups (
  ts_uuid UUID REFERENCES timeseries(uuid) ON DELETE CASCADE NOT NULL,
  bucket_ts TIMESTAMPTZ NOT NULL,
  sample_count BIGINT NOT NULL,
  sample_sum DOUBLE PRECISION NOT NULL,
  sample_min DOUBLE PRECISION NOT NULL,
  sample_max DOUBLE PRECISION NOT NULL,

  PRIMARY KEY (ts_uuid, bucket_ts)
);

CREATE INDEX tsdata_hourly_rollups_bucket_ts_idx ON tsdata_hourly_rollups(bucket_ts);

CREATE OR REPLACE FUNCTION refresh_tsdata_hourly_rollups_for_affected_from_old()
RETURNS trigger AS
$$
BEGIN
  DELETE FROM tsdata_hourly_rollups rollup
  USING (
    SELECT DISTINCT ts_uuid, date_trunc('hour', ts) AS bucket_ts
    FROM old_rows
  ) buckets
  WHERE rollup.ts_uuid = buckets.ts_uuid
  AND rollup.bucket_ts = buckets.bucket_ts;

  INSERT INTO tsdata_hourly_rollups(ts_uuid, bucket_ts, sample_count, sample_sum, sample_min, sample_max)
  SELECT
    buckets.ts_uuid,
    buckets.bucket_ts,
    COUNT(tsdata.*) AS sample_count,
    SUM(tsdata.value) AS sample_sum,
    MIN(tsdata.value) AS sample_min,
    MAX(tsdata.value) AS sample_max
  FROM (
    SELECT DISTINCT ts_uuid, date_trunc('hour', ts) AS bucket_ts
    FROM old_rows
  ) buckets
  INNER JOIN tsdata
    ON tsdata.ts_uuid = buckets.ts_uuid
   AND tsdata.ts >= buckets.bucket_ts
   AND tsdata.ts < buckets.bucket_ts + interval '1 hour'
  GROUP BY buckets.ts_uuid, buckets.bucket_ts;

  RETURN NULL;
END;
$$ LANGUAGE plpgsql;

CREATE OR REPLACE FUNCTION refresh_tsdata_hourly_rollups_for_affected()
RETURNS trigger AS
$$
BEGIN
  DELETE FROM tsdata_hourly_rollups rollup
  USING (
    SELECT DISTINCT ts_uuid, date_trunc('hour', ts) AS bucket_ts
    FROM (
      SELECT ts_uuid, ts FROM old_rows
      UNION
      SELECT ts_uuid, ts FROM new_rows
    ) affected
  ) buckets
  WHERE rollup.ts_uuid = buckets.ts_uuid
  AND rollup.bucket_ts = buckets.bucket_ts;

  INSERT INTO tsdata_hourly_rollups(ts_uuid, bucket_ts, sample_count, sample_sum, sample_min, sample_max)
  SELECT
    buckets.ts_uuid,
    buckets.bucket_ts,
    COUNT(tsdata.*) AS sample_count,
    SUM(tsdata.value) AS sample_sum,
    MIN(tsdata.value) AS sample_min,
    MAX(tsdata.value) AS sample_max
  FROM (
    SELECT DISTINCT ts_uuid, date_trunc('hour', ts) AS bucket_ts
    FROM (
      SELECT ts_uuid, ts FROM old_rows
      UNION
      SELECT ts_uuid, ts FROM new_rows
    ) affected
  ) buckets
  INNER JOIN tsdata
    ON tsdata.ts_uuid = buckets.ts_uuid
   AND tsdata.ts >= buckets.bucket_ts
   AND tsdata.ts < buckets.bucket_ts + interval '1 hour'
  GROUP BY buckets.ts_uuid, buckets.bucket_ts;

  RETURN NULL;
END;
$$ LANGUAGE plpgsql;

CREATE OR REPLACE FUNCTION upsert_tsdata_hourly_rollups_from_new()
RETURNS trigger AS
$$
BEGIN
  INSERT INTO tsdata_hourly_rollups(ts_uuid, bucket_ts, sample_count, sample_sum, sample_min, sample_max)
  SELECT
    ts_uuid,
    date_trunc('hour', ts) AS bucket_ts,
    COUNT(*) AS sample_count,
    SUM(value) AS sample_sum,
    MIN(value) AS sample_min,
    MAX(value) AS sample_max
  FROM new_rows
  GROUP BY ts_uuid, date_trunc('hour', ts)
  ON CONFLICT (ts_uuid, bucket_ts)
  DO UPDATE SET
    sample_count = tsdata_hourly_rollups.sample_count + EXCLUDED.sample_count,
    sample_sum = tsdata_hourly_rollups.sample_sum + EXCLUDED.sample_sum,
    sample_min = LEAST(tsdata_hourly_rollups.sample_min, EXCLUDED.sample_min),
    sample_max = GREATEST(tsdata_hourly_rollups.sample_max, EXCLUDED.sample_max);

  RETURN NULL;
END;
$$ LANGUAGE plpgsql;

CREATE TRIGGER tsdata_hourly_rollups_after_insert
AFTER INSERT ON tsdata
REFERENCING NEW TABLE AS new_rows
FOR EACH STATEMENT
EXECUTE FUNCTION upsert_tsdata_hourly_rollups_from_new();

CREATE TRIGGER tsdata_hourly_rollups_after_delete
AFTER DELETE ON tsdata
REFERENCING OLD TABLE AS old_rows
FOR EACH STATEMENT
EXECUTE FUNCTION refresh_tsdata_hourly_rollups_for_affected_from_old();

CREATE TRIGGER tsdata_hourly_rollups_after_update
AFTER UPDATE ON tsdata
REFERENCING OLD TABLE AS old_rows NEW TABLE AS new_rows
FOR EACH STATEMENT
EXECUTE FUNCTION refresh_tsdata_hourly_rollups_for_affected();

INSERT INTO tsdata_hourly_rollups(ts_uuid, bucket_ts, sample_count, sample_sum, sample_min, sample_max)
SELECT
  ts_uuid,
  date_trunc('hour', ts) AS bucket_ts,
  COUNT(*) AS sample_count,
  SUM(value) AS sample_sum,
  MIN(value) AS sample_min,
  MAX(value) AS sample_max
FROM tsdata
GROUP BY ts_uuid, date_trunc('hour', ts);

COMMIT;
