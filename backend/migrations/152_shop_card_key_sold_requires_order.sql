DO $$
BEGIN
    IF NOT EXISTS (
        SELECT 1
        FROM pg_constraint
        WHERE conname = 'shop_card_keys_sold_requires_order'
    ) THEN
        ALTER TABLE shop_card_keys
            ADD CONSTRAINT shop_card_keys_sold_requires_order
            CHECK (NOT (status = 'sold' AND (order_id IS NULL OR sold_at IS NULL)));
    END IF;
END $$;
