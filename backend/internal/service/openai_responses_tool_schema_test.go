package service

import (
	"testing"

	"github.com/stretchr/testify/require"
	"github.com/tidwall/gjson"
)

func TestSanitizeOpenAIResponsesToolParameterTypes(t *testing.T) {
	body := []byte(`{
 	"tools":[
  	{"type":"function","parameters":{"type":null}},
  	{"type":"function","function":{"parameters":{"type":null}}},
  	{"type":"function","parameters":{"properties":{}}}
 	],
 	"input":[{"type":"message","tools":[
  	{"type":"function","parameters":{"type":null}},
  	{"type":"function","tools":[{"parameters":{"type":null}}]}
 ]}]
}`)

	sanitized, changed, err := sanitizeOpenAIResponsesToolParameterTypes(body)

	require.NoError(t, err)
	require.True(t, changed)
	for _, path := range []string{
		"tools.0.parameters.type",
		"tools.1.function.parameters.type",
		"input.0.tools.0.parameters.type",
		"input.0.tools.1.tools.0.parameters.type",
	} {
		require.Equal(t, "object", gjson.GetBytes(sanitized, path).String(), path)
	}
	require.False(t, gjson.GetBytes(sanitized, "tools.2.parameters.type").Exists(), "missing type must remain unchanged")
}

func TestSanitizeOpenAIResponsesToolParameterTypesLeavesValidSchemasUntouched(t *testing.T) {
	body := []byte(`{"tools":[{"type":"function","parameters":{"properties":{}}},{"type":"function","parameters":{"type":"array"}}]}`)

	sanitized, changed, err := sanitizeOpenAIResponsesToolParameterTypes(body)

	require.NoError(t, err)
	require.False(t, changed)
	require.Equal(t, body, sanitized)
}
