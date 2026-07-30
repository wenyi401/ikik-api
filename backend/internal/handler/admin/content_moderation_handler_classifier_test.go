//go:build unit

package admin

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"

	"ikik-api/internal/service"
)

type contentModerationHandlerSettingRepo struct {
	values map[string]string
}

func (r *contentModerationHandlerSettingRepo) Get(context.Context, string) (*service.Setting, error) {
	return nil, service.ErrSettingNotFound
}

func (r *contentModerationHandlerSettingRepo) GetValue(_ context.Context, key string) (string, error) {
	value, ok := r.values[key]
	if !ok {
		return "", service.ErrSettingNotFound
	}
	return value, nil
}

func (r *contentModerationHandlerSettingRepo) Set(_ context.Context, key, value string) error {
	r.values[key] = value
	return nil
}

func (r *contentModerationHandlerSettingRepo) GetMultiple(context.Context, []string) (map[string]string, error) {
	return nil, nil
}

func (r *contentModerationHandlerSettingRepo) SetMultiple(context.Context, map[string]string) error {
	return nil
}

func (r *contentModerationHandlerSettingRepo) GetAll(context.Context) (map[string]string, error) {
	return r.values, nil
}

func (r *contentModerationHandlerSettingRepo) Delete(_ context.Context, key string) error {
	delete(r.values, key)
	return nil
}

func TestContentModerationHandlerPersistsClassifierRoutingFields(t *testing.T) {
	gin.SetMode(gin.TestMode)
	repo := &contentModerationHandlerSettingRepo{values: map[string]string{}}
	svc := service.NewContentModerationService(repo, nil, nil, nil, nil, nil, nil)
	handler := NewContentModerationHandler(svc)
	router := gin.New()
	router.PUT("/config", handler.UpdateConfig)

	body := []byte(`{
		"moderation_provider":"openai",
		"classifier_group_id":16,
		"classifier_models":["gpt-5.4-mini","gpt-5.4-mini-fallback"]
	}`)
	recorder := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodPut, "/config", bytes.NewReader(body))
	request.Header.Set("Content-Type", "application/json")
	router.ServeHTTP(recorder, request)

	require.Equal(t, http.StatusOK, recorder.Code, recorder.Body.String())
	var saved service.ContentModerationConfig
	require.NoError(t, json.Unmarshal([]byte(repo.values[service.SettingKeyContentModerationConfig]), &saved))
	require.Equal(t, int64(16), saved.ClassifierGroupID)
	require.Equal(t, []string{"gpt-5.4-mini", "gpt-5.4-mini-fallback"}, saved.ClassifierModels)
}

func TestContentModerationAPIKeyTestRequestBindsClassifierRoutingFields(t *testing.T) {
	var request contentModerationAPIKeyTestRequest
	require.NoError(t, json.Unmarshal([]byte(`{
		"classifier_group_id":16,
		"classifier_models":["gpt-5.4-mini","gpt-5.4-mini-fallback"]
	}`), &request))

	require.Equal(t, int64(16), request.ClassifierGroupID)
	require.Equal(t, []string{"gpt-5.4-mini", "gpt-5.4-mini-fallback"}, request.ClassifierModels)
}
