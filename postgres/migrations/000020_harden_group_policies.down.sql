BEGIN;

ALTER TABLE group_policies
DROP CONSTRAINT IF EXISTS group_policies_resource_shape_chk,
DROP CONSTRAINT IF EXISTS group_policies_priority_nonnegative_chk;

COMMIT;
