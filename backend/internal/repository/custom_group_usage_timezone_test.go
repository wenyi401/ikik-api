package repository

import (
	"testing"

	"github.com/stretchr/testify/require"
	appTimezone "ikik-api/internal/pkg/timezone"
)

func useGroupUsageRepositoryTestTimezone(t *testing.T, name string) {
	t.Helper()

	previousName := appTimezone.Name()
	require.NoError(t, appTimezone.Init(name))
	t.Cleanup(func() { require.NoError(t, appTimezone.Init(previousName)) })
}
