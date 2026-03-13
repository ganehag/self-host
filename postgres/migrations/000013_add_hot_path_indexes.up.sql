BEGIN;

-- Every authenticated request resolves a token hash to a user.
CREATE INDEX user_tokens_token_hash_idx ON user_tokens(token_hash);

-- Policy checks join on group_uuid and filter by action/effect.
CREATE INDEX group_policies_group_uuid_action_effect_idx
ON group_policies(group_uuid, action, effect);

-- Thing-scoped timeseries reads filter by thing_uuid and order by name.
CREATE INDEX timeseries_thing_uuid_name_idx
ON timeseries(thing_uuid, name);

-- Program lookups frequently filter on active/module-like state and join by uuid.
CREATE INDEX programs_type_state_language_uuid_idx
ON programs(type, state, language, uuid);

-- Fetching the latest signed revision is a hot path for module/routine loading.
CREATE INDEX program_code_revisions_signed_head_idx
ON program_code_revisions(program_uuid, revision DESC)
WHERE signed IS NOT NULL;

COMMIT;
