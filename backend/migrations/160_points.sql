-- Points are a non-withdrawable platform credit balance.

ALTER TABLE users
    ADD COLUMN IF NOT EXISTS points_balance DECIMAL(20, 10) NOT NULL DEFAULT 0,
    ADD COLUMN IF NOT EXISTS prefer_points_billing BOOLEAN NOT NULL DEFAULT FALSE;

ALTER TABLE shop_products
    ADD COLUMN IF NOT EXISTS allow_points_payment BOOLEAN NOT NULL DEFAULT FALSE;

ALTER TABLE shop_orders
    ADD COLUMN IF NOT EXISTS points_amount DECIMAL(20, 2) NOT NULL DEFAULT 0;

CREATE TABLE IF NOT EXISTS points_ledger (
    id BIGSERIAL PRIMARY KEY,
    user_id BIGINT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    direction VARCHAR(10) NOT NULL,
    amount DECIMAL(20, 10) NOT NULL,
    reason VARCHAR(50) NOT NULL,
    ref_type VARCHAR(50) NOT NULL,
    ref_id BIGINT,
    balance_before DECIMAL(20, 10) NOT NULL,
    balance_after DECIMAL(20, 10) NOT NULL,
    operator_user_id BIGINT REFERENCES users(id) ON DELETE SET NULL,
    metadata JSONB NOT NULL DEFAULT '{}'::jsonb,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    CONSTRAINT points_ledger_direction_check
        CHECK (direction IN ('debit', 'credit')),
    CONSTRAINT points_ledger_amount_check
        CHECK (amount >= 0),
    CONSTRAINT points_ledger_balance_check
        CHECK (balance_before >= 0 AND balance_after >= 0)
);

CREATE INDEX IF NOT EXISTS idx_points_ledger_user_time
    ON points_ledger (user_id, created_at DESC);

CREATE INDEX IF NOT EXISTS idx_points_ledger_ref
    ON points_ledger (ref_type, ref_id);

CREATE INDEX IF NOT EXISTS idx_points_ledger_operator_time
    ON points_ledger (operator_user_id, created_at DESC)
    WHERE operator_user_id IS NOT NULL;

CREATE UNIQUE INDEX IF NOT EXISTS idx_points_ledger_unique_ref_reason
    ON points_ledger (user_id, direction, reason, ref_type, ref_id)
    WHERE ref_id IS NOT NULL;
