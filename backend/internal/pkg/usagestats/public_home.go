package usagestats

// PublicTodayUsageStats contains the public homepage counters and health data.
type PublicTodayUsageStats struct {
	TodayRequests       int64    `json:"today_requests"`
	TodayTokens         int64    `json:"today_tokens"`
	SuccessCount        int64    `json:"success_count"`
	ErrorCount          int64    `json:"error_count"`
	SuccessRate         *float64 `json:"success_rate"`
	AverageDurationMs   *float64 `json:"average_duration_ms"`
	AverageFirstTokenMs *float64 `json:"average_first_token_ms"`
}
