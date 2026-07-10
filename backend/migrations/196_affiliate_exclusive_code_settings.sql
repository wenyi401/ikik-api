-- Complete the admin-managed exclusive invitation configuration.
ALTER TABLE user_affiliates
    ADD COLUMN IF NOT EXISTS aff_code_usage_limit INTEGER,
    ADD COLUMN IF NOT EXISTS aff_code_expires_at TIMESTAMPTZ,
    ADD COLUMN IF NOT EXISTS aff_signup_bonus_balance DECIMAL(20,8) NOT NULL DEFAULT 0,
    ADD COLUMN IF NOT EXISTS aff_auto_group_id BIGINT REFERENCES groups(id) ON DELETE SET NULL;

ALTER TABLE user_affiliates
    DROP CONSTRAINT IF EXISTS user_affiliates_aff_code_usage_limit_check;

ALTER TABLE user_affiliates
    ADD CONSTRAINT user_affiliates_aff_code_usage_limit_check
    CHECK (aff_code_usage_limit IS NULL OR aff_code_usage_limit >= 0);

CREATE INDEX IF NOT EXISTS idx_user_affiliates_exclusive_code_settings
    ON user_affiliates (aff_code_expires_at, aff_auto_group_id)
    WHERE aff_code_usage_limit IS NOT NULL
       OR aff_code_expires_at IS NOT NULL
       OR aff_signup_bonus_balance <> 0
       OR aff_auto_group_id IS NOT NULL;
