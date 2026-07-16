-- Keep user-owned shared accounts available through both the owner's private
-- group and the public shared pool. Existing deployments may only have the
-- public group binding because older application logic replaced all bindings
-- when sharing was enabled.

INSERT INTO account_groups (account_id, group_id, priority, created_at)
SELECT
    a.id,
    g.id,
    1,
    NOW()
FROM accounts a
JOIN groups g
    ON g.owner_user_id = a.owner_user_id
    AND g.platform = a.platform
    AND g.scope = 'user_private'
    AND g.deleted_at IS NULL
WHERE a.deleted_at IS NULL
    AND a.owner_user_id IS NOT NULL
    AND a.share_mode = 'public'
ON CONFLICT (account_id, group_id) DO NOTHING;
