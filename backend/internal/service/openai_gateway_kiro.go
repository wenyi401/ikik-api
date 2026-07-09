package service

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"go.uber.org/zap"
	"ikik-api/internal/pkg/apicompat"
	"ikik-api/internal/pkg/kiro"
	"ikik-api/internal/pkg/logger"
	"ikik-api/internal/util/responseheaders"
)

func isKiroOAuthAccount(account *Account) bool {
	return account != nil && account.Platform == PlatformKiro && account.Type == AccountTypeOAuth
}

func normalizeKiroRuntimeModel(model string) string {
	model = strings.TrimSpace(model)
	if model == "" {
		return ""
	}
	if mapped := kiro.MapModel(model); strings.TrimSpace(mapped) != "" {
		return mapped
	}
	return model
}

func kiroUsageToOpenAI(usage kiro.Usage, fallbackInput int) OpenAIUsage {
	inputTokens := usage.InputTokens
	if inputTokens <= 0 {
		inputTokens = fallbackInput
	}
	return OpenAIUsage{
		InputTokens:              inputTokens,
		OutputTokens:             usage.OutputTokens,
		CacheReadInputTokens:     usage.CacheReadInputTokens,
		CacheCreationInputTokens: usage.CacheCreationInputTokens,
		KiroCredits:              usage.KiroCredits,
	}
}

func (s *OpenAIGatewayService) sendKiroOAuthRuntimeRequest(
	ctx context.Context,
	c *gin.Context,
	account *Account,
	anthropicBody []byte,
	upstreamModel string,
) (*http.Response, *kiro.KiroBuildResult, error) {
	token, _, err := s.GetAccessToken(ctx, account)
	token = strings.TrimSpace(token)
	if err != nil || token == "" {
		if err != nil {
			return nil, nil, err
		}
		return nil, nil, fmt.Errorf("kiro access_token not found in credentials")
	}
	upstreamModel = normalizeKiroRuntimeModel(upstreamModel)
	if upstreamModel == "" {
		upstreamModel = normalizeKiroRuntimeModel(gjsonModel(anthropicBody))
	}
	buildResult, err := kiro.BuildKiroPayloadWithContext(anthropicBody, upstreamModel, resolveKiroPayloadProfileArn(account), "AI_EDITOR", c.Request.Header)
	if err != nil {
		return nil, nil, fmt.Errorf("build kiro request: %w", err)
	}
	endpoints := buildKiroEndpoints(account, KiroEndpointModeQ)
	if len(endpoints) == 0 {
		return nil, nil, fmt.Errorf("no kiro endpoint available")
	}
	accountKey := buildKiroAccountKey(account)
	endpoint := endpoints[0]
	req, err := newKiroJSONRequest(ctx, endpoint.URL, buildResult.Payload, token, accountKey, buildKiroMachineID(account), endpoint.AmzTarget, account)
	if err != nil {
		return nil, nil, fmt.Errorf("create kiro request: %w", err)
	}
	resp, err := s.httpUpstream.Do(req, kiroProxyURL(account), account.ID, account.Concurrency)
	if err != nil {
		return nil, nil, err
	}
	return resp, buildResult, nil
}

func gjsonModel(body []byte) string {
	var payload struct {
		Model string `json:"model"`
	}
	_ = json.Unmarshal(body, &payload)
	return payload.Model
}

func (s *OpenAIGatewayService) handleKiroOAuthErrorResponse(
	ctx context.Context,
	c *gin.Context,
	account *Account,
	resp *http.Response,
	writeErr func(*gin.Context, int, string, string),
) (*OpenAIForwardResult, error) {
	respBody := s.readUpstreamErrorBody(resp)
	_ = resp.Body.Close()
	resp.Body = io.NopCloser(bytes.NewReader(respBody))
	upstreamMsg := strings.TrimSpace(extractUpstreamErrorMessage(respBody))
	if upstreamMsg == "" {
		upstreamMsg = strings.TrimSpace(string(respBody))
	}
	upstreamMsg = sanitizeUpstreamErrorMessage(upstreamMsg)

	if s.shouldFailoverOpenAIUpstreamResponse(resp.StatusCode, upstreamMsg, respBody) {
		appendOpsUpstreamError(c, OpsUpstreamErrorEvent{
			Platform:           account.Platform,
			AccountID:          account.ID,
			AccountName:        account.Name,
			UpstreamStatusCode: resp.StatusCode,
			UpstreamRequestID:  firstNonEmpty(resp.Header.Get("x-request-id"), resp.Header.Get("x-amzn-requestid"), resp.Header.Get("x-amz-request-id")),
			Kind:               "failover",
			Message:            upstreamMsg,
		})
		if s.rateLimitService != nil && !isKiroModelAccessError(resp.StatusCode, upstreamMsg, respBody) {
			s.rateLimitService.HandleUpstreamError(ctx, account, resp.StatusCode, resp.Header, respBody)
		}
		return nil, &UpstreamFailoverError{
			StatusCode:             resp.StatusCode,
			ResponseBody:           respBody,
			ResponseHeaders:        resp.Header.Clone(),
			RetryableOnSameAccount: account.IsPoolMode() && account.IsPoolModeRetryableStatus(resp.StatusCode),
		}
	}

	writeErr(c, mapUpstreamStatusCode(resp.StatusCode), "api_error", upstreamMsg)
	return nil, fmt.Errorf("kiro upstream error: %d %s", resp.StatusCode, upstreamMsg)
}

func (s *OpenAIGatewayService) forwardKiroOAuthAsAnthropic(
	ctx context.Context,
	c *gin.Context,
	account *Account,
	anthropicBody []byte,
	originalModel string,
	billingModel string,
	upstreamModel string,
	clientStream bool,
	startTime time.Time,
) (*OpenAIForwardResult, error) {
	resp, buildResult, err := s.sendKiroOAuthRuntimeRequest(ctx, c, account, anthropicBody, upstreamModel)
	if err != nil {
		safeErr := sanitizeUpstreamErrorMessage(err.Error())
		setOpsUpstreamError(c, 0, safeErr, "")
		appendOpsUpstreamError(c, OpsUpstreamErrorEvent{
			Platform:  account.Platform,
			AccountID: account.ID,
			Kind:      "request_error",
			Message:   safeErr,
		})
		writeAnthropicError(c, http.StatusBadGateway, "api_error", "Upstream request failed")
		return nil, fmt.Errorf("kiro upstream request failed: %s", safeErr)
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode >= 400 {
		return s.handleKiroOAuthErrorResponse(ctx, c, account, resp, writeAnthropicError)
	}

	requestID := firstNonEmpty(resp.Header.Get("x-request-id"), resp.Header.Get("x-amzn-requestid"), resp.Header.Get("x-amz-request-id"))
	inputTokens := estimateKiroInputTokens(anthropicBody)
	if clientStream {
		if s.responseHeaderFilter != nil {
			responseheaders.WriteFilteredHeaders(c.Writer.Header(), resp.Header, s.responseHeaderFilter)
		}
		c.Writer.Header().Set("Content-Type", "text/event-stream")
		c.Writer.Header().Set("Cache-Control", "no-cache")
		c.Writer.Header().Set("Connection", "keep-alive")
		c.Writer.Header().Set("X-Accel-Buffering", "no")
		c.Writer.WriteHeader(http.StatusOK)
		streamResult, err := kiro.StreamEventStreamAsAnthropicWithContext(ctx, resp.Body, c.Writer, originalModel, inputTokens, buildResult.Context)
		if err != nil {
			return nil, err
		}
		return &OpenAIForwardResult{
			RequestID:     requestID,
			Usage:         kiroUsageToOpenAI(streamResult.Usage, inputTokens),
			Model:         originalModel,
			BillingModel:  billingModel,
			UpstreamModel: normalizeKiroRuntimeModel(upstreamModel),
			Stream:        true,
			Duration:      time.Since(startTime),
		}, nil
	}

	parseResult, err := kiro.ParseNonStreamingEventStreamWithContext(resp.Body, originalModel, buildResult.Context)
	if err != nil {
		return nil, err
	}
	if s.responseHeaderFilter != nil {
		responseheaders.WriteFilteredHeaders(c.Writer.Header(), resp.Header, s.responseHeaderFilter)
	}
	c.Data(http.StatusOK, "application/json; charset=utf-8", parseResult.ResponseBody)
	return &OpenAIForwardResult{
		RequestID:     requestID,
		Usage:         kiroUsageToOpenAI(parseResult.Usage, inputTokens),
		Model:         originalModel,
		BillingModel:  billingModel,
		UpstreamModel: normalizeKiroRuntimeModel(upstreamModel),
		Stream:        false,
		Duration:      time.Since(startTime),
	}, nil
}

func (s *OpenAIGatewayService) forwardKiroOAuthAsChatCompletions(
	ctx context.Context,
	c *gin.Context,
	account *Account,
	anthropicBody []byte,
	originalModel string,
	billingModel string,
	upstreamModel string,
	clientStream bool,
	includeUsage bool,
	reasoningEffort *string,
	startTime time.Time,
) (*OpenAIForwardResult, error) {
	resp, buildResult, err := s.sendKiroOAuthRuntimeRequest(ctx, c, account, anthropicBody, upstreamModel)
	if err != nil {
		safeErr := sanitizeUpstreamErrorMessage(err.Error())
		setOpsUpstreamError(c, 0, safeErr, "")
		appendOpsUpstreamError(c, OpsUpstreamErrorEvent{
			Platform:  account.Platform,
			AccountID: account.ID,
			Kind:      "request_error",
			Message:   safeErr,
		})
		writeChatCompletionsError(c, http.StatusBadGateway, "api_error", "Upstream request failed")
		return nil, fmt.Errorf("kiro upstream request failed: %s", safeErr)
	}

	if resp.StatusCode >= 400 {
		defer func() { _ = resp.Body.Close() }()
		return s.handleKiroOAuthErrorResponse(ctx, c, account, resp, writeChatCompletionsError)
	}

	anthropicReader := s.kiroRuntimeAnthropicStream(ctx, resp, buildResult, originalModel, anthropicBody)
	responsesReader := anthropicSSEToResponsesReadCloser(ctx, anthropicReader, originalModel)
	defer func() { _ = responsesReader.Close() }()

	convertedResp := *resp
	convertedResp.Body = responsesReader
	convertedResp.Header = resp.Header.Clone()

	var result *OpenAIForwardResult
	var handleErr error
	if clientStream {
		result, handleErr = s.handleChatStreamingResponse(&convertedResp, c, originalModel, billingModel, normalizeKiroRuntimeModel(upstreamModel), includeUsage, startTime)
	} else {
		result, handleErr = s.handleChatBufferedStreamingResponse(&convertedResp, c, originalModel, billingModel, normalizeKiroRuntimeModel(upstreamModel), startTime)
	}
	if handleErr == nil && result != nil && reasoningEffort != nil {
		result.ReasoningEffort = reasoningEffort
	}
	return result, handleErr
}

func (s *OpenAIGatewayService) kiroRuntimeAnthropicStream(
	ctx context.Context,
	resp *http.Response,
	buildResult *kiro.KiroBuildResult,
	model string,
	anthropicBody []byte,
) io.ReadCloser {
	pr, pw := io.Pipe()
	go func() {
		defer func() { _ = resp.Body.Close() }()
		_, err := kiro.StreamEventStreamAsAnthropicWithContext(ctx, resp.Body, pw, model, estimateKiroInputTokens(anthropicBody), buildResult.Context)
		if err != nil {
			_ = pw.CloseWithError(err)
			return
		}
		_ = pw.Close()
	}()
	return pr
}

func anthropicSSEToResponsesReadCloser(ctx context.Context, anthropicStream io.ReadCloser, model string) io.ReadCloser {
	pr, pw := io.Pipe()
	go func() {
		defer func() { _ = anthropicStream.Close() }()
		writer := bufio.NewWriter(pw)
		state := apicompat.NewAnthropicEventToResponsesState()
		state.Model = model
		scanner := bufio.NewScanner(anthropicStream)
		scanner.Buffer(make([]byte, 0, 64*1024), defaultMaxLineSize)

		writeEvents := func(events []apicompat.ResponsesStreamEvent) error {
			for _, event := range events {
				sse, err := apicompat.ResponsesEventToSSE(event)
				if err != nil {
					return err
				}
				if _, err := writer.WriteString(sse); err != nil {
					return err
				}
				if err := writer.Flush(); err != nil {
					return err
				}
			}
			return nil
		}

		for scanner.Scan() {
			select {
			case <-ctx.Done():
				_ = pw.CloseWithError(ctx.Err())
				return
			default:
			}
			line := scanner.Text()
			if !strings.HasPrefix(line, "event: ") {
				continue
			}
			if !scanner.Scan() {
				break
			}
			dataLine := scanner.Text()
			if !strings.HasPrefix(dataLine, "data: ") {
				continue
			}
			var event apicompat.AnthropicStreamEvent
			if err := json.Unmarshal([]byte(dataLine[6:]), &event); err != nil {
				logger.L().Warn("kiro anthropic stream: failed to parse event", zap.Error(err))
				continue
			}
			if err := writeEvents(apicompat.AnthropicEventToResponsesEvents(&event, state)); err != nil {
				_ = pw.CloseWithError(err)
				return
			}
		}
		if err := scanner.Err(); err != nil {
			_ = pw.CloseWithError(err)
			return
		}
		if err := writeEvents(apicompat.FinalizeAnthropicResponsesStream(state)); err != nil {
			_ = pw.CloseWithError(err)
			return
		}
		_, _ = writer.WriteString("data: [DONE]\n\n")
		_ = writer.Flush()
		_ = pw.Close()
	}()
	return pr
}
