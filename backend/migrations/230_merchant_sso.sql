CREATE TABLE IF NOT EXISTS merchant_sso_integrations (
    id BIGSERIAL PRIMARY KEY,
    merchant_code VARCHAR(128) NOT NULL UNIQUE,
    merchant_name VARCHAR(255) NOT NULL DEFAULT '',
    enabled BOOLEAN NOT NULL DEFAULT FALSE,
    register_login_url TEXT NOT NULL,
    login_url TEXT NOT NULL,
    user_sync_url TEXT NOT NULL,
    user_sync_auth_type VARCHAR(16) NOT NULL DEFAULT 'none',
    user_sync_hmac_secret TEXT NOT NULL DEFAULT '',
    allowed_redirect_hosts JSONB NOT NULL DEFAULT '[]'::jsonb,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    CONSTRAINT merchant_sso_integrations_auth_type_check
        CHECK (user_sync_auth_type IN ('none', 'hmac'))
);

CREATE TABLE IF NOT EXISTS merchant_sso_bindings (
    id BIGSERIAL PRIMARY KEY,
    integration_id BIGINT NOT NULL REFERENCES merchant_sso_integrations(id) ON DELETE CASCADE,
    user_id BIGINT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    external_user_id VARCHAR(255) NOT NULL,
    external_account VARCHAR(255) NOT NULL DEFAULT '',
    email VARCHAR(320) NOT NULL DEFAULT '',
    status VARCHAR(64) NOT NULL DEFAULT '',
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    UNIQUE (integration_id, user_id),
    UNIQUE (integration_id, external_user_id)
);

CREATE INDEX IF NOT EXISTS merchant_sso_bindings_email_idx
    ON merchant_sso_bindings (integration_id, lower(email));
