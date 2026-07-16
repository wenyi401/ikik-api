CREATE TABLE IF NOT EXISTS api_key_group_routes (
    id BIGSERIAL PRIMARY KEY,
    api_key_id BIGINT NOT NULL REFERENCES api_keys(id) ON DELETE CASCADE,
    group_id BIGINT NOT NULL REFERENCES groups(id) ON DELETE CASCADE,
    priority INT NOT NULL DEFAULT 100,
    weight INT NOT NULL DEFAULT 1,
    enabled BOOLEAN NOT NULL DEFAULT TRUE,
    cooldown_seconds INT NOT NULL DEFAULT 30,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    CONSTRAINT api_key_group_routes_api_key_group_unique UNIQUE (api_key_id, group_id)
);

CREATE INDEX IF NOT EXISTS idx_api_key_group_routes_key_enabled_priority
ON api_key_group_routes(api_key_id, enabled, priority);

CREATE INDEX IF NOT EXISTS idx_api_key_group_routes_group_id
ON api_key_group_routes(group_id);

INSERT INTO api_key_group_routes (api_key_id, group_id, priority, weight, enabled, cooldown_seconds, created_at, updated_at)
SELECT id, group_id, 100, 1, TRUE, 30, NOW(), NOW()
FROM api_keys
WHERE group_id IS NOT NULL
ON CONFLICT (api_key_id, group_id) DO NOTHING;
