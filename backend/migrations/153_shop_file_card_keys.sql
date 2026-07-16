ALTER TABLE shop_card_keys
    ADD COLUMN IF NOT EXISTS card_type VARCHAR(20) NOT NULL DEFAULT 'text',
    ADD COLUMN IF NOT EXISTS storage_provider VARCHAR(20),
    ADD COLUMN IF NOT EXISTS storage_key TEXT,
    ADD COLUMN IF NOT EXISTS original_filename VARCHAR(255),
    ADD COLUMN IF NOT EXISTS content_type VARCHAR(120),
    ADD COLUMN IF NOT EXISTS byte_size INTEGER,
    ADD COLUMN IF NOT EXISTS sha256 VARCHAR(64);

DO $$
BEGIN
    IF NOT EXISTS (
        SELECT 1 FROM pg_constraint
        WHERE conname = 'shop_card_keys_card_type_valid'
    ) THEN
        ALTER TABLE shop_card_keys
            ADD CONSTRAINT shop_card_keys_card_type_valid
            CHECK (card_type IN ('text', 'file'));
    END IF;

    IF NOT EXISTS (
        SELECT 1 FROM pg_constraint
        WHERE conname = 'shop_card_keys_file_storage_required'
    ) THEN
        ALTER TABLE shop_card_keys
            ADD CONSTRAINT shop_card_keys_file_storage_required
            CHECK (
                card_type <> 'file'
                OR (
                    storage_provider IS NOT NULL
                    AND storage_key IS NOT NULL
                    AND original_filename IS NOT NULL
                    AND content_type IS NOT NULL
                    AND byte_size IS NOT NULL
                    AND byte_size > 0
                    AND byte_size <= 204800
                    AND sha256 IS NOT NULL
                    AND length(sha256) = 64
                )
            );
    END IF;
END $$;

CREATE INDEX IF NOT EXISTS idx_shop_card_keys_product_type_status
    ON shop_card_keys(product_id, card_type, status);

