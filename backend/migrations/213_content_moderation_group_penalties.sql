-- Progressive per-user, per-group penalties applied by content moderation.
-- Manual group deny entries remain isolated in user_blocked_groups.

CREATE TABLE IF NOT EXISTS content_moderation_user_group_penalties (
    user_id          BIGINT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    group_id         BIGINT NOT NULL REFERENCES groups(id) ON DELETE CASCADE,
    strike_count     INT NOT NULL DEFAULT 1,
    blocked_until    TIMESTAMPTZ,
    permanent        BOOLEAN NOT NULL DEFAULT FALSE,
    last_category    VARCHAR(128) NOT NULL DEFAULT '',
    last_request_id  VARCHAR(160) NOT NULL DEFAULT '',
    last_score       NUMERIC(7, 6) NOT NULL DEFAULT 0,
    created_at       TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at       TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    PRIMARY KEY (user_id, group_id),
    CONSTRAINT content_moderation_group_penalty_strike_check
        CHECK (strike_count >= 1 AND strike_count <= 3),
    CONSTRAINT content_moderation_group_penalty_window_check
        CHECK ((permanent = TRUE AND blocked_until IS NULL) OR
               (permanent = FALSE AND blocked_until IS NOT NULL))
);

CREATE TABLE IF NOT EXISTS content_moderation_user_group_penalty_events (
    id          BIGSERIAL PRIMARY KEY,
    user_id     BIGINT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    group_id    BIGINT NOT NULL REFERENCES groups(id) ON DELETE CASCADE,
    request_id  VARCHAR(160) NOT NULL,
    category    VARCHAR(128) NOT NULL DEFAULT '',
    score       NUMERIC(7, 6) NOT NULL DEFAULT 0,
    created_at  TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    CONSTRAINT content_moderation_group_penalty_event_unique
        UNIQUE (user_id, group_id, request_id)
);

CREATE INDEX IF NOT EXISTS idx_content_moderation_group_penalties_active
    ON content_moderation_user_group_penalties(user_id, permanent, blocked_until, group_id);

CREATE INDEX IF NOT EXISTS idx_content_moderation_group_penalty_events_created_at
    ON content_moderation_user_group_penalty_events(created_at DESC);

CREATE OR REPLACE FUNCTION enqueue_content_moderation_group_penalty_auth_cache_invalidation()
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

DROP TRIGGER IF EXISTS trg_content_moderation_group_penalty_auth_cache_invalidation
    ON content_moderation_user_group_penalties;
CREATE TRIGGER trg_content_moderation_group_penalty_auth_cache_invalidation
AFTER INSERT OR UPDATE OR DELETE ON content_moderation_user_group_penalties
FOR EACH ROW EXECUTE FUNCTION enqueue_content_moderation_group_penalty_auth_cache_invalidation();

COMMENT ON TABLE content_moderation_user_group_penalties IS
    'Progressive content-moderation blocks: 24 hours, 36 hours, then permanent per user and group';
COMMENT ON TABLE content_moderation_user_group_penalty_events IS
    'Immutable request-level idempotency ledger for content-moderation group penalties';
