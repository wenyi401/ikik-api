-- Backfill per-user private subscription groups for existing users.
-- Migration 186 expanded the supported platforms, but already-registered users
-- only receive those groups when the application provisions them. Keep this
-- migration idempotent so older deployments can safely catch up.

WITH platforms(platform, allow_messages_dispatch) AS (
    VALUES
        ('anthropic', false),
        ('openai', true),
        ('gemini', false),
        ('antigravity', false),
        ('grok', false),
        ('custom', false)
),
template AS (
    SELECT
        CASE
            WHEN COALESCE(NULLIF((SELECT value FROM settings WHERE key = 'user_private_group_daily_limit_usd'), '')::numeric, 0) > 0
                THEN NULLIF((SELECT value FROM settings WHERE key = 'user_private_group_daily_limit_usd'), '')::numeric
            ELSE NULL
        END AS daily_limit_usd,
        CASE
            WHEN COALESCE(NULLIF((SELECT value FROM settings WHERE key = 'user_private_group_weekly_limit_usd'), '')::numeric, 0) > 0
                THEN NULLIF((SELECT value FROM settings WHERE key = 'user_private_group_weekly_limit_usd'), '')::numeric
            ELSE NULL
        END AS weekly_limit_usd,
        CASE
            WHEN COALESCE(NULLIF((SELECT value FROM settings WHERE key = 'user_private_group_monthly_limit_usd'), '')::numeric, 0) > 0
                THEN NULLIF((SELECT value FROM settings WHERE key = 'user_private_group_monthly_limit_usd'), '')::numeric
            ELSE NULL
        END AS monthly_limit_usd,
        GREATEST(COALESCE(NULLIF((SELECT value FROM settings WHERE key = 'user_private_group_rate_multiplier'), '')::numeric, 1), 1) AS rate_multiplier,
        GREATEST(COALESCE(NULLIF((SELECT value FROM settings WHERE key = 'user_private_group_rpm_limit'), '')::integer, 0), 0) AS rpm_limit
),
active_users AS (
    SELECT id
    FROM users
    WHERE deleted_at IS NULL
)
INSERT INTO groups (
    name,
    description,
    rate_multiplier,
    is_exclusive,
    status,
    owner_user_id,
    scope,
    platform,
    subscription_type,
    daily_limit_usd,
    weekly_limit_usd,
    monthly_limit_usd,
    default_validity_days,
    allow_messages_dispatch,
    supported_model_scopes,
    model_routing,
    messages_dispatch_model_config,
    models_list_config,
    rpm_limit,
    created_at,
    updated_at
)
SELECT
    format('private-u%s-%s', u.id, p.platform),
    format('Private subscription group for user %s on %s.', u.id, p.platform),
    t.rate_multiplier,
    true,
    'active',
    u.id,
    'user_private',
    p.platform,
    'subscription',
    t.daily_limit_usd,
    t.weekly_limit_usd,
    t.monthly_limit_usd,
    365,
    p.allow_messages_dispatch,
    '[]'::jsonb,
    '{}'::jsonb,
    '{}'::jsonb,
    '{}'::jsonb,
    t.rpm_limit,
    NOW(),
    NOW()
FROM active_users u
CROSS JOIN platforms p
CROSS JOIN template t
WHERE NOT EXISTS (
    SELECT 1
    FROM groups g
    WHERE g.owner_user_id = u.id
        AND g.platform = p.platform
        AND g.scope = 'user_private'
        AND g.deleted_at IS NULL
)
ON CONFLICT DO NOTHING;

WITH platforms(platform) AS (
    VALUES
        ('anthropic'),
        ('openai'),
        ('gemini'),
        ('antigravity'),
        ('grok'),
        ('custom')
),
all_private_groups AS (
    SELECT g.id, g.owner_user_id AS user_id
    FROM groups g
    JOIN users u ON u.id = g.owner_user_id AND u.deleted_at IS NULL
    WHERE g.scope = 'user_private'
        AND g.deleted_at IS NULL
        AND g.owner_user_id IS NOT NULL
        AND g.platform IN (SELECT platform FROM platforms)
)
INSERT INTO user_subscriptions (
    user_id,
    group_id,
    starts_at,
    expires_at,
    status,
    assigned_at,
    notes,
    created_at,
    updated_at
)
SELECT
    g.user_id,
    g.id,
    NOW(),
    NOW() + INTERVAL '365 days',
    'active',
    NOW(),
    'auto assigned by user private group backfill',
    NOW(),
    NOW()
FROM all_private_groups g
WHERE NOT EXISTS (
    SELECT 1
    FROM user_subscriptions us
    WHERE us.user_id = g.user_id
        AND us.group_id = g.id
        AND us.deleted_at IS NULL
)
ON CONFLICT DO NOTHING;

WITH platforms(platform) AS (
    VALUES
        ('anthropic'),
        ('openai'),
        ('gemini'),
        ('antigravity'),
        ('grok'),
        ('custom')
),
all_private_groups AS (
    SELECT g.id, g.owner_user_id AS user_id
    FROM groups g
    JOIN users u ON u.id = g.owner_user_id AND u.deleted_at IS NULL
    WHERE g.scope = 'user_private'
        AND g.deleted_at IS NULL
        AND g.owner_user_id IS NOT NULL
        AND g.platform IN (SELECT platform FROM platforms)
)
INSERT INTO user_allowed_groups (user_id, group_id, created_at)
SELECT g.user_id, g.id, NOW()
FROM all_private_groups g
ON CONFLICT (user_id, group_id) DO NOTHING;
