DO $$
BEGIN
    IF to_regclass('public.account_share_memberships') IS NULL THEN
        RETURN;
    END IF;

    ALTER TABLE account_share_memberships
        ADD COLUMN IF NOT EXISTS idle_timeout_minutes INTEGER NOT NULL DEFAULT 0,
        ADD COLUMN IF NOT EXISTS last_request_at TIMESTAMPTZ,
        ADD COLUMN IF NOT EXISTS ended_reason VARCHAR(32);

    IF NOT EXISTS (SELECT 1 FROM pg_constraint WHERE conname = 'account_share_memberships_idle_timeout_chk') THEN
        ALTER TABLE account_share_memberships
            ADD CONSTRAINT account_share_memberships_idle_timeout_chk
            CHECK (idle_timeout_minutes BETWEEN 0 AND 10080);
    END IF;

    IF NOT EXISTS (SELECT 1 FROM pg_constraint WHERE conname = 'account_share_memberships_ended_reason_chk') THEN
        ALTER TABLE account_share_memberships
            ADD CONSTRAINT account_share_memberships_ended_reason_chk
            CHECK (
                ended_reason IS NULL
                OR ended_reason IN ('manual', 'idle_timeout', 'prepay_insufficient')
            );
    END IF;

    CREATE INDEX IF NOT EXISTS idx_account_share_memberships_idle_deadline
        ON account_share_memberships ((COALESCE(last_request_at, joined_at)))
        WHERE status = 'active'
            AND deleted_at IS NULL
            AND idle_timeout_minutes > 0;
END $$;
