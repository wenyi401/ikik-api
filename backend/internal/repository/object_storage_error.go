package repository

import (
	"errors"
	"fmt"
	"strings"

	"github.com/aws/smithy-go"
	infraerrors "ikik-api/internal/pkg/errors"
)

func mapObjectStorageError(operation string, err error) error {
	if err == nil {
		return nil
	}

	cause := fmt.Errorf("%s: %w", operation, err)
	var apiErr smithy.APIError
	if errors.As(err, &apiErr) {
		switch strings.ToLower(strings.TrimSpace(apiErr.ErrorCode())) {
		case "invalidaccesskeyid", "invalidaccesskey", "signaturedoesnotmatch", "invalidsignatureexception":
			return infraerrors.ServiceUnavailable(
				"OBJECT_STORAGE_CREDENTIALS_INVALID",
				"object storage credentials are invalid; update the AccessKey ID and Secret AccessKey in Data Management",
			).WithCause(cause)
		case "accessdenied", "forbidden", "allaccessdisabled":
			return infraerrors.ServiceUnavailable(
				"OBJECT_STORAGE_ACCESS_DENIED",
				"object storage access was denied; verify the bucket permissions in Data Management",
			).WithCause(cause)
		case "nosuchbucket", "notfound", "noexistbucket":
			return infraerrors.ServiceUnavailable(
				"OBJECT_STORAGE_BUCKET_NOT_FOUND",
				"object storage bucket was not found; verify the bucket and endpoint in Data Management",
			).WithCause(cause)
		}
	}

	return infraerrors.ServiceUnavailable(
		"OBJECT_STORAGE_UNAVAILABLE",
		"object storage request failed; verify the storage settings in Data Management",
	).WithCause(cause)
}
