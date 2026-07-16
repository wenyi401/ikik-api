ALTER TABLE shop_products
    ADD COLUMN IF NOT EXISTS product_type VARCHAR(30) NOT NULL DEFAULT 'card_key',
    ADD COLUMN IF NOT EXISTS balance_only BOOLEAN NOT NULL DEFAULT FALSE,
    ADD COLUMN IF NOT EXISTS draw_enabled BOOLEAN NOT NULL DEFAULT FALSE,
    ADD COLUMN IF NOT EXISTS draw_min_amount DECIMAL(20,2) NOT NULL DEFAULT 0,
    ADD COLUMN IF NOT EXISTS draw_max_amount DECIMAL(20,2) NOT NULL DEFAULT 0,
    ADD COLUMN IF NOT EXISTS draw_guarantee_count INTEGER NOT NULL DEFAULT 0,
    ADD COLUMN IF NOT EXISTS draw_return_rate DECIMAL(10,4) NOT NULL DEFAULT 1;

CREATE TABLE IF NOT EXISTS shop_draw_cycles (
    id BIGSERIAL PRIMARY KEY,
    user_id BIGINT NOT NULL REFERENCES users(id) ON DELETE RESTRICT,
    product_id BIGINT NOT NULL REFERENCES shop_products(id) ON DELETE RESTRICT,
    cycle_no INTEGER NOT NULL,
    guarantee_count INTEGER NOT NULL,
    target_amount DECIMAL(20,2) NOT NULL,
    remaining_amounts JSONB NOT NULL DEFAULT '[]'::jsonb,
    drawn_count INTEGER NOT NULL DEFAULT 0,
    drawn_amount DECIMAL(20,2) NOT NULL DEFAULT 0,
    completed BOOLEAN NOT NULL DEFAULT FALSE,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    CONSTRAINT shop_draw_cycles_count_positive CHECK (guarantee_count > 0),
    CONSTRAINT shop_draw_cycles_amount_nonnegative CHECK (target_amount >= 0 AND drawn_amount >= 0),
    CONSTRAINT shop_draw_cycles_drawn_range CHECK (drawn_count >= 0 AND drawn_count <= guarantee_count)
);

ALTER TABLE shop_orders
    ADD COLUMN IF NOT EXISTS draw_reward_amount DECIMAL(20,2),
    ADD COLUMN IF NOT EXISTS draw_cycle_id BIGINT REFERENCES shop_draw_cycles(id) ON DELETE SET NULL,
    ADD COLUMN IF NOT EXISTS draw_cycle_index INTEGER;

ALTER TABLE shop_products
    DROP CONSTRAINT IF EXISTS shop_products_product_type_valid,
    ADD CONSTRAINT shop_products_product_type_valid CHECK (product_type IN ('card_key', 'balance_draw'));

ALTER TABLE shop_products
    DROP CONSTRAINT IF EXISTS shop_products_draw_config_valid,
    ADD CONSTRAINT shop_products_draw_config_valid CHECK (
        (
            product_type = 'card_key'
            AND draw_enabled = FALSE
        )
        OR
        (
            product_type = 'balance_draw'
            AND balance_only = TRUE
            AND auto_delivery = TRUE
            AND min_purchase = 1
            AND max_purchase = 1
            AND draw_enabled = TRUE
            AND draw_min_amount > 0
            AND draw_max_amount >= draw_min_amount
            AND draw_guarantee_count > 0
            AND draw_return_rate > 0
            AND ROUND(price * draw_guarantee_count * draw_return_rate * 100) >= ROUND(draw_min_amount * 100) * draw_guarantee_count
            AND ROUND(price * draw_guarantee_count * draw_return_rate * 100) <= ROUND(draw_max_amount * 100) * draw_guarantee_count
        )
    );

ALTER TABLE shop_orders
    DROP CONSTRAINT IF EXISTS shop_orders_draw_reward_nonnegative,
    ADD CONSTRAINT shop_orders_draw_reward_nonnegative CHECK (draw_reward_amount IS NULL OR draw_reward_amount > 0);

CREATE INDEX IF NOT EXISTS idx_shop_products_product_type ON shop_products(product_type);
CREATE INDEX IF NOT EXISTS idx_shop_draw_cycles_user_product_completed ON shop_draw_cycles(user_id, product_id, completed);
CREATE UNIQUE INDEX IF NOT EXISTS idx_shop_draw_cycles_user_product_cycle_no ON shop_draw_cycles(user_id, product_id, cycle_no);
CREATE INDEX IF NOT EXISTS idx_shop_orders_draw_cycle_id ON shop_orders(draw_cycle_id);

CREATE TABLE IF NOT EXISTS shop_balance_ledger (
    id BIGSERIAL PRIMARY KEY,
    user_id BIGINT NOT NULL REFERENCES users(id) ON DELETE RESTRICT,
    shop_order_id BIGINT NOT NULL REFERENCES shop_orders(id) ON DELETE RESTRICT,
    entry_type VARCHAR(30) NOT NULL,
    debit_amount DECIMAL(20,2) NOT NULL DEFAULT 0,
    credit_amount DECIMAL(20,2) NOT NULL DEFAULT 0,
    balance_before DECIMAL(20,8) NOT NULL,
    balance_after DECIMAL(20,8) NOT NULL,
    draw_cycle_id BIGINT REFERENCES shop_draw_cycles(id) ON DELETE SET NULL,
    draw_cycle_index INTEGER,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    CONSTRAINT shop_balance_ledger_entry_type_valid CHECK (entry_type = 'net'),
    CONSTRAINT shop_balance_ledger_amount_nonnegative CHECK (debit_amount >= 0 AND credit_amount >= 0),
    CONSTRAINT shop_balance_ledger_net_amounts CHECK (
        entry_type = 'net'
        AND debit_amount > 0
        AND (
            (draw_cycle_id IS NULL AND draw_cycle_index IS NULL AND credit_amount = 0)
            OR (draw_cycle_id IS NOT NULL AND draw_cycle_index > 0 AND credit_amount > 0)
        )
    ),
    CONSTRAINT shop_balance_ledger_balance_math CHECK (
        ROUND(balance_after * 100) = ROUND((balance_before - debit_amount + credit_amount) * 100)
    )
);

CREATE INDEX IF NOT EXISTS idx_shop_balance_ledger_user_time ON shop_balance_ledger(user_id, created_at DESC);
CREATE UNIQUE INDEX IF NOT EXISTS idx_shop_balance_ledger_order_entry ON shop_balance_ledger(shop_order_id, entry_type);
CREATE INDEX IF NOT EXISTS idx_shop_balance_ledger_draw_cycle ON shop_balance_ledger(draw_cycle_id);
