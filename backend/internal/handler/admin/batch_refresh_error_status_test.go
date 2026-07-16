//go:build unit

package admin

import (
	"context"
	"errors"
	"net/http"
	"testing"

	"github.com/stretchr/testify/require"

	infraerrors "ikik-api/internal/pkg/errors"
)

func TestBatchRefreshUnauthorizedErrorMarksAccountError(t *testing.T) {
	svc := newStubAdminService()
	handler := newBatchRefreshTestHandler(svc)
	err := infraerrors.Unauthorized("UPSTREAM_UNAUTHORIZED", "refresh token unauthorized")

	require.NoError(t, handler.markBatchRefreshAccountError(context.Background(), 42, err))
	require.Len(t, svc.setAccountErrorCalls, 1)
	require.Equal(t, int64(42), svc.setAccountErrorCalls[0].accountID)
	require.Contains(t, svc.setAccountErrorCalls[0].message, "refresh token unauthorized")
}

func TestBatchRefreshNonUnauthorizedErrorDoesNotMarkAccountError(t *testing.T) {
	svc := newStubAdminService()
	handler := newBatchRefreshTestHandler(svc)

	require.NoError(t, handler.markBatchRefreshAccountError(
		context.Background(),
		43,
		infraerrors.BadRequest("NOT_OAUTH", "cannot refresh non-OAuth account"),
	))
	require.NoError(t, handler.markBatchRefreshAccountError(
		context.Background(),
		44,
		errors.New("upstream timeout"),
	))
	require.Empty(t, svc.setAccountErrorCalls)
}

func TestBatchRefreshHTTP401TextErrorMarksAccountError(t *testing.T) {
	svc := newStubAdminService()
	handler := newBatchRefreshTestHandler(svc)
	err := errors.New("token refresh failed (HTTP 401): invalid refresh token")

	require.NoError(t, handler.markBatchRefreshAccountError(context.Background(), 45, err))
	require.Len(t, svc.setAccountErrorCalls, 1)
	require.Equal(t, int64(45), svc.setAccountErrorCalls[0].accountID)
}

func TestBatchRefreshHTTPStatusClassifier(t *testing.T) {
	require.Equal(t, http.StatusUnauthorized, batchRefreshErrorHTTPStatus(
		infraerrors.Unauthorized("UPSTREAM_UNAUTHORIZED", "refresh token unauthorized"),
	))
	require.Equal(t, http.StatusBadRequest, batchRefreshErrorHTTPStatus(
		infraerrors.BadRequest("NOT_OAUTH", "cannot refresh non-OAuth account"),
	))
	require.Equal(t, http.StatusUnauthorized, batchRefreshErrorHTTPStatus(
		errors.New("token refresh failed (HTTP 401): invalid refresh token"),
	))
	require.Equal(t, 0, batchRefreshErrorHTTPStatus(errors.New("temporary network error")))
}

func newBatchRefreshTestHandler(svc *stubAdminService) *AccountHandler {
	return NewAccountHandler(
		svc,
		nil, // accountService
		nil, // oauthService
		nil, // openaiOAuthService
		nil, // geminiOAuthService
		nil, // antigravityOAuthService
		nil, // kiroOAuthService
		nil, // rateLimitService
		nil, // accountUsageService
		nil, // accountTestService
		nil, // concurrencyService
		nil, // crsSyncService
		nil, // sessionLimitCache
		nil, // rpmCache
		nil, // tokenCacheInvalidator
	)
}
