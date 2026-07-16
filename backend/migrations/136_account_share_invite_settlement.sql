-- Consumption-based invitation sharing for public account pool settlements.

ALTER TABLE account_share_policies
    ADD COLUMN IF NOT EXISTS invite_share_ratio DECIMAL(10, 6) NOT NULL DEFAULT 0;

DO $$
BEGIN
    IF NOT EXISTS (
        SELECT 1 FROM pg_constraint WHERE conname = 'account_share_policies_invite_ratio_check'
    ) THEN
        ALTER TABLE account_share_policies
            ADD CONSTRAINT account_share_policies_invite_ratio_check
            CHECK (
                invite_share_ratio >= 0
                AND invite_share_ratio <= 1
                AND owner_share_ratio + invite_share_ratio <= 1
            );
    END IF;
END $$;

ALTER TABLE user_affiliates
    ADD COLUMN IF NOT EXISTS inviter_bound_at TIMESTAMPTZ,
    ADD COLUMN IF NOT EXISTS invite_reward_expires_at TIMESTAMPTZ;

WITH duration AS (
    SELECT CASE
        WHEN value ~ '^[0-9]+$' THEN LEAST(value::integer, 3650)
        ELSE 0
    END AS days
    FROM settings
    WHERE key = 'affiliate_rebate_duration_days'
    LIMIT 1
)
UPDATE user_affiliates ua
SET inviter_bound_at = COALESCE(ua.inviter_bound_at, ua.created_at),
    invite_reward_expires_at = COALESCE(
        ua.invite_reward_expires_at,
        CASE
            WHEN COALESCE((SELECT days FROM duration), 0) > 0
            THEN COALESCE(ua.inviter_bound_at, ua.created_at) + make_interval(days => COALESCE((SELECT days FROM duration), 0))
            ELSE NULL
        END
    ),
    updated_at = NOW()
WHERE ua.inviter_id IS NOT NULL
  AND ua.inviter_bound_at IS NULL;

CREATE INDEX IF NOT EXISTS idx_user_affiliates_invite_reward
    ON user_affiliates (user_id, inviter_id, invite_reward_expires_at)
    WHERE inviter_id IS NOT NULL;

ALTER TABLE account_share_settlement_entries
    ADD COLUMN IF NOT EXISTS inviter_user_id BIGINT REFERENCES users(id) ON DELETE SET NULL,
    ADD COLUMN IF NOT EXISTS invite_bound_at_snapshot TIMESTAMPTZ,
    ADD COLUMN IF NOT EXISTS invite_expires_at_snapshot TIMESTAMPTZ,
    ADD COLUMN IF NOT EXISTS invite_share_ratio DECIMAL(10, 6) NOT NULL DEFAULT 0,
    ADD COLUMN IF NOT EXISTS invite_credit DECIMAL(20, 10) NOT NULL DEFAULT 0,
    ADD COLUMN IF NOT EXISTS platform_share_ratio DECIMAL(10, 6) NOT NULL DEFAULT 0;

UPDATE account_share_settlement_entries
SET platform_share_ratio = GREATEST(0, 1 - owner_share_ratio - invite_share_ratio)
WHERE platform_share_ratio = 0
  AND status = 'applied';

DO $$
BEGIN
    IF NOT EXISTS (
        SELECT 1 FROM pg_constraint WHERE conname = 'account_share_settlement_invite_amounts_check'
    ) THEN
        ALTER TABLE account_share_settlement_entries
            ADD CONSTRAINT account_share_settlement_invite_amounts_check
            CHECK (
                invite_share_ratio >= 0
                AND invite_share_ratio <= 1
                AND platform_share_ratio >= 0
                AND platform_share_ratio <= 1
                AND owner_share_ratio + invite_share_ratio + platform_share_ratio <= 1.000001
                AND invite_credit >= 0
                AND owner_credit + invite_credit <= consumer_charge
            );
    END IF;
END $$;

CREATE INDEX IF NOT EXISTS idx_account_share_settlement_inviter_time
    ON account_share_settlement_entries (inviter_user_id, created_at DESC)
    WHERE inviter_user_id IS NOT NULL;
