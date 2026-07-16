ALTER TABLE shop_products
    DROP CONSTRAINT IF EXISTS shop_products_product_type_valid,
    ADD CONSTRAINT shop_products_product_type_valid CHECK (product_type IN ('card_key', 'balance_draw', 'points_draw'));

ALTER TABLE shop_products
    DROP CONSTRAINT IF EXISTS shop_products_draw_config_valid,
    ADD CONSTRAINT shop_products_draw_config_valid CHECK (
        (
            product_type = 'card_key'
            AND draw_enabled = FALSE
        )
        OR
        (
            product_type IN ('balance_draw', 'points_draw')
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
    ADD COLUMN IF NOT EXISTS product_type VARCHAR(30) NOT NULL DEFAULT 'card_key';

UPDATE shop_orders o
SET product_type = p.product_type
FROM shop_products p
WHERE o.product_id = p.id
  AND (o.product_type IS NULL OR o.product_type = 'card_key');

CREATE INDEX IF NOT EXISTS idx_shop_orders_product_type ON shop_orders(product_type);
