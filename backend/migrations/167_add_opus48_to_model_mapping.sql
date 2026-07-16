-- Backfill claude-opus-4-8 into persisted Antigravity model_mapping objects.
UPDATE accounts
SET credentials = jsonb_set(
    credentials,
    '{model_mapping,claude-opus-4-8}',
    '"claude-opus-4-8"'::jsonb,
    true
)
WHERE platform = 'antigravity'
  AND deleted_at IS NULL
  AND jsonb_typeof(credentials->'model_mapping') = 'object'
  AND credentials->'model_mapping'->>'claude-opus-4-8' IS NULL;
