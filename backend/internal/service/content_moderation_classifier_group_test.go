//go:build unit

package service

import (
	"context"
	"net/http"
	"testing"

	"github.com/stretchr/testify/require"
)

type contentModerationClassifierGroupRepo struct {
	stubGroupRepoForAvailable
	group *Group
}

func (r *contentModerationClassifierGroupRepo) GetByIDLite(context.Context, int64) (*Group, error) {
	return r.group, nil
}

type contentModerationClassifierGatewayStub struct {
	models []string
	call   func(ContentModerationClassifierGatewayInput) (*ContentModerationClassifierGatewayResponse, error)
}

func (s *contentModerationClassifierGatewayStub) ForwardContentModerationClassifier(
	_ context.Context,
	input ContentModerationClassifierGatewayInput,
) (*ContentModerationClassifierGatewayResponse, error) {
	s.models = append(s.models, input.Model)
	return s.call(input)
}

func TestModelClassifierGroupFallsBackAfterRateLimit(t *testing.T) {
	repo := &contentModerationClassifierGroupRepo{group: &Group{
		ID:       42,
		Name:     "classifier",
		Platform: PlatformOpenAI,
		Status:   StatusActive,
	}}
	gateway := &contentModerationClassifierGatewayStub{
		call: func(input ContentModerationClassifierGatewayInput) (*ContentModerationClassifierGatewayResponse, error) {
			if input.Model == "classifier-primary" {
				return nil, &ContentModerationClassifierGatewayError{
					StatusCode: http.StatusTooManyRequests,
					Message:    "rate limited",
				}
			}
			return &ContentModerationClassifierGatewayResponse{
				StatusCode: http.StatusOK,
				Body:       []byte(`{"choices":[{"message":{"content":"{\"decision\":\"safe\",\"category\":\"none\",\"confidence\":0.99,\"severity\":0}"}}]}`),
			}, nil
		},
	}
	svc := &ContentModerationService{groupRepo: repo}
	svc.SetClassifierGateway(gateway)
	cfg := defaultContentModerationConfig()
	cfg.ModerationProvider = ContentModerationProviderModelClassifier
	cfg.ClassifierGroupID = 42
	cfg.ClassifierModels = []string{"classifier-primary", "classifier-fallback"}
	cfg.TimeoutMS = 1000

	status := 0
	result, trace, err := svc.callModelClassifierThroughGroupWithTrace(context.Background(), cfg, "hello", &status)

	require.NoError(t, err)
	require.False(t, result.Flagged)
	require.Equal(t, "classifier-fallback", result.ClassifierModel)
	require.Equal(t, http.StatusOK, status)
	require.Equal(t, []string{"classifier-primary", "classifier-fallback"}, gateway.models)
	require.Equal(t, int64(42), trace.GroupID)
	require.Equal(t, "classifier", trace.GroupName)
	require.Len(t, trace.Attempts, 2)
	require.Equal(t, http.StatusTooManyRequests, trace.Attempts[0].StatusCode)
	require.False(t, trace.Attempts[0].Success)
	require.Contains(t, trace.Attempts[0].Error, "rate limited")
	require.Equal(t, http.StatusOK, trace.Attempts[1].StatusCode)
	require.True(t, trace.Attempts[1].Success)
}

func TestModelClassifierGroupDoesNotFallbackAfterBadRequest(t *testing.T) {
	repo := &contentModerationClassifierGroupRepo{group: &Group{
		ID:       42,
		Name:     "classifier",
		Platform: PlatformOpenAI,
		Status:   StatusActive,
	}}
	gateway := &contentModerationClassifierGatewayStub{
		call: func(ContentModerationClassifierGatewayInput) (*ContentModerationClassifierGatewayResponse, error) {
			return nil, &ContentModerationClassifierGatewayError{
				StatusCode: http.StatusBadRequest,
				Message:    "invalid request",
			}
		},
	}
	svc := &ContentModerationService{groupRepo: repo}
	svc.SetClassifierGateway(gateway)
	cfg := defaultContentModerationConfig()
	cfg.ModerationProvider = ContentModerationProviderModelClassifier
	cfg.ClassifierGroupID = 42
	cfg.ClassifierModels = []string{"classifier-primary", "classifier-fallback"}
	cfg.TimeoutMS = 1000

	status := 0
	_, trace, err := svc.callModelClassifierThroughGroupWithTrace(context.Background(), cfg, "hello", &status)

	require.Error(t, err)
	require.Equal(t, http.StatusBadRequest, status)
	require.Equal(t, []string{"classifier-primary"}, gateway.models)
	require.Len(t, trace.Attempts, 1)
	require.Equal(t, http.StatusBadRequest, trace.Attempts[0].StatusCode)
	require.False(t, trace.Attempts[0].Success)
	require.Contains(t, trace.Attempts[0].Error, "invalid request")
}
