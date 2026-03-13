BEGIN;

DROP TRIGGER IF EXISTS tsdata_daily_rollups_after_hourly_update ON tsdata_hourly_rollups;
DROP TRIGGER IF EXISTS tsdata_daily_rollups_after_hourly_delete ON tsdata_hourly_rollups;
DROP TRIGGER IF EXISTS tsdata_daily_rollups_after_hourly_insert ON tsdata_hourly_rollups;

DROP FUNCTION IF EXISTS upsert_tsdata_daily_rollups_from_hourly_new();
DROP FUNCTION IF EXISTS refresh_tsdata_daily_rollups_for_affected_hourly();
DROP FUNCTION IF EXISTS refresh_tsdata_daily_rollups_for_affected_hourly_from_old();

DROP INDEX IF EXISTS tsdata_daily_rollups_bucket_ts_idx;
DROP TABLE IF EXISTS tsdata_daily_rollups;

COMMIT;
