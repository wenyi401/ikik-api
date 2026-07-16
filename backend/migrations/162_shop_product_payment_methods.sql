ALTER TABLE shop_products
    ADD COLUMN IF NOT EXISTS allow_balance_payment BOOLEAN NOT NULL DEFAULT TRUE,
    ADD COLUMN IF NOT EXISTS allow_platform_payment BOOLEAN NOT NULL DEFAULT TRUE;

UPDATE shop_products
SET allow_platform_payment = FALSE
WHERE balance_only = TRUE;

UPDATE shop_products
SET allow_balance_payment = TRUE
WHERE product_type IN ('balance_draw', 'points_draw');

ALTER TABLE shop_products
    DROP CONSTRAINT IF EXISTS shop_products_payment_method_valid,
    ADD CONSTRAINT shop_products_payment_method_valid CHECK (
        allow_balance_payment = TRUE
        OR allow_points_payment = TRUE
        OR allow_platform_payment = TRUE
    );

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
            AND allow_balance_payment = TRUE
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
