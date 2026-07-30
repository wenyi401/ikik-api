-- Preserve the full redacted text that was actually sent to moderation.
-- Existing rows remain empty because their truncated input cannot be recovered.
ALTER TABLE content_moderation_logs
    ADD COLUMN IF NOT EXISTS input_content TEXT NOT NULL DEFAULT '';
