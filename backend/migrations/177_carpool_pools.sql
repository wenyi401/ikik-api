CREATE TABLE IF NOT EXISTS carpool_pools (
    id BIGSERIAL PRIMARY KEY,
    owner_user_id BIGINT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    group_id BIGINT UNIQUE REFERENCES groups(id) ON DELETE SET NULL,
    invite_code VARCHAR(32) NOT NULL UNIQUE,
    name VARCHAR(160) NOT NULL,
    platform VARCHAR(32) NOT NULL,
    status VARCHAR(24) NOT NULL DEFAULT 'recruiting',
    visibility VARCHAR(24) NOT NULL DEFAULT 'public',
    target_seats INTEGER NOT NULL,
    duration_days INTEGER NOT NULL DEFAULT 30,
    seat_price NUMERIC(12, 2) NOT NULL DEFAULT 0,
    extra_fee NUMERIC(12, 2) NOT NULL DEFAULT 0,
    extra_fee_description TEXT NOT NULL DEFAULT '',
    notes TEXT NOT NULL DEFAULT '',
    total_five_hour_limit_usd NUMERIC(18, 6) NOT NULL DEFAULT 0,
    total_weekly_limit_usd NUMERIC(18, 6) NOT NULL DEFAULT 0,
    per_member_five_hour_limit_usd NUMERIC(18, 6) NOT NULL DEFAULT 0,
    per_member_weekly_limit_usd NUMERIC(18, 6) NOT NULL DEFAULT 0,
    quota_snapshot_at TIMESTAMPTZ,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    deleted_at TIMESTAMPTZ,
    CONSTRAINT carpool_pools_status_check CHECK (status IN ('recruiting', 'full', 'closed')),
    CONSTRAINT carpool_pools_visibility_check CHECK (visibility IN ('public', 'invite_only')),
    CONSTRAINT carpool_pools_target_seats_check CHECK (target_seats BETWEEN 2 AND 6),
    CONSTRAINT carpool_pools_duration_days_check CHECK (duration_days BETWEEN 1 AND 365)
);

CREATE INDEX IF NOT EXISTS idx_carpool_pools_owner_user_id ON carpool_pools(owner_user_id) WHERE deleted_at IS NULL;
CREATE INDEX IF NOT EXISTS idx_carpool_pools_public_hall ON carpool_pools(status, visibility, platform) WHERE deleted_at IS NULL;

CREATE TABLE IF NOT EXISTS carpool_pool_accounts (
    id BIGSERIAL PRIMARY KEY,
    pool_id BIGINT NOT NULL REFERENCES carpool_pools(id) ON DELETE CASCADE,
    account_id BIGINT NOT NULL REFERENCES accounts(id) ON DELETE CASCADE,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    UNIQUE (pool_id, account_id)
);

CREATE INDEX IF NOT EXISTS idx_carpool_pool_accounts_pool_id ON carpool_pool_accounts(pool_id);
CREATE INDEX IF NOT EXISTS idx_carpool_pool_accounts_account_id ON carpool_pool_accounts(account_id);

CREATE TABLE IF NOT EXISTS carpool_join_requests (
    id BIGSERIAL PRIMARY KEY,
    pool_id BIGINT NOT NULL REFERENCES carpool_pools(id) ON DELETE CASCADE,
    user_id BIGINT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    status VARCHAR(24) NOT NULL DEFAULT 'pending',
    note TEXT NOT NULL DEFAULT '',
    review_note TEXT NOT NULL DEFAULT '',
    reviewed_at TIMESTAMPTZ,
    activated_at TIMESTAMPTZ,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    deleted_at TIMESTAMPTZ,
    CONSTRAINT carpool_join_requests_status_check CHECK (status IN ('pending', 'approved', 'rejected', 'activated'))
);

CREATE UNIQUE INDEX IF NOT EXISTS uniq_carpool_join_requests_pool_user_active
    ON carpool_join_requests(pool_id, user_id)
    WHERE deleted_at IS NULL AND status IN ('pending', 'approved');
CREATE INDEX IF NOT EXISTS idx_carpool_join_requests_pool_id ON carpool_join_requests(pool_id) WHERE deleted_at IS NULL;
CREATE INDEX IF NOT EXISTS idx_carpool_join_requests_user_id ON carpool_join_requests(user_id) WHERE deleted_at IS NULL;

CREATE TABLE IF NOT EXISTS carpool_members (
    id BIGSERIAL PRIMARY KEY,
    pool_id BIGINT NOT NULL REFERENCES carpool_pools(id) ON DELETE CASCADE,
    user_id BIGINT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    subscription_id BIGINT REFERENCES user_subscriptions(id) ON DELETE SET NULL,
    role VARCHAR(24) NOT NULL DEFAULT 'member',
    status VARCHAR(24) NOT NULL DEFAULT 'active',
    paid_confirmed_at TIMESTAMPTZ,
    five_hour_limit_usd NUMERIC(18, 6) NOT NULL DEFAULT 0,
    five_hour_used_usd NUMERIC(18, 6) NOT NULL DEFAULT 0,
    five_hour_window_start TIMESTAMPTZ,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    deleted_at TIMESTAMPTZ,
    CONSTRAINT carpool_members_role_check CHECK (role IN ('owner', 'member')),
    CONSTRAINT carpool_members_status_check CHECK (status IN ('active', 'removed')),
    CONSTRAINT carpool_members_pool_user_unique UNIQUE (pool_id, user_id)
);

CREATE INDEX IF NOT EXISTS idx_carpool_members_pool_id ON carpool_members(pool_id) WHERE deleted_at IS NULL;
CREATE INDEX IF NOT EXISTS idx_carpool_members_user_id ON carpool_members(user_id) WHERE deleted_at IS NULL;
