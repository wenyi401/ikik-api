-- 风控中心账号模式作用域日志字段

ALTER TABLE content_moderation_logs ADD COLUMN IF NOT EXISTS scope_type VARCHAR(32) NOT NULL DEFAULT 'group';
ALTER TABLE content_moderation_logs ADD COLUMN IF NOT EXISTS account_share_listing_id BIGINT;
ALTER TABLE content_moderation_logs ADD COLUMN IF NOT EXISTS account_id BIGINT REFERENCES accounts(id) ON DELETE SET NULL;
ALTER TABLE content_moderation_logs ADD COLUMN IF NOT EXISTS owner_user_id BIGINT REFERENCES users(id) ON DELETE SET NULL;
ALTER TABLE content_moderation_logs ADD COLUMN IF NOT EXISTS consumer_user_id BIGINT REFERENCES users(id) ON DELETE SET NULL;
ALTER TABLE content_moderation_logs ADD COLUMN IF NOT EXISTS membership_id BIGINT;

DO $$
BEGIN
    IF to_regclass('public.account_share_listings') IS NOT NULL
        AND NOT EXISTS (
            SELECT 1
            FROM pg_constraint
            WHERE conname = 'content_moderation_logs_account_share_listing_id_fkey'
        )
    THEN
        ALTER TABLE content_moderation_logs
            ADD CONSTRAINT content_moderation_logs_account_share_listing_id_fkey
            FOREIGN KEY (account_share_listing_id)
            REFERENCES account_share_listings(id)
            ON DELETE SET NULL;
    END IF;

    IF to_regclass('public.account_share_memberships') IS NOT NULL
        AND NOT EXISTS (
            SELECT 1
            FROM pg_constraint
            WHERE conname = 'content_moderation_logs_membership_id_fkey'
        )
    THEN
        ALTER TABLE content_moderation_logs
            ADD CONSTRAINT content_moderation_logs_membership_id_fkey
            FOREIGN KEY (membership_id)
            REFERENCES account_share_memberships(id)
            ON DELETE SET NULL;
    END IF;
END $$;

CREATE INDEX IF NOT EXISTS idx_content_moderation_logs_scope_created_at ON content_moderation_logs(scope_type, created_at DESC);
CREATE INDEX IF NOT EXISTS idx_content_moderation_logs_account_share_listing_created_at ON content_moderation_logs(account_share_listing_id, created_at DESC);
CREATE INDEX IF NOT EXISTS idx_content_moderation_logs_consumer_created_at ON content_moderation_logs(consumer_user_id, created_at DESC);
CREATE INDEX IF NOT EXISTS idx_content_moderation_logs_owner_created_at ON content_moderation_logs(owner_user_id, created_at DESC);
