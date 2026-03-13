BEGIN;

DROP INDEX IF EXISTS program_code_revisions_signed_head_idx;
DROP INDEX IF EXISTS programs_type_state_language_uuid_idx;
DROP INDEX IF EXISTS timeseries_thing_uuid_name_idx;
DROP INDEX IF EXISTS group_policies_group_uuid_action_effect_idx;
DROP INDEX IF EXISTS user_tokens_token_hash_idx;

COMMIT;
