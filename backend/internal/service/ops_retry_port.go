package service

import "time"

type OpsInsertRetryAttemptInput struct {
	RequestedByUserID int64
	SourceErrorID     int64
	Mode              string
	PinnedAccountID   *int64
	Status            string
	StartedAt         time.Time
}

type OpsUpdateRetryAttemptInput struct {
	ID                int64
	Status            string
	FinishedAt        time.Time
	DurationMs        int64
	Success           *bool
	HTTPStatusCode    *int
	UpstreamRequestID *string
	UsedAccountID     *int64
	ResponsePreview   *string
	ResponseTruncated *bool
	ResultRequestID   *string
	ResultErrorID     *int64
	ErrorMessage      *string
}
