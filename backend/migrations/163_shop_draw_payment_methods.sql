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
