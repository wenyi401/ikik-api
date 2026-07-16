ALTER TABLE accounts
    DROP CONSTRAINT IF EXISTS accounts_account_level_check;

ALTER TABLE accounts
    ADD CONSTRAINT accounts_account_level_check
    CHECK (account_level IN ('unknown', 'free', 'plus', 'pro', 'team', 'k12'));

ALTER TABLE groups
    DROP CONSTRAINT IF EXISTS groups_required_account_level_check;

ALTER TABLE groups
    ADD CONSTRAINT groups_required_account_level_check
    CHECK (required_account_level IN ('', 'free', 'plus', 'pro', 'team', 'k12'));
