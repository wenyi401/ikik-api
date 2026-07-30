-- User-private OpenAI and Grok groups created before image generation support
-- defaulted this capability to disabled. Backfill only those private groups;
-- public groups and private groups for other platforms retain their settings.
WITH updated_groups AS (
    UPDATE groups
    SET allow_image_generation = true
    WHERE scope = 'user_private'
      AND platform IN ('openai', 'grok')
      AND allow_image_generation = false
    RETURNING id
), affected_api_keys AS (
    SELECT k.key
    FROM api_keys AS k
    JOIN updated_groups AS g ON g.id = k.group_id
    WHERE k.deleted_at IS NULL
      AND k.key <> ''
    UNION
    SELECT k.key
    FROM api_keys AS k
    JOIN api_key_group_routes AS r ON r.api_key_id = k.id
    JOIN updated_groups AS g ON g.id = r.group_id
    WHERE k.deleted_at IS NULL
      AND k.key <> ''
)
INSERT INTO auth_cache_invalidation_outbox (cache_key)
SELECT DISTINCT encode(sha256(convert_to(key, 'UTF8')), 'hex')
FROM affected_api_keys;
