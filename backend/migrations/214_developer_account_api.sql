ALTER TABLE users
    ADD COLUMN IF NOT EXISTS developer_api_enabled BOOLEAN NOT NULL DEFAULT FALSE;

CREATE TABLE IF NOT EXISTS developer_tokens (
    id BIGSERIAL PRIMARY KEY,
    user_id BIGINT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    name VARCHAR(100) NOT NULL,
    token_prefix VARCHAR(20) NOT NULL,
    token_hash VARCHAR(64) NOT NULL,
    scopes JSONB NOT NULL DEFAULT '[]'::jsonb,
    status VARCHAR(20) NOT NULL DEFAULT 'active',
    expires_at TIMESTAMPTZ,
    last_used_at TIMESTAMPTZ,
    last_used_ip VARCHAR(64),
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    deleted_at TIMESTAMPTZ
);

CREATE UNIQUE INDEX IF NOT EXISTS developer_tokens_token_hash_key
    ON developer_tokens (token_hash);

CREATE INDEX IF NOT EXISTS developer_tokens_user_id_created_at_idx
    ON developer_tokens (user_id, created_at DESC)
    WHERE deleted_at IS NULL;

CREATE INDEX IF NOT EXISTS developer_tokens_status_expires_at_idx
    ON developer_tokens (status, expires_at)
    WHERE deleted_at IS NULL;

COMMENT ON COLUMN users.developer_api_enabled IS
    'Whether the user may create and authenticate with scoped developer API tokens';

COMMENT ON COLUMN developer_tokens.token_hash IS
    'SHA-256 hash of the high-entropy developer token; plaintext is shown once and never persisted';
