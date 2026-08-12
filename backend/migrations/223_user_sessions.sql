-- Browser login sessions. Refresh secrets are never stored in plaintext.

CREATE TABLE IF NOT EXISTS user_sessions (
    id BIGSERIAL PRIMARY KEY,
    sid VARCHAR(36) NOT NULL UNIQUE,
    user_id BIGINT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    version BIGINT NOT NULL DEFAULT 1,
    user_auth_version BIGINT NOT NULL,
    status VARCHAR(16) NOT NULL DEFAULT 'active',
    refresh_hash VARCHAR(64) NOT NULL,
    previous_refresh_hash VARCHAR(64) NOT NULL DEFAULT '',
    previous_valid_until TIMESTAMPTZ,
    login_method VARCHAR(32) NOT NULL DEFAULT 'unknown',
    ip VARCHAR(64) NOT NULL DEFAULT '',
    user_agent VARCHAR(512) NOT NULL DEFAULT '',
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    last_active_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    expires_at TIMESTAMPTZ NOT NULL,
    revoked_at TIMESTAMPTZ,
    revoked_reason VARCHAR(64) NOT NULL DEFAULT '',
    CONSTRAINT user_sessions_status_check CHECK (status IN ('active', 'revoking', 'revoked'))
);

CREATE INDEX IF NOT EXISTS idx_user_sessions_user_active
    ON user_sessions(user_id, last_active_at DESC)
    WHERE status = 'active';

CREATE INDEX IF NOT EXISTS idx_user_sessions_created_at
    ON user_sessions(user_id, created_at DESC);

CREATE INDEX IF NOT EXISTS idx_user_sessions_expiry
    ON user_sessions(expires_at);
