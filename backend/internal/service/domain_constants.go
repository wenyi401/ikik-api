package service

import "ikik-api/internal/domain"

// Status constants
const (
	StatusActive   = domain.StatusActive
	StatusDisabled = domain.StatusDisabled
	StatusError    = domain.StatusError
	StatusUnused   = domain.StatusUnused
	StatusUsed     = domain.StatusUsed
	StatusExpired  = domain.StatusExpired
)

// Role constants
const (
	RoleAdmin = domain.RoleAdmin
	RoleUser  = domain.RoleUser
)

const (
	UserMinConcurrency     = 1
	UserDefaultConcurrency = 5
)

// Affiliate rebate settings
const (
	AffiliateRebateRateDefault          = 20.0
	AffiliateRebateRateMin              = 0.0
	AffiliateRebateRateMax              = 100.0
	AffiliateEnabledDefault             = false
	AffiliateRebateFreezeHoursDefault   = 0
	AffiliateRebateFreezeHoursMax       = 720
	AffiliateRebateDurationDaysDefault  = 0
	AffiliateRebateDurationDaysMax      = 3650
	AffiliateRebatePerInviteeCapDefault = 0.0
)

// Carpool owner-paid service fee defaults (USD).
const (
	CarpoolBaseServiceFeeUSDDefault = 75.0
	CarpoolSystemProxyFeeUSDDefault = 10.0
	CarpoolRiskControlFeeUSDDefault = 15.0
)

// Platform constants
const (
	PlatformAnthropic   = domain.PlatformAnthropic
	PlatformOpenAI      = domain.PlatformOpenAI
	PlatformGemini      = domain.PlatformGemini
	PlatformAntigravity = domain.PlatformAntigravity
	PlatformGrok        = domain.PlatformGrok
	PlatformKiro        = domain.PlatformKiro
	PlatformCustom      = domain.PlatformCustom
)

// Account type constants
const (
	AccountTypeOAuth          = domain.AccountTypeOAuth
	AccountTypeSetupToken     = domain.AccountTypeSetupToken
	AccountTypeAPIKey         = domain.AccountTypeAPIKey
	AccountTypeUpstream       = domain.AccountTypeUpstream
	AccountTypeBedrock        = domain.AccountTypeBedrock
	AccountTypeServiceAccount = domain.AccountTypeServiceAccount
)

// Redeem type constants
const (
	RedeemTypeBalance      = domain.RedeemTypeBalance
	RedeemTypePoints       = domain.RedeemTypePoints
	RedeemTypeConcurrency  = domain.RedeemTypeConcurrency
	RedeemTypeSubscription = domain.RedeemTypeSubscription
	RedeemTypeInvitation   = domain.RedeemTypeInvitation
)

// PromoCode status constants
const (
	PromoCodeStatusActive   = domain.PromoCodeStatusActive
	PromoCodeStatusDisabled = domain.PromoCodeStatusDisabled
)

// Admin adjustment type constants
const (
	AdjustmentTypeAdminBalance     = domain.AdjustmentTypeAdminBalance
	AdjustmentTypeAdminPoints      = domain.AdjustmentTypeAdminPoints
	AdjustmentTypeAdminConcurrency = domain.AdjustmentTypeAdminConcurrency
)

// Group subscription type constants
const (
	SubscriptionTypeStandard     = domain.SubscriptionTypeStandard
	SubscriptionTypeSubscription = domain.SubscriptionTypeSubscription
)

// Group scope constants
const (
	GroupScopePublic      = domain.GroupScopePublic
	GroupScopeUserPrivate = domain.GroupScopeUserPrivate
	GroupScopeUserCarpool = domain.GroupScopeUserCarpool
)

// Subscription status constants
const (
	SubscriptionStatusActive    = domain.SubscriptionStatusActive
	SubscriptionStatusExpired   = domain.SubscriptionStatusExpired
	SubscriptionStatusRevoked   = domain.SubscriptionStatusRevoked
	SubscriptionStatusSuspended = domain.SubscriptionStatusSuspended
)

const LinuxDoConnectSyntheticEmailDomain = "@linuxdo-connect.invalid"
const OIDCConnectSyntheticEmailDomain = "@oidc-connect.invalid"
const WeChatConnectSyntheticEmailDomain = "@wechat-connect.invalid"

// Setting keys
const (
	SettingKeyRegistrationEnabled              = "registration_enabled"
	SettingKeyEmailVerifyEnabled               = "email_verify_enabled"
	SettingKeyRegistrationEmailSuffixWhitelist = "registration_email_suffix_whitelist"
	SettingKeyPromoCodeEnabled                 = "promo_code_enabled"
	SettingKeyPasswordResetEnabled             = "password_reset_enabled"
	SettingKeyFrontendURL                      = "frontend_url"
	SettingKeyInvitationCodeEnabled            = "invitation_code_enabled"
	SettingKeyAffiliateEnabled                 = "affiliate_enabled"
	SettingKeyAffiliateRebateRate              = "affiliate_rebate_rate"
	SettingKeyAffiliateRebateFreezeHours       = "affiliate_rebate_freeze_hours"
	SettingKeyAffiliateRebateDurationDays      = "affiliate_rebate_duration_days"
	SettingKeyAffiliateRebatePerInviteeCap     = "affiliate_rebate_per_invitee_cap"

	SettingKeySMTPHost     = "smtp_host"
	SettingKeySMTPPort     = "smtp_port"
	SettingKeySMTPUsername = "smtp_username"
	SettingKeySMTPPassword = "smtp_password"
	SettingKeySMTPFrom     = "smtp_from"
	SettingKeySMTPFromName = "smtp_from_name"
	SettingKeySMTPUseTLS   = "smtp_use_tls"

	SettingKeyTurnstileEnabled   = "turnstile_enabled"
	SettingKeyTurnstileSiteKey   = "turnstile_site_key"
	SettingKeyTurnstileSecretKey = "turnstile_secret_key"

	SettingKeyAPIKeyACLTrustForwardedIP = "api_key_acl_trust_forwarded_ip"

	SettingKeyTotpEnabled = "totp_enabled"

	SettingKeyLinuxDoConnectEnabled      = "linuxdo_connect_enabled"
	SettingKeyLinuxDoConnectClientID     = "linuxdo_connect_client_id"
	SettingKeyLinuxDoConnectClientSecret = "linuxdo_connect_client_secret"
	SettingKeyLinuxDoConnectRedirectURL  = "linuxdo_connect_redirect_url"

	SettingKeyWeChatConnectEnabled             = "wechat_connect_enabled"
	SettingKeyWeChatConnectAppID               = "wechat_connect_app_id"
	SettingKeyWeChatConnectAppSecret           = "wechat_connect_app_secret"
	SettingKeyWeChatConnectOpenAppID           = "wechat_connect_open_app_id"
	SettingKeyWeChatConnectOpenAppSecret       = "wechat_connect_open_app_secret"
	SettingKeyWeChatConnectMPAppID             = "wechat_connect_mp_app_id"
	SettingKeyWeChatConnectMPAppSecret         = "wechat_connect_mp_app_secret"
	SettingKeyWeChatConnectMobileAppID         = "wechat_connect_mobile_app_id"
	SettingKeyWeChatConnectMobileAppSecret     = "wechat_connect_mobile_app_secret"
	SettingKeyWeChatConnectOpenEnabled         = "wechat_connect_open_enabled"
	SettingKeyWeChatConnectMPEnabled           = "wechat_connect_mp_enabled"
	SettingKeyWeChatConnectMobileEnabled       = "wechat_connect_mobile_enabled"
	SettingKeyWeChatConnectMode                = "wechat_connect_mode"
	SettingKeyWeChatConnectScopes              = "wechat_connect_scopes"
	SettingKeyWeChatConnectRedirectURL         = "wechat_connect_redirect_url"
	SettingKeyWeChatConnectFrontendRedirectURL = "wechat_connect_frontend_redirect_url"

	SettingKeyOIDCConnectEnabled              = "oidc_connect_enabled"
	SettingKeyOIDCConnectProviderName         = "oidc_connect_provider_name"
	SettingKeyOIDCConnectClientID             = "oidc_connect_client_id"
	SettingKeyOIDCConnectClientSecret         = "oidc_connect_client_secret"
	SettingKeyOIDCConnectIssuerURL            = "oidc_connect_issuer_url"
	SettingKeyOIDCConnectDiscoveryURL         = "oidc_connect_discovery_url"
	SettingKeyOIDCConnectAuthorizeURL         = "oidc_connect_authorize_url"
	SettingKeyOIDCConnectTokenURL             = "oidc_connect_token_url"
	SettingKeyOIDCConnectUserInfoURL          = "oidc_connect_userinfo_url"
	SettingKeyOIDCConnectJWKSURL              = "oidc_connect_jwks_url"
	SettingKeyOIDCConnectScopes               = "oidc_connect_scopes"
	SettingKeyOIDCConnectRedirectURL          = "oidc_connect_redirect_url"
	SettingKeyOIDCConnectFrontendRedirectURL  = "oidc_connect_frontend_redirect_url"
	SettingKeyOIDCConnectTokenAuthMethod      = "oidc_connect_token_auth_method"
	SettingKeyOIDCConnectUsePKCE              = "oidc_connect_use_pkce"
	SettingKeyOIDCConnectValidateIDToken      = "oidc_connect_validate_id_token"
	SettingKeyOIDCConnectAllowedSigningAlgs   = "oidc_connect_allowed_signing_algs"
	SettingKeyOIDCConnectClockSkewSeconds     = "oidc_connect_clock_skew_seconds"
	SettingKeyOIDCConnectRequireEmailVerified = "oidc_connect_require_email_verified"
	SettingKeyOIDCConnectUserInfoEmailPath    = "oidc_connect_userinfo_email_path"
	SettingKeyOIDCConnectUserInfoIDPath       = "oidc_connect_userinfo_id_path"
	SettingKeyOIDCConnectUserInfoUsernamePath = "oidc_connect_userinfo_username_path"

	SettingKeySiteName                       = "site_name"
	SettingKeySiteLogo                       = "site_logo"
	SettingKeySiteSubtitle                   = "site_subtitle"
	SettingKeyAPIBaseURL                     = "api_base_url"
	SettingKeyContactInfo                    = "contact_info"
	SettingKeyDocURL                         = "doc_url"
	SettingKeyHomeContent                    = "home_content"
	SettingKeyHideCcsImportButton            = "hide_ccs_import_button"
	SettingKeyPurchaseSubscriptionEnabled    = "purchase_subscription_enabled"
	SettingKeyPurchaseSubscriptionURL        = "purchase_subscription_url"
	SettingKeyTableDefaultPageSize           = "table_default_page_size"
	SettingKeyTablePageSizeOptions           = "table_page_size_options"
	SettingKeyCustomMenuItems                = "custom_menu_items"
	SettingKeyCustomEndpoints                = "custom_endpoints"
	SettingKeyRiskControlEnabled             = "risk_control_enabled"
	SettingKeyAutoModelSettings              = "auto_model_settings"
	SettingKeyFreeModelsEnabled              = "free_models_enabled"
	SettingKeyCarpoolEnabled                 = "carpool_enabled"
	SettingKeyCarpoolBaseServiceFeeUSD       = "carpool_base_service_fee_usd"
	SettingKeyCarpoolSystemProxyFeeUSD       = "carpool_system_proxy_fee_usd"
	SettingKeyCarpoolRiskControlFeeUSD       = "carpool_risk_control_fee_usd"
	SettingKeyContentModerationConfig        = "content_moderation_config"
	SettingKeyLoginAgreementEnabled          = "login_agreement_enabled"
	SettingKeyLoginAgreementMode             = "login_agreement_mode"
	SettingKeyLoginAgreementUpdatedAt        = "login_agreement_updated_at"
	SettingKeyLoginAgreementDocuments        = "login_agreement_documents"
	SettingKeyGitHubOAuthEnabled             = "github_oauth_enabled"
	SettingKeyGitHubOAuthClientID            = "github_oauth_client_id"
	SettingKeyGitHubOAuthClientSecret        = "github_oauth_client_secret"
	SettingKeyGitHubOAuthRedirectURL         = "github_oauth_redirect_url"
	SettingKeyGitHubOAuthFrontendRedirectURL = "github_oauth_frontend_redirect_url"
	SettingKeyGoogleOAuthEnabled             = "google_oauth_enabled"
	SettingKeyGoogleOAuthClientID            = "google_oauth_client_id"
	SettingKeyGoogleOAuthClientSecret        = "google_oauth_client_secret"
	SettingKeyGoogleOAuthRedirectURL         = "google_oauth_redirect_url"
	SettingKeyGoogleOAuthFrontendRedirectURL = "google_oauth_frontend_redirect_url"

	SettingKeyDefaultConcurrency   = "default_concurrency"
	SettingKeyDefaultBalance       = "default_balance"
	SettingKeyDefaultSubscriptions = "default_subscriptions"
	SettingKeyDefaultUserRPMLimit  = "default_user_rpm_limit"

	SettingKeyAuthSourceDefaultEmailBalance            = "auth_source_default_email_balance"
	SettingKeyAuthSourceDefaultEmailConcurrency        = "auth_source_default_email_concurrency"
	SettingKeyAuthSourceDefaultEmailSubscriptions      = "auth_source_default_email_subscriptions"
	SettingKeyAuthSourceDefaultEmailGrantOnSignup      = "auth_source_default_email_grant_on_signup"
	SettingKeyAuthSourceDefaultEmailGrantOnFirstBind   = "auth_source_default_email_grant_on_first_bind"
	SettingKeyAuthSourceDefaultLinuxDoBalance          = "auth_source_default_linuxdo_balance"
	SettingKeyAuthSourceDefaultLinuxDoConcurrency      = "auth_source_default_linuxdo_concurrency"
	SettingKeyAuthSourceDefaultLinuxDoSubscriptions    = "auth_source_default_linuxdo_subscriptions"
	SettingKeyAuthSourceDefaultLinuxDoGrantOnSignup    = "auth_source_default_linuxdo_grant_on_signup"
	SettingKeyAuthSourceDefaultLinuxDoGrantOnFirstBind = "auth_source_default_linuxdo_grant_on_first_bind"
	SettingKeyAuthSourceDefaultOIDCBalance             = "auth_source_default_oidc_balance"
	SettingKeyAuthSourceDefaultOIDCConcurrency         = "auth_source_default_oidc_concurrency"
	SettingKeyAuthSourceDefaultOIDCSubscriptions       = "auth_source_default_oidc_subscriptions"
	SettingKeyAuthSourceDefaultOIDCGrantOnSignup       = "auth_source_default_oidc_grant_on_signup"
	SettingKeyAuthSourceDefaultOIDCGrantOnFirstBind    = "auth_source_default_oidc_grant_on_first_bind"
	SettingKeyAuthSourceDefaultWeChatBalance           = "auth_source_default_wechat_balance"
	SettingKeyAuthSourceDefaultWeChatConcurrency       = "auth_source_default_wechat_concurrency"
	SettingKeyAuthSourceDefaultWeChatSubscriptions     = "auth_source_default_wechat_subscriptions"
	SettingKeyAuthSourceDefaultWeChatGrantOnSignup     = "auth_source_default_wechat_grant_on_signup"
	SettingKeyAuthSourceDefaultWeChatGrantOnFirstBind  = "auth_source_default_wechat_grant_on_first_bind"
	SettingKeyAuthSourceDefaultGitHubBalance           = "auth_source_default_github_balance"
	SettingKeyAuthSourceDefaultGitHubConcurrency       = "auth_source_default_github_concurrency"
	SettingKeyAuthSourceDefaultGitHubSubscriptions     = "auth_source_default_github_subscriptions"
	SettingKeyAuthSourceDefaultGitHubGrantOnSignup     = "auth_source_default_github_grant_on_signup"
	SettingKeyAuthSourceDefaultGitHubGrantOnFirstBind  = "auth_source_default_github_grant_on_first_bind"
	SettingKeyAuthSourceDefaultGoogleBalance           = "auth_source_default_google_balance"
	SettingKeyAuthSourceDefaultGoogleConcurrency       = "auth_source_default_google_concurrency"
	SettingKeyAuthSourceDefaultGoogleSubscriptions     = "auth_source_default_google_subscriptions"
	SettingKeyAuthSourceDefaultGoogleGrantOnSignup     = "auth_source_default_google_grant_on_signup"
	SettingKeyAuthSourceDefaultGoogleGrantOnFirstBind  = "auth_source_default_google_grant_on_first_bind"
	SettingKeyForceEmailOnThirdPartySignup             = "force_email_on_third_party_signup"

	SettingKeyAdminAPIKey = "admin_api_key"

	SettingKeyGeminiQuotaPolicy = "gemini_quota_policy"

	SettingKeyEnableModelFallback      = "enable_model_fallback"
	SettingKeyFallbackModelAnthropic   = "fallback_model_anthropic"
	SettingKeyFallbackModelOpenAI      = "fallback_model_openai"
	SettingKeyFallbackModelGemini      = "fallback_model_gemini"
	SettingKeyFallbackModelAntigravity = "fallback_model_antigravity"

	SettingKeyEnableIdentityPatch = "enable_identity_patch"
	SettingKeyIdentityPatchPrompt = "identity_patch_prompt"

	SettingKeyOpsMonitoringEnabled                      = "ops_monitoring_enabled"
	SettingKeyOpsRealtimeMonitoringEnabled              = "ops_realtime_monitoring_enabled"
	SettingKeyOpsQueryModeDefault                       = "ops_query_mode_default"
	SettingKeyOpsEmailNotificationConfig                = "ops_email_notification_config"
	SettingKeyOpsAlertRuntimeSettings                   = "ops_alert_runtime_settings"
	SettingKeyOpsMetricsIntervalSeconds                 = "ops_metrics_interval_seconds"
	SettingKeyOpsAdvancedSettings                       = "ops_advanced_settings"
	SettingKeyOpsRuntimeLogConfig                       = "ops_runtime_log_config"
	SettingKeyChannelMonitorEnabled                     = "channel_monitor_enabled"
	SettingKeyChannelMonitorDefaultIntervalSeconds      = "channel_monitor_default_interval_seconds"
	SettingKeyAvailableChannelsEnabled                  = "available_channels_enabled"
	SettingKeyOverloadCooldownSettings                  = "overload_cooldown_settings"
	SettingKeyStreamTimeoutSettings                     = "stream_timeout_settings"
	SettingKeyRectifierSettings                         = "rectifier_settings"
	SettingKeyBetaPolicySettings                        = "beta_policy_settings"
	SettingKeyOpenAIFastPolicySettings                  = "openai_fast_policy_settings"
	SettingKeyMinClaudeCodeVersion                      = "min_claude_code_version"
	SettingKeyMaxClaudeCodeVersion                      = "max_claude_code_version"
	SettingKeyAllowUngroupedKeyScheduling               = "allow_ungrouped_key_scheduling"
	SettingKeyUserPrivateGroupDailyLimitUSD             = "user_private_group_daily_limit_usd"
	SettingKeyUserPrivateGroupWeeklyLimitUSD            = "user_private_group_weekly_limit_usd"
	SettingKeyUserPrivateGroupMonthlyLimitUSD           = "user_private_group_monthly_limit_usd"
	SettingKeyUserPrivateGroupRateMultiplier            = "user_private_group_rate_multiplier"
	SettingKeyUserPrivateGroupRPMLimit                  = "user_private_group_rpm_limit"
	SettingKeyUserPrivateGroupCommissionRate            = "user_private_group_commission_rate"
	SettingKeyBackendModeEnabled                        = "backend_mode_enabled"
	SettingKeyEnableFingerprintUnification              = "enable_fingerprint_unification"
	SettingKeyEnableMetadataPassthrough                 = "enable_metadata_passthrough"
	SettingKeyEnableCCHSigning                          = "enable_cch_signing"
	SettingKeyEnableAnthropicCacheTTL1hInjection        = "enable_anthropic_cache_ttl_1h_injection"
	SettingKeyOpenAIImagesResponsesReasoningEffort      = "openai_images_responses_reasoning_effort"
	SettingKeyBalanceLowNotifyEnabled                   = "balance_low_notify_enabled"
	SettingKeyBalanceLowNotifyThreshold                 = "balance_low_notify_threshold"
	SettingKeyBalanceLowNotifyRechargeURL               = "balance_low_notify_recharge_url"
	SettingKeyAccountQuotaNotifyEnabled                 = "account_quota_notify_enabled"
	SettingKeyAccountQuotaNotifyEmails                  = "account_quota_notify_emails"
	SettingKeyOpenAIFreeAccountRepairEnabled            = "openai_free_account_repair_enabled"
	SettingKeyOpenAIFreeAccountRepairWeeklyThresholdUSD = "openai_free_account_repair_weekly_threshold_usd"
	SettingKeyWebSearchEmulationConfig                  = "web_search_emulation_config"
)

// AdminAPIKeyPrefix is the prefix for admin API keys.
const AdminAPIKeyPrefix = "admin-"
