-- Durable, transactionally-enqueued API-key auth cache invalidation.
-- cache_key is always SHA-256 hex; plaintext credentials never leave api_keys.

CREATE TABLE IF NOT EXISTS auth_cache_invalidation_outbox (
    id            BIGSERIAL PRIMARY KEY,
    cache_key     CHAR(64) NOT NULL CHECK (cache_key ~ '^[0-9a-f]{64}$'),
    created_at    TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    available_at  TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    delivery_stage SMALLINT NOT NULL DEFAULT 0 CHECK (delivery_stage IN (0, 1)),
    attempts      INTEGER NOT NULL DEFAULT 0 CHECK (attempts >= 0),
    last_error    TEXT,
    claimed_at    TIMESTAMPTZ,
    claimed_by    TEXT
);

CREATE INDEX IF NOT EXISTS idx_auth_cache_invalidation_outbox_available
    ON auth_cache_invalidation_outbox (available_at, id)
    WHERE claimed_at IS NULL;
CREATE INDEX IF NOT EXISTS idx_auth_cache_invalidation_outbox_lease
    ON auth_cache_invalidation_outbox (claimed_at)
    WHERE claimed_at IS NOT NULL;
CREATE INDEX IF NOT EXISTS idx_auth_cache_invalidation_outbox_cache_key
    ON auth_cache_invalidation_outbox (cache_key);
CREATE INDEX IF NOT EXISTS idx_auth_cache_invalidation_outbox_created_at
    ON auth_cache_invalidation_outbox (created_at);

CREATE OR REPLACE FUNCTION enqueue_auth_cache_invalidation(raw_key TEXT)
RETURNS VOID
LANGUAGE plpgsql
AS $$
BEGIN
    IF raw_key IS NULL OR raw_key = '' THEN
        RETURN;
    END IF;
    INSERT INTO auth_cache_invalidation_outbox (cache_key)
    VALUES (encode(sha256(convert_to(raw_key, 'UTF8')), 'hex'));
END;
$$;

CREATE OR REPLACE FUNCTION enqueue_api_key_auth_cache_invalidation()
RETURNS TRIGGER
LANGUAGE plpgsql
AS $$
BEGIN
    IF TG_OP = 'DELETE' THEN
        PERFORM enqueue_auth_cache_invalidation(OLD.key);
        RETURN OLD;
    END IF;

    IF OLD.key IS DISTINCT FROM NEW.key
       OR OLD.status IS DISTINCT FROM NEW.status
       OR OLD.deleted_at IS DISTINCT FROM NEW.deleted_at
       OR OLD.user_id IS DISTINCT FROM NEW.user_id
       OR OLD.group_id IS DISTINCT FROM NEW.group_id
       OR OLD.ip_whitelist IS DISTINCT FROM NEW.ip_whitelist
       OR OLD.ip_blacklist IS DISTINCT FROM NEW.ip_blacklist
       OR OLD.expires_at IS DISTINCT FROM NEW.expires_at THEN
        PERFORM enqueue_auth_cache_invalidation(OLD.key);
        IF NEW.deleted_at IS NULL AND NEW.key IS DISTINCT FROM OLD.key THEN
            PERFORM enqueue_auth_cache_invalidation(NEW.key);
        END IF;
    END IF;
    RETURN NEW;
END;
$$;

DROP TRIGGER IF EXISTS trg_api_keys_auth_cache_invalidation ON api_keys;
CREATE TRIGGER trg_api_keys_auth_cache_invalidation
AFTER UPDATE OR DELETE ON api_keys
FOR EACH ROW EXECUTE FUNCTION enqueue_api_key_auth_cache_invalidation();

-- API keys may route through multiple groups. Use statement-level transition
-- tables so replacing several routes for one key produces one invalidation.
CREATE OR REPLACE FUNCTION enqueue_api_key_group_route_auth_cache_invalidation()
RETURNS TRIGGER
LANGUAGE plpgsql
AS $$
BEGIN
    IF TG_OP = 'INSERT' THEN
        INSERT INTO auth_cache_invalidation_outbox (cache_key)
        SELECT DISTINCT encode(sha256(convert_to(k.key, 'UTF8')), 'hex')
        FROM api_keys AS k
        JOIN new_routes AS r ON r.api_key_id = k.id
        WHERE k.deleted_at IS NULL
          AND k.key <> '';
        RETURN NULL;
    END IF;

    IF TG_OP = 'DELETE' THEN
        INSERT INTO auth_cache_invalidation_outbox (cache_key)
        SELECT DISTINCT encode(sha256(convert_to(k.key, 'UTF8')), 'hex')
        FROM api_keys AS k
        JOIN old_routes AS r ON r.api_key_id = k.id
        WHERE k.deleted_at IS NULL
          AND k.key <> '';
        RETURN NULL;
    END IF;

    WITH changed_routes AS (
        SELECT
            o.api_key_id AS old_api_key_id,
            n.api_key_id AS new_api_key_id
        FROM old_routes AS o
        FULL JOIN new_routes AS n USING (id)
        WHERE o.api_key_id IS DISTINCT FROM n.api_key_id
           OR o.group_id IS DISTINCT FROM n.group_id
           OR o.priority IS DISTINCT FROM n.priority
           OR o.weight IS DISTINCT FROM n.weight
           OR o.enabled IS DISTINCT FROM n.enabled
           OR o.cooldown_seconds IS DISTINCT FROM n.cooldown_seconds
    ), changed_api_keys AS (
        SELECT DISTINCT ids.api_key_id
        FROM changed_routes AS r
        CROSS JOIN LATERAL (
            VALUES (r.old_api_key_id), (r.new_api_key_id)
        ) AS ids(api_key_id)
        WHERE ids.api_key_id IS NOT NULL
    )
    INSERT INTO auth_cache_invalidation_outbox (cache_key)
    SELECT DISTINCT encode(sha256(convert_to(k.key, 'UTF8')), 'hex')
    FROM api_keys AS k
    JOIN changed_api_keys AS changed ON changed.api_key_id = k.id
    WHERE k.deleted_at IS NULL
      AND k.key <> '';
    RETURN NULL;
END;
$$;

DROP TRIGGER IF EXISTS trg_api_key_group_routes_auth_cache_invalidation ON api_key_group_routes;
DROP TRIGGER IF EXISTS trg_api_key_group_routes_auth_cache_invalidation_insert ON api_key_group_routes;
CREATE TRIGGER trg_api_key_group_routes_auth_cache_invalidation_insert
AFTER INSERT ON api_key_group_routes
REFERENCING NEW TABLE AS new_routes
FOR EACH STATEMENT EXECUTE FUNCTION enqueue_api_key_group_route_auth_cache_invalidation();

DROP TRIGGER IF EXISTS trg_api_key_group_routes_auth_cache_invalidation_update ON api_key_group_routes;
CREATE TRIGGER trg_api_key_group_routes_auth_cache_invalidation_update
AFTER UPDATE ON api_key_group_routes
REFERENCING OLD TABLE AS old_routes NEW TABLE AS new_routes
FOR EACH STATEMENT EXECUTE FUNCTION enqueue_api_key_group_route_auth_cache_invalidation();

DROP TRIGGER IF EXISTS trg_api_key_group_routes_auth_cache_invalidation_delete ON api_key_group_routes;
CREATE TRIGGER trg_api_key_group_routes_auth_cache_invalidation_delete
AFTER DELETE ON api_key_group_routes
REFERENCING OLD TABLE AS old_routes
FOR EACH STATEMENT EXECUTE FUNCTION enqueue_api_key_group_route_auth_cache_invalidation();

CREATE OR REPLACE FUNCTION enqueue_user_auth_cache_invalidation()
RETURNS TRIGGER
LANGUAGE plpgsql
AS $$
DECLARE
    target_user_id BIGINT;
BEGIN
    target_user_id := OLD.id;
    IF TG_OP = 'UPDATE'
       AND OLD.status IS NOT DISTINCT FROM NEW.status
       AND OLD.role IS NOT DISTINCT FROM NEW.role
       AND OLD.deleted_at IS NOT DISTINCT FROM NEW.deleted_at THEN
        RETURN NEW;
    END IF;

    INSERT INTO auth_cache_invalidation_outbox (cache_key)
    SELECT encode(sha256(convert_to(k.key, 'UTF8')), 'hex')
    FROM api_keys AS k
    WHERE k.user_id = target_user_id
      AND k.deleted_at IS NULL
      AND k.key <> '';
    IF TG_OP = 'DELETE' THEN
        RETURN OLD;
    END IF;
    RETURN NEW;
END;
$$;

DROP TRIGGER IF EXISTS trg_users_auth_cache_invalidation ON users;
CREATE TRIGGER trg_users_auth_cache_invalidation
AFTER UPDATE OR DELETE ON users
FOR EACH ROW EXECUTE FUNCTION enqueue_user_auth_cache_invalidation();

CREATE OR REPLACE FUNCTION enqueue_group_auth_cache_invalidations(target_group_id BIGINT)
RETURNS VOID
LANGUAGE plpgsql
AS $$
BEGIN
    IF target_group_id IS NULL THEN
        RETURN;
    END IF;

    INSERT INTO auth_cache_invalidation_outbox (cache_key)
    SELECT DISTINCT encode(sha256(convert_to(k.key, 'UTF8')), 'hex')
    FROM api_keys AS k
    WHERE k.deleted_at IS NULL
      AND k.key <> ''
      AND (
          k.group_id = target_group_id
          OR EXISTS (
              SELECT 1
              FROM api_key_group_routes AS r
              WHERE r.api_key_id = k.id
                AND r.group_id = target_group_id
          )
      );
END;
$$;

CREATE OR REPLACE FUNCTION enqueue_group_auth_cache_invalidation()
RETURNS TRIGGER
LANGUAGE plpgsql
AS $$
DECLARE
    target_group_id BIGINT;
BEGIN
    target_group_id := OLD.id;
    IF TG_OP = 'UPDATE'
       AND OLD.status IS NOT DISTINCT FROM NEW.status
       AND OLD.is_exclusive IS NOT DISTINCT FROM NEW.is_exclusive
       AND OLD.deleted_at IS NOT DISTINCT FROM NEW.deleted_at THEN
        RETURN NEW;
    END IF;

    PERFORM enqueue_group_auth_cache_invalidations(target_group_id);
    IF TG_OP = 'DELETE' THEN
        RETURN OLD;
    END IF;
    RETURN NEW;
END;
$$;

DROP TRIGGER IF EXISTS trg_groups_auth_cache_invalidation ON groups;
CREATE TRIGGER trg_groups_auth_cache_invalidation
AFTER UPDATE OR DELETE ON groups
FOR EACH ROW EXECUTE FUNCTION enqueue_group_auth_cache_invalidation();

CREATE OR REPLACE FUNCTION enqueue_allowed_group_auth_cache_invalidations(
    old_user_id BIGINT,
    old_group_id BIGINT,
    new_user_id BIGINT,
    new_group_id BIGINT
)
RETURNS VOID
LANGUAGE plpgsql
AS $$
BEGIN
    INSERT INTO auth_cache_invalidation_outbox (cache_key)
    SELECT DISTINCT encode(sha256(convert_to(k.key, 'UTF8')), 'hex')
    FROM api_keys AS k
    WHERE k.deleted_at IS NULL
      AND k.key <> ''
      AND (
          (
              old_user_id IS NOT NULL
              AND old_group_id IS NOT NULL
              AND k.user_id = old_user_id
              AND EXISTS (
                  SELECT 1 FROM groups AS g
                  WHERE g.id = old_group_id
                    AND g.is_exclusive = TRUE
              )
              AND (
                  k.group_id = old_group_id
                  OR EXISTS (
                      SELECT 1 FROM api_key_group_routes AS r
                      WHERE r.api_key_id = k.id
                        AND r.group_id = old_group_id
                  )
              )
          )
          OR (
              new_user_id IS NOT NULL
              AND new_group_id IS NOT NULL
              AND k.user_id = new_user_id
              AND EXISTS (
                  SELECT 1 FROM groups AS g
                  WHERE g.id = new_group_id
                    AND g.is_exclusive = TRUE
              )
              AND (
                  k.group_id = new_group_id
                  OR EXISTS (
                      SELECT 1 FROM api_key_group_routes AS r
                      WHERE r.api_key_id = k.id
                        AND r.group_id = new_group_id
                  )
              )
          )
      );
END;
$$;

CREATE OR REPLACE FUNCTION enqueue_allowed_group_auth_cache_invalidation()
RETURNS TRIGGER
LANGUAGE plpgsql
AS $$
BEGIN
    IF TG_OP = 'UPDATE'
       AND (OLD.user_id IS DISTINCT FROM NEW.user_id
            OR OLD.group_id IS DISTINCT FROM NEW.group_id) THEN
        PERFORM enqueue_allowed_group_auth_cache_invalidations(
            OLD.user_id, OLD.group_id, NEW.user_id, NEW.group_id
        );
        RETURN NEW;
    ELSIF TG_OP = 'UPDATE' THEN
        RETURN NEW;
    ELSIF TG_OP = 'INSERT' THEN
        PERFORM enqueue_allowed_group_auth_cache_invalidations(
            NULL, NULL, NEW.user_id, NEW.group_id
        );
        RETURN NEW;
    ELSE
        PERFORM enqueue_allowed_group_auth_cache_invalidations(
            OLD.user_id, OLD.group_id, NULL, NULL
        );
        RETURN OLD;
    END IF;
END;
$$;

DROP TRIGGER IF EXISTS trg_user_allowed_groups_auth_cache_invalidation ON user_allowed_groups;
CREATE TRIGGER trg_user_allowed_groups_auth_cache_invalidation
AFTER INSERT OR UPDATE OR DELETE ON user_allowed_groups
FOR EACH ROW EXECUTE FUNCTION enqueue_allowed_group_auth_cache_invalidation();

COMMENT ON TABLE auth_cache_invalidation_outbox IS
    'Durable cross-instance auth cache invalidations; cache_key is SHA-256 hex, never plaintext API key';
