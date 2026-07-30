-- Explicit per-user group denylist. A blocked group takes precedence over
-- public visibility, exclusive-group grants, and active subscriptions.

CREATE TABLE IF NOT EXISTS user_blocked_groups (
    user_id     BIGINT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    group_id    BIGINT NOT NULL REFERENCES groups(id) ON DELETE CASCADE,
    created_at  TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    PRIMARY KEY (user_id, group_id)
);

CREATE INDEX IF NOT EXISTS idx_user_blocked_groups_group_id
    ON user_blocked_groups(group_id);

CREATE OR REPLACE FUNCTION enqueue_blocked_group_auth_cache_invalidation()
RETURNS TRIGGER
LANGUAGE plpgsql
AS $$
DECLARE
    old_user_id BIGINT;
    new_user_id BIGINT;
BEGIN
    IF TG_OP <> 'INSERT' THEN
        old_user_id := OLD.user_id;
    END IF;
    IF TG_OP <> 'DELETE' THEN
        new_user_id := NEW.user_id;
    END IF;

    INSERT INTO auth_cache_invalidation_outbox (cache_key)
    SELECT DISTINCT encode(sha256(convert_to(k.key, 'UTF8')), 'hex')
    FROM api_keys AS k
    WHERE k.deleted_at IS NULL
      AND k.key <> ''
      AND k.user_id IN (old_user_id, new_user_id);

    IF TG_OP = 'DELETE' THEN
        RETURN OLD;
    END IF;
    RETURN NEW;
END;
$$;

DROP TRIGGER IF EXISTS trg_user_blocked_groups_auth_cache_invalidation
    ON user_blocked_groups;
CREATE TRIGGER trg_user_blocked_groups_auth_cache_invalidation
AFTER INSERT OR UPDATE OR DELETE ON user_blocked_groups
FOR EACH ROW EXECUTE FUNCTION enqueue_blocked_group_auth_cache_invalidation();

COMMENT ON TABLE user_blocked_groups IS
    'Groups explicitly denied to a user; deny entries override grants and subscriptions';
