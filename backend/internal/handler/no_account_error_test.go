package handler

import (
	"context"
	"net/http"
	"testing"

	"github.com/stretchr/testify/require"

	"ikik-api/internal/service"
)

type fakeModelAvailabilityDiagnoser struct {
	calls []fakeModelAvailabilityCall
	resp  service.ModelAvailabilityDiagnosis
}

type fakeModelAvailabilityCall struct {
	GroupID  *int64
	Model    string
	Platform string
}

func (f *fakeModelAvailabilityDiagnoser) DiagnoseModelAvailabilityForPlatform(
	_ context.Context,
	groupID *int64,
	model string,
	platform string,
) service.ModelAvailabilityDiagnosis {
	f.calls = append(f.calls, fakeModelAvailabilityCall{
		GroupID:  groupID,
		Model:    model,
		Platform: platform,
	})
	return f.resp
}

func ptrNoAccountGroupID(v int64) *int64 { return &v }

func TestClassifyNoAccountErrorModelUnsupportedReturns404(t *testing.T) {
	fd := &fakeModelAvailabilityDiagnoser{
		resp: service.ModelAvailabilityDiagnosis{HasAccountsInPool: true, HasModelSupport: false},
	}
	apiKey := &service.APIKey{GroupID: ptrNoAccountGroupID(42)}

	cls := classifyNoAccountError(context.Background(), fd, apiKey, "gpt-5.1-codex-mini", "ik-auto-pro", service.PlatformOpenAI)

	require.Equal(t, http.StatusNotFound, cls.Status)
	require.Equal(t, "model_not_found", cls.ErrType)
	require.True(t, cls.ModelNotFound)
	require.Contains(t, cls.Message, "ik-auto-pro")
	require.Len(t, fd.calls, 1)
	require.Equal(t, "gpt-5.1-codex-mini", fd.calls[0].Model)
	require.Equal(t, service.PlatformOpenAI, fd.calls[0].Platform)
}

func TestClassifyNoAccountErrorTransientOrEmptyPoolStays503(t *testing.T) {
	for _, resp := range []service.ModelAvailabilityDiagnosis{
		{HasAccountsInPool: true, HasModelSupport: true},
		{HasAccountsInPool: false, HasModelSupport: false},
	} {
		fd := &fakeModelAvailabilityDiagnoser{resp: resp}
		apiKey := &service.APIKey{GroupID: ptrNoAccountGroupID(7)}

		cls := classifyNoAccountError(context.Background(), fd, apiKey, "gpt-5", "gpt-5", service.PlatformOpenAI)

		require.Equal(t, http.StatusServiceUnavailable, cls.Status)
		require.Equal(t, "api_error", cls.ErrType)
		require.False(t, cls.ModelNotFound)
	}
}
