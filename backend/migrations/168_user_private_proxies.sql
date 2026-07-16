ALTER TABLE proxies
    ADD COLUMN IF NOT EXISTS owner_user_id BIGINT REFERENCES users(id) ON DELETE CASCADE;

CREATE INDEX IF NOT EXISTS idx_proxies_owner_user_id
    ON proxies(owner_user_id)
    WHERE deleted_at IS NULL AND owner_user_id IS NOT NULL;

CREATE INDEX IF NOT EXISTS idx_proxies_owner_status
    ON proxies(owner_user_id, status)
    WHERE deleted_at IS NULL AND owner_user_id IS NOT NULL;

COMMENT ON COLUMN proxies.owner_user_id IS 'Owner user for user-private proxies. NULL means admin/global proxy.';
