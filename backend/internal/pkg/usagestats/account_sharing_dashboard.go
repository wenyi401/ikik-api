package usagestats

// AccountSharingDashboardStats summarizes usage and settlement for accounts owned by one user.
type AccountSharingDashboardStats struct {
	Summary     AccountSharingSummary       `json:"summary"`
	Accounts    []AccountSharingAccountStat `json:"accounts"`
	AccountPage AccountSharingAccountPage   `json:"accounts_pagination"`
	Trend       []AccountSharingTrendPoint  `json:"trend"`
	StartDate   string                      `json:"start_date"`
	EndDate     string                      `json:"end_date"`
	Granularity string                      `json:"granularity"`
}

type AccountSharingSummary struct {
	OwnedAccounts           int64   `json:"owned_accounts"`
	PrivateAccounts         int64   `json:"private_accounts"`
	PublicPendingAccounts   int64   `json:"public_pending_accounts"`
	PublicApprovedAccounts  int64   `json:"public_approved_accounts"`
	PublicSuspendedAccounts int64   `json:"public_suspended_accounts"`
	SelfRequests            int64   `json:"self_requests"`
	SelfTokens              int64   `json:"self_tokens"`
	SelfActualCost          float64 `json:"self_actual_cost"`
	SelfAccountCost         float64 `json:"self_account_cost"`
	ExternalRequests        int64   `json:"external_requests"`
	ExternalConsumerCharge  float64 `json:"external_consumer_charge"`
	ExternalAccountCost     float64 `json:"external_account_cost"`
	ExternalOwnerCredit     float64 `json:"external_owner_credit"`
	ExternalPlatformFee     float64 `json:"external_platform_fee"`
	TotalAccountCost        float64 `json:"total_account_cost"`
	BalanceNetChange        float64 `json:"balance_net_change"`
}

type AccountSharingAccountStat struct {
	AccountID              int64   `json:"account_id"`
	Name                   string  `json:"name"`
	Platform               string  `json:"platform"`
	ShareMode              string  `json:"share_mode"`
	ShareStatus            string  `json:"share_status"`
	SelfRequests           int64   `json:"self_requests"`
	SelfTokens             int64   `json:"self_tokens"`
	SelfActualCost         float64 `json:"self_actual_cost"`
	SelfAccountCost        float64 `json:"self_account_cost"`
	ExternalRequests       int64   `json:"external_requests"`
	ExternalConsumerCharge float64 `json:"external_consumer_charge"`
	ExternalAccountCost    float64 `json:"external_account_cost"`
	ExternalOwnerCredit    float64 `json:"external_owner_credit"`
	ExternalPlatformFee    float64 `json:"external_platform_fee"`
}

type AccountSharingAccountPage struct {
	Total    int64 `json:"total"`
	Page     int   `json:"page"`
	PageSize int   `json:"page_size"`
	Pages    int   `json:"pages"`
}

type AccountSharingTrendPoint struct {
	Date                   string  `json:"date"`
	SelfRequests           int64   `json:"self_requests"`
	SelfTokens             int64   `json:"self_tokens"`
	SelfActualCost         float64 `json:"self_actual_cost"`
	SelfAccountCost        float64 `json:"self_account_cost"`
	ExternalRequests       int64   `json:"external_requests"`
	ExternalConsumerCharge float64 `json:"external_consumer_charge"`
	ExternalAccountCost    float64 `json:"external_account_cost"`
	ExternalOwnerCredit    float64 `json:"external_owner_credit"`
	ExternalPlatformFee    float64 `json:"external_platform_fee"`
}
