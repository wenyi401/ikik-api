package service

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"

	"ikik-api/internal/pkg/apicompat"
	"ikik-api/internal/pkg/claudeweb"
	"ikik-api/internal/pkg/ctxkey"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
)

func TestBuildClaudeWebPromptModeLatestTurnOnly(t *testing.T) {
	body := []byte(`{
		"system":"system rule",
		"messages":[
			{"role":"user","content":"first question"},
			{"role":"assistant","content":"first answer"},
			{"role":"user","content":"latest question"}
		]
	}`)

	full, fullTokens, err := buildClaudeWebPromptMode(body, false)
	require.NoError(t, err)
	require.Contains(t, full, "system rule")
	require.Contains(t, full, "first question")
	require.Contains(t, full, "latest question")
	require.Positive(t, fullTokens)

	latest, latestTokens, err := buildClaudeWebPromptMode(body, true)
	require.NoError(t, err)
	require.NotContains(t, latest, "system rule")
	require.NotContains(t, latest, "first question")
	require.Contains(t, latest, "latest question")
	require.Less(t, latestTokens, fullTokens)
}

func TestPrepareClaudeWebPromptAddsClientToolBridge(t *testing.T) {
	body := []byte(`{
		"system":"be concise",
		"tools":[{"name":"get_weather","description":"Get weather","input_schema":{"type":"object","properties":{"city":{"type":"string"}}}}],
		"tool_choice":{"type":"tool","name":"get_weather"},
		"messages":[{"role":"user","content":"weather in Tokyo"}]
	}`)

	prepared, err := prepareClaudeWebPromptMode(body, false)
	require.NoError(t, err)
	require.NotNil(t, prepared.ToolBridge)
	require.True(t, prepared.ToolBridge.Enabled())
	require.Equal(t, claudeweb.ToolChoiceSpecific, prepared.ToolBridge.ChoiceMode)
	require.Len(t, prepared.ToolBridge.Tools, 1)
	require.Equal(t, "get_weather", prepared.ToolBridge.Tools[0].Name)
	require.Contains(t, prepared.Text, "be concise")
	require.Contains(t, prepared.Text, "<tool_call>")
	require.Contains(t, prepared.Text, "<think>")
	require.Contains(t, prepared.Text, "Human: weather in Tokyo")
	require.Contains(t, prepared.Text, "You must call one available tool")
	require.Contains(t, prepared.Text, "Name: get_weather")
	require.Positive(t, prepared.InputTokens)
}

func TestPrepareClaudeWebPromptKeepsStructuredToolHistory(t *testing.T) {
	body := []byte(`{
		"tools":[{"name":"get_weather","input_schema":{"type":"object","properties":{}}}],
		"messages":[
			{"role":"assistant","content":[{"type":"tool_use","id":"toolu_1","name":"get_weather","input":{"city":"Tokyo"}}]},
			{"role":"user","content":[{"type":"tool_result","tool_use_id":"toolu_1","content":"sunny"}]}
		]
	}`)

	prepared, err := prepareClaudeWebPromptMode(body, false)
	require.NoError(t, err)
	require.Contains(t, prepared.Text, "Assistant tool call id=toolu_1")
	require.Contains(t, prepared.Text, "<tool_calls>")
	require.Contains(t, prepared.Text, `"city":"Tokyo"`)
	require.Contains(t, prepared.Text, "Tool result for call_id=toolu_1")
	require.Contains(t, prepared.Text, "<tool_result>")
	require.Contains(t, prepared.Text, "sunny")
	require.Contains(t, prepared.Text, "You must then output exactly one terminal block")
	require.NotContains(t, prepared.Text, "ikik_previous_tool_call")
	require.NotContains(t, prepared.Text, "ikik_tool_result")
}

func TestClaudeWebToolBridgeAcceptsChatAndResponsesConversions(t *testing.T) {
	parallel := false
	chat := &apicompat.ChatCompletionsRequest{
		Model: "claude-sonnet-5",
		Messages: []apicompat.ChatMessage{{
			Role:    "user",
			Content: json.RawMessage(`"weather in Tokyo"`),
		}},
		Tools: []apicompat.ChatTool{{
			Type: "function",
			Function: &apicompat.ChatFunction{
				Name:       "get_weather",
				Parameters: json.RawMessage(`{"type":"object","properties":{"city":{"type":"string"}}}`),
			},
		}},
		ToolChoice:        json.RawMessage(`{"type":"function","function":{"name":"get_weather"}}`),
		ParallelToolCalls: &parallel,
	}
	responses, err := apicompat.ChatCompletionsToResponses(chat)
	require.NoError(t, err)
	anthropic, err := apicompat.ResponsesToAnthropicRequest(responses)
	require.NoError(t, err)
	anthropic.ToolChoice = applyClaudeWebParallelToolChoice(anthropic.ToolChoice, responses.ParallelToolCalls)
	body, err := json.Marshal(anthropic)
	require.NoError(t, err)

	prepared, err := prepareClaudeWebPromptMode(body, false)
	require.NoError(t, err)
	require.NotNil(t, prepared.ToolBridge)
	require.False(t, prepared.ToolBridge.AllowParallel)
	require.Equal(t, claudeweb.ToolChoiceSpecific, prepared.ToolBridge.ChoiceMode)
	require.Equal(t, "get_weather", prepared.ToolBridge.ForcedTool)
	require.Contains(t, prepared.Text, "get_weather")
}

func TestApplyClaudeWebParallelToolChoice(t *testing.T) {
	parallel := false
	updated := applyClaudeWebParallelToolChoice(json.RawMessage(`{"type":"any"}`), &parallel)
	require.JSONEq(t, `{"type":"any","disable_parallel_tool_use":true}`, string(updated))
}

func TestClaudeWebToolBridgeBufferedResponseRoundTrip(t *testing.T) {
	config, err := claudeweb.NewToolBridgeConfig([]claudeweb.ToolDefinition{{
		Name:        "get_weather",
		InputSchema: json.RawMessage(`{"type":"object","properties":{"city":{"type":"string"}}}`),
	}}, nil)
	require.NoError(t, err)
	upstream := strings.NewReader(strings.Join([]string{
		"event: content_block_start",
		`data: {"index":0,"content_block":{"type":"text","text":""}}`,
		"",
		"event: content_block_delta",
		`data: {"index":0,"delta":{"type":"text_delta","text":"<ikik_tool_calls>[{\"name\":\"get_weather\",\"arguments\":{\"city\":\"Tokyo\"}}]</ikik_tool_calls>"}}`,
		"",
		"event: content_block_stop",
		`data: {"index":0}`,
		"",
	}, "\n"))
	var stream bytes.Buffer
	_, err = claudeweb.ConvertStreamWithOptions(context.Background(), upstream, &stream, claudeweb.StreamOptions{
		Model:      claudeweb.DefaultModel,
		ToolBridge: config,
	})
	require.NoError(t, err)

	response, _, err := collectClaudeWebAnthropicResponse(&stream)
	require.NoError(t, err)
	require.Equal(t, "tool_use", response.StopReason)
	require.Len(t, response.Content, 1)
	require.Equal(t, "tool_use", response.Content[0].Type)
	require.Equal(t, "get_weather", response.Content[0].Name)
	require.JSONEq(t, `{"city":"Tokyo"}`, string(response.Content[0].Input))
}

func TestClaudeWebToolBridgeReachesChatCompletionsAndResponses(t *testing.T) {
	gin.SetMode(gin.TestMode)
	buildResponse := func() *http.Response {
		return &http.Response{
			Header: http.Header{"x-request-id": []string{"claude-web-tool-test"}},
			Body: io.NopCloser(strings.NewReader(strings.Join([]string{
				`event: message_start`,
				`data: {"type":"message_start","message":{"id":"msg_tool","type":"message","role":"assistant","content":[],"model":"claude-sonnet-5","stop_reason":null,"usage":{"input_tokens":10,"output_tokens":0}}}`,
				``,
				`event: content_block_start`,
				`data: {"type":"content_block_start","index":0,"content_block":{"type":"tool_use","id":"toolu_test","name":"get_weather","input":{}}}`,
				``,
				`event: content_block_delta`,
				`data: {"type":"content_block_delta","index":0,"delta":{"type":"input_json_delta","partial_json":"{\"city\":\"Tokyo\"}"}}`,
				``,
				`event: content_block_stop`,
				`data: {"type":"content_block_stop","index":0}`,
				``,
				`event: message_delta`,
				`data: {"type":"message_delta","delta":{"stop_reason":"tool_use"},"usage":{"output_tokens":8}}`,
				``,
				`event: message_stop`,
				`data: {"type":"message_stop"}`,
				``,
			}, "\n"))),
		}
	}

	t.Run("chat completions", func(t *testing.T) {
		recorder := httptest.NewRecorder()
		ctx, _ := gin.CreateTestContext(recorder)
		result, err := (&GatewayService{}).handleCCBufferedFromAnthropic(
			buildResponse(), ctx, "claude-sonnet-5", "claude-sonnet-5", nil, time.Now(),
		)
		require.NoError(t, err)
		require.NotNil(t, result)
		require.Contains(t, recorder.Body.String(), `"finish_reason":"tool_calls"`)
		require.Contains(t, recorder.Body.String(), `"name":"get_weather"`)
		require.Contains(t, recorder.Body.String(), `\"city\":\"Tokyo\"`)
	})

	t.Run("responses", func(t *testing.T) {
		recorder := httptest.NewRecorder()
		ctx, _ := gin.CreateTestContext(recorder)
		result, err := (&GatewayService{}).handleResponsesBufferedStreamingResponse(
			buildResponse(), ctx, "claude-sonnet-5", "claude-sonnet-5", nil, time.Now(),
		)
		require.NoError(t, err)
		require.NotNil(t, result)
		require.Contains(t, recorder.Body.String(), `"type":"function_call"`)
		require.Contains(t, recorder.Body.String(), `"name":"get_weather"`)
		require.Contains(t, recorder.Body.String(), `\"city\":\"Tokyo\"`)
	})
}

func TestClaudeWebConversationKeyIsIsolated(t *testing.T) {
	groupID := int64(9)
	parsed := &ParsedRequest{ExplicitSessionID: "conversation-1", GroupID: &groupID}
	account := &Account{ID: 17}
	first := claudeWebConversationKey(context.WithValue(context.Background(), ctxkey.AuthenticatedUserID, int64(5)), account, parsed)
	second := claudeWebConversationKey(context.WithValue(context.Background(), ctxkey.AuthenticatedUserID, int64(6)), account, parsed)
	require.NotEmpty(t, first)
	require.NotEqual(t, first, second)
	require.NotContains(t, first, "conversation-1")
}

func TestClaudeWebConversationStorePersistsAndInvalidates(t *testing.T) {
	store := newClaudeWebConversationStore(0, 2)
	state := store.acquire("one")
	state.conversationID = "conversation-upstream"
	state.lastAssistantUUID = "assistant-1"
	store.release(state)

	again := store.acquire("one")
	require.Equal(t, "conversation-upstream", again.conversationID)
	require.Equal(t, "assistant-1", again.lastAssistantUUID)
	store.invalidate("one", again)
	store.release(again)

	replacement := store.acquire("one")
	require.Empty(t, replacement.conversationID)
	store.release(replacement)
}

func TestClaudeWebUnsupportedModelReturnsClientError(t *testing.T) {
	err := (&GatewayService{}).claudeWebForwardError(
		context.Background(),
		&Account{ID: 17},
		&claudeweb.UnsupportedModelError{Model: "claude-unknown"},
	)

	var failoverErr *UpstreamFailoverError
	require.ErrorAs(t, err, &failoverErr)
	require.Equal(t, 400, failoverErr.StatusCode)
	require.Contains(t, string(failoverErr.ResponseBody), "invalid_request_error")
	require.Contains(t, string(failoverErr.ResponseBody), "claude-unknown")
}

func TestClaudeWebInvalidToolChoiceReturnsClientError(t *testing.T) {
	_, toolErr := claudeweb.NewToolBridgeConfig([]claudeweb.ToolDefinition{{
		Name:        "known",
		InputSchema: json.RawMessage(`{"type":"object","properties":{}}`),
	}}, json.RawMessage(`{"type":"tool","name":"missing"}`))
	require.Error(t, toolErr)

	err := (&GatewayService{}).claudeWebForwardError(context.Background(), &Account{ID: 17}, toolErr)
	var failoverErr *UpstreamFailoverError
	require.ErrorAs(t, err, &failoverErr)
	require.Equal(t, http.StatusBadRequest, failoverErr.StatusCode)
	require.Contains(t, string(failoverErr.ResponseBody), "unknown tool")
}

func TestClaudeWebConversationStoreConcurrentAccess(t *testing.T) {
	store := newClaudeWebConversationStore(0, 128)
	var wait sync.WaitGroup
	for worker := 0; worker < 64; worker++ {
		worker := worker
		wait.Add(1)
		go func() {
			defer wait.Done()
			for iteration := 0; iteration < 50; iteration++ {
				key := fmt.Sprintf("session-%d", worker%8)
				state := store.acquire(key)
				state.conversationID = fmt.Sprintf("conversation-%d-%d", worker, iteration)
				state.lastAssistantUUID = fmt.Sprintf("assistant-%d-%d", worker, iteration)
				store.release(state)
			}
		}()
	}
	wait.Wait()

	store.mu.Lock()
	require.Len(t, store.entries, 8)
	store.mu.Unlock()
}
