package service

import (
	"ikik-api/internal/domain"
)

// Admin adjustment type constants
const (
	AdjustmentTypeAdminPoints = domain.AdjustmentTypeAdminPoints
)

// Carpool owner-paid service fee defaults (USD).
const (
	CarpoolBaseServiceFeeUSDDefault = 75.0
)

// Carpool owner-paid service fee defaults (USD).
const (
	CarpoolRiskControlFeeUSDDefault = 15.0
)

// Carpool owner-paid service fee defaults (USD).
const (
	CarpoolSystemProxyFeeUSDDefault = 10.0
)

// Redeem type constants
const (
	RedeemTypePoints = domain.RedeemTypePoints
)

// Setting keys
const (
	SettingKeyAutoModelSettings = "auto_model_settings"
)

// Setting keys
const (
	SettingKeyCarpoolBaseServiceFeeUSD = "carpool_base_service_fee_usd"
)

// Setting keys
const (
	SettingKeyCarpoolEnabled = "carpool_enabled"
)

// Setting keys
const (
	SettingKeyCarpoolRiskControlFeeUSD = "carpool_risk_control_fee_usd"
)

// Setting keys
const (
	SettingKeyCarpoolSystemProxyFeeUSD = "carpool_system_proxy_fee_usd"
)

// Setting keys
const (
	SettingKeyFreeModelsEnabled = "free_models_enabled"
)

// Setting keys
const (
	SettingKeyHomeStatsGroupID = "home_stats_group_id"
)

// Setting keys
const (
	SettingKeyOpenAIFreeAccountRepairEnabled = "openai_free_account_repair_enabled"
)

// Setting keys
const (
	SettingKeyOpenAIFreeAccountRepairWeeklyThresholdUSD = "openai_free_account_repair_weekly_threshold_usd"
)

// Setting keys
const (
	SettingKeyOpenAIImagesResponsesReasoningEffort = "openai_images_responses_reasoning_effort"
)

// Setting keys
const (
	SettingKeyUserPrivateGroupCommissionRate = "user_private_group_commission_rate"
)

// Setting keys
const (
	SettingKeyUserPrivateGroupDailyLimitUSD = "user_private_group_daily_limit_usd"
)

// Setting keys
const (
	SettingKeyUserPrivateGroupMonthlyLimitUSD = "user_private_group_monthly_limit_usd"
)

// Setting keys
const (
	SettingKeyUserPrivateGroupRPMLimit = "user_private_group_rpm_limit"
)

// Setting keys
const (
	SettingKeyUserPrivateGroupRateMultiplier = "user_private_group_rate_multiplier"
)

// Setting keys
const (
	SettingKeyUserPrivateGroupWeeklyLimitUSD = "user_private_group_weekly_limit_usd"
)
const (
	UserDefaultConcurrency = 5
)
const (
	UserMinConcurrency = 1
)
