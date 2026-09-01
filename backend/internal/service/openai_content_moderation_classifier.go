package service

import (
	"bytes"
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"

	"github.com/gin-gonic/gin"
)

const defaultContentModerationClassifierAccountSwitches = 3

// ForwardContentModerationClassifier runs a non-streaming classifier request
// through the existing group scheduler without making a public HTTP round trip.
func (s *OpenAIGatewayService) ForwardContentModerationClassifier(
	ctx context.Context,
	input ContentModerationClassifierGatewayInput,
) (*ContentModerationClassifierGatewayResponse, error) {
	if s == nil {
		return nil, newContentModerationClassifierGatewayError(0, "OpenAI gateway is unavailable")
	}
	if input.GroupID <= 0 {
		return nil, newContentModerationClassifierGatewayError(http.StatusBadRequest, "classifier group is required")
	}
	model := strings.TrimSpace(input.Model)
	if model == "" {
		return nil, newContentModerationClassifierGatewayError(http.StatusBadRequest, "classifier model is required")
	}
	platform := normalizeOpenAICompatiblePlatform(input.Platform)
	if !isContentModerationClassifierPlatform(platform) {
		return nil, newContentModerationClassifierGatewayError(http.StatusBadRequest, "classifier group platform is not OpenAI-compatible")
	}

	groupID := input.GroupID
	failedAccountIDs := make(map[int64]struct{})
	maxSwitches := defaultContentModerationClassifierAccountSwitches
	if s.cfg != nil && s.cfg.Gateway.MaxAccountSwitches > 0 {
		maxSwitches = s.cfg.Gateway.MaxAccountSwitches
	}
	var lastErr error

	for switchCount := 0; switchCount <= maxSwitches; switchCount++ {
		if err := ctx.Err(); err != nil {
			return nil, newContentModerationClassifierGatewayError(0, err.Error())
		}
		selection, _, err := s.SelectAccountWithSchedulerForCapability(
			ctx,
			&groupID,
			"",
			"",
			model,
			failedAccountIDs,
			OpenAIUpstreamTransportAny,
			OpenAIEndpointCapabilityChatCompletions,
			false,
			false,
			true,
			platform,
		)
		if err != nil {
			lastErr = err
			break
		}
		if selection == nil || selection.Account == nil {
			lastErr = ErrNoAvailableAccounts
			break
		}

		account := selection.Account
		release, acquired, acquireErr := s.acquireContentModerationClassifierAccountSlot(ctx, selection)
		if acquireErr != nil {
			lastErr = acquireErr
			failedAccountIDs[account.ID] = struct{}{}
			continue
		}
		if !acquired {
			lastErr = ErrNoAvailableAccounts
			failedAccountIDs[account.ID] = struct{}{}
			continue
		}

		response, forwardErr := func() (*ContentModerationClassifierGatewayResponse, error) {
			defer release()
			return s.forwardContentModerationClassifierAccount(ctx, &groupID, platform, model, account, input.Body)
		}()
		if forwardErr == nil {
			s.ReportOpenAIAccountScheduleResult(account, account.GetMappedModel(model), true, nil)
			return response, nil
		}

		lastErr = forwardErr
		var failoverErr *UpstreamFailoverError
		if !errors.As(forwardErr, &failoverErr) || failoverErr == nil {
			s.ReportOpenAIAccountScheduleResult(account, account.GetMappedModel(model), false, nil)
			break
		}
		if failoverErr.ShouldReportAccountScheduleFailure() {
			s.ReportOpenAIAccountScheduleResult(account, account.GetMappedModel(model), false, nil)
		}
		if !failoverErr.ShouldRetryNextAccount() {
			break
		}
		failedAccountIDs[account.ID] = struct{}{}
	}

	return nil, contentModerationClassifierGatewayErrorFrom(lastErr)
}

func (s *OpenAIGatewayService) acquireContentModerationClassifierAccountSlot(
	ctx context.Context,
	selection *AccountSelectionResult,
) (func(), bool, error) {
	if selection == nil || selection.Account == nil {
		return func() {}, false, nil
	}
	if selection.Acquired {
		if selection.ReleaseFunc == nil {
			return func() {}, true, nil
		}
		return selection.ReleaseFunc, true, nil
	}
	result, err := s.tryAcquireAccountSlot(ctx, selection.Account.ID, selection.Account.Concurrency)
	if err != nil {
		return func() {}, false, err
	}
	if result == nil || !result.Acquired {
		return func() {}, false, nil
	}
	if result.ReleaseFunc == nil {
		return func() {}, true, nil
	}
	return result.ReleaseFunc, true, nil
}

func (s *OpenAIGatewayService) forwardContentModerationClassifierAccount(
	ctx context.Context,
	groupID *int64,
	platform string,
	model string,
	account *Account,
	body []byte,
) (*ContentModerationClassifierGatewayResponse, error) {
	forwardBody := body
	if mapping, _ := s.ResolveChannelMappingAndRestrict(ctx, groupID, model); mapping.Mapped {
		forwardBody = s.ReplaceModelInBody(body, mapping.MappedModel)
	}

	recorder := httptest.NewRecorder()
	ginContext, _ := gin.CreateTestContext(recorder)
	request, err := http.NewRequestWithContext(ctx, http.MethodPost, "/v1/chat/completions", bytes.NewReader(forwardBody))
	if err != nil {
		return nil, newContentModerationClassifierGatewayError(0, err.Error())
	}
	request.Header.Set("Content-Type", "application/json")
	request.Header.Set("User-Agent", "ikik-api/internal-moderation")
	ginContext.Request = request

	_, err = s.ForwardAsChatCompletions(ctx, ginContext, account, forwardBody, "", "")
	status := recorder.Code
	if status == 0 {
		status = http.StatusOK
	}
	if err != nil {
		var failoverErr *UpstreamFailoverError
		if errors.As(err, &failoverErr) && failoverErr != nil {
			return nil, failoverErr
		}
		return nil, newContentModerationClassifierGatewayError(status, classifierGatewayErrorMessage(recorder.Body.Bytes(), err))
	}
	if status < http.StatusOK || status >= http.StatusMultipleChoices {
		return nil, newContentModerationClassifierGatewayError(status, classifierGatewayErrorMessage(recorder.Body.Bytes(), nil))
	}
	if recorder.Body.Len() == 0 {
		return nil, newContentModerationClassifierGatewayError(http.StatusBadGateway, "classifier returned an empty response")
	}
	return &ContentModerationClassifierGatewayResponse{
		StatusCode: status,
		Body:       append([]byte(nil), recorder.Body.Bytes()...),
		AccountID:  account.ID,
	}, nil
}

func contentModerationClassifierGatewayErrorFrom(err error) error {
	if err == nil {
		return newContentModerationClassifierGatewayError(http.StatusServiceUnavailable, "no available classifier account")
	}
	var gatewayErr *ContentModerationClassifierGatewayError
	if errors.As(err, &gatewayErr) {
		return gatewayErr
	}
	var failoverErr *UpstreamFailoverError
	if errors.As(err, &failoverErr) && failoverErr != nil {
		return newContentModerationClassifierGatewayError(failoverErr.StatusCode, classifierGatewayErrorMessage(failoverErr.ResponseBody, err))
	}
	if errors.Is(err, ErrNoAvailableAccounts) {
		return newContentModerationClassifierGatewayError(http.StatusServiceUnavailable, "no available classifier account")
	}
	return newContentModerationClassifierGatewayError(0, err.Error())
}

func newContentModerationClassifierGatewayError(status int, message string) error {
	message = trimRunes(strings.TrimSpace(message), maxModerationExcerptRunes)
	if message == "" {
		message = http.StatusText(status)
	}
	return &ContentModerationClassifierGatewayError{StatusCode: status, Message: message}
}

func classifierGatewayErrorMessage(body []byte, fallback error) string {
	message := strings.TrimSpace(string(body))
	if message == "" && fallback != nil {
		message = fallback.Error()
	}
	return trimRunes(message, maxModerationExcerptRunes)
}
