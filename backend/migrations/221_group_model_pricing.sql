ALTER TABLE groups
    ADD COLUMN IF NOT EXISTS long_context_pricing_enabled BOOLEAN NOT NULL DEFAULT TRUE,
    ADD COLUMN IF NOT EXISTS model_pricing JSONB;

-- Existing IKIK deployments may already expose this switch and contain explicit
-- FALSE values. The column default initializes legacy rows when the column is
-- first added; do not overwrite an administrator's existing pricing policy.

COMMENT ON COLUMN groups.long_context_pricing_enabled IS
    'Whether token pricing selects official/preset long-context tiers; default true preserves existing long-context billing';
COMMENT ON COLUMN groups.model_pricing IS
    'Per-model group pricing overrides channel and built-in model pricing';
