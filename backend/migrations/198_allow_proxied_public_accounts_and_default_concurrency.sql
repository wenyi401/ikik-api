ALTER TABLE accounts
    ALTER COLUMN concurrency SET DEFAULT 10;

-- Public user accounts use a managed concurrency value, so move accounts that
-- still carry the previous template default to the new default. Preserve all
-- private-account values because users may have customized them.
UPDATE accounts
SET concurrency = 10
WHERE owner_user_id IS NOT NULL
  AND share_mode = 'public'
  AND concurrency = 3;
