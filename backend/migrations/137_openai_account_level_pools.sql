ALTER TABLE accounts
    ADD COLUMN IF NOT EXISTS account_level VARCHAR(20) NOT NULL DEFAULT 'unknown';

ALTER TABLE groups
    ADD COLUMN IF NOT EXISTS required_account_level VARCHAR(20) NOT NULL DEFAULT '';

DO $$
BEGIN
    IF NOT EXISTS (
        SELECT 1 FROM pg_constraint WHERE conname = 'accounts_account_level_check'
    ) THEN
        ALTER TABLE accounts
            ADD CONSTRAINT accounts_account_level_check
            CHECK (account_level IN ('unknown', 'free', 'plus', 'pro'));
    END IF;

    IF NOT EXISTS (
        SELECT 1 FROM pg_constraint WHERE conname = 'groups_required_account_level_check'
    ) THEN
        ALTER TABLE groups
            ADD CONSTRAINT groups_required_account_level_check
            CHECK (required_account_level IN ('', 'free', 'plus', 'pro'));
    END IF;
END $$;

CREATE INDEX IF NOT EXISTS idx_accounts_account_level
    ON accounts (account_level);

CREATE INDEX IF NOT EXISTS idx_accounts_platform_account_level
    ON accounts (platform, account_level);

CREATE INDEX IF NOT EXISTS idx_groups_required_account_level
    ON groups (required_account_level);
