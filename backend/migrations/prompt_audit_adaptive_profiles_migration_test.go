package migrations

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestPromptAuditAdaptiveProfilesMigrationIsPolicyIsolated(t *testing.T) {
	content, err := FS.ReadFile("208_prompt_audit_adaptive_profiles.sql")
	require.NoError(t, err)
	sql := string(content)

	require.Contains(t, sql, "CREATE TABLE IF NOT EXISTS prompt_audit_user_profiles")
	require.Contains(t, sql, "CREATE TABLE IF NOT EXISTS prompt_audit_fingerprints")
	require.NotContains(t, strings.ToLower(sql), "content_moderation_user_risk_profiles")
}
