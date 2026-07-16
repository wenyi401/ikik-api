-- Store OpenAI GPT reasoning token usage for usage record display.
ALTER TABLE usage_logs
    ADD COLUMN IF NOT EXISTS reasoning_tokens INTEGER NOT NULL DEFAULT 0;
