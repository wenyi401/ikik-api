-- API-key-level opt-in for the OpenAI experimental prompt.
-- Existing and newly created keys remain disabled until the user explicitly enables them.

ALTER TABLE api_keys
    ADD COLUMN IF NOT EXISTS openai_experimental_prompt_enabled BOOLEAN NOT NULL DEFAULT FALSE;
