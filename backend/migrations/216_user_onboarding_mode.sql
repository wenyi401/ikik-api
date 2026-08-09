ALTER TABLE users
    ADD COLUMN IF NOT EXISTS onboarding_mode varchar(16) NOT NULL DEFAULT 'unset';

ALTER TABLE users
    DROP CONSTRAINT IF EXISTS users_onboarding_mode_check;

ALTER TABLE users
    ADD CONSTRAINT users_onboarding_mode_check
    CHECK (onboarding_mode IN ('unset', 'beginner', 'expert'));
