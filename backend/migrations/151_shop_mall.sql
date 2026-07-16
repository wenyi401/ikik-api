CREATE TABLE IF NOT EXISTS shop_categories (
    id BIGSERIAL PRIMARY KEY,
    name VARCHAR(100) NOT NULL,
    icon VARCHAR(255),
    sort_order INTEGER NOT NULL DEFAULT 0,
    enabled BOOLEAN NOT NULL DEFAULT TRUE,
    description TEXT,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE TABLE IF NOT EXISTS shop_products (
    id BIGSERIAL PRIMARY KEY,
    category_id BIGINT REFERENCES shop_categories(id) ON DELETE SET NULL,
    name VARCHAR(150) NOT NULL,
    cover_url TEXT,
    description TEXT,
    price DECIMAL(20,2) NOT NULL DEFAULT 0,
    original_price DECIMAL(20,2),
    enabled BOOLEAN NOT NULL DEFAULT TRUE,
    sort_order INTEGER NOT NULL DEFAULT 0,
    min_purchase INTEGER NOT NULL DEFAULT 1,
    max_purchase INTEGER NOT NULL DEFAULT 1,
    auto_delivery BOOLEAN NOT NULL DEFAULT TRUE,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    CONSTRAINT shop_products_price_nonnegative CHECK (price >= 0 AND (original_price IS NULL OR original_price >= 0)),
    CONSTRAINT shop_products_purchase_range CHECK (min_purchase > 0 AND max_purchase >= min_purchase)
);

CREATE TABLE IF NOT EXISTS shop_orders (
    id BIGSERIAL PRIMARY KEY,
    order_no VARCHAR(64) NOT NULL UNIQUE,
    user_id BIGINT NOT NULL REFERENCES users(id) ON DELETE RESTRICT,
    product_id BIGINT NOT NULL REFERENCES shop_products(id) ON DELETE RESTRICT,
    product_name VARCHAR(150) NOT NULL,
    product_cover_url TEXT,
    product_description TEXT,
    unit_price DECIMAL(20,2) NOT NULL,
    quantity INTEGER NOT NULL,
    total_amount DECIMAL(20,2) NOT NULL,
    payment_method VARCHAR(30) NOT NULL,
    payment_order_id BIGINT REFERENCES payment_orders(id) ON DELETE SET NULL,
    status VARCHAR(30) NOT NULL DEFAULT 'pending',
    delivered_cards JSONB NOT NULL DEFAULT '[]'::jsonb,
    paid_at TIMESTAMPTZ,
    completed_at TIMESTAMPTZ,
    cancelled_at TIMESTAMPTZ,
    failed_reason TEXT,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    CONSTRAINT shop_orders_quantity_positive CHECK (quantity > 0),
    CONSTRAINT shop_orders_amount_nonnegative CHECK (unit_price >= 0 AND total_amount >= 0)
);

CREATE TABLE IF NOT EXISTS shop_card_keys (
    id BIGSERIAL PRIMARY KEY,
    product_id BIGINT NOT NULL REFERENCES shop_products(id) ON DELETE CASCADE,
    content TEXT NOT NULL,
    status VARCHAR(20) NOT NULL DEFAULT 'available',
    order_id BIGINT REFERENCES shop_orders(id) ON DELETE SET NULL,
    locked_at TIMESTAMPTZ,
    locked_until TIMESTAMPTZ,
    sold_at TIMESTAMPTZ,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    CONSTRAINT shop_card_keys_status_valid CHECK (status IN ('available', 'locked', 'sold', 'disabled'))
);

ALTER TABLE payment_orders
    ADD COLUMN IF NOT EXISTS shop_order_id BIGINT REFERENCES shop_orders(id) ON DELETE SET NULL;

ALTER TABLE shop_products
    ADD COLUMN IF NOT EXISTS original_price DECIMAL(20,2);

ALTER TABLE shop_card_keys
    ADD COLUMN IF NOT EXISTS locked_at TIMESTAMPTZ,
    ADD COLUMN IF NOT EXISTS locked_until TIMESTAMPTZ;

CREATE INDEX IF NOT EXISTS idx_shop_categories_enabled ON shop_categories(enabled);
CREATE INDEX IF NOT EXISTS idx_shop_categories_sort_order ON shop_categories(sort_order);

CREATE INDEX IF NOT EXISTS idx_shop_products_category_id ON shop_products(category_id);
CREATE INDEX IF NOT EXISTS idx_shop_products_enabled ON shop_products(enabled);
CREATE INDEX IF NOT EXISTS idx_shop_products_sort_order ON shop_products(sort_order);

CREATE INDEX IF NOT EXISTS idx_shop_card_keys_product_status ON shop_card_keys(product_id, status);
CREATE INDEX IF NOT EXISTS idx_shop_card_keys_order_id ON shop_card_keys(order_id);
CREATE INDEX IF NOT EXISTS idx_shop_card_keys_status ON shop_card_keys(status);
CREATE INDEX IF NOT EXISTS idx_shop_card_keys_locked_until ON shop_card_keys(locked_until);

CREATE INDEX IF NOT EXISTS idx_shop_orders_user_id ON shop_orders(user_id);
CREATE INDEX IF NOT EXISTS idx_shop_orders_product_id ON shop_orders(product_id);
CREATE UNIQUE INDEX IF NOT EXISTS idx_shop_orders_payment_order_id ON shop_orders(payment_order_id) WHERE payment_order_id IS NOT NULL;
CREATE INDEX IF NOT EXISTS idx_shop_orders_status ON shop_orders(status);
CREATE INDEX IF NOT EXISTS idx_shop_orders_created_at ON shop_orders(created_at);

CREATE UNIQUE INDEX IF NOT EXISTS idx_payment_orders_shop_order_id ON payment_orders(shop_order_id) WHERE shop_order_id IS NOT NULL;
