BEGIN;

DROP TRIGGER IF EXISTS tsdata_hourly_rollups_after_update ON tsdata;
DROP TRIGGER IF EXISTS tsdata_hourly_rollups_after_delete ON tsdata;
DROP TRIGGER IF EXISTS tsdata_hourly_rollups_after_insert ON tsdata;

DROP FUNCTION IF EXISTS upsert_tsdata_hourly_rollups_from_new();
DROP FUNCTION IF EXISTS refresh_tsdata_hourly_rollups_for_affected();
DROP FUNCTION IF EXISTS refresh_tsdata_hourly_rollups_for_affected_from_old();

DROP TABLE IF EXISTS tsdata_hourly_rollups;

COMMIT;
