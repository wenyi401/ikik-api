//go:build unit

package dto

import (
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestCustomMenuItemOpenInNewWindowRoundTrip(t *testing.T) {
	raw := `[{"id":"docs","label":"Docs","url":"https://example.com","visibility":"user","sort_order":1,"open_in_new_window":true}]`

	items := ParseCustomMenuItems(raw)
	require.Len(t, items, 1)
	require.True(t, items[0].OpenInNewWindow)

	encoded, err := json.Marshal(items)
	require.NoError(t, err)
	require.Contains(t, string(encoded), `"open_in_new_window":true`)
}
