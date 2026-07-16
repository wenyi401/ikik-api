-- Add explicit ownership and scope metadata for per-user private subscription groups.

ALTER TABLE groups
    ADD COLUMN IF NOT EXISTS owner_user_id BIGINT REFERENCES users(id) ON DELETE SET NULL,
    ADD COLUMN IF NOT EXISTS scope VARCHAR(20) NOT NULL DEFAULT 'public';

UPDATE groups
SET scope = 'public'
WHERE scope IS NULL OR scope = '';

DO $$
BEGIN
    IF NOT EXISTS (
        SELECT 1 FROM pg_constraint WHERE conname = 'groups_scope_check'
    ) THEN
        ALTER TABLE groups
            ADD CONSTRAINT groups_scope_check
            CHECK (scope IN ('public', 'user_private'));
    END IF;
END $$;

CREATE INDEX IF NOT EXISTS idx_groups_owner_user_id
    ON groups (owner_user_id)
    WHERE deleted_at IS NULL AND owner_user_id IS NOT NULL;

CREATE INDEX IF NOT EXISTS idx_groups_scope
    ON groups (scope)
    WHERE deleted_at IS NULL;

CREATE UNIQUE INDEX IF NOT EXISTS idx_groups_user_private_owner_platform_unique
    ON groups (owner_user_id, platform)
    WHERE deleted_at IS NULL
        AND owner_user_id IS NOT NULL
        AND scope = 'user_private';
