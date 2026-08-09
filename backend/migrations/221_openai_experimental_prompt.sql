-- OpenAI experimental prompt entitlement and group gate.
-- The feature is fail-closed: existing groups remain disabled and users remain locked.

ALTER TABLE groups
    ADD COLUMN IF NOT EXISTS openai_experimental_prompt_enabled BOOLEAN NOT NULL DEFAULT FALSE;

ALTER TABLE users
    ADD COLUMN IF NOT EXISTS openai_experimental_prompt_unlocked BOOLEAN NOT NULL DEFAULT FALSE;

ALTER TABLE redeem_codes
    ADD COLUMN IF NOT EXISTS feature_key VARCHAR(80) NOT NULL DEFAULT '';

CREATE INDEX IF NOT EXISTS idx_redeem_codes_feature_key
    ON redeem_codes(feature_key)
    WHERE feature_key <> '';
