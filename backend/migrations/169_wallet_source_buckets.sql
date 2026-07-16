ALTER TABLE users
    ADD COLUMN IF NOT EXISTS recharge_balance DECIMAL(20, 8) NOT NULL DEFAULT 0,
    ADD COLUMN IF NOT EXISTS invite_income_balance DECIMAL(20, 8) NOT NULL DEFAULT 0,
    ADD COLUMN IF NOT EXISTS share_income_balance DECIMAL(20, 8) NOT NULL DEFAULT 0,
    ADD COLUMN IF NOT EXISTS total_invite_income DECIMAL(20, 8) NOT NULL DEFAULT 0,
    ADD COLUMN IF NOT EXISTS total_share_income DECIMAL(20, 8) NOT NULL DEFAULT 0;

-- Existing balances predate source buckets. Treat them as non-withdrawable
-- consumption balance; shared income starts accruing into its own bucket after
-- this migration.
UPDATE users
SET recharge_balance = balance
WHERE recharge_balance = 0
  AND invite_income_balance = 0
  AND share_income_balance = 0
  AND balance > 0;

CREATE INDEX IF NOT EXISTS idx_users_share_income_balance
    ON users (share_income_balance)
    WHERE deleted_at IS NULL AND share_income_balance > 0;
