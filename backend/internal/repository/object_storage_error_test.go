package repository

import (
	"errors"
	"net/http"
	"testing"

	"github.com/aws/smithy-go"
	"github.com/stretchr/testify/require"
	infraerrors "ikik-api/internal/pkg/errors"
)

func TestMapObjectStorageError(t *testing.T) {
	tests := []struct {
		name    string
		code    string
		reason  string
		message string
	}{
		{
			name:    "invalid credentials",
			code:    "InvalidAccessKeyId",
			reason:  "OBJECT_STORAGE_CREDENTIALS_INVALID",
			message: "object storage credentials are invalid; update the AccessKey ID and Secret AccessKey in Data Management",
		},
		{
			name:    "access denied",
			code:    "AccessDenied",
			reason:  "OBJECT_STORAGE_ACCESS_DENIED",
			message: "object storage access was denied; verify the bucket permissions in Data Management",
		},
		{
			name:    "bucket missing",
			code:    "NoSuchBucket",
			reason:  "OBJECT_STORAGE_BUCKET_NOT_FOUND",
			message: "object storage bucket was not found; verify the bucket and endpoint in Data Management",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			upstream := &smithy.GenericAPIError{Code: tt.code, Message: "upstream detail"}
			err := mapObjectStorageError("test operation", upstream)

			require.Equal(t, http.StatusServiceUnavailable, infraerrors.Code(err))
			require.Equal(t, tt.reason, infraerrors.Reason(err))
			require.Equal(t, tt.message, infraerrors.Message(err))
			require.ErrorContains(t, err, "test operation")
		})
	}
}

func TestMapObjectStorageErrorFallsBackToUnavailable(t *testing.T) {
	err := mapObjectStorageError("put object", errors.New("network failure"))

	require.Equal(t, http.StatusServiceUnavailable, infraerrors.Code(err))
	require.Equal(t, "OBJECT_STORAGE_UNAVAILABLE", infraerrors.Reason(err))
	require.ErrorContains(t, err, "put object")
}
