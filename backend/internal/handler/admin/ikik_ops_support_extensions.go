package admin

type opsRetryRequest struct {
	Mode            string `json:"mode"`
	PinnedAccountID *int64 `json:"pinned_account_id"`
	Force           bool   `json:"force"`
}
