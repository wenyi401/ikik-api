package handler

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestNormalizeUserGrokSSOTokens(t *testing.T) {
	tokens := normalizeUserGrokSSOTokens(
		[]string{"  first-token  ", "sso=second-token", "first-token", ""},
		"second-token",
	)

	require.Equal(t, []string{"first-token", "second-token"}, tokens)
}

func TestUserGrokSSOAccountName(t *testing.T) {
	require.Equal(t, "named #2", userGrokSSOAccountName(" named ", "user@example.com", 2, 3))
	require.Equal(t, "user@example.com", userGrokSSOAccountName("", " user@example.com ", 1, 1))
	require.Equal(t, "Grok OAuth Account", userGrokSSOAccountName("", "", 1, 1))
}
