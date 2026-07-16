UPDATE groups
SET allow_messages_dispatch = true
WHERE scope = 'user_private'
  AND platform = 'openai'
  AND deleted_at IS NULL
  AND allow_messages_dispatch = false;
