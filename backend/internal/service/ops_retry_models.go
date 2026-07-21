package service

import "time"

type OpsRetryAttempt struct {
	ID                int64      `json:"id"`
	CreatedAt         time.Time  `json:"created_at"`
	RequestedByUserID int64      `json:"requested_by_user_id"`
	SourceErrorID     int64      `json:"source_error_id"`
	Mode              string     `json:"mode"`
	PinnedAccountID   *int64     `json:"pinned_account_id"`
	PinnedAccountName string     `json:"pinned_account_name"`
	Status            string     `json:"status"`
	StartedAt         *time.Time `json:"started_at"`
	FinishedAt        *time.Time `json:"finished_at"`
	DurationMs        *int64     `json:"duration_ms"`
	Success           *bool      `json:"success"`
	HTTPStatusCode    *int       `json:"http_status_code"`
	UpstreamRequestID *string    `json:"upstream_request_id"`
	UsedAccountID     *int64     `json:"used_account_id"`
	UsedAccountName   string     `json:"used_account_name"`
	ResponsePreview   *string    `json:"response_preview"`
	ResponseTruncated *bool      `json:"response_truncated"`
	ResultRequestID   *string    `json:"result_request_id"`
	ResultErrorID     *int64     `json:"result_error_id"`
	ErrorMessage      *string    `json:"error_message"`
}

type OpsRetryResult struct {
	AttemptID int64  `json:"attempt_id"`
	Mode      string `json:"mode"`
	Status    string `json:"status"`

	PinnedAccountID *int64 `json:"pinned_account_id"`
	UsedAccountID   *int64 `json:"used_account_id"`

	HTTPStatusCode    int    `json:"http_status_code"`
	UpstreamRequestID string `json:"upstream_request_id"`

	ResponsePreview   string `json:"response_preview"`
	ResponseTruncated bool   `json:"response_truncated"`
	ErrorMessage      string `json:"error_message"`

	StartedAt  time.Time `json:"started_at"`
	FinishedAt time.Time `json:"finished_at"`
	DurationMs int64     `json:"duration_ms"`
}
