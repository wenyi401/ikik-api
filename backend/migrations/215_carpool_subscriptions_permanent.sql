-- Carpool access groups are stable routing groups; their subscriptions must not
-- expire with the pool's seat or pricing window.
UPDATE groups g
SET
    default_validity_days = 36500,
    updated_at = NOW()
WHERE g.deleted_at IS NULL
  AND (
      g.scope = 'user_carpool'
      OR EXISTS (
          SELECT 1
          FROM carpool_pools p
          WHERE p.group_id = g.id
            AND p.deleted_at IS NULL
      )
  )
  AND g.default_validity_days <> 36500;

UPDATE user_subscriptions us
SET
    expires_at = TIMESTAMPTZ '2099-12-31 23:59:59+00',
    status = 'active',
    updated_at = NOW()
WHERE us.deleted_at IS NULL
  AND us.status IN ('active', 'expired')
  AND EXISTS (
      SELECT 1
      FROM carpool_members m
      JOIN carpool_pools p ON p.id = m.pool_id
      WHERE m.subscription_id = us.id
        AND m.status = 'active'
        AND m.deleted_at IS NULL
        AND p.deleted_at IS NULL
        AND p.status <> 'closed'
  );
