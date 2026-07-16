-- Account ownership, public sharing policies, and immutable settlement ledgers.

CREATE TABLE IF NOT EXISTS account_share_policies (
    id BIGSERIAL PRIMARY KEY,
    scope_type VARCHAR(20) NOT NULL DEFAULT 'global',
    scope_id BIGINT,
    platform VARCHAR(50),
    owner_share_ratio DECIMAL(10, 6) NOT NULL DEFAULT 0,
    version INTEGER NOT NULL DEFAULT 1,
    enabled BOOLEAN NOT NULL DEFAULT TRUE,
    effective_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    created_by_admin_id BIGINT REFERENCES users(id) ON DELETE SET NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    deleted_at TIMESTAMPTZ,
    CONSTRAINT account_share_policies_scope_type_check
        CHECK (scope_type IN ('global', 'platform', 'group', 'account')),
    CONSTRAINT account_share_policies_ratio_check
        CHECK (owner_share_ratio >= 0 AND owner_share_ratio <= 1)
);

CREATE INDEX IF NOT EXISTS idx_account_share_policies_lookup
    ON account_share_policies (enabled, scope_type, scope_id, platform, effective_at DESC)
    WHERE deleted_at IS NULL;

CREATE INDEX IF NOT EXISTS idx_account_share_policies_created_by_admin
    ON account_share_policies (created_by_admin_id, created_at DESC)
    WHERE deleted_at IS NULL;

ALTER TABLE accounts
    ADD COLUMN IF NOT EXISTS owner_user_id BIGINT REFERENCES users(id) ON DELETE SET NULL,
    ADD COLUMN IF NOT EXISTS share_mode VARCHAR(20) NOT NULL DEFAULT 'private',
    ADD COLUMN IF NOT EXISTS share_status VARCHAR(20) NOT NULL DEFAULT 'approved',
    ADD COLUMN IF NOT EXISTS share_policy_id BIGINT REFERENCES account_share_policies(id) ON DELETE SET NULL;

DO $$
BEGIN
    IF NOT EXISTS (
        SELECT 1 FROM pg_constraint WHERE conname = 'accounts_share_mode_check'
    ) THEN
        ALTER TABLE accounts
            ADD CONSTRAINT accounts_share_mode_check
            CHECK (share_mode IN ('private', 'public'));
    END IF;

    IF NOT EXISTS (
        SELECT 1 FROM pg_constraint WHERE conname = 'accounts_share_status_check'
    ) THEN
        ALTER TABLE accounts
            ADD CONSTRAINT accounts_share_status_check
            CHECK (share_status IN ('pending', 'approved', 'suspended'));
    END IF;
END $$;

CREATE INDEX IF NOT EXISTS idx_accounts_owner_user_id
    ON accounts (owner_user_id)
    WHERE deleted_at IS NULL;

CREATE INDEX IF NOT EXISTS idx_accounts_public_share
    ON accounts (share_mode, share_status, platform)
    WHERE deleted_at IS NULL AND owner_user_id IS NOT NULL;

CREATE INDEX IF NOT EXISTS idx_accounts_share_policy_id
    ON accounts (share_policy_id)
    WHERE deleted_at IS NULL AND share_policy_id IS NOT NULL;

CREATE TABLE IF NOT EXISTS user_balance_ledger (
    id BIGSERIAL PRIMARY KEY,
    user_id BIGINT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    direction VARCHAR(10) NOT NULL,
    amount DECIMAL(20, 10) NOT NULL,
    reason VARCHAR(50) NOT NULL,
    ref_type VARCHAR(50) NOT NULL,
    ref_id BIGINT,
    balance_after DECIMAL(20, 10) NOT NULL,
    metadata JSONB NOT NULL DEFAULT '{}'::jsonb,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    CONSTRAINT user_balance_ledger_direction_check
        CHECK (direction IN ('debit', 'credit')),
    CONSTRAINT user_balance_ledger_amount_check
        CHECK (amount >= 0)
);

CREATE INDEX IF NOT EXISTS idx_user_balance_ledger_user_time
    ON user_balance_ledger (user_id, created_at DESC);

CREATE INDEX IF NOT EXISTS idx_user_balance_ledger_ref
    ON user_balance_ledger (ref_type, ref_id);

CREATE UNIQUE INDEX IF NOT EXISTS idx_user_balance_ledger_unique_usage_reason
    ON user_balance_ledger (user_id, direction, reason, ref_type, ref_id)
    WHERE ref_id IS NOT NULL;

CREATE TABLE IF NOT EXISTS account_share_settlement_entries (
    id BIGSERIAL PRIMARY KEY,
    usage_log_id BIGINT REFERENCES usage_logs(id) ON DELETE SET NULL,
    request_id VARCHAR(128) NOT NULL,
    api_key_id BIGINT NOT NULL REFERENCES api_keys(id) ON DELETE CASCADE,
    consumer_user_id BIGINT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    owner_user_id BIGINT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    account_id BIGINT NOT NULL REFERENCES accounts(id) ON DELETE CASCADE,
    group_id BIGINT REFERENCES groups(id) ON DELETE SET NULL,
    policy_id BIGINT REFERENCES account_share_policies(id) ON DELETE SET NULL,
    policy_version INTEGER NOT NULL DEFAULT 0,
    share_mode_snapshot VARCHAR(20) NOT NULL,
    share_status_snapshot VARCHAR(20) NOT NULL,
    consumer_charge DECIMAL(20, 10) NOT NULL DEFAULT 0,
    account_cost DECIMAL(20, 10) NOT NULL DEFAULT 0,
    owner_share_ratio DECIMAL(10, 6) NOT NULL DEFAULT 0,
    owner_credit DECIMAL(20, 10) NOT NULL DEFAULT 0,
    platform_fee DECIMAL(20, 10) NOT NULL DEFAULT 0,
    status VARCHAR(20) NOT NULL DEFAULT 'applied',
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    CONSTRAINT account_share_settlement_status_check
        CHECK (status IN ('applied', 'reversed', 'frozen')),
    CONSTRAINT account_share_settlement_amounts_check
        CHECK (
            consumer_charge >= 0
            AND account_cost >= 0
            AND owner_share_ratio >= 0
            AND owner_share_ratio <= 1
            AND owner_credit >= 0
            AND platform_fee >= 0
            AND owner_credit <= consumer_charge
        )
);

CREATE UNIQUE INDEX IF NOT EXISTS idx_account_share_settlement_usage_log_unique
    ON account_share_settlement_entries (usage_log_id)
    WHERE usage_log_id IS NOT NULL;

CREATE UNIQUE INDEX IF NOT EXISTS idx_account_share_settlement_request_unique
    ON account_share_settlement_entries (request_id, api_key_id);

CREATE INDEX IF NOT EXISTS idx_account_share_settlement_owner_time
    ON account_share_settlement_entries (owner_user_id, created_at DESC);

CREATE INDEX IF NOT EXISTS idx_account_share_settlement_consumer_time
    ON account_share_settlement_entries (consumer_user_id, created_at DESC);

CREATE INDEX IF NOT EXISTS idx_account_share_settlement_account_time
    ON account_share_settlement_entries (account_id, created_at DESC);
