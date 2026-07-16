CREATE TABLE IF NOT EXISTS user_withdrawal_requests (
    id BIGSERIAL PRIMARY KEY,
    user_id BIGINT NOT NULL REFERENCES users(id) ON DELETE RESTRICT,
    user_email VARCHAR(255) NOT NULL,
    amount DECIMAL(20,2) NOT NULL,
    fee_amount DECIMAL(20,2) NOT NULL DEFAULT 0,
    total_deducted DECIMAL(20,2) NOT NULL,
    balance_before DECIMAL(20,8) NOT NULL,
    balance_after DECIMAL(20,8) NOT NULL,
    payment_method VARCHAR(20) NOT NULL,
    receipt_code_storage_provider VARCHAR(20) NOT NULL DEFAULT 'oss',
    receipt_code_storage_key TEXT NOT NULL,
    receipt_code_url TEXT NOT NULL DEFAULT '',
    receipt_code_content_type VARCHAR(100) NOT NULL,
    receipt_code_byte_size INTEGER NOT NULL,
    receipt_code_sha256 VARCHAR(64) NOT NULL,
    receipt_code_updated_at TIMESTAMPTZ NOT NULL,
    status VARCHAR(20) NOT NULL DEFAULT 'PENDING',
    user_cancel_reason TEXT,
    admin_note TEXT,
    processed_by_user_id BIGINT REFERENCES users(id) ON DELETE SET NULL,
    processed_at TIMESTAMPTZ,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    CONSTRAINT user_withdrawal_requests_amount_positive CHECK (amount >= 1),
    CONSTRAINT user_withdrawal_requests_fee_non_negative CHECK (fee_amount >= 0),
    CONSTRAINT user_withdrawal_requests_total_consistent CHECK (total_deducted = amount + fee_amount),
    CONSTRAINT user_withdrawal_requests_status_valid CHECK (status IN ('PENDING', 'SETTLED', 'CANCELLED', 'REJECTED')),
    CONSTRAINT user_withdrawal_requests_payment_method_valid CHECK (payment_method IN ('alipay', 'wechat'))
);

CREATE UNIQUE INDEX IF NOT EXISTS user_withdrawal_requests_one_pending_per_user
    ON user_withdrawal_requests (user_id)
    WHERE status = 'PENDING';

CREATE INDEX IF NOT EXISTS idx_user_withdrawal_requests_user_id
    ON user_withdrawal_requests (user_id);

CREATE INDEX IF NOT EXISTS idx_user_withdrawal_requests_status_created_at
    ON user_withdrawal_requests (status, created_at DESC);

CREATE INDEX IF NOT EXISTS idx_user_withdrawal_requests_created_at
    ON user_withdrawal_requests (created_at DESC);

CREATE INDEX IF NOT EXISTS idx_user_withdrawal_requests_receipt_code_storage_key
    ON user_withdrawal_requests (receipt_code_storage_key);
