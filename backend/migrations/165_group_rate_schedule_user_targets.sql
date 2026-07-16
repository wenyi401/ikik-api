-- Add optional per-user targets to time-range rate schedules.
ALTER TABLE group_rate_schedules
    ADD COLUMN IF NOT EXISTS target_user_id BIGINT NULL REFERENCES users(id) ON DELETE CASCADE;

CREATE INDEX IF NOT EXISTS idx_group_rate_schedules_group_target_enabled
    ON group_rate_schedules(group_id, target_user_id, enabled);

CREATE TABLE IF NOT EXISTS group_rate_schedule_user_states (
    group_id              BIGINT NOT NULL REFERENCES groups(id) ON DELETE CASCADE,
    user_id               BIGINT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    base_rate_multiplier  DECIMAL(10,4) NULL,
    applied_schedule_id   BIGINT NULL REFERENCES group_rate_schedules(id) ON DELETE SET NULL,
    created_at            TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at            TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    PRIMARY KEY (group_id, user_id),
    CONSTRAINT chk_group_rate_schedule_user_states_base_multiplier
        CHECK (base_rate_multiplier IS NULL OR base_rate_multiplier > 0)
);

COMMENT ON COLUMN group_rate_schedules.target_user_id IS 'Optional user target. NULL means the schedule applies to the group default multiplier.';
COMMENT ON TABLE group_rate_schedule_user_states IS 'Runtime state for per-user time-range rate schedules.';
COMMENT ON COLUMN group_rate_schedule_user_states.base_rate_multiplier IS 'Original per-user multiplier before entering a scheduled range. NULL means no per-user override existed.';

