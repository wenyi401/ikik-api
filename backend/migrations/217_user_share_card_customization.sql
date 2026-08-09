ALTER TABLE users
    ADD COLUMN IF NOT EXISTS share_card_text varchar(80) NOT NULL DEFAULT '',
    ADD COLUMN IF NOT EXISTS share_card_text_color varchar(7) NOT NULL DEFAULT '#08775c';

ALTER TABLE users
    DROP CONSTRAINT IF EXISTS users_share_card_text_color_check;

ALTER TABLE users
    ADD CONSTRAINT users_share_card_text_color_check
    CHECK (share_card_text_color ~ '^#[0-9a-fA-F]{6}$');
