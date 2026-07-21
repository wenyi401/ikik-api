package service

import (
	"bufio"
	"context"
	"crypto/sha256"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"ikik-api/internal/pkg/anthropictokenizer"
	"ikik-api/internal/pkg/apicompat"
	"ikik-api/internal/pkg/claudeweb"
	"ikik-api/internal/pkg/ctxkey"

	"github.com/gin-gonic/gin"
)

type claudeWebStreamOptions struct {
	Model           string
	Effort          string
	ThinkingMode    string
	ConversationKey string
}

type claudeWebPreparedPrompt struct {
	Text        string
	InputTokens int
	ToolBridge  *claudeweb.ToolBridgeConfig
}

func (s *GatewayService) forwardClaudeWebMessages(
	ctx context.Context,
	c *gin.Context,
	account *Account,
	parsed *ParsedRequest,
) (*ForwardResult, error) {
	startTime := time.Now()
	originalModel := strings.TrimSpace(parsed.Model)
	mappedModel := resolveClaudeWebModel(account, originalModel)
	resp, err := startClaudeWebAnthropicStream(ctx, account, parsed.Body.Bytes(), claudeWebStreamOptions{
		Model:           mappedModel,
		Effort:          parsed.OutputEffort,
		ThinkingMode:    "auto",
		ConversationKey: claudeWebConversationKey(ctx, account, parsed),
	})
	if err != nil {
		return nil, s.claudeWebForwardError(ctx, account, err)
	}
	defer func() { _ = resp.Body.Close() }()

	if parsed.Stream {
		streamResult, streamErr := s.handleStreamingResponseAnthropicAPIKeyPassthrough(ctx, resp, c, account, startTime, mappedModel)
		if streamErr != nil {
			return nil, streamErr
		}
		usage := ClaudeUsage{}
		if streamResult.usage != nil {
			usage = *streamResult.usage
		}
		return &ForwardResult{
			RequestID:        resp.Header.Get("x-request-id"),
			Usage:            usage,
			Model:            originalModel,
			UpstreamModel:    mappedModel,
			Stream:           true,
			Duration:         time.Since(startTime),
			FirstTokenMs:     streamResult.firstTokenMs,
			ClientDisconnect: streamResult.clientDisconnect,
		}, nil
	}

	message, usage, err := collectClaudeWebAnthropicResponse(resp.Body)
	if err != nil {
		return nil, err
	}
	message.Model = originalModel
	c.JSON(http.StatusOK, message)
	return &ForwardResult{
		RequestID:     resp.Header.Get("x-request-id"),
		Usage:         usage,
		Model:         originalModel,
		UpstreamModel: mappedModel,
		Stream:        false,
		Duration:      time.Since(startTime),
	}, nil
}

func (s *GatewayService) forwardClaudeWebAsChatCompletions(
	ctx context.Context,
	c *gin.Context,
	account *Account,
	anthropicBody []byte,
	originalModel string,
	mappedModel string,
	clientStream bool,
	includeUsage bool,
	reasoningEffort *string,
	startTime time.Time,
	parsed *ParsedRequest,
) (*ForwardResult, error) {
	resp, err := startClaudeWebAnthropicStream(ctx, account, anthropicBody, claudeWebStreamOptions{
		Model:           mappedModel,
		Effort:          claudeWebEffort(reasoningEffort),
		ThinkingMode:    "auto",
		ConversationKey: claudeWebConversationKey(ctx, account, parsed),
	})
	if err != nil {
		return nil, s.claudeWebForwardError(ctx, account, err)
	}
	defer func() { _ = resp.Body.Close() }()
	if clientStream {
		return s.handleCCStreamingFromAnthropic(resp, c, originalModel, mappedModel, reasoningEffort, startTime, includeUsage)
	}
	return s.handleCCBufferedFromAnthropic(resp, c, originalModel, mappedModel, reasoningEffort, startTime)
}

func (s *GatewayService) forwardClaudeWebAsResponses(
	ctx context.Context,
	c *gin.Context,
	account *Account,
	anthropicBody []byte,
	originalModel string,
	mappedModel string,
	clientStream bool,
	reasoningEffort *string,
	startTime time.Time,
	parsed *ParsedRequest,
) (*ForwardResult, error) {
	resp, err := startClaudeWebAnthropicStream(ctx, account, anthropicBody, claudeWebStreamOptions{
		Model:           mappedModel,
		Effort:          claudeWebEffort(reasoningEffort),
		ThinkingMode:    "auto",
		ConversationKey: claudeWebConversationKey(ctx, account, parsed),
	})
	if err != nil {
		return nil, s.claudeWebForwardError(ctx, account, err)
	}
	defer func() { _ = resp.Body.Close() }()
	if clientStream {
		return s.handleResponsesStreamingResponse(resp, c, originalModel, mappedModel, reasoningEffort, startTime)
	}
	return s.handleResponsesBufferedStreamingResponse(resp, c, originalModel, mappedModel, reasoningEffort, startTime)
}

func startClaudeWebAnthropicStream(ctx context.Context, account *Account, anthropicBody []byte, options claudeWebStreamOptions) (*http.Response, error) {
	if account == nil || !account.IsClaudeWebSession() {
		return nil, fmt.Errorf("claude Web account is not configured")
	}
	options.Model = resolveClaudeWebModel(account, options.Model)
	if err := claudeweb.ValidateModel(options.Model); err != nil {
		return nil, err
	}

	var state *claudeWebConversationState
	persistent := strings.TrimSpace(options.ConversationKey) != ""
	if persistent {
		state = defaultClaudeWebConversationStore.acquire(options.ConversationKey)
	}
	releaseState := func() {
		if state != nil {
			defaultClaudeWebConversationStore.release(state)
			state = nil
		}
	}
	hasExistingConversation := state != nil && state.conversationID != ""
	prepared, err := prepareClaudeWebPromptMode(anthropicBody, hasExistingConversation)
	if err != nil {
		releaseState()
		return nil, err
	}
	client, err := claudeweb.NewClient(account.ClaudeWebCredentials(), account.ClaudeWebProxyURL())
	if err != nil {
		releaseState()
		return nil, err
	}
	startOptions := claudeweb.CompletionOptions{
		Model:           options.Model,
		Prompt:          prepared.Text,
		Effort:          options.Effort,
		ThinkingMode:    options.ThinkingMode,
		Persistent:      persistent,
		DisableWebTools: prepared.ToolBridge != nil,
	}
	if state != nil {
		startOptions.ConversationID = state.conversationID
		startOptions.ParentMessageUUID = state.lastAssistantUUID
	}
	stream, err := client.StartCompletion(ctx, startOptions)
	if err != nil && hasExistingConversation && shouldResetClaudeWebConversation(err) {
		state.conversationID = ""
		state.lastHumanUUID = ""
		state.lastAssistantUUID = ""
		prepared, err = prepareClaudeWebPromptMode(anthropicBody, false)
		if err == nil {
			startOptions.Prompt = prepared.Text
			startOptions.DisableWebTools = prepared.ToolBridge != nil
			startOptions.ConversationID = ""
			startOptions.ParentMessageUUID = ""
			stream, err = client.StartCompletion(ctx, startOptions)
		}
	}
	if err != nil {
		if state != nil {
			defaultClaudeWebConversationStore.invalidate(options.ConversationKey, state)
		}
		releaseState()
		client.Close()
		return nil, err
	}

	reader, writer := io.Pipe()
	go func() {
		defer func() { _ = stream.Body.Close() }()
		defer client.Close()
		_, convertErr := claudeweb.ConvertStreamWithOptions(ctx, stream.Body, writer, claudeweb.StreamOptions{
			Model:       options.Model,
			InputTokens: prepared.InputTokens,
			ToolBridge:  prepared.ToolBridge,
		})
		if state != nil {
			if convertErr == nil {
				state.conversationID = stream.ConversationID
				state.lastHumanUUID = stream.HumanMessageUUID
				state.lastAssistantUUID = stream.AssistantMessageUUID
			} else {
				defaultClaudeWebConversationStore.invalidate(options.ConversationKey, state)
			}
		}
		if stream.Temporary || convertErr != nil {
			cleanupCtx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
			_ = client.DeleteConversation(cleanupCtx, stream.ConversationID)
			cancel()
		}
		releaseState()
		if convertErr != nil {
			_ = writer.CloseWithError(convertErr)
			return
		}
		_ = writer.Close()
	}()

	header := stream.Header.Clone()
	header.Set("Content-Type", "text/event-stream")
	header.Set("Cache-Control", "no-cache")
	header.Set("x-request-id", stream.ConversationID)
	return &http.Response{
		StatusCode: http.StatusOK,
		Header:     header,
		Body:       reader,
	}, nil
}

func claudeWebEffort(value *string) string {
	if value == nil {
		return ""
	}
	return strings.TrimSpace(*value)
}

func applyClaudeWebParallelToolChoice(raw json.RawMessage, parallel *bool) json.RawMessage {
	if parallel == nil || *parallel {
		return raw
	}
	payload := map[string]any{"type": "auto"}
	trimmed := strings.TrimSpace(string(raw))
	if trimmed != "" && trimmed != "null" {
		if err := json.Unmarshal(raw, &payload); err != nil {
			var choiceType string
			if json.Unmarshal(raw, &choiceType) != nil || strings.TrimSpace(choiceType) == "" {
				return raw
			}
			payload = map[string]any{"type": choiceType}
		}
	}
	payload["disable_parallel_tool_use"] = true
	updated, err := json.Marshal(payload)
	if err != nil {
		return raw
	}
	return updated
}

func claudeWebConversationKey(ctx context.Context, account *Account, parsed *ParsedRequest) string {
	if account == nil || parsed == nil {
		return ""
	}
	hint := strings.TrimSpace(parsed.ExplicitSessionID)
	if hint == "" {
		hint = strings.TrimSpace(parsed.BodySessionID)
	}
	if hint == "" {
		return ""
	}
	userID := contextInt64(ctx, ctxkey.AuthenticatedUserID)
	groupID := int64(0)
	if parsed.GroupID != nil {
		groupID = *parsed.GroupID
	}
	hash := sha256.Sum256([]byte(hint))
	return fmt.Sprintf("account:%d:user:%d:group:%d:session:%x", account.ID, userID, groupID, hash[:16])
}

func contextInt64(ctx context.Context, key any) int64 {
	if ctx == nil {
		return 0
	}
	switch value := ctx.Value(key).(type) {
	case int64:
		return value
	case int:
		return int64(value)
	case int32:
		return int64(value)
	case uint64:
		if value <= uint64(^uint64(0)>>1) {
			return int64(value)
		}
	}
	return 0
}

func shouldResetClaudeWebConversation(err error) bool {
	var upstreamErr *claudeweb.HTTPError
	if !errors.As(err, &upstreamErr) {
		return false
	}
	if upstreamErr.StatusCode == http.StatusNotFound || upstreamErr.StatusCode == http.StatusGone {
		return true
	}
	return upstreamErr.StatusCode == http.StatusBadRequest && strings.Contains(strings.ToLower(string(upstreamErr.Body)), "conversation")
}

func resolveClaudeWebModel(account *Account, requestedModel string) string {
	requestedModel = strings.TrimSpace(requestedModel)
	if account != nil && requestedModel != "" {
		if mapped := strings.TrimSpace(account.GetMappedModel(requestedModel)); mapped != "" {
			return mapped
		}
	}
	if requestedModel != "" {
		return requestedModel
	}
	return ClaudeWebDefaultTestModel
}

func buildClaudeWebPrompt(body []byte) (string, int, error) {
	return buildClaudeWebPromptMode(body, false)
}

func buildClaudeWebPromptMode(body []byte, latestTurnOnly bool) (string, int, error) {
	prepared, err := prepareClaudeWebPromptMode(body, latestTurnOnly)
	if err != nil {
		return "", 0, err
	}
	return prepared.Text, prepared.InputTokens, nil
}

func prepareClaudeWebPromptMode(body []byte, latestTurnOnly bool) (*claudeWebPreparedPrompt, error) {
	var request apicompat.AnthropicRequest
	if err := json.Unmarshal(body, &request); err != nil {
		return nil, fmt.Errorf("parse Claude Web request: %w", err)
	}
	toolDefinitions := make([]claudeweb.ToolDefinition, 0, len(request.Tools))
	for _, tool := range request.Tools {
		toolDefinitions = append(toolDefinitions, claudeweb.ToolDefinition{
			Name:        tool.Name,
			Description: tool.Description,
			InputSchema: tool.InputSchema,
		})
	}
	toolBridge, err := claudeweb.NewToolBridgeConfig(toolDefinitions, request.ToolChoice)
	if err != nil {
		return nil, err
	}
	var prompt strings.Builder
	writePrompt := func(text string) error {
		_, err := prompt.WriteString(text)
		return err
	}
	if !latestTurnOnly {
		if system := claudeWebRawContentText(request.System); system != "" {
			if err := writePrompt("System: "); err != nil {
				return nil, err
			}
			if err := writePrompt(system); err != nil {
				return nil, err
			}
			if err := writePrompt("\n\n"); err != nil {
				return nil, err
			}
		}
	}
	if toolBridge != nil && toolBridge.Enabled() {
		toolPrompt, promptErr := toolBridge.Prompt()
		if promptErr != nil {
			return nil, promptErr
		}
		if err := writePrompt(toolPrompt); err != nil {
			return nil, err
		}
		if err := writePrompt("\n\n"); err != nil {
			return nil, err
		}
	}
	messages := request.Messages
	if latestTurnOnly && len(messages) > 0 {
		messages = messages[len(messages)-1:]
	}
	for _, message := range messages {
		switch strings.ToLower(strings.TrimSpace(message.Role)) {
		case "assistant":
			if err := writePrompt("Assistant: "); err != nil {
				return nil, err
			}
		default:
			if err := writePrompt("Human: "); err != nil {
				return nil, err
			}
		}
		messageText := claudeWebRawContentText(message.Content)
		if err := writePrompt(messageText); err != nil {
			return nil, err
		}
		if err := writePrompt("\n\n"); err != nil {
			return nil, err
		}
	}
	text := strings.TrimSpace(prompt.String())
	if text == "" {
		return nil, fmt.Errorf("claude Web request contains no text content")
	}
	return &claudeWebPreparedPrompt{
		Text:        text,
		InputTokens: anthropictokenizer.CountTokens(text),
		ToolBridge:  toolBridge,
	}, nil
}

func claudeWebRawContentText(raw json.RawMessage) string {
	if len(raw) == 0 || string(raw) == "null" {
		return ""
	}
	var text string
	if json.Unmarshal(raw, &text) == nil {
		return strings.TrimSpace(text)
	}
	var blocks []apicompat.AnthropicContentBlock
	if json.Unmarshal(raw, &blocks) != nil {
		return ""
	}
	parts := make([]string, 0, len(blocks))
	for _, block := range blocks {
		switch block.Type {
		case "text":
			if value := strings.TrimSpace(block.Text); value != "" {
				parts = append(parts, value)
			}
		case "thinking":
			if value := strings.TrimSpace(block.Thinking); value != "" {
				parts = append(parts, value)
			}
		case "tool_use":
			input := block.Input
			if len(input) == 0 || !json.Valid(input) {
				input = json.RawMessage(`{}`)
			}
			payload, err := json.Marshal([]map[string]any{{
				"name":      block.Name,
				"arguments": input,
			}})
			if err == nil {
				parts = append(parts, fmt.Sprintf("Assistant tool call id=%s:\n<tool_calls>%s</tool_calls>", strings.TrimSpace(block.ID), string(payload)))
			}
		case "tool_result":
			result := claudeWebRawContentText(block.Content)
			if block.IsError {
				result = "Tool returned an error: " + result
			}
			parts = append(parts, fmt.Sprintf("Tool result for call_id=%s:\n<tool_result>\n%s\n</tool_result>", strings.TrimSpace(block.ToolUseID), result))
		case "image":
			parts = append(parts, "[Image attachment]")
		}
	}
	return strings.Join(parts, "\n")
}

func collectClaudeWebAnthropicResponse(body io.Reader) (*apicompat.AnthropicResponse, ClaudeUsage, error) {
	scanner := bufio.NewScanner(body)
	scanner.Buffer(make([]byte, 0, 64*1024), 16<<20)
	var response *apicompat.AnthropicResponse
	usage := ClaudeUsage{}
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if !strings.HasPrefix(line, "data:") {
			continue
		}
		payload := strings.TrimSpace(strings.TrimPrefix(line, "data:"))
		if payload == "" || payload == "[DONE]" {
			continue
		}
		var event apicompat.AnthropicStreamEvent
		if json.Unmarshal([]byte(payload), &event) != nil {
			continue
		}
		switch event.Type {
		case "message_start":
			if event.Message != nil {
				response = event.Message
				mergeAnthropicUsage(&usage, event.Message.Usage)
			}
		case "content_block_start":
			if response == nil || event.ContentBlock == nil || event.Index == nil {
				continue
			}
			ensureClaudeWebContentIndex(response, *event.Index)
			response.Content[*event.Index] = *event.ContentBlock
		case "content_block_delta":
			if response == nil || event.Delta == nil || event.Index == nil {
				continue
			}
			ensureClaudeWebContentIndex(response, *event.Index)
			block := &response.Content[*event.Index]
			switch event.Delta.Type {
			case "text_delta":
				block.Text += event.Delta.Text
			case "thinking_delta":
				block.Thinking += event.Delta.Thinking
			case "input_json_delta":
				if block.Type == "tool_use" && strings.TrimSpace(string(block.Input)) == "{}" {
					block.Input = nil
				}
				block.Input = appendRawJSON(block.Input, event.Delta.PartialJSON)
			}
		case "message_delta":
			if response != nil && event.Delta != nil {
				if strings.TrimSpace(event.Delta.StopReason) != "" {
					response.StopReason = apicompat.AnthropicStopReasonPtr(event.Delta.StopReason)
				}
				response.StopSequence = event.Delta.StopSequence
			}
			if event.Usage != nil {
				mergeAnthropicUsage(&usage, *event.Usage)
			}
		}
	}
	if err := scanner.Err(); err != nil {
		return nil, usage, fmt.Errorf("read Claude Web response: %w", err)
	}
	if response == nil {
		return nil, usage, fmt.Errorf("claude Web stream ended without a response")
	}
	response.Usage = apicompat.AnthropicUsage{
		InputTokens:              usage.InputTokens,
		OutputTokens:             usage.OutputTokens,
		CacheCreationInputTokens: usage.CacheCreationInputTokens,
		CacheReadInputTokens:     usage.CacheReadInputTokens,
	}
	if apicompat.AnthropicStopReasonString(response.StopReason) == "" {
		response.StopReason = apicompat.AnthropicStopReasonPtr("end_turn")
	}
	return response, usage, nil
}

func ensureClaudeWebContentIndex(response *apicompat.AnthropicResponse, index int) {
	if response == nil || index < 0 {
		return
	}
	for len(response.Content) <= index {
		response.Content = append(response.Content, apicompat.AnthropicContentBlock{})
	}
}

func (s *GatewayService) claudeWebForwardError(ctx context.Context, account *Account, err error) error {
	statusCode := http.StatusBadGateway
	responseBody := []byte(`{"error":{"type":"upstream_error","message":"Claude Web request failed"}}`)
	var modelErr *claudeweb.UnsupportedModelError
	var toolRequestErr *claudeweb.ToolBridgeRequestError
	isLocalValidationError := errors.As(err, &modelErr) || errors.As(err, &toolRequestErr)
	if isLocalValidationError {
		statusCode = http.StatusBadRequest
		responseBody, _ = json.Marshal(map[string]any{
			"error": map[string]any{
				"type":    "invalid_request_error",
				"message": err.Error(),
			},
		})
	}
	var upstreamErr *claudeweb.HTTPError
	if errors.As(err, &upstreamErr) {
		if upstreamErr.StatusCode > 0 {
			statusCode = upstreamErr.StatusCode
		}
		if len(upstreamErr.Body) > 0 {
			responseBody = upstreamErr.Body
		}
	}
	if !isLocalValidationError && s != nil && s.rateLimitService != nil && account != nil {
		s.rateLimitService.HandleUpstreamError(ctx, account, statusCode, nil, responseBody)
	}
	return &UpstreamFailoverError{
		StatusCode:   statusCode,
		ResponseBody: responseBody,
	}
}
