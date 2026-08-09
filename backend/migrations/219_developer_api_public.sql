ALTER TABLE users
    ALTER COLUMN developer_api_enabled SET DEFAULT TRUE;

UPDATE users
SET developer_api_enabled = TRUE
WHERE developer_api_enabled = FALSE;

COMMENT ON COLUMN users.developer_api_enabled IS
    'Developer API access is enabled by default; administrators may disable it for individual users when necessary';
