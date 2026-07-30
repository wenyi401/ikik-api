ALTER TABLE groups
    ADD COLUMN IF NOT EXISTS is_shared_pool BOOLEAN NOT NULL DEFAULT FALSE;

-- OpenAI shared pools already use required_account_level as their stable marker.
-- Backfill the new display field without changing the existing OpenAI resolver.
UPDATE groups
SET is_shared_pool = TRUE
WHERE deleted_at IS NULL
  AND owner_user_id IS NULL
  AND scope = 'public'
  AND platform = 'openai'
  AND BTRIM(required_account_level) <> '';

CREATE INDEX IF NOT EXISTS idx_groups_shared_pool_platform
    ON groups (platform, sort_order, id)
    WHERE deleted_at IS NULL
      AND owner_user_id IS NULL
      AND scope = 'public'
      AND status = 'active'
      AND is_shared_pool = TRUE;

COMMENT ON COLUMN groups.is_shared_pool IS
    'Whether this public system group accepts user-shared accounts';
