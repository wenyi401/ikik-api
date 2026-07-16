ALTER TABLE accounts
    DROP CONSTRAINT IF EXISTS accounts_account_level_check;

ALTER TABLE accounts
    ADD CONSTRAINT accounts_account_level_check
    CHECK (account_level IN ('unknown', 'free', 'plus', 'pro', 'team'));

ALTER TABLE groups
    DROP CONSTRAINT IF EXISTS groups_required_account_level_check;

ALTER TABLE groups
    ADD CONSTRAINT groups_required_account_level_check
    CHECK (required_account_level IN ('', 'free', 'plus', 'pro', 'team'));

UPDATE accounts
SET account_level = 'team',
    updated_at = NOW()
WHERE platform = 'openai'
  AND credentials->>'plan_type' = 'team'
  AND account_level <> 'team';

UPDATE groups
SET required_account_level = 'team',
    updated_at = NOW()
WHERE platform = 'openai'
  AND name = 'TEAM共享号池'
  AND required_account_level <> 'team';
