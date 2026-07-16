ALTER TABLE carpool_pool_accounts
    ADD COLUMN IF NOT EXISTS external_5h_used_usd NUMERIC(18, 6) NOT NULL DEFAULT 0,
    ADD COLUMN IF NOT EXISTS external_weekly_used_usd NUMERIC(18, 6) NOT NULL DEFAULT 0,
    ADD COLUMN IF NOT EXISTS external_5h_reset_at TIMESTAMPTZ,
    ADD COLUMN IF NOT EXISTS external_weekly_reset_at TIMESTAMPTZ,
    ADD COLUMN IF NOT EXISTS external_checked_at TIMESTAMPTZ,
    ADD COLUMN IF NOT EXISTS external_overage_notified_at TIMESTAMPTZ;
