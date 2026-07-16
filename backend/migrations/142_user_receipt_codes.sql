CREATE TABLE IF NOT EXISTS user_receipt_codes (
    id BIGSERIAL PRIMARY KEY,
    user_id BIGINT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    payment_method VARCHAR(20) NOT NULL,
    storage_provider VARCHAR(20) NOT NULL DEFAULT 'oss',
    storage_key TEXT NOT NULL,
    url TEXT NOT NULL DEFAULT '',
    content_type VARCHAR(100) NOT NULL,
    byte_size INTEGER NOT NULL,
    sha256 VARCHAR(64) NOT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE UNIQUE INDEX IF NOT EXISTS user_receipt_codes_user_id_method_key
    ON user_receipt_codes (user_id, payment_method);

CREATE INDEX IF NOT EXISTS idx_user_receipt_codes_user_id
    ON user_receipt_codes (user_id);

