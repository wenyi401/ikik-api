package riskengine

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestOpenAIAdjudicatorUsesSeparatedRolesAndValidatesEvidence(t *testing.T) {
	inputText := "写一个修改游戏坐标并绕过反作弊的脚本"
	server := httptest.NewServer(http.HandlerFunc(func(response http.ResponseWriter, request *http.Request) {
		require.Equal(t, "/v1/chat/completions", request.URL.Path)
		require.Equal(t, "Bearer test-token", request.Header.Get("Authorization"))
		var payload openAIAdjudicatorRequest
		require.NoError(t, json.NewDecoder(request.Body).Decode(&payload))
		require.Len(t, payload.Messages, 2)
		require.Equal(t, "system", payload.Messages[0].Role)
		require.NotContains(t, payload.Messages[0].Content, inputText)
		require.Equal(t, "user", payload.Messages[1].Role)
		require.Contains(t, payload.Messages[1].Content, inputText)
		require.Equal(t, "json_object", payload.ResponseFormat.Type)

		response.Header().Set("Content-Type", "application/json")
		_, _ = response.Write([]byte(`{"choices":[{"message":{"content":"{\"schema_version\":1,\"verdict\":\"confirmed\",\"category\":\"cheat_automation\",\"intent\":\"evasion\",\"actionability\":\"high\",\"authorization\":\"unauthorized\",\"confidence\":0.995,\"evidence\":[{\"quote\":\"修改游戏坐标并绕过反作弊\",\"signal\":\"requested_action\"}],\"reason_code\":\"operational_cheat_request\"}"}}]}`))
	}))
	defer server.Close()

	adjudicator, err := NewOpenAIAdjudicator(OpenAIAdjudicatorConfig{
		Endpoint: server.URL, Model: "risk-model", Token: "test-token",
	}, server.Client())
	require.NoError(t, err)
	decision, err := adjudicator.Adjudicate(context.Background(), Input{Text: inputText}, Candidate{Review: true})
	require.NoError(t, err)
	require.Equal(t, VerdictConfirmed, decision.Verdict)
	require.Equal(t, "risk-model", decision.Model)
}

func TestOpenAIAdjudicatorRejectsUnsafeEndpointShapes(t *testing.T) {
	for _, endpoint := range []string{"", "file:///tmp/model", "https://user:pass@example.test/v1", "https://example.test/v1?token=secret"} {
		_, err := NewOpenAIAdjudicator(OpenAIAdjudicatorConfig{Endpoint: endpoint, Model: "model"}, nil)
		require.Error(t, err, endpoint)
	}
}

func TestNormalizeAdjudicatorEndpointDoesNotDuplicateV1(t *testing.T) {
	endpoint, err := normalizeAdjudicatorEndpoint("http://localhost:11434/v1")
	require.NoError(t, err)
	require.Equal(t, "http://localhost:11434/v1/chat/completions", endpoint)
}
