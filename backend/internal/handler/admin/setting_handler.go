package admin

import (
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"regexp"
	"strings"

	"ikik-api/internal/config"
	"ikik-api/internal/handler/dto"
	"ikik-api/internal/pkg/response"
	"ikik-api/internal/server/middleware"
	"ikik-api/internal/service"

	"github.com/gin-gonic/gin"
)

// semverPattern 预编译 semver 格式校验正则
var semverPattern = regexp.MustCompile(`^\d+\.\d+\.\d+$`)

// menuItemIDPattern validates custom menu item IDs: alphanumeric, hyphens, underscores only.
var menuItemIDPattern = regexp.MustCompile(`^[a-zA-Z0-9_-]+$`)

// generateMenuItemID generates a short random hex ID for a custom menu item.
func generateMenuItemID() (string, error) {
	b := make([]byte, 8)
	if _, err := rand.Read(b); err != nil {
		return "", fmt.Errorf("generate menu item ID: %w", err)
	}
	return hex.EncodeToString(b), nil
}

func scopesContainOpenID(scopes string) bool {
	for _, scope := range strings.Fields(strings.ToLower(strings.TrimSpace(scopes))) {
		if scope == "openid" {
			return true
		}
	}
	return false
}

func firstNonEmpty(values ...string) string {
	for _, value := range values {
		if trimmed := strings.TrimSpace(value); trimmed != "" {
			return trimmed
		}
	}
	return ""
}

func autoModelSettingsToDTO(settings service.AutoModelSettings) dto.AutoModelSettings {
	models := make([]dto.AutoModelRule, 0, len(settings.Models))
	for _, model := range settings.Models {
		models = append(models, dto.AutoModelRule{
			Name:               model.Name,
			Enabled:            model.Enabled,
			Description:        model.Description,
			AllowedGroupIDs:    model.AllowedGroupIDs,
			RoutingMode:        model.RoutingMode,
			SmallModel:         model.SmallModel,
			BalancedModel:      model.BalancedModel,
			LargeModel:         model.LargeModel,
			BalancedThreshold:  model.BalancedThreshold,
			LargeThreshold:     model.LargeThreshold,
			AllowedModels:      model.AllowedModels,
			CostQuality:        model.CostQuality,
			StickySession:      model.StickySession,
			AIRouterEnabled:    model.AIRouterEnabled,
			RouterModel:        model.RouterModel,
			RouterBaseURL:      model.RouterBaseURL,
			RouterAPIKey:       model.RouterAPIKey,
			RouterTimeoutMS:    model.RouterTimeoutMS,
			RouterMaxTokens:    model.RouterMaxTokens,
			RouterReasoning:    model.RouterReasoning,
			RouterPrompt:       model.RouterPrompt,
			RouterConservative: model.RouterConservative,
		})
	}
	return dto.AutoModelSettings{
		Enabled: settings.Enabled,
		Models:  models,
	}
}

func autoModelSettingsToService(settings dto.AutoModelSettings) service.AutoModelSettings {
	models := make([]service.AutoModelRule, 0, len(settings.Models))
	for _, model := range settings.Models {
		models = append(models, service.AutoModelRule{
			Name:               model.Name,
			Enabled:            model.Enabled,
			Description:        model.Description,
			AllowedGroupIDs:    model.AllowedGroupIDs,
			RoutingMode:        model.RoutingMode,
			SmallModel:         model.SmallModel,
			BalancedModel:      model.BalancedModel,
			LargeModel:         model.LargeModel,
			BalancedThreshold:  model.BalancedThreshold,
			LargeThreshold:     model.LargeThreshold,
			AllowedModels:      model.AllowedModels,
			CostQuality:        model.CostQuality,
			StickySession:      model.StickySession,
			AIRouterEnabled:    model.AIRouterEnabled,
			RouterModel:        model.RouterModel,
			RouterBaseURL:      model.RouterBaseURL,
			RouterAPIKey:       model.RouterAPIKey,
			RouterTimeoutMS:    model.RouterTimeoutMS,
			RouterMaxTokens:    model.RouterMaxTokens,
			RouterReasoning:    model.RouterReasoning,
			RouterPrompt:       model.RouterPrompt,
			RouterConservative: model.RouterConservative,
		})
	}
	return service.AutoModelSettings{
		Enabled: settings.Enabled,
		Models:  models,
	}
}

// SettingHandler 系统设置处理器
type SettingHandler struct {
	settingService       *service.SettingService
	emailService         *service.EmailService
	turnstileService     *service.TurnstileService
	opsService           *service.OpsService
	paymentConfigService *service.PaymentConfigService
	paymentService       *service.PaymentService
}

// NewSettingHandler 创建系统设置处理器
func NewSettingHandler(settingService *service.SettingService, emailService *service.EmailService, turnstileService *service.TurnstileService, opsService *service.OpsService, paymentConfigService *service.PaymentConfigService, paymentService *service.PaymentService) *SettingHandler {
	return &SettingHandler{
		settingService:       settingService,
		emailService:         emailService,
		turnstileService:     turnstileService,
		opsService:           opsService,
		paymentConfigService: paymentConfigService,
		paymentService:       paymentService,
	}
}

// GetSettings 获取所有系统设置
// GET /api/v1/admin/settings
func (h *SettingHandler) GetSettings(c *gin.Context) {
	settings, err := h.settingService.GetAllSettings(c.Request.Context())
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}
	authSourceDefaults, err := h.settingService.GetAuthSourceDefaultSettings(c.Request.Context())
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}

	// Check if ops monitoring is enabled (respects config.ops.enabled)
	opsEnabled := h.opsService != nil && h.opsService.IsMonitoringEnabled(c.Request.Context())
	defaultSubscriptions := make([]dto.DefaultSubscriptionSetting, 0, len(settings.DefaultSubscriptions))
	for _, sub := range settings.DefaultSubscriptions {
		defaultSubscriptions = append(defaultSubscriptions, dto.DefaultSubscriptionSetting{
			GroupID:      sub.GroupID,
			ValidityDays: sub.ValidityDays,
		})
	}

	// Load payment config
	var paymentCfg *service.PaymentConfig
	if h.paymentConfigService != nil {
		paymentCfg, _ = h.paymentConfigService.GetPaymentConfig(c.Request.Context())
	}
	if paymentCfg == nil {
		paymentCfg = &service.PaymentConfig{}
	}

	payload := dto.SystemSettings{
		RegistrationEnabled:                       settings.RegistrationEnabled,
		EmailVerifyEnabled:                        settings.EmailVerifyEnabled,
		RegistrationEmailSuffixWhitelist:          settings.RegistrationEmailSuffixWhitelist,
		PromoCodeEnabled:                          settings.PromoCodeEnabled,
		PasswordResetEnabled:                      settings.PasswordResetEnabled,
		FrontendURL:                               settings.FrontendURL,
		InvitationCodeEnabled:                     settings.InvitationCodeEnabled,
		TotpEnabled:                               settings.TotpEnabled,
		TotpEncryptionKeyConfigured:               h.settingService.IsTotpEncryptionKeyConfigured(),
		LoginAgreementEnabled:                     settings.LoginAgreementEnabled,
		LoginAgreementMode:                        settings.LoginAgreementMode,
		LoginAgreementUpdatedAt:                   settings.LoginAgreementUpdatedAt,
		LoginAgreementDocuments:                   loginAgreementDocumentsToDTO(settings.LoginAgreementDocuments),
		SMTPHost:                                  settings.SMTPHost,
		SMTPPort:                                  settings.SMTPPort,
		SMTPUsername:                              settings.SMTPUsername,
		SMTPPasswordConfigured:                    settings.SMTPPasswordConfigured,
		SMTPFrom:                                  settings.SMTPFrom,
		SMTPFromName:                              settings.SMTPFromName,
		SMTPUseTLS:                                settings.SMTPUseTLS,
		TurnstileEnabled:                          settings.TurnstileEnabled,
		TurnstileSiteKey:                          settings.TurnstileSiteKey,
		TurnstileSecretKeyConfigured:              settings.TurnstileSecretKeyConfigured,
		APIKeyACLTrustForwardedIP:                 settings.APIKeyACLTrustForwardedIP,
		LinuxDoConnectEnabled:                     settings.LinuxDoConnectEnabled,
		LinuxDoConnectClientID:                    settings.LinuxDoConnectClientID,
		LinuxDoConnectClientSecretConfigured:      settings.LinuxDoConnectClientSecretConfigured,
		LinuxDoConnectRedirectURL:                 settings.LinuxDoConnectRedirectURL,
		WeChatConnectEnabled:                      settings.WeChatConnectEnabled,
		WeChatConnectAppID:                        settings.WeChatConnectAppID,
		WeChatConnectAppSecretConfigured:          settings.WeChatConnectAppSecretConfigured,
		WeChatConnectOpenAppID:                    settings.WeChatConnectOpenAppID,
		WeChatConnectOpenAppSecretConfigured:      settings.WeChatConnectOpenAppSecretConfigured,
		WeChatConnectMPAppID:                      settings.WeChatConnectMPAppID,
		WeChatConnectMPAppSecretConfigured:        settings.WeChatConnectMPAppSecretConfigured,
		WeChatConnectMobileAppID:                  settings.WeChatConnectMobileAppID,
		WeChatConnectMobileAppSecretConfigured:    settings.WeChatConnectMobileAppSecretConfigured,
		WeChatConnectOpenEnabled:                  settings.WeChatConnectOpenEnabled,
		WeChatConnectMPEnabled:                    settings.WeChatConnectMPEnabled,
		WeChatConnectMobileEnabled:                settings.WeChatConnectMobileEnabled,
		WeChatConnectMode:                         settings.WeChatConnectMode,
		WeChatConnectScopes:                       settings.WeChatConnectScopes,
		WeChatConnectRedirectURL:                  settings.WeChatConnectRedirectURL,
		WeChatConnectFrontendRedirectURL:          settings.WeChatConnectFrontendRedirectURL,
		OIDCConnectEnabled:                        settings.OIDCConnectEnabled,
		OIDCConnectProviderName:                   settings.OIDCConnectProviderName,
		OIDCConnectClientID:                       settings.OIDCConnectClientID,
		OIDCConnectClientSecretConfigured:         settings.OIDCConnectClientSecretConfigured,
		OIDCConnectIssuerURL:                      settings.OIDCConnectIssuerURL,
		OIDCConnectDiscoveryURL:                   settings.OIDCConnectDiscoveryURL,
		OIDCConnectAuthorizeURL:                   settings.OIDCConnectAuthorizeURL,
		OIDCConnectTokenURL:                       settings.OIDCConnectTokenURL,
		OIDCConnectUserInfoURL:                    settings.OIDCConnectUserInfoURL,
		OIDCConnectJWKSURL:                        settings.OIDCConnectJWKSURL,
		OIDCConnectScopes:                         settings.OIDCConnectScopes,
		OIDCConnectRedirectURL:                    settings.OIDCConnectRedirectURL,
		OIDCConnectFrontendRedirectURL:            settings.OIDCConnectFrontendRedirectURL,
		OIDCConnectTokenAuthMethod:                settings.OIDCConnectTokenAuthMethod,
		OIDCConnectUsePKCE:                        settings.OIDCConnectUsePKCE,
		OIDCConnectValidateIDToken:                settings.OIDCConnectValidateIDToken,
		OIDCConnectAllowedSigningAlgs:             settings.OIDCConnectAllowedSigningAlgs,
		OIDCConnectClockSkewSeconds:               settings.OIDCConnectClockSkewSeconds,
		OIDCConnectRequireEmailVerified:           settings.OIDCConnectRequireEmailVerified,
		OIDCConnectUserInfoEmailPath:              settings.OIDCConnectUserInfoEmailPath,
		OIDCConnectUserInfoIDPath:                 settings.OIDCConnectUserInfoIDPath,
		OIDCConnectUserInfoUsernamePath:           settings.OIDCConnectUserInfoUsernamePath,
		GitHubOAuthEnabled:                        settings.GitHubOAuthEnabled,
		GitHubOAuthClientID:                       settings.GitHubOAuthClientID,
		GitHubOAuthClientSecretConfigured:         settings.GitHubOAuthClientSecretConfigured,
		GitHubOAuthRedirectURL:                    settings.GitHubOAuthRedirectURL,
		GitHubOAuthFrontendRedirectURL:            settings.GitHubOAuthFrontendRedirectURL,
		GoogleOAuthEnabled:                        settings.GoogleOAuthEnabled,
		GoogleOAuthClientID:                       settings.GoogleOAuthClientID,
		GoogleOAuthClientSecretConfigured:         settings.GoogleOAuthClientSecretConfigured,
		GoogleOAuthRedirectURL:                    settings.GoogleOAuthRedirectURL,
		GoogleOAuthFrontendRedirectURL:            settings.GoogleOAuthFrontendRedirectURL,
		SiteName:                                  settings.SiteName,
		SiteLogo:                                  settings.SiteLogo,
		SiteSubtitle:                              settings.SiteSubtitle,
		APIBaseURL:                                settings.APIBaseURL,
		ContactInfo:                               settings.ContactInfo,
		DocURL:                                    settings.DocURL,
		HomeContent:                               settings.HomeContent,
		HomeStatsGroupID:                          settings.HomeStatsGroupID,
		HideCcsImportButton:                       settings.HideCcsImportButton,
		PurchaseSubscriptionEnabled:               settings.PurchaseSubscriptionEnabled,
		PurchaseSubscriptionURL:                   settings.PurchaseSubscriptionURL,
		TableDefaultPageSize:                      settings.TableDefaultPageSize,
		TablePageSizeOptions:                      settings.TablePageSizeOptions,
		CustomMenuItems:                           dto.ParseCustomMenuItems(settings.CustomMenuItems),
		CustomEndpoints:                           dto.ParseCustomEndpoints(settings.CustomEndpoints),
		DefaultConcurrency:                        settings.DefaultConcurrency,
		DefaultBalance:                            settings.DefaultBalance,
		RiskControlEnabled:                        settings.RiskControlEnabled,
		AffiliateRebateRate:                       settings.AffiliateRebateRate,
		AffiliateRebateFreezeHours:                settings.AffiliateRebateFreezeHours,
		AffiliateRebateDurationDays:               settings.AffiliateRebateDurationDays,
		AffiliateRebatePerInviteeCap:              settings.AffiliateRebatePerInviteeCap,
		DefaultUserRPMLimit:                       settings.DefaultUserRPMLimit,
		UserPrivateGroupDailyLimitUSD:             settings.UserPrivateGroupDailyLimitUSD,
		UserPrivateGroupWeeklyLimitUSD:            settings.UserPrivateGroupWeeklyLimitUSD,
		UserPrivateGroupMonthlyLimitUSD:           settings.UserPrivateGroupMonthlyLimitUSD,
		UserPrivateGroupRateMultiplier:            settings.UserPrivateGroupRateMultiplier,
		UserPrivateGroupRPMLimit:                  settings.UserPrivateGroupRPMLimit,
		UserPrivateGroupCommissionRate:            settings.UserPrivateGroupCommissionRate,
		DefaultSubscriptions:                      defaultSubscriptions,
		EnableModelFallback:                       settings.EnableModelFallback,
		FallbackModelAnthropic:                    settings.FallbackModelAnthropic,
		FallbackModelOpenAI:                       settings.FallbackModelOpenAI,
		FallbackModelGemini:                       settings.FallbackModelGemini,
		FallbackModelAntigravity:                  settings.FallbackModelAntigravity,
		EnableIdentityPatch:                       settings.EnableIdentityPatch,
		IdentityPatchPrompt:                       settings.IdentityPatchPrompt,
		OpsMonitoringEnabled:                      opsEnabled && settings.OpsMonitoringEnabled,
		OpsRealtimeMonitoringEnabled:              settings.OpsRealtimeMonitoringEnabled,
		OpsQueryModeDefault:                       settings.OpsQueryModeDefault,
		OpsMetricsIntervalSeconds:                 settings.OpsMetricsIntervalSeconds,
		MinClaudeCodeVersion:                      settings.MinClaudeCodeVersion,
		MaxClaudeCodeVersion:                      settings.MaxClaudeCodeVersion,
		AllowUngroupedKeyScheduling:               settings.AllowUngroupedKeyScheduling,
		BackendModeEnabled:                        settings.BackendModeEnabled,
		EnableFingerprintUnification:              settings.EnableFingerprintUnification,
		EnableMetadataPassthrough:                 settings.EnableMetadataPassthrough,
		EnableCCHSigning:                          settings.EnableCCHSigning,
		EnableAnthropicCacheTTL1hInjection:        settings.EnableAnthropicCacheTTL1hInjection,
		OpenAIImagesResponsesReasoningEffort:      settings.OpenAIImagesResponsesReasoningEffort,
		WebSearchEmulationEnabled:                 settings.WebSearchEmulationEnabled,
		PaymentVisibleMethodAlipaySource:          settings.PaymentVisibleMethodAlipaySource,
		PaymentVisibleMethodWxpaySource:           settings.PaymentVisibleMethodWxpaySource,
		PaymentVisibleMethodAlipayEnabled:         settings.PaymentVisibleMethodAlipayEnabled,
		PaymentVisibleMethodWxpayEnabled:          settings.PaymentVisibleMethodWxpayEnabled,
		OpenAIAdvancedSchedulerEnabled:            settings.OpenAIAdvancedSchedulerEnabled,
		OpenAIFreeAccountRepairEnabled:            settings.OpenAIFreeAccountRepairEnabled,
		OpenAIFreeAccountRepairWeeklyThresholdUSD: settings.OpenAIFreeAccountRepairWeeklyThresholdUSD,
		BalanceLowNotifyEnabled:                   settings.BalanceLowNotifyEnabled,
		BalanceLowNotifyThreshold:                 settings.BalanceLowNotifyThreshold,
		BalanceLowNotifyRechargeURL:               settings.BalanceLowNotifyRechargeURL,
		AccountQuotaNotifyEnabled:                 settings.AccountQuotaNotifyEnabled,
		AccountQuotaNotifyEmails:                  dto.NotifyEmailEntriesFromService(settings.AccountQuotaNotifyEmails),
		AutoModelSettings:                         autoModelSettingsToDTO(settings.AutoModelSettings),
		PaymentEnabled:                            paymentCfg.Enabled,
		PaymentMinAmount:                          paymentCfg.MinAmount,
		PaymentMaxAmount:                          paymentCfg.MaxAmount,
		PaymentDailyLimit:                         paymentCfg.DailyLimit,
		PaymentOrderTimeoutMin:                    paymentCfg.OrderTimeoutMin,
		PaymentMaxPendingOrders:                   paymentCfg.MaxPendingOrders,
		PaymentEnabledTypes:                       paymentCfg.EnabledTypes,
		PaymentBalanceDisabled:                    paymentCfg.BalanceDisabled,
		PaymentBalanceRechargeMultiplier:          paymentCfg.BalanceRechargeMultiplier,
		PaymentRechargeFeeRate:                    paymentCfg.RechargeFeeRate,
		PaymentLoadBalanceStrat:                   paymentCfg.LoadBalanceStrategy,
		PaymentProductNamePrefix:                  paymentCfg.ProductNamePrefix,
		PaymentProductNameSuffix:                  paymentCfg.ProductNameSuffix,
		PaymentHelpImageURL:                       paymentCfg.HelpImageURL,
		PaymentHelpText:                           paymentCfg.HelpText,
		PaymentReceiptCodeOSSEnabled:              paymentCfg.ReceiptCodeOSS.Enabled,
		PaymentReceiptCodeOSSEndpoint:             paymentCfg.ReceiptCodeOSS.Endpoint,
		PaymentReceiptCodeOSSRegion:               paymentCfg.ReceiptCodeOSS.Region,
		PaymentReceiptCodeOSSBucket:               paymentCfg.ReceiptCodeOSS.Bucket,
		PaymentReceiptCodeOSSAccessKeyID:          paymentCfg.ReceiptCodeOSS.AccessKeyID,
		PaymentReceiptCodeOSSSecretConfigured:     paymentCfg.ReceiptCodeOSS.SecretAccessKeyConfigured,
		PaymentReceiptCodeOSSPrefix:               paymentCfg.ReceiptCodeOSS.Prefix,
		PaymentReceiptCodeOSSPublicBaseURL:        paymentCfg.ReceiptCodeOSS.PublicBaseURL,
		PaymentReceiptCodeOSSForcePathStyle:       paymentCfg.ReceiptCodeOSS.ForcePathStyle,
		PaymentReceiptCodeOSSMaxSizeBytes:         paymentCfg.ReceiptCodeOSS.MaxSizeBytes,
		PaymentReceiptCodeOSSPresignExpireSeconds: paymentCfg.ReceiptCodeOSS.PresignExpireSeconds,
		PaymentCancelRateLimitEnabled:             paymentCfg.CancelRateLimitEnabled,
		PaymentCancelRateLimitMax:                 paymentCfg.CancelRateLimitMax,
		PaymentCancelRateLimitWindow:              paymentCfg.CancelRateLimitWindow,
		PaymentCancelRateLimitUnit:                paymentCfg.CancelRateLimitUnit,
		PaymentCancelRateLimitMode:                paymentCfg.CancelRateLimitMode,

		ChannelMonitorEnabled:                settings.ChannelMonitorEnabled,
		ChannelMonitorDefaultIntervalSeconds: settings.ChannelMonitorDefaultIntervalSeconds,

		AvailableChannelsEnabled: settings.AvailableChannelsEnabled,
		FreeModelsEnabled:        settings.FreeModelsEnabled,

		CarpoolEnabled:           settings.CarpoolEnabled,
		CarpoolBaseServiceFeeUSD: settings.CarpoolBaseServiceFeeUSD,
		CarpoolSystemProxyFeeUSD: settings.CarpoolSystemProxyFeeUSD,
		CarpoolRiskControlFeeUSD: settings.CarpoolRiskControlFeeUSD,

		AffiliateEnabled: settings.AffiliateEnabled,
	}

	// OpenAI fast policy (stored under a dedicated setting key)
	if fastPolicy, err := h.settingService.GetOpenAIFastPolicySettings(c.Request.Context()); err != nil {
		slog.Error("openai_fast_policy_settings_get_failed", "error", err)
	} else if fastPolicy != nil {
		payload.OpenAIFastPolicySettings = openaiFastPolicySettingsToDTO(fastPolicy)
	}

	response.Success(c, systemSettingsResponseData(payload, authSourceDefaults))
}

// openaiFastPolicySettingsToDTO converts service -> dto for OpenAI fast policy.
func openaiFastPolicySettingsToDTO(s *service.OpenAIFastPolicySettings) *dto.OpenAIFastPolicySettings {
	if s == nil {
		return nil
	}
	rules := make([]dto.OpenAIFastPolicyRule, len(s.Rules))
	for i, r := range s.Rules {
		rules[i] = dto.OpenAIFastPolicyRule(r)
	}
	return &dto.OpenAIFastPolicySettings{Rules: rules}
}

// openaiFastPolicySettingsFromDTO converts dto -> service for OpenAI fast policy.
//
// 规范化 ServiceTier：在 DTO 进入 service 层之前统一把空字符串归一为
// service.OpenAIFastTierAny ("all")，避免管理员保存时空串与 "all" 同时
// 表达"匹配任意 tier"造成数据库取值的二义性。其它非空值原样透传，由
// service.SetOpenAIFastPolicySettings 负责合法值校验。
func openaiFastPolicySettingsFromDTO(s *dto.OpenAIFastPolicySettings) *service.OpenAIFastPolicySettings {
	if s == nil {
		return nil
	}
	rules := make([]service.OpenAIFastPolicyRule, len(s.Rules))
	for i, r := range s.Rules {
		rules[i] = service.OpenAIFastPolicyRule(r)
		tier := strings.ToLower(strings.TrimSpace(rules[i].ServiceTier))
		if tier == "" {
			tier = service.OpenAIFastTierAny
		}
		rules[i].ServiceTier = tier
	}
	return &service.OpenAIFastPolicySettings{Rules: rules}
}

func loginAgreementDocumentsToDTO(docs []service.LoginAgreementDocument) []dto.LoginAgreementDocument {
	out := make([]dto.LoginAgreementDocument, 0, len(docs))
	for _, doc := range docs {
		out = append(out, dto.LoginAgreementDocument{
			ID:        doc.ID,
			Title:     doc.Title,
			ContentMD: doc.ContentMD,
		})
	}
	return out
}

func loginAgreementDocumentsToService(docs []dto.LoginAgreementDocument) []service.LoginAgreementDocument {
	out := make([]service.LoginAgreementDocument, 0, len(docs))
	for _, doc := range docs {
		out = append(out, service.LoginAgreementDocument{
			ID:        doc.ID,
			Title:     doc.Title,
			ContentMD: doc.ContentMD,
		})
	}
	return out
}

// UpdateSettingsRequest 更新设置请求
type UpdateSettingsRequest struct {
	// 注册设置
	RegistrationEnabled              bool                         `json:"registration_enabled"`
	EmailVerifyEnabled               bool                         `json:"email_verify_enabled"`
	RegistrationEmailSuffixWhitelist []string                     `json:"registration_email_suffix_whitelist"`
	PromoCodeEnabled                 bool                         `json:"promo_code_enabled"`
	PasswordResetEnabled             bool                         `json:"password_reset_enabled"`
	FrontendURL                      string                       `json:"frontend_url"`
	InvitationCodeEnabled            bool                         `json:"invitation_code_enabled"`
	TotpEnabled                      bool                         `json:"totp_enabled"` // TOTP 双因素认证
	LoginAgreementEnabled            *bool                        `json:"login_agreement_enabled"`
	LoginAgreementMode               string                       `json:"login_agreement_mode"`
	LoginAgreementUpdatedAt          string                       `json:"login_agreement_updated_at"`
	LoginAgreementDocuments          []dto.LoginAgreementDocument `json:"login_agreement_documents"`

	// 邮件服务设置
	SMTPHost     string `json:"smtp_host"`
	SMTPPort     int    `json:"smtp_port"`
	SMTPUsername string `json:"smtp_username"`
	SMTPPassword string `json:"smtp_password"`
	SMTPFrom     string `json:"smtp_from_email"`
	SMTPFromName string `json:"smtp_from_name"`
	SMTPUseTLS   bool   `json:"smtp_use_tls"`

	// Cloudflare Turnstile 设置
	TurnstileEnabled   bool   `json:"turnstile_enabled"`
	TurnstileSiteKey   string `json:"turnstile_site_key"`
	TurnstileSecretKey string `json:"turnstile_secret_key"`

	// API Key IP 访问控制设置
	APIKeyACLTrustForwardedIP *bool `json:"api_key_acl_trust_forwarded_ip"`

	// LinuxDo Connect OAuth 登录
	LinuxDoConnectEnabled      bool   `json:"linuxdo_connect_enabled"`
	LinuxDoConnectClientID     string `json:"linuxdo_connect_client_id"`
	LinuxDoConnectClientSecret string `json:"linuxdo_connect_client_secret"`
	LinuxDoConnectRedirectURL  string `json:"linuxdo_connect_redirect_url"`

	// WeChat Connect OAuth 登录
	WeChatConnectEnabled             bool   `json:"wechat_connect_enabled"`
	WeChatConnectAppID               string `json:"wechat_connect_app_id"`
	WeChatConnectAppSecret           string `json:"wechat_connect_app_secret"`
	WeChatConnectOpenAppID           string `json:"wechat_connect_open_app_id"`
	WeChatConnectOpenAppSecret       string `json:"wechat_connect_open_app_secret"`
	WeChatConnectMPAppID             string `json:"wechat_connect_mp_app_id"`
	WeChatConnectMPAppSecret         string `json:"wechat_connect_mp_app_secret"`
	WeChatConnectMobileAppID         string `json:"wechat_connect_mobile_app_id"`
	WeChatConnectMobileAppSecret     string `json:"wechat_connect_mobile_app_secret"`
	WeChatConnectOpenEnabled         bool   `json:"wechat_connect_open_enabled"`
	WeChatConnectMPEnabled           bool   `json:"wechat_connect_mp_enabled"`
	WeChatConnectMobileEnabled       bool   `json:"wechat_connect_mobile_enabled"`
	WeChatConnectMode                string `json:"wechat_connect_mode"`
	WeChatConnectScopes              string `json:"wechat_connect_scopes"`
	WeChatConnectRedirectURL         string `json:"wechat_connect_redirect_url"`
	WeChatConnectFrontendRedirectURL string `json:"wechat_connect_frontend_redirect_url"`

	// Generic OIDC OAuth 登录
	OIDCConnectEnabled              bool   `json:"oidc_connect_enabled"`
	OIDCConnectProviderName         string `json:"oidc_connect_provider_name"`
	OIDCConnectClientID             string `json:"oidc_connect_client_id"`
	OIDCConnectClientSecret         string `json:"oidc_connect_client_secret"`
	OIDCConnectIssuerURL            string `json:"oidc_connect_issuer_url"`
	OIDCConnectDiscoveryURL         string `json:"oidc_connect_discovery_url"`
	OIDCConnectAuthorizeURL         string `json:"oidc_connect_authorize_url"`
	OIDCConnectTokenURL             string `json:"oidc_connect_token_url"`
	OIDCConnectUserInfoURL          string `json:"oidc_connect_userinfo_url"`
	OIDCConnectJWKSURL              string `json:"oidc_connect_jwks_url"`
	OIDCConnectScopes               string `json:"oidc_connect_scopes"`
	OIDCConnectRedirectURL          string `json:"oidc_connect_redirect_url"`
	OIDCConnectFrontendRedirectURL  string `json:"oidc_connect_frontend_redirect_url"`
	OIDCConnectTokenAuthMethod      string `json:"oidc_connect_token_auth_method"`
	OIDCConnectUsePKCE              *bool  `json:"oidc_connect_use_pkce"`
	OIDCConnectValidateIDToken      *bool  `json:"oidc_connect_validate_id_token"`
	OIDCConnectAllowedSigningAlgs   string `json:"oidc_connect_allowed_signing_algs"`
	OIDCConnectClockSkewSeconds     int    `json:"oidc_connect_clock_skew_seconds"`
	OIDCConnectRequireEmailVerified bool   `json:"oidc_connect_require_email_verified"`
	OIDCConnectUserInfoEmailPath    string `json:"oidc_connect_userinfo_email_path"`
	OIDCConnectUserInfoIDPath       string `json:"oidc_connect_userinfo_id_path"`
	OIDCConnectUserInfoUsernamePath string `json:"oidc_connect_userinfo_username_path"`

	// GitHub / Google 邮箱快捷登录
	GitHubOAuthEnabled             *bool  `json:"github_oauth_enabled"`
	GitHubOAuthClientID            string `json:"github_oauth_client_id"`
	GitHubOAuthClientSecret        string `json:"github_oauth_client_secret"`
	GitHubOAuthRedirectURL         string `json:"github_oauth_redirect_url"`
	GitHubOAuthFrontendRedirectURL string `json:"github_oauth_frontend_redirect_url"`
	GoogleOAuthEnabled             *bool  `json:"google_oauth_enabled"`
	GoogleOAuthClientID            string `json:"google_oauth_client_id"`
	GoogleOAuthClientSecret        string `json:"google_oauth_client_secret"`
	GoogleOAuthRedirectURL         string `json:"google_oauth_redirect_url"`
	GoogleOAuthFrontendRedirectURL string `json:"google_oauth_frontend_redirect_url"`

	// OEM设置
	SiteName                    string                `json:"site_name"`
	SiteLogo                    string                `json:"site_logo"`
	SiteSubtitle                string                `json:"site_subtitle"`
	APIBaseURL                  string                `json:"api_base_url"`
	ContactInfo                 string                `json:"contact_info"`
	DocURL                      string                `json:"doc_url"`
	HomeContent                 string                `json:"home_content"`
	HomeStatsGroupID            int64                 `json:"home_stats_group_id"`
	HideCcsImportButton         bool                  `json:"hide_ccs_import_button"`
	PurchaseSubscriptionEnabled *bool                 `json:"purchase_subscription_enabled"`
	PurchaseSubscriptionURL     *string               `json:"purchase_subscription_url"`
	TableDefaultPageSize        int                   `json:"table_default_page_size"`
	TablePageSizeOptions        []int                 `json:"table_page_size_options"`
	CustomMenuItems             *[]dto.CustomMenuItem `json:"custom_menu_items"`
	CustomEndpoints             *[]dto.CustomEndpoint `json:"custom_endpoints"`

	// 默认配置
	DefaultConcurrency                       int                               `json:"default_concurrency"`
	DefaultBalance                           float64                           `json:"default_balance"`
	RiskControlEnabled                       *bool                             `json:"risk_control_enabled"`
	AffiliateRebateRate                      *float64                          `json:"affiliate_rebate_rate"`
	AffiliateRebateFreezeHours               *int                              `json:"affiliate_rebate_freeze_hours"`
	AffiliateRebateDurationDays              *int                              `json:"affiliate_rebate_duration_days"`
	AffiliateRebatePerInviteeCap             *float64                          `json:"affiliate_rebate_per_invitee_cap"`
	DefaultUserRPMLimit                      int                               `json:"default_user_rpm_limit"`
	UserPrivateGroupDailyLimitUSD            float64                           `json:"user_private_group_daily_limit_usd"`
	UserPrivateGroupWeeklyLimitUSD           float64                           `json:"user_private_group_weekly_limit_usd"`
	UserPrivateGroupMonthlyLimitUSD          float64                           `json:"user_private_group_monthly_limit_usd"`
	UserPrivateGroupRateMultiplier           float64                           `json:"user_private_group_rate_multiplier"`
	UserPrivateGroupRPMLimit                 int                               `json:"user_private_group_rpm_limit"`
	UserPrivateGroupCommissionRate           float64                           `json:"user_private_group_commission_rate"`
	DefaultSubscriptions                     []dto.DefaultSubscriptionSetting  `json:"default_subscriptions"`
	AuthSourceDefaultEmailBalance            *float64                          `json:"auth_source_default_email_balance"`
	AuthSourceDefaultEmailConcurrency        *int                              `json:"auth_source_default_email_concurrency"`
	AuthSourceDefaultEmailSubscriptions      *[]dto.DefaultSubscriptionSetting `json:"auth_source_default_email_subscriptions"`
	AuthSourceDefaultEmailGrantOnSignup      *bool                             `json:"auth_source_default_email_grant_on_signup"`
	AuthSourceDefaultEmailGrantOnFirstBind   *bool                             `json:"auth_source_default_email_grant_on_first_bind"`
	AuthSourceDefaultLinuxDoBalance          *float64                          `json:"auth_source_default_linuxdo_balance"`
	AuthSourceDefaultLinuxDoConcurrency      *int                              `json:"auth_source_default_linuxdo_concurrency"`
	AuthSourceDefaultLinuxDoSubscriptions    *[]dto.DefaultSubscriptionSetting `json:"auth_source_default_linuxdo_subscriptions"`
	AuthSourceDefaultLinuxDoGrantOnSignup    *bool                             `json:"auth_source_default_linuxdo_grant_on_signup"`
	AuthSourceDefaultLinuxDoGrantOnFirstBind *bool                             `json:"auth_source_default_linuxdo_grant_on_first_bind"`
	AuthSourceDefaultOIDCBalance             *float64                          `json:"auth_source_default_oidc_balance"`
	AuthSourceDefaultOIDCConcurrency         *int                              `json:"auth_source_default_oidc_concurrency"`
	AuthSourceDefaultOIDCSubscriptions       *[]dto.DefaultSubscriptionSetting `json:"auth_source_default_oidc_subscriptions"`
	AuthSourceDefaultOIDCGrantOnSignup       *bool                             `json:"auth_source_default_oidc_grant_on_signup"`
	AuthSourceDefaultOIDCGrantOnFirstBind    *bool                             `json:"auth_source_default_oidc_grant_on_first_bind"`
	AuthSourceDefaultWeChatBalance           *float64                          `json:"auth_source_default_wechat_balance"`
	AuthSourceDefaultWeChatConcurrency       *int                              `json:"auth_source_default_wechat_concurrency"`
	AuthSourceDefaultWeChatSubscriptions     *[]dto.DefaultSubscriptionSetting `json:"auth_source_default_wechat_subscriptions"`
	AuthSourceDefaultWeChatGrantOnSignup     *bool                             `json:"auth_source_default_wechat_grant_on_signup"`
	AuthSourceDefaultWeChatGrantOnFirstBind  *bool                             `json:"auth_source_default_wechat_grant_on_first_bind"`
	AuthSourceDefaultGitHubBalance           *float64                          `json:"auth_source_default_github_balance"`
	AuthSourceDefaultGitHubConcurrency       *int                              `json:"auth_source_default_github_concurrency"`
	AuthSourceDefaultGitHubSubscriptions     *[]dto.DefaultSubscriptionSetting `json:"auth_source_default_github_subscriptions"`
	AuthSourceDefaultGitHubGrantOnSignup     *bool                             `json:"auth_source_default_github_grant_on_signup"`
	AuthSourceDefaultGitHubGrantOnFirstBind  *bool                             `json:"auth_source_default_github_grant_on_first_bind"`
	AuthSourceDefaultGoogleBalance           *float64                          `json:"auth_source_default_google_balance"`
	AuthSourceDefaultGoogleConcurrency       *int                              `json:"auth_source_default_google_concurrency"`
	AuthSourceDefaultGoogleSubscriptions     *[]dto.DefaultSubscriptionSetting `json:"auth_source_default_google_subscriptions"`
	AuthSourceDefaultGoogleGrantOnSignup     *bool                             `json:"auth_source_default_google_grant_on_signup"`
	AuthSourceDefaultGoogleGrantOnFirstBind  *bool                             `json:"auth_source_default_google_grant_on_first_bind"`
	ForceEmailOnThirdPartySignup             *bool                             `json:"force_email_on_third_party_signup"`

	// Model fallback configuration
	EnableModelFallback      bool   `json:"enable_model_fallback"`
	FallbackModelAnthropic   string `json:"fallback_model_anthropic"`
	FallbackModelOpenAI      string `json:"fallback_model_openai"`
	FallbackModelGemini      string `json:"fallback_model_gemini"`
	FallbackModelAntigravity string `json:"fallback_model_antigravity"`

	// Identity patch configuration (Claude -> Gemini)
	EnableIdentityPatch bool   `json:"enable_identity_patch"`
	IdentityPatchPrompt string `json:"identity_patch_prompt"`

	// Ops monitoring (vNext)
	OpsMonitoringEnabled         *bool   `json:"ops_monitoring_enabled"`
	OpsRealtimeMonitoringEnabled *bool   `json:"ops_realtime_monitoring_enabled"`
	OpsQueryModeDefault          *string `json:"ops_query_mode_default"`
	OpsMetricsIntervalSeconds    *int    `json:"ops_metrics_interval_seconds"`

	MinClaudeCodeVersion string `json:"min_claude_code_version"`
	MaxClaudeCodeVersion string `json:"max_claude_code_version"`

	// 分组隔离
	AllowUngroupedKeyScheduling bool `json:"allow_ungrouped_key_scheduling"`

	// Backend Mode
	BackendModeEnabled bool `json:"backend_mode_enabled"`

	// Gateway forwarding behavior
	EnableFingerprintUnification         *bool   `json:"enable_fingerprint_unification"`
	EnableMetadataPassthrough            *bool   `json:"enable_metadata_passthrough"`
	EnableCCHSigning                     *bool   `json:"enable_cch_signing"`
	EnableAnthropicCacheTTL1hInjection   *bool   `json:"enable_anthropic_cache_ttl_1h_injection"`
	OpenAIImagesResponsesReasoningEffort *string `json:"openai_images_responses_reasoning_effort"`

	// Payment visible method routing
	PaymentVisibleMethodAlipaySource  *string `json:"payment_visible_method_alipay_source"`
	PaymentVisibleMethodWxpaySource   *string `json:"payment_visible_method_wxpay_source"`
	PaymentVisibleMethodAlipayEnabled *bool   `json:"payment_visible_method_alipay_enabled"`
	PaymentVisibleMethodWxpayEnabled  *bool   `json:"payment_visible_method_wxpay_enabled"`

	// OpenAI account scheduling
	OpenAIAdvancedSchedulerEnabled            *bool    `json:"openai_advanced_scheduler_enabled"`
	OpenAIFreeAccountRepairEnabled            *bool    `json:"openai_free_account_repair_enabled"`
	OpenAIFreeAccountRepairWeeklyThresholdUSD *float64 `json:"openai_free_account_repair_weekly_threshold_usd"`

	// Balance low notification
	BalanceLowNotifyEnabled     *bool                   `json:"balance_low_notify_enabled"`
	BalanceLowNotifyThreshold   *float64                `json:"balance_low_notify_threshold"`
	BalanceLowNotifyRechargeURL *string                 `json:"balance_low_notify_recharge_url"`
	AccountQuotaNotifyEnabled   *bool                   `json:"account_quota_notify_enabled"`
	AccountQuotaNotifyEmails    *[]dto.NotifyEmailEntry `json:"account_quota_notify_emails"`

	// Payment configuration (integrated into settings, full replace)
	PaymentEnabled                   *bool    `json:"payment_enabled"`
	PaymentMinAmount                 *float64 `json:"payment_min_amount"`
	PaymentMaxAmount                 *float64 `json:"payment_max_amount"`
	PaymentDailyLimit                *float64 `json:"payment_daily_limit"`
	PaymentOrderTimeoutMin           *int     `json:"payment_order_timeout_minutes"`
	PaymentMaxPendingOrders          *int     `json:"payment_max_pending_orders"`
	PaymentEnabledTypes              []string `json:"payment_enabled_types"`
	PaymentBalanceDisabled           *bool    `json:"payment_balance_disabled"`
	PaymentBalanceRechargeMultiplier *float64 `json:"payment_balance_recharge_multiplier"`
	PaymentRechargeFeeRate           *float64 `json:"payment_recharge_fee_rate"`
	PaymentLoadBalanceStrat          *string  `json:"payment_load_balance_strategy"`
	PaymentProductNamePrefix         *string  `json:"payment_product_name_prefix"`
	PaymentProductNameSuffix         *string  `json:"payment_product_name_suffix"`
	PaymentHelpImageURL              *string  `json:"payment_help_image_url"`
	PaymentHelpText                  *string  `json:"payment_help_text"`

	PaymentReceiptCodeOSSEnabled              *bool   `json:"payment_receipt_code_oss_enabled"`
	PaymentReceiptCodeOSSEndpoint             *string `json:"payment_receipt_code_oss_endpoint"`
	PaymentReceiptCodeOSSRegion               *string `json:"payment_receipt_code_oss_region"`
	PaymentReceiptCodeOSSBucket               *string `json:"payment_receipt_code_oss_bucket"`
	PaymentReceiptCodeOSSAccessKeyID          *string `json:"payment_receipt_code_oss_access_key_id"`
	PaymentReceiptCodeOSSSecretAccessKey      *string `json:"payment_receipt_code_oss_secret_access_key"`
	PaymentReceiptCodeOSSPrefix               *string `json:"payment_receipt_code_oss_prefix"`
	PaymentReceiptCodeOSSPublicBaseURL        *string `json:"payment_receipt_code_oss_public_base_url"`
	PaymentReceiptCodeOSSForcePathStyle       *bool   `json:"payment_receipt_code_oss_force_path_style"`
	PaymentReceiptCodeOSSMaxSizeBytes         *int64  `json:"payment_receipt_code_oss_max_size_bytes"`
	PaymentReceiptCodeOSSPresignExpireSeconds *int    `json:"payment_receipt_code_oss_presign_expire_seconds"`

	// Cancel rate limit
	PaymentCancelRateLimitEnabled *bool   `json:"payment_cancel_rate_limit_enabled"`
	PaymentCancelRateLimitMax     *int    `json:"payment_cancel_rate_limit_max"`
	PaymentCancelRateLimitWindow  *int    `json:"payment_cancel_rate_limit_window"`
	PaymentCancelRateLimitUnit    *string `json:"payment_cancel_rate_limit_unit"`
	PaymentCancelRateLimitMode    *string `json:"payment_cancel_rate_limit_window_mode"`

	// Channel Monitor feature switch
	ChannelMonitorEnabled                *bool `json:"channel_monitor_enabled"`
	ChannelMonitorDefaultIntervalSeconds *int  `json:"channel_monitor_default_interval_seconds"`

	// Available Channels feature switch (user-facing)
	AvailableChannelsEnabled *bool `json:"available_channels_enabled"`

	// Free Models feature switch (user-facing)
	FreeModelsEnabled *bool `json:"free_models_enabled"`

	// Auto model routing
	AutoModelSettings *dto.AutoModelSettings `json:"auto_model_settings"`

	// Carpool Pools feature switch
	CarpoolEnabled           *bool    `json:"carpool_enabled"`
	CarpoolBaseServiceFeeUSD *float64 `json:"carpool_base_service_fee_usd"`
	CarpoolSystemProxyFeeUSD *float64 `json:"carpool_system_proxy_fee_usd"`
	CarpoolRiskControlFeeUSD *float64 `json:"carpool_risk_control_fee_usd"`

	// Affiliate (邀请返利) feature switch
	AffiliateEnabled *bool `json:"affiliate_enabled"`

	// OpenAI fast/flex policy (optional, only updated when provided)
	OpenAIFastPolicySettings *dto.OpenAIFastPolicySettings `json:"openai_fast_policy_settings,omitempty"`
}

func normalizeCustomEndpointURL(raw string) string {
	value := strings.TrimSpace(raw)
	if value == "" {
		return ""
	}
	if !strings.Contains(value, "://") {
		value = "https://" + value
	}
	return strings.TrimRight(value, "/")
}

// UpdateSettings 更新系统设置
// PUT /api/v1/admin/settings
func (h *SettingHandler) UpdateSettings(c *gin.Context) {
	var req UpdateSettingsRequest
	rawBody, err := io.ReadAll(c.Request.Body)
	if err != nil {
		response.BadRequest(c, "Invalid request: "+err.Error())
		return
	}
	if err := json.Unmarshal(rawBody, &req); err != nil {
		response.BadRequest(c, "Invalid request: "+err.Error())
		return
	}
	providedFields, err := decodeProvidedJSONFields(rawBody)
	if err != nil {
		response.BadRequest(c, "Invalid request: "+err.Error())
		return
	}

	previousSettings, err := h.settingService.GetAllSettings(c.Request.Context())
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}
	preserveOmittedUpdateSettingsFields(&req, previousSettings, providedFields)
	previousAuthSourceDefaults, err := h.settingService.GetAuthSourceDefaultSettings(c.Request.Context())
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}

	if req.DefaultConcurrency < 1 {
		req.DefaultConcurrency = 1
	}
	if req.DefaultBalance < 0 {
		req.DefaultBalance = 0
	}
	affiliateRebateRate := previousSettings.AffiliateRebateRate
	if req.AffiliateRebateRate != nil {
		affiliateRebateRate = *req.AffiliateRebateRate
	}
	if affiliateRebateRate < service.AffiliateRebateRateMin {
		affiliateRebateRate = service.AffiliateRebateRateMin
	}
	if affiliateRebateRate > service.AffiliateRebateRateMax {
		affiliateRebateRate = service.AffiliateRebateRateMax
	}
	affiliateRebateFreezeHours := previousSettings.AffiliateRebateFreezeHours
	if req.AffiliateRebateFreezeHours != nil {
		affiliateRebateFreezeHours = *req.AffiliateRebateFreezeHours
	}
	if affiliateRebateFreezeHours < 0 {
		affiliateRebateFreezeHours = service.AffiliateRebateFreezeHoursDefault
	}
	if affiliateRebateFreezeHours > service.AffiliateRebateFreezeHoursMax {
		affiliateRebateFreezeHours = service.AffiliateRebateFreezeHoursMax
	}
	affiliateRebateDurationDays := previousSettings.AffiliateRebateDurationDays
	if req.AffiliateRebateDurationDays != nil {
		affiliateRebateDurationDays = *req.AffiliateRebateDurationDays
	}
	if affiliateRebateDurationDays < 0 {
		affiliateRebateDurationDays = service.AffiliateRebateDurationDaysDefault
	}
	if affiliateRebateDurationDays > service.AffiliateRebateDurationDaysMax {
		affiliateRebateDurationDays = service.AffiliateRebateDurationDaysMax
	}
	affiliateRebatePerInviteeCap := previousSettings.AffiliateRebatePerInviteeCap
	if req.AffiliateRebatePerInviteeCap != nil {
		affiliateRebatePerInviteeCap = *req.AffiliateRebatePerInviteeCap
	}
	if affiliateRebatePerInviteeCap < 0 {
		affiliateRebatePerInviteeCap = service.AffiliateRebatePerInviteeCapDefault
	}
	// 通用表格配置：兼容旧客户端未传字段时保留当前值。
	if req.TableDefaultPageSize <= 0 {
		req.TableDefaultPageSize = previousSettings.TableDefaultPageSize
	}
	if req.TablePageSizeOptions == nil {
		req.TablePageSizeOptions = previousSettings.TablePageSizeOptions
	}
	req.SMTPHost = strings.TrimSpace(req.SMTPHost)
	req.SMTPUsername = strings.TrimSpace(req.SMTPUsername)
	req.SMTPPassword = strings.TrimSpace(req.SMTPPassword)
	req.SMTPFrom = strings.TrimSpace(req.SMTPFrom)
	req.SMTPFromName = strings.TrimSpace(req.SMTPFromName)
	if req.SMTPPort <= 0 {
		req.SMTPPort = 587
	}
	req.DefaultSubscriptions = normalizeDefaultSubscriptions(req.DefaultSubscriptions)
	req.AuthSourceDefaultEmailSubscriptions = normalizeOptionalDefaultSubscriptions(req.AuthSourceDefaultEmailSubscriptions)
	req.AuthSourceDefaultLinuxDoSubscriptions = normalizeOptionalDefaultSubscriptions(req.AuthSourceDefaultLinuxDoSubscriptions)
	req.AuthSourceDefaultOIDCSubscriptions = normalizeOptionalDefaultSubscriptions(req.AuthSourceDefaultOIDCSubscriptions)
	req.AuthSourceDefaultWeChatSubscriptions = normalizeOptionalDefaultSubscriptions(req.AuthSourceDefaultWeChatSubscriptions)
	req.AuthSourceDefaultGitHubSubscriptions = normalizeOptionalDefaultSubscriptions(req.AuthSourceDefaultGitHubSubscriptions)
	req.AuthSourceDefaultGoogleSubscriptions = normalizeOptionalDefaultSubscriptions(req.AuthSourceDefaultGoogleSubscriptions)
	req.GitHubOAuthClientID = strings.TrimSpace(req.GitHubOAuthClientID)
	req.GitHubOAuthClientSecret = strings.TrimSpace(req.GitHubOAuthClientSecret)
	req.GitHubOAuthRedirectURL = strings.TrimSpace(req.GitHubOAuthRedirectURL)
	req.GitHubOAuthFrontendRedirectURL = strings.TrimSpace(req.GitHubOAuthFrontendRedirectURL)
	req.GoogleOAuthClientID = strings.TrimSpace(req.GoogleOAuthClientID)
	req.GoogleOAuthClientSecret = strings.TrimSpace(req.GoogleOAuthClientSecret)
	req.GoogleOAuthRedirectURL = strings.TrimSpace(req.GoogleOAuthRedirectURL)
	req.GoogleOAuthFrontendRedirectURL = strings.TrimSpace(req.GoogleOAuthFrontendRedirectURL)

	// SMTP 配置保护：如果请求中 smtp_host 为空但数据库中已有配置，则保留已有 SMTP 配置
	// 防止前端加载设置失败时空表单覆盖已保存的 SMTP 配置
	if req.SMTPHost == "" && previousSettings.SMTPHost != "" {
		req.SMTPHost = previousSettings.SMTPHost
		req.SMTPPort = previousSettings.SMTPPort
		req.SMTPUsername = previousSettings.SMTPUsername
		req.SMTPFrom = previousSettings.SMTPFrom
		req.SMTPFromName = previousSettings.SMTPFromName
		req.SMTPUseTLS = previousSettings.SMTPUseTLS
	}

	// Turnstile 参数验证
	if req.TurnstileEnabled {
		// 检查必填字段
		if req.TurnstileSiteKey == "" {
			response.BadRequest(c, "Turnstile Site Key is required when enabled")
			return
		}
		// 如果未提供 secret key，使用已保存的值（留空保留当前值）
		if req.TurnstileSecretKey == "" {
			if previousSettings.TurnstileSecretKey == "" {
				response.BadRequest(c, "Turnstile Secret Key is required when enabled")
				return
			}
			req.TurnstileSecretKey = previousSettings.TurnstileSecretKey
		}

		// 当 site_key 或 secret_key 任一变化时验证（避免配置错误导致无法登录）
		siteKeyChanged := previousSettings.TurnstileSiteKey != req.TurnstileSiteKey
		secretKeyChanged := previousSettings.TurnstileSecretKey != req.TurnstileSecretKey
		if siteKeyChanged || secretKeyChanged {
			if err := h.turnstileService.ValidateSecretKey(c.Request.Context(), req.TurnstileSecretKey); err != nil {
				response.ErrorFrom(c, err)
				return
			}
		}
	}

	// TOTP 双因素认证参数验证
	// 只有手动配置了加密密钥才允许启用 TOTP 功能
	if req.TotpEnabled && !previousSettings.TotpEnabled {
		// 尝试启用 TOTP，检查加密密钥是否已手动配置
		if !h.settingService.IsTotpEncryptionKeyConfigured() {
			response.BadRequest(c, "Cannot enable TOTP: TOTP_ENCRYPTION_KEY environment variable must be configured first. Generate a key with 'openssl rand -hex 32' and set it in your environment.")
			return
		}
	}

	// LinuxDo Connect 参数验证
	if req.LinuxDoConnectEnabled {
		req.LinuxDoConnectClientID = strings.TrimSpace(req.LinuxDoConnectClientID)
		req.LinuxDoConnectClientSecret = strings.TrimSpace(req.LinuxDoConnectClientSecret)
		req.LinuxDoConnectRedirectURL = strings.TrimSpace(req.LinuxDoConnectRedirectURL)

		if req.LinuxDoConnectClientID == "" {
			response.BadRequest(c, "LinuxDo Client ID is required when enabled")
			return
		}
		if req.LinuxDoConnectRedirectURL == "" {
			response.BadRequest(c, "LinuxDo Redirect URL is required when enabled")
			return
		}
		if err := config.ValidateAbsoluteHTTPURL(req.LinuxDoConnectRedirectURL); err != nil {
			response.BadRequest(c, "LinuxDo Redirect URL must be an absolute http(s) URL")
			return
		}

		// 如果未提供 client_secret，则保留现有值（如有）。
		if req.LinuxDoConnectClientSecret == "" {
			if previousSettings.LinuxDoConnectClientSecret == "" {
				response.BadRequest(c, "LinuxDo Client Secret is required when enabled")
				return
			}
			req.LinuxDoConnectClientSecret = previousSettings.LinuxDoConnectClientSecret
		}
	}

	if req.WeChatConnectEnabled {
		req.WeChatConnectAppID = strings.TrimSpace(req.WeChatConnectAppID)
		req.WeChatConnectAppSecret = strings.TrimSpace(req.WeChatConnectAppSecret)
		req.WeChatConnectOpenAppID = strings.TrimSpace(req.WeChatConnectOpenAppID)
		req.WeChatConnectOpenAppSecret = strings.TrimSpace(req.WeChatConnectOpenAppSecret)
		req.WeChatConnectMPAppID = strings.TrimSpace(req.WeChatConnectMPAppID)
		req.WeChatConnectMPAppSecret = strings.TrimSpace(req.WeChatConnectMPAppSecret)
		req.WeChatConnectMobileAppID = strings.TrimSpace(req.WeChatConnectMobileAppID)
		req.WeChatConnectMobileAppSecret = strings.TrimSpace(req.WeChatConnectMobileAppSecret)
		req.WeChatConnectMode = strings.ToLower(strings.TrimSpace(req.WeChatConnectMode))
		req.WeChatConnectScopes = strings.TrimSpace(req.WeChatConnectScopes)
		req.WeChatConnectRedirectURL = strings.TrimSpace(req.WeChatConnectRedirectURL)
		req.WeChatConnectFrontendRedirectURL = strings.TrimSpace(req.WeChatConnectFrontendRedirectURL)
		req.WeChatConnectAppID = strings.TrimSpace(firstNonEmpty(req.WeChatConnectAppID, previousSettings.WeChatConnectAppID))
		req.WeChatConnectRedirectURL = strings.TrimSpace(firstNonEmpty(req.WeChatConnectRedirectURL, previousSettings.WeChatConnectRedirectURL))
		req.WeChatConnectFrontendRedirectURL = strings.TrimSpace(firstNonEmpty(req.WeChatConnectFrontendRedirectURL, previousSettings.WeChatConnectFrontendRedirectURL))
		if req.WeChatConnectMode == "" {
			req.WeChatConnectMode = strings.ToLower(strings.TrimSpace(previousSettings.WeChatConnectMode))
		}
		if req.WeChatConnectScopes == "" {
			req.WeChatConnectScopes = strings.TrimSpace(previousSettings.WeChatConnectScopes)
		}

		if req.WeChatConnectMPEnabled && req.WeChatConnectMobileEnabled {
			response.BadRequest(c, "WeChat Official Account and Mobile App cannot be enabled at the same time")
			return
		}
		if req.WeChatConnectMode != "" {
			switch req.WeChatConnectMode {
			case "open", "mp", "mobile":
			default:
				response.BadRequest(c, "WeChat mode must be open, mp, or mobile")
				return
			}
		}
		if !req.WeChatConnectOpenEnabled && !req.WeChatConnectMPEnabled && !req.WeChatConnectMobileEnabled {
			switch req.WeChatConnectMode {
			case "mp":
				req.WeChatConnectMPEnabled = true
			case "mobile":
				req.WeChatConnectMobileEnabled = true
			default:
				req.WeChatConnectOpenEnabled = true
			}
		}
		if req.WeChatConnectMode == "" {
			if req.WeChatConnectMPEnabled {
				req.WeChatConnectMode = "mp"
			} else if req.WeChatConnectMobileEnabled {
				req.WeChatConnectMode = "mobile"
			} else {
				req.WeChatConnectMode = "open"
			}
		}

		req.WeChatConnectOpenAppID = strings.TrimSpace(firstNonEmpty(req.WeChatConnectOpenAppID, req.WeChatConnectAppID, previousSettings.WeChatConnectOpenAppID, previousSettings.WeChatConnectAppID))
		req.WeChatConnectMPAppID = strings.TrimSpace(firstNonEmpty(req.WeChatConnectMPAppID, req.WeChatConnectAppID, previousSettings.WeChatConnectMPAppID, previousSettings.WeChatConnectAppID))
		req.WeChatConnectMobileAppID = strings.TrimSpace(firstNonEmpty(req.WeChatConnectMobileAppID, req.WeChatConnectAppID, previousSettings.WeChatConnectMobileAppID, previousSettings.WeChatConnectAppID))

		if req.WeChatConnectOpenAppSecret == "" {
			req.WeChatConnectOpenAppSecret = strings.TrimSpace(firstNonEmpty(previousSettings.WeChatConnectOpenAppSecret, previousSettings.WeChatConnectAppSecret, req.WeChatConnectAppSecret))
		}
		if req.WeChatConnectMPAppSecret == "" {
			req.WeChatConnectMPAppSecret = strings.TrimSpace(firstNonEmpty(previousSettings.WeChatConnectMPAppSecret, previousSettings.WeChatConnectAppSecret, req.WeChatConnectAppSecret))
		}
		if req.WeChatConnectMobileAppSecret == "" {
			req.WeChatConnectMobileAppSecret = strings.TrimSpace(firstNonEmpty(previousSettings.WeChatConnectMobileAppSecret, previousSettings.WeChatConnectAppSecret, req.WeChatConnectAppSecret))
		}
		if req.WeChatConnectAppSecret == "" {
			req.WeChatConnectAppSecret = strings.TrimSpace(firstNonEmpty(req.WeChatConnectOpenAppSecret, req.WeChatConnectMPAppSecret, req.WeChatConnectMobileAppSecret, previousSettings.WeChatConnectAppSecret))
		}

		if req.WeChatConnectOpenEnabled {
			if req.WeChatConnectOpenAppID == "" {
				response.BadRequest(c, "WeChat PC App ID is required when enabled")
				return
			}
			if req.WeChatConnectOpenAppSecret == "" {
				response.BadRequest(c, "WeChat PC App Secret is required when enabled")
				return
			}
		}
		if req.WeChatConnectMPEnabled {
			if req.WeChatConnectMPAppID == "" {
				response.BadRequest(c, "WeChat Official Account App ID is required when enabled")
				return
			}
			if req.WeChatConnectMPAppSecret == "" {
				response.BadRequest(c, "WeChat Official Account App Secret is required when enabled")
				return
			}
		}
		if req.WeChatConnectMobileEnabled {
			if req.WeChatConnectMobileAppID == "" {
				response.BadRequest(c, "WeChat Mobile App ID is required when enabled")
				return
			}
			if req.WeChatConnectMobileAppSecret == "" {
				response.BadRequest(c, "WeChat Mobile App Secret is required when enabled")
				return
			}
		}

		if req.WeChatConnectScopes == "" {
			if req.WeChatConnectMPEnabled {
				req.WeChatConnectScopes = service.DefaultWeChatConnectScopesForMode("mp")
			} else {
				req.WeChatConnectScopes = service.DefaultWeChatConnectScopesForMode(req.WeChatConnectMode)
			}
		}
		if req.WeChatConnectOpenEnabled || req.WeChatConnectMPEnabled {
			if req.WeChatConnectRedirectURL == "" {
				response.BadRequest(c, "WeChat Redirect URL is required when web oauth is enabled")
				return
			}
			if err := config.ValidateAbsoluteHTTPURL(req.WeChatConnectRedirectURL); err != nil {
				response.BadRequest(c, "WeChat Redirect URL must be an absolute http(s) URL")
				return
			}
			if req.WeChatConnectFrontendRedirectURL == "" {
				req.WeChatConnectFrontendRedirectURL = "/auth/wechat/callback"
			}
			if err := config.ValidateFrontendRedirectURL(req.WeChatConnectFrontendRedirectURL); err != nil {
				response.BadRequest(c, "WeChat Frontend Redirect URL is invalid")
				return
			}
		}
	}

	// Generic OIDC 参数验证
	oidcUsePKCE, oidcValidateIDToken, err := h.settingService.OIDCSecurityWriteDefaults(c.Request.Context())
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}
	if req.OIDCConnectEnabled {
		req.OIDCConnectProviderName = strings.TrimSpace(req.OIDCConnectProviderName)
		req.OIDCConnectClientID = strings.TrimSpace(req.OIDCConnectClientID)
		req.OIDCConnectClientSecret = strings.TrimSpace(req.OIDCConnectClientSecret)
		req.OIDCConnectIssuerURL = strings.TrimSpace(req.OIDCConnectIssuerURL)
		req.OIDCConnectDiscoveryURL = strings.TrimSpace(req.OIDCConnectDiscoveryURL)
		req.OIDCConnectAuthorizeURL = strings.TrimSpace(req.OIDCConnectAuthorizeURL)
		req.OIDCConnectTokenURL = strings.TrimSpace(req.OIDCConnectTokenURL)
		req.OIDCConnectUserInfoURL = strings.TrimSpace(req.OIDCConnectUserInfoURL)
		req.OIDCConnectJWKSURL = strings.TrimSpace(req.OIDCConnectJWKSURL)
		req.OIDCConnectScopes = strings.TrimSpace(req.OIDCConnectScopes)
		req.OIDCConnectRedirectURL = strings.TrimSpace(req.OIDCConnectRedirectURL)
		req.OIDCConnectFrontendRedirectURL = strings.TrimSpace(req.OIDCConnectFrontendRedirectURL)
		req.OIDCConnectTokenAuthMethod = strings.ToLower(strings.TrimSpace(req.OIDCConnectTokenAuthMethod))
		req.OIDCConnectAllowedSigningAlgs = strings.TrimSpace(req.OIDCConnectAllowedSigningAlgs)
		req.OIDCConnectUserInfoEmailPath = strings.TrimSpace(req.OIDCConnectUserInfoEmailPath)
		req.OIDCConnectUserInfoIDPath = strings.TrimSpace(req.OIDCConnectUserInfoIDPath)
		req.OIDCConnectUserInfoUsernamePath = strings.TrimSpace(req.OIDCConnectUserInfoUsernamePath)
		req.OIDCConnectProviderName = strings.TrimSpace(firstNonEmpty(req.OIDCConnectProviderName, previousSettings.OIDCConnectProviderName, "OIDC"))
		req.OIDCConnectClientID = strings.TrimSpace(firstNonEmpty(req.OIDCConnectClientID, previousSettings.OIDCConnectClientID))
		req.OIDCConnectIssuerURL = strings.TrimSpace(firstNonEmpty(req.OIDCConnectIssuerURL, previousSettings.OIDCConnectIssuerURL))
		req.OIDCConnectDiscoveryURL = strings.TrimSpace(firstNonEmpty(req.OIDCConnectDiscoveryURL, previousSettings.OIDCConnectDiscoveryURL))
		req.OIDCConnectAuthorizeURL = strings.TrimSpace(firstNonEmpty(req.OIDCConnectAuthorizeURL, previousSettings.OIDCConnectAuthorizeURL))
		req.OIDCConnectTokenURL = strings.TrimSpace(firstNonEmpty(req.OIDCConnectTokenURL, previousSettings.OIDCConnectTokenURL))
		req.OIDCConnectUserInfoURL = strings.TrimSpace(firstNonEmpty(req.OIDCConnectUserInfoURL, previousSettings.OIDCConnectUserInfoURL))
		req.OIDCConnectJWKSURL = strings.TrimSpace(firstNonEmpty(req.OIDCConnectJWKSURL, previousSettings.OIDCConnectJWKSURL))
		req.OIDCConnectScopes = strings.TrimSpace(firstNonEmpty(req.OIDCConnectScopes, previousSettings.OIDCConnectScopes, "openid email profile"))
		req.OIDCConnectRedirectURL = strings.TrimSpace(firstNonEmpty(req.OIDCConnectRedirectURL, previousSettings.OIDCConnectRedirectURL))
		req.OIDCConnectFrontendRedirectURL = strings.TrimSpace(firstNonEmpty(req.OIDCConnectFrontendRedirectURL, previousSettings.OIDCConnectFrontendRedirectURL, "/auth/oidc/callback"))
		req.OIDCConnectTokenAuthMethod = strings.ToLower(strings.TrimSpace(firstNonEmpty(req.OIDCConnectTokenAuthMethod, previousSettings.OIDCConnectTokenAuthMethod, "client_secret_post")))
		req.OIDCConnectAllowedSigningAlgs = strings.TrimSpace(firstNonEmpty(req.OIDCConnectAllowedSigningAlgs, previousSettings.OIDCConnectAllowedSigningAlgs, "RS256,ES256,PS256"))
		req.OIDCConnectUserInfoEmailPath = strings.TrimSpace(firstNonEmpty(req.OIDCConnectUserInfoEmailPath, previousSettings.OIDCConnectUserInfoEmailPath))
		req.OIDCConnectUserInfoIDPath = strings.TrimSpace(firstNonEmpty(req.OIDCConnectUserInfoIDPath, previousSettings.OIDCConnectUserInfoIDPath))
		req.OIDCConnectUserInfoUsernamePath = strings.TrimSpace(firstNonEmpty(req.OIDCConnectUserInfoUsernamePath, previousSettings.OIDCConnectUserInfoUsernamePath))
		if req.OIDCConnectUsePKCE != nil {
			oidcUsePKCE = *req.OIDCConnectUsePKCE
		}
		if req.OIDCConnectValidateIDToken != nil {
			oidcValidateIDToken = *req.OIDCConnectValidateIDToken
		}
		if req.OIDCConnectClockSkewSeconds == 0 {
			req.OIDCConnectClockSkewSeconds = previousSettings.OIDCConnectClockSkewSeconds
			if req.OIDCConnectClockSkewSeconds == 0 {
				req.OIDCConnectClockSkewSeconds = 120
			}
		}

		if req.OIDCConnectClientID == "" {
			response.BadRequest(c, "OIDC Client ID is required when enabled")
			return
		}
		if req.OIDCConnectIssuerURL == "" {
			response.BadRequest(c, "OIDC Issuer URL is required when enabled")
			return
		}
		if err := config.ValidateAbsoluteHTTPURL(req.OIDCConnectIssuerURL); err != nil {
			response.BadRequest(c, "OIDC Issuer URL must be an absolute http(s) URL")
			return
		}
		if req.OIDCConnectDiscoveryURL != "" {
			if err := config.ValidateAbsoluteHTTPURL(req.OIDCConnectDiscoveryURL); err != nil {
				response.BadRequest(c, "OIDC Discovery URL must be an absolute http(s) URL")
				return
			}
		}
		if req.OIDCConnectAuthorizeURL != "" {
			if err := config.ValidateAbsoluteHTTPURL(req.OIDCConnectAuthorizeURL); err != nil {
				response.BadRequest(c, "OIDC Authorize URL must be an absolute http(s) URL")
				return
			}
		}
		if req.OIDCConnectTokenURL != "" {
			if err := config.ValidateAbsoluteHTTPURL(req.OIDCConnectTokenURL); err != nil {
				response.BadRequest(c, "OIDC Token URL must be an absolute http(s) URL")
				return
			}
		}
		if req.OIDCConnectUserInfoURL != "" {
			if err := config.ValidateAbsoluteHTTPURL(req.OIDCConnectUserInfoURL); err != nil {
				response.BadRequest(c, "OIDC UserInfo URL must be an absolute http(s) URL")
				return
			}
		}
		if req.OIDCConnectRedirectURL == "" {
			response.BadRequest(c, "OIDC Redirect URL is required when enabled")
			return
		}
		if err := config.ValidateAbsoluteHTTPURL(req.OIDCConnectRedirectURL); err != nil {
			response.BadRequest(c, "OIDC Redirect URL must be an absolute http(s) URL")
			return
		}
		if req.OIDCConnectFrontendRedirectURL == "" {
			response.BadRequest(c, "OIDC Frontend Redirect URL is required when enabled")
			return
		}
		if err := config.ValidateFrontendRedirectURL(req.OIDCConnectFrontendRedirectURL); err != nil {
			response.BadRequest(c, "OIDC Frontend Redirect URL is invalid")
			return
		}
		if !scopesContainOpenID(req.OIDCConnectScopes) {
			response.BadRequest(c, "OIDC scopes must contain openid")
			return
		}
		switch req.OIDCConnectTokenAuthMethod {
		case "", "client_secret_post", "client_secret_basic", "none":
		default:
			response.BadRequest(c, "OIDC Token Auth Method must be one of client_secret_post/client_secret_basic/none")
			return
		}
		if req.OIDCConnectClockSkewSeconds < 0 || req.OIDCConnectClockSkewSeconds > 600 {
			response.BadRequest(c, "OIDC clock skew seconds must be between 0 and 600")
			return
		}
		if oidcValidateIDToken && req.OIDCConnectAllowedSigningAlgs == "" {
			response.BadRequest(c, "OIDC Allowed Signing Algs is required when validate_id_token=true")
			return
		}
		if req.OIDCConnectJWKSURL != "" {
			if err := config.ValidateAbsoluteHTTPURL(req.OIDCConnectJWKSURL); err != nil {
				response.BadRequest(c, "OIDC JWKS URL must be an absolute http(s) URL")
				return
			}
		}
		if req.OIDCConnectTokenAuthMethod == "" || req.OIDCConnectTokenAuthMethod == "client_secret_post" || req.OIDCConnectTokenAuthMethod == "client_secret_basic" {
			if req.OIDCConnectClientSecret == "" {
				if previousSettings.OIDCConnectClientSecret == "" {
					response.BadRequest(c, "OIDC Client Secret is required when enabled")
					return
				}
				req.OIDCConnectClientSecret = previousSettings.OIDCConnectClientSecret
			}
		}
	}

	// “购买订阅”页面配置验证
	purchaseEnabled := previousSettings.PurchaseSubscriptionEnabled
	if req.PurchaseSubscriptionEnabled != nil {
		purchaseEnabled = *req.PurchaseSubscriptionEnabled
	}
	purchaseURL := previousSettings.PurchaseSubscriptionURL
	if req.PurchaseSubscriptionURL != nil {
		purchaseURL = strings.TrimSpace(*req.PurchaseSubscriptionURL)
	}

	// - 启用时要求 URL 合法且非空
	// - 禁用时允许为空；若提供了 URL 也做基本校验，避免误配置
	if purchaseEnabled {
		if purchaseURL == "" {
			response.BadRequest(c, "Purchase Subscription URL is required when enabled")
			return
		}
		if err := config.ValidateAbsoluteHTTPURL(purchaseURL); err != nil {
			response.BadRequest(c, "Purchase Subscription URL must be an absolute http(s) URL")
			return
		}
	} else if purchaseURL != "" {
		if err := config.ValidateAbsoluteHTTPURL(purchaseURL); err != nil {
			response.BadRequest(c, "Purchase Subscription URL must be an absolute http(s) URL")
			return
		}
	}

	// Frontend URL 验证
	req.FrontendURL = strings.TrimSpace(req.FrontendURL)
	if req.FrontendURL != "" {
		if err := config.ValidateAbsoluteHTTPURL(req.FrontendURL); err != nil {
			response.BadRequest(c, "Frontend URL must be an absolute http(s) URL")
			return
		}
	}

	// 自定义菜单项验证
	const (
		maxCustomMenuItems    = 20
		maxMenuItemLabelLen   = 50
		maxMenuItemURLLen     = 2048
		maxMenuItemIconSVGLen = 10 * 1024 // 10KB
		maxMenuItemIDLen      = 32
	)

	customMenuJSON := previousSettings.CustomMenuItems
	if req.CustomMenuItems != nil {
		items := *req.CustomMenuItems
		if len(items) > maxCustomMenuItems {
			response.BadRequest(c, "Too many custom menu items (max 20)")
			return
		}
		for i, item := range items {
			if strings.TrimSpace(item.Label) == "" {
				response.BadRequest(c, "Custom menu item label is required")
				return
			}
			if len(item.Label) > maxMenuItemLabelLen {
				response.BadRequest(c, "Custom menu item label is too long (max 50 characters)")
				return
			}
			if strings.TrimSpace(item.URL) == "" {
				response.BadRequest(c, "Custom menu item URL is required")
				return
			}
			if len(item.URL) > maxMenuItemURLLen {
				response.BadRequest(c, "Custom menu item URL is too long (max 2048 characters)")
				return
			}
			if err := config.ValidateAbsoluteHTTPURL(strings.TrimSpace(item.URL)); err != nil {
				response.BadRequest(c, "Custom menu item URL must be an absolute http(s) URL")
				return
			}
			if item.Visibility != "user" && item.Visibility != "admin" {
				response.BadRequest(c, "Custom menu item visibility must be 'user' or 'admin'")
				return
			}
			if len(item.IconSVG) > maxMenuItemIconSVGLen {
				response.BadRequest(c, "Custom menu item icon SVG is too large (max 10KB)")
				return
			}
			// Auto-generate ID if missing
			if strings.TrimSpace(item.ID) == "" {
				id, err := generateMenuItemID()
				if err != nil {
					response.Error(c, http.StatusInternalServerError, "Failed to generate menu item ID")
					return
				}
				items[i].ID = id
			} else if len(item.ID) > maxMenuItemIDLen {
				response.BadRequest(c, "Custom menu item ID is too long (max 32 characters)")
				return
			} else if !menuItemIDPattern.MatchString(item.ID) {
				response.BadRequest(c, "Custom menu item ID contains invalid characters (only a-z, A-Z, 0-9, - and _ are allowed)")
				return
			}
		}
		// ID uniqueness check
		seen := make(map[string]struct{}, len(items))
		for _, item := range items {
			if _, exists := seen[item.ID]; exists {
				response.BadRequest(c, "Duplicate custom menu item ID: "+item.ID)
				return
			}
			seen[item.ID] = struct{}{}
		}
		menuBytes, err := json.Marshal(items)
		if err != nil {
			response.BadRequest(c, "Failed to serialize custom menu items")
			return
		}
		customMenuJSON = string(menuBytes)
	}

	// 自定义端点验证
	const (
		maxCustomEndpoints        = 10
		maxEndpointNameLen        = 50
		maxEndpointURLLen         = 2048
		maxEndpointDescriptionLen = 200
	)

	customEndpointsJSON := previousSettings.CustomEndpoints
	if req.CustomEndpoints != nil {
		endpoints := *req.CustomEndpoints
		if len(endpoints) > maxCustomEndpoints {
			response.BadRequest(c, "Too many custom endpoints (max 10)")
			return
		}
		for i := range endpoints {
			endpoints[i].Name = strings.TrimSpace(endpoints[i].Name)
			endpoints[i].Endpoint = normalizeCustomEndpointURL(endpoints[i].Endpoint)
			endpoints[i].Description = strings.TrimSpace(endpoints[i].Description)

			if endpoints[i].Name == "" {
				response.BadRequest(c, "Custom endpoint name is required")
				return
			}
			if len(endpoints[i].Name) > maxEndpointNameLen {
				response.BadRequest(c, "Custom endpoint name is too long (max 50 characters)")
				return
			}
			if endpoints[i].Endpoint == "" {
				response.BadRequest(c, "Custom endpoint URL is required")
				return
			}
			if len(endpoints[i].Endpoint) > maxEndpointURLLen {
				response.BadRequest(c, "Custom endpoint URL is too long (max 2048 characters)")
				return
			}
			if err := config.ValidateAbsoluteHTTPURL(endpoints[i].Endpoint); err != nil {
				response.BadRequest(c, "Custom endpoint URL must be an absolute http(s) URL")
				return
			}
			if len(endpoints[i].Description) > maxEndpointDescriptionLen {
				response.BadRequest(c, "Custom endpoint description is too long (max 200 characters)")
				return
			}
		}
		endpointBytes, err := json.Marshal(endpoints)
		if err != nil {
			response.BadRequest(c, "Failed to serialize custom endpoints")
			return
		}
		customEndpointsJSON = string(endpointBytes)
	}

	// Ops metrics collector interval validation (seconds).
	if req.OpsMetricsIntervalSeconds != nil {
		v := *req.OpsMetricsIntervalSeconds
		if v < 60 {
			v = 60
		}
		if v > 3600 {
			v = 3600
		}
		req.OpsMetricsIntervalSeconds = &v
	}
	defaultSubscriptions := make([]service.DefaultSubscriptionSetting, 0, len(req.DefaultSubscriptions))
	for _, sub := range req.DefaultSubscriptions {
		defaultSubscriptions = append(defaultSubscriptions, service.DefaultSubscriptionSetting{
			GroupID:      sub.GroupID,
			ValidityDays: sub.ValidityDays,
		})
	}

	// 验证最低版本号格式（空字符串=禁用，或合法 semver）
	if req.MinClaudeCodeVersion != "" {
		if !semverPattern.MatchString(req.MinClaudeCodeVersion) {
			response.Error(c, http.StatusBadRequest, "min_claude_code_version must be empty or a valid semver (e.g. 2.1.63)")
			return
		}
	}

	// 验证最高版本号格式（空字符串=禁用，或合法 semver）
	if req.MaxClaudeCodeVersion != "" {
		if !semverPattern.MatchString(req.MaxClaudeCodeVersion) {
			response.Error(c, http.StatusBadRequest, "max_claude_code_version must be empty or a valid semver (e.g. 3.0.0)")
			return
		}
	}
	if req.OpenAIImagesResponsesReasoningEffort != nil {
		normalized := strings.ToLower(strings.TrimSpace(*req.OpenAIImagesResponsesReasoningEffort))
		req.OpenAIImagesResponsesReasoningEffort = &normalized
		if normalized != "" && !service.IsValidOpenAIImagesResponsesReasoningEffort(normalized) {
			response.Error(c, http.StatusBadRequest, "openai_images_responses_reasoning_effort must be one of: low, medium, high, xhigh")
			return
		}
	}

	// 交叉验证：如果同时设置了最低和最高版本号，最高版本号必须 >= 最低版本号
	if req.MinClaudeCodeVersion != "" && req.MaxClaudeCodeVersion != "" {
		if service.CompareVersions(req.MaxClaudeCodeVersion, req.MinClaudeCodeVersion) < 0 {
			response.Error(c, http.StatusBadRequest, "max_claude_code_version must be greater than or equal to min_claude_code_version")
			return
		}
	}

	settings := &service.SystemSettings{
		RegistrationEnabled:              req.RegistrationEnabled,
		EmailVerifyEnabled:               req.EmailVerifyEnabled,
		RegistrationEmailSuffixWhitelist: req.RegistrationEmailSuffixWhitelist,
		PromoCodeEnabled:                 req.PromoCodeEnabled,
		PasswordResetEnabled:             req.PasswordResetEnabled,
		FrontendURL:                      req.FrontendURL,
		InvitationCodeEnabled:            req.InvitationCodeEnabled,
		TotpEnabled:                      req.TotpEnabled,
		LoginAgreementEnabled: func() bool {
			if req.LoginAgreementEnabled != nil {
				return *req.LoginAgreementEnabled
			}
			return previousSettings.LoginAgreementEnabled
		}(),
		LoginAgreementMode: func() string {
			if fieldProvided(providedFields, "login_agreement_mode") {
				return req.LoginAgreementMode
			}
			return previousSettings.LoginAgreementMode
		}(),
		LoginAgreementUpdatedAt: func() string {
			if fieldProvided(providedFields, "login_agreement_updated_at") {
				return req.LoginAgreementUpdatedAt
			}
			return previousSettings.LoginAgreementUpdatedAt
		}(),
		LoginAgreementDocuments: func() []service.LoginAgreementDocument {
			if fieldProvided(providedFields, "login_agreement_documents") {
				return loginAgreementDocumentsToService(req.LoginAgreementDocuments)
			}
			return previousSettings.LoginAgreementDocuments
		}(),
		SMTPHost:           req.SMTPHost,
		SMTPPort:           req.SMTPPort,
		SMTPUsername:       req.SMTPUsername,
		SMTPPassword:       req.SMTPPassword,
		SMTPFrom:           req.SMTPFrom,
		SMTPFromName:       req.SMTPFromName,
		SMTPUseTLS:         req.SMTPUseTLS,
		TurnstileEnabled:   req.TurnstileEnabled,
		TurnstileSiteKey:   req.TurnstileSiteKey,
		TurnstileSecretKey: req.TurnstileSecretKey,
		APIKeyACLTrustForwardedIP: func() bool {
			if req.APIKeyACLTrustForwardedIP != nil {
				return *req.APIKeyACLTrustForwardedIP
			}
			return previousSettings.APIKeyACLTrustForwardedIP
		}(),
		LinuxDoConnectEnabled:            req.LinuxDoConnectEnabled,
		LinuxDoConnectClientID:           req.LinuxDoConnectClientID,
		LinuxDoConnectClientSecret:       req.LinuxDoConnectClientSecret,
		LinuxDoConnectRedirectURL:        req.LinuxDoConnectRedirectURL,
		WeChatConnectEnabled:             req.WeChatConnectEnabled,
		WeChatConnectAppID:               req.WeChatConnectAppID,
		WeChatConnectAppSecret:           req.WeChatConnectAppSecret,
		WeChatConnectOpenAppID:           req.WeChatConnectOpenAppID,
		WeChatConnectOpenAppSecret:       req.WeChatConnectOpenAppSecret,
		WeChatConnectMPAppID:             req.WeChatConnectMPAppID,
		WeChatConnectMPAppSecret:         req.WeChatConnectMPAppSecret,
		WeChatConnectMobileAppID:         req.WeChatConnectMobileAppID,
		WeChatConnectMobileAppSecret:     req.WeChatConnectMobileAppSecret,
		WeChatConnectOpenEnabled:         req.WeChatConnectOpenEnabled,
		WeChatConnectMPEnabled:           req.WeChatConnectMPEnabled,
		WeChatConnectMobileEnabled:       req.WeChatConnectMobileEnabled,
		WeChatConnectMode:                req.WeChatConnectMode,
		WeChatConnectScopes:              req.WeChatConnectScopes,
		WeChatConnectRedirectURL:         req.WeChatConnectRedirectURL,
		WeChatConnectFrontendRedirectURL: req.WeChatConnectFrontendRedirectURL,
		OIDCConnectEnabled:               req.OIDCConnectEnabled,
		OIDCConnectProviderName:          req.OIDCConnectProviderName,
		OIDCConnectClientID:              req.OIDCConnectClientID,
		OIDCConnectClientSecret:          req.OIDCConnectClientSecret,
		OIDCConnectIssuerURL:             req.OIDCConnectIssuerURL,
		OIDCConnectDiscoveryURL:          req.OIDCConnectDiscoveryURL,
		OIDCConnectAuthorizeURL:          req.OIDCConnectAuthorizeURL,
		OIDCConnectTokenURL:              req.OIDCConnectTokenURL,
		OIDCConnectUserInfoURL:           req.OIDCConnectUserInfoURL,
		OIDCConnectJWKSURL:               req.OIDCConnectJWKSURL,
		OIDCConnectScopes:                req.OIDCConnectScopes,
		OIDCConnectRedirectURL:           req.OIDCConnectRedirectURL,
		OIDCConnectFrontendRedirectURL:   req.OIDCConnectFrontendRedirectURL,
		OIDCConnectTokenAuthMethod:       req.OIDCConnectTokenAuthMethod,
		OIDCConnectUsePKCE:               oidcUsePKCE,
		OIDCConnectValidateIDToken:       oidcValidateIDToken,
		OIDCConnectAllowedSigningAlgs:    req.OIDCConnectAllowedSigningAlgs,
		OIDCConnectClockSkewSeconds:      req.OIDCConnectClockSkewSeconds,
		OIDCConnectRequireEmailVerified:  req.OIDCConnectRequireEmailVerified,
		OIDCConnectUserInfoEmailPath:     req.OIDCConnectUserInfoEmailPath,
		OIDCConnectUserInfoIDPath:        req.OIDCConnectUserInfoIDPath,
		OIDCConnectUserInfoUsernamePath:  req.OIDCConnectUserInfoUsernamePath,
		GitHubOAuthEnabled: func() bool {
			if req.GitHubOAuthEnabled != nil {
				return *req.GitHubOAuthEnabled
			}
			return previousSettings.GitHubOAuthEnabled
		}(),
		GitHubOAuthClientID: func() string {
			if fieldProvided(providedFields, "github_oauth_client_id") {
				return req.GitHubOAuthClientID
			}
			return previousSettings.GitHubOAuthClientID
		}(),
		GitHubOAuthClientSecret: req.GitHubOAuthClientSecret,
		GitHubOAuthRedirectURL: func() string {
			if fieldProvided(providedFields, "github_oauth_redirect_url") {
				return req.GitHubOAuthRedirectURL
			}
			return previousSettings.GitHubOAuthRedirectURL
		}(),
		GitHubOAuthFrontendRedirectURL: func() string {
			if fieldProvided(providedFields, "github_oauth_frontend_redirect_url") {
				return req.GitHubOAuthFrontendRedirectURL
			}
			return previousSettings.GitHubOAuthFrontendRedirectURL
		}(),
		GoogleOAuthEnabled: func() bool {
			if req.GoogleOAuthEnabled != nil {
				return *req.GoogleOAuthEnabled
			}
			return previousSettings.GoogleOAuthEnabled
		}(),
		GoogleOAuthClientID: func() string {
			if fieldProvided(providedFields, "google_oauth_client_id") {
				return req.GoogleOAuthClientID
			}
			return previousSettings.GoogleOAuthClientID
		}(),
		GoogleOAuthClientSecret: req.GoogleOAuthClientSecret,
		GoogleOAuthRedirectURL: func() string {
			if fieldProvided(providedFields, "google_oauth_redirect_url") {
				return req.GoogleOAuthRedirectURL
			}
			return previousSettings.GoogleOAuthRedirectURL
		}(),
		GoogleOAuthFrontendRedirectURL: func() string {
			if fieldProvided(providedFields, "google_oauth_frontend_redirect_url") {
				return req.GoogleOAuthFrontendRedirectURL
			}
			return previousSettings.GoogleOAuthFrontendRedirectURL
		}(),
		SiteName:                    req.SiteName,
		SiteLogo:                    req.SiteLogo,
		SiteSubtitle:                req.SiteSubtitle,
		APIBaseURL:                  req.APIBaseURL,
		ContactInfo:                 req.ContactInfo,
		DocURL:                      req.DocURL,
		HomeContent:                 req.HomeContent,
		HomeStatsGroupID:            req.HomeStatsGroupID,
		HideCcsImportButton:         req.HideCcsImportButton,
		PurchaseSubscriptionEnabled: purchaseEnabled,
		PurchaseSubscriptionURL:     purchaseURL,
		TableDefaultPageSize:        req.TableDefaultPageSize,
		TablePageSizeOptions:        req.TablePageSizeOptions,
		CustomMenuItems:             customMenuJSON,
		CustomEndpoints:             customEndpointsJSON,
		DefaultConcurrency:          req.DefaultConcurrency,
		DefaultBalance:              req.DefaultBalance,
		RiskControlEnabled: func() bool {
			if req.RiskControlEnabled != nil {
				return *req.RiskControlEnabled
			}
			return previousSettings.RiskControlEnabled
		}(),
		AffiliateRebateRate:             affiliateRebateRate,
		AffiliateRebateFreezeHours:      affiliateRebateFreezeHours,
		AffiliateRebateDurationDays:     affiliateRebateDurationDays,
		AffiliateRebatePerInviteeCap:    affiliateRebatePerInviteeCap,
		DefaultUserRPMLimit:             req.DefaultUserRPMLimit,
		UserPrivateGroupDailyLimitUSD:   positiveFloat64Ptr(req.UserPrivateGroupDailyLimitUSD),
		UserPrivateGroupWeeklyLimitUSD:  positiveFloat64Ptr(req.UserPrivateGroupWeeklyLimitUSD),
		UserPrivateGroupMonthlyLimitUSD: positiveFloat64Ptr(req.UserPrivateGroupMonthlyLimitUSD),
		UserPrivateGroupRateMultiplier:  req.UserPrivateGroupRateMultiplier,
		UserPrivateGroupRPMLimit:        req.UserPrivateGroupRPMLimit,
		UserPrivateGroupCommissionRate:  req.UserPrivateGroupCommissionRate,
		DefaultSubscriptions:            defaultSubscriptions,
		EnableModelFallback:             req.EnableModelFallback,
		FallbackModelAnthropic:          req.FallbackModelAnthropic,
		FallbackModelOpenAI:             req.FallbackModelOpenAI,
		FallbackModelGemini:             req.FallbackModelGemini,
		FallbackModelAntigravity:        req.FallbackModelAntigravity,
		EnableIdentityPatch:             req.EnableIdentityPatch,
		IdentityPatchPrompt:             req.IdentityPatchPrompt,
		MinClaudeCodeVersion:            req.MinClaudeCodeVersion,
		MaxClaudeCodeVersion:            req.MaxClaudeCodeVersion,
		AllowUngroupedKeyScheduling:     req.AllowUngroupedKeyScheduling,
		BackendModeEnabled:              req.BackendModeEnabled,
		OpsMonitoringEnabled: func() bool {
			if req.OpsMonitoringEnabled != nil {
				return *req.OpsMonitoringEnabled
			}
			return previousSettings.OpsMonitoringEnabled
		}(),
		OpsRealtimeMonitoringEnabled: func() bool {
			if req.OpsRealtimeMonitoringEnabled != nil {
				return *req.OpsRealtimeMonitoringEnabled
			}
			return previousSettings.OpsRealtimeMonitoringEnabled
		}(),
		OpsQueryModeDefault: func() string {
			if req.OpsQueryModeDefault != nil {
				return *req.OpsQueryModeDefault
			}
			return previousSettings.OpsQueryModeDefault
		}(),
		OpsMetricsIntervalSeconds: func() int {
			if req.OpsMetricsIntervalSeconds != nil {
				return *req.OpsMetricsIntervalSeconds
			}
			return previousSettings.OpsMetricsIntervalSeconds
		}(),
		EnableFingerprintUnification: func() bool {
			if req.EnableFingerprintUnification != nil {
				return *req.EnableFingerprintUnification
			}
			return previousSettings.EnableFingerprintUnification
		}(),
		EnableMetadataPassthrough: func() bool {
			if req.EnableMetadataPassthrough != nil {
				return *req.EnableMetadataPassthrough
			}
			return previousSettings.EnableMetadataPassthrough
		}(),
		EnableCCHSigning: func() bool {
			if req.EnableCCHSigning != nil {
				return *req.EnableCCHSigning
			}
			return previousSettings.EnableCCHSigning
		}(),
		EnableAnthropicCacheTTL1hInjection: func() bool {
			if req.EnableAnthropicCacheTTL1hInjection != nil {
				return *req.EnableAnthropicCacheTTL1hInjection
			}
			return previousSettings.EnableAnthropicCacheTTL1hInjection
		}(),
		OpenAIImagesResponsesReasoningEffort: func() string {
			if req.OpenAIImagesResponsesReasoningEffort != nil {
				return *req.OpenAIImagesResponsesReasoningEffort
			}
			return previousSettings.OpenAIImagesResponsesReasoningEffort
		}(),
		PaymentVisibleMethodAlipaySource: func() string {
			if req.PaymentVisibleMethodAlipaySource != nil {
				return strings.TrimSpace(*req.PaymentVisibleMethodAlipaySource)
			}
			return previousSettings.PaymentVisibleMethodAlipaySource
		}(),
		PaymentVisibleMethodWxpaySource: func() string {
			if req.PaymentVisibleMethodWxpaySource != nil {
				return strings.TrimSpace(*req.PaymentVisibleMethodWxpaySource)
			}
			return previousSettings.PaymentVisibleMethodWxpaySource
		}(),
		PaymentVisibleMethodAlipayEnabled: func() bool {
			if req.PaymentVisibleMethodAlipayEnabled != nil {
				return *req.PaymentVisibleMethodAlipayEnabled
			}
			return previousSettings.PaymentVisibleMethodAlipayEnabled
		}(),
		PaymentVisibleMethodWxpayEnabled: func() bool {
			if req.PaymentVisibleMethodWxpayEnabled != nil {
				return *req.PaymentVisibleMethodWxpayEnabled
			}
			return previousSettings.PaymentVisibleMethodWxpayEnabled
		}(),
		OpenAIAdvancedSchedulerEnabled: func() bool {
			if req.OpenAIAdvancedSchedulerEnabled != nil {
				return *req.OpenAIAdvancedSchedulerEnabled
			}
			return previousSettings.OpenAIAdvancedSchedulerEnabled
		}(),
		OpenAIFreeAccountRepairEnabled: func() bool {
			if req.OpenAIFreeAccountRepairEnabled != nil {
				return *req.OpenAIFreeAccountRepairEnabled
			}
			return previousSettings.OpenAIFreeAccountRepairEnabled
		}(),
		OpenAIFreeAccountRepairWeeklyThresholdUSD: func() float64 {
			if req.OpenAIFreeAccountRepairWeeklyThresholdUSD != nil {
				return *req.OpenAIFreeAccountRepairWeeklyThresholdUSD
			}
			return previousSettings.OpenAIFreeAccountRepairWeeklyThresholdUSD
		}(),
		BalanceLowNotifyEnabled: func() bool {
			if req.BalanceLowNotifyEnabled != nil {
				return *req.BalanceLowNotifyEnabled
			}
			return previousSettings.BalanceLowNotifyEnabled
		}(),
		BalanceLowNotifyThreshold: func() float64 {
			if req.BalanceLowNotifyThreshold != nil {
				return *req.BalanceLowNotifyThreshold
			}
			return previousSettings.BalanceLowNotifyThreshold
		}(),
		BalanceLowNotifyRechargeURL: func() string {
			if req.BalanceLowNotifyRechargeURL != nil {
				return *req.BalanceLowNotifyRechargeURL
			}
			return previousSettings.BalanceLowNotifyRechargeURL
		}(),
		AccountQuotaNotifyEnabled: func() bool {
			if req.AccountQuotaNotifyEnabled != nil {
				return *req.AccountQuotaNotifyEnabled
			}
			return previousSettings.AccountQuotaNotifyEnabled
		}(),
		AccountQuotaNotifyEmails: func() []service.NotifyEmailEntry {
			if req.AccountQuotaNotifyEmails != nil {
				return dto.NotifyEmailEntriesToService(*req.AccountQuotaNotifyEmails)
			}
			return previousSettings.AccountQuotaNotifyEmails
		}(),
		ChannelMonitorEnabled: func() bool {
			if req.ChannelMonitorEnabled != nil {
				return *req.ChannelMonitorEnabled
			}
			return previousSettings.ChannelMonitorEnabled
		}(),
		ChannelMonitorDefaultIntervalSeconds: func() int {
			if req.ChannelMonitorDefaultIntervalSeconds != nil {
				return *req.ChannelMonitorDefaultIntervalSeconds
			}
			return previousSettings.ChannelMonitorDefaultIntervalSeconds
		}(),
		AvailableChannelsEnabled: func() bool {
			if req.AvailableChannelsEnabled != nil {
				return *req.AvailableChannelsEnabled
			}
			return previousSettings.AvailableChannelsEnabled
		}(),
		FreeModelsEnabled: func() bool {
			if req.FreeModelsEnabled != nil {
				return *req.FreeModelsEnabled
			}
			return previousSettings.FreeModelsEnabled
		}(),
		AutoModelSettings: func() service.AutoModelSettings {
			if req.AutoModelSettings != nil {
				return autoModelSettingsToService(*req.AutoModelSettings)
			}
			return previousSettings.AutoModelSettings
		}(),
		CarpoolEnabled: func() bool {
			if req.CarpoolEnabled != nil {
				return *req.CarpoolEnabled
			}
			return previousSettings.CarpoolEnabled
		}(),
		CarpoolBaseServiceFeeUSD: func() float64 {
			if req.CarpoolBaseServiceFeeUSD != nil {
				return *req.CarpoolBaseServiceFeeUSD
			}
			return previousSettings.CarpoolBaseServiceFeeUSD
		}(),
		CarpoolSystemProxyFeeUSD: func() float64 {
			if req.CarpoolSystemProxyFeeUSD != nil {
				return *req.CarpoolSystemProxyFeeUSD
			}
			return previousSettings.CarpoolSystemProxyFeeUSD
		}(),
		CarpoolRiskControlFeeUSD: func() float64 {
			if req.CarpoolRiskControlFeeUSD != nil {
				return *req.CarpoolRiskControlFeeUSD
			}
			return previousSettings.CarpoolRiskControlFeeUSD
		}(),
		AffiliateEnabled: func() bool {
			if req.AffiliateEnabled != nil {
				return *req.AffiliateEnabled
			}
			return previousSettings.AffiliateEnabled
		}(),
	}

	authSourceDefaults := &service.AuthSourceDefaultSettings{
		Email: service.ProviderDefaultGrantSettings{
			Balance:          float64ValueOrDefault(req.AuthSourceDefaultEmailBalance, previousAuthSourceDefaults.Email.Balance),
			Concurrency:      intValueOrDefault(req.AuthSourceDefaultEmailConcurrency, previousAuthSourceDefaults.Email.Concurrency),
			Subscriptions:    defaultSubscriptionsValueOrDefault(req.AuthSourceDefaultEmailSubscriptions, previousAuthSourceDefaults.Email.Subscriptions),
			GrantOnSignup:    boolValueOrDefault(req.AuthSourceDefaultEmailGrantOnSignup, previousAuthSourceDefaults.Email.GrantOnSignup),
			GrantOnFirstBind: boolValueOrDefault(req.AuthSourceDefaultEmailGrantOnFirstBind, previousAuthSourceDefaults.Email.GrantOnFirstBind),
		},
		LinuxDo: service.ProviderDefaultGrantSettings{
			Balance:          float64ValueOrDefault(req.AuthSourceDefaultLinuxDoBalance, previousAuthSourceDefaults.LinuxDo.Balance),
			Concurrency:      intValueOrDefault(req.AuthSourceDefaultLinuxDoConcurrency, previousAuthSourceDefaults.LinuxDo.Concurrency),
			Subscriptions:    defaultSubscriptionsValueOrDefault(req.AuthSourceDefaultLinuxDoSubscriptions, previousAuthSourceDefaults.LinuxDo.Subscriptions),
			GrantOnSignup:    boolValueOrDefault(req.AuthSourceDefaultLinuxDoGrantOnSignup, previousAuthSourceDefaults.LinuxDo.GrantOnSignup),
			GrantOnFirstBind: boolValueOrDefault(req.AuthSourceDefaultLinuxDoGrantOnFirstBind, previousAuthSourceDefaults.LinuxDo.GrantOnFirstBind),
		},
		OIDC: service.ProviderDefaultGrantSettings{
			Balance:          float64ValueOrDefault(req.AuthSourceDefaultOIDCBalance, previousAuthSourceDefaults.OIDC.Balance),
			Concurrency:      intValueOrDefault(req.AuthSourceDefaultOIDCConcurrency, previousAuthSourceDefaults.OIDC.Concurrency),
			Subscriptions:    defaultSubscriptionsValueOrDefault(req.AuthSourceDefaultOIDCSubscriptions, previousAuthSourceDefaults.OIDC.Subscriptions),
			GrantOnSignup:    boolValueOrDefault(req.AuthSourceDefaultOIDCGrantOnSignup, previousAuthSourceDefaults.OIDC.GrantOnSignup),
			GrantOnFirstBind: boolValueOrDefault(req.AuthSourceDefaultOIDCGrantOnFirstBind, previousAuthSourceDefaults.OIDC.GrantOnFirstBind),
		},
		WeChat: service.ProviderDefaultGrantSettings{
			Balance:          float64ValueOrDefault(req.AuthSourceDefaultWeChatBalance, previousAuthSourceDefaults.WeChat.Balance),
			Concurrency:      intValueOrDefault(req.AuthSourceDefaultWeChatConcurrency, previousAuthSourceDefaults.WeChat.Concurrency),
			Subscriptions:    defaultSubscriptionsValueOrDefault(req.AuthSourceDefaultWeChatSubscriptions, previousAuthSourceDefaults.WeChat.Subscriptions),
			GrantOnSignup:    boolValueOrDefault(req.AuthSourceDefaultWeChatGrantOnSignup, previousAuthSourceDefaults.WeChat.GrantOnSignup),
			GrantOnFirstBind: boolValueOrDefault(req.AuthSourceDefaultWeChatGrantOnFirstBind, previousAuthSourceDefaults.WeChat.GrantOnFirstBind),
		},
		GitHub: service.ProviderDefaultGrantSettings{
			Balance:          float64ValueOrDefault(req.AuthSourceDefaultGitHubBalance, previousAuthSourceDefaults.GitHub.Balance),
			Concurrency:      intValueOrDefault(req.AuthSourceDefaultGitHubConcurrency, previousAuthSourceDefaults.GitHub.Concurrency),
			Subscriptions:    defaultSubscriptionsValueOrDefault(req.AuthSourceDefaultGitHubSubscriptions, previousAuthSourceDefaults.GitHub.Subscriptions),
			GrantOnSignup:    boolValueOrDefault(req.AuthSourceDefaultGitHubGrantOnSignup, previousAuthSourceDefaults.GitHub.GrantOnSignup),
			GrantOnFirstBind: boolValueOrDefault(req.AuthSourceDefaultGitHubGrantOnFirstBind, previousAuthSourceDefaults.GitHub.GrantOnFirstBind),
		},
		Google: service.ProviderDefaultGrantSettings{
			Balance:          float64ValueOrDefault(req.AuthSourceDefaultGoogleBalance, previousAuthSourceDefaults.Google.Balance),
			Concurrency:      intValueOrDefault(req.AuthSourceDefaultGoogleConcurrency, previousAuthSourceDefaults.Google.Concurrency),
			Subscriptions:    defaultSubscriptionsValueOrDefault(req.AuthSourceDefaultGoogleSubscriptions, previousAuthSourceDefaults.Google.Subscriptions),
			GrantOnSignup:    boolValueOrDefault(req.AuthSourceDefaultGoogleGrantOnSignup, previousAuthSourceDefaults.Google.GrantOnSignup),
			GrantOnFirstBind: boolValueOrDefault(req.AuthSourceDefaultGoogleGrantOnFirstBind, previousAuthSourceDefaults.Google.GrantOnFirstBind),
		},
		ForceEmailOnThirdPartySignup: boolValueOrDefault(req.ForceEmailOnThirdPartySignup, previousAuthSourceDefaults.ForceEmailOnThirdPartySignup),
	}
	if err := h.settingService.UpdateSettingsWithAuthSourceDefaults(c.Request.Context(), settings, authSourceDefaults); err != nil {
		response.ErrorFrom(c, err)
		return
	}

	// Update OpenAI fast policy (stored under dedicated key, only when provided).
	if req.OpenAIFastPolicySettings != nil {
		if err := h.settingService.SetOpenAIFastPolicySettings(c.Request.Context(), openaiFastPolicySettingsFromDTO(req.OpenAIFastPolicySettings)); err != nil {
			response.BadRequest(c, err.Error())
			return
		}
	}

	// Update payment configuration (integrated into system settings).
	// Skip if no payment fields were provided (prevents accidental wipe).
	if h.paymentConfigService != nil && hasPaymentFields(req) {
		paymentReq := service.UpdatePaymentConfigRequest{
			Enabled:                            req.PaymentEnabled,
			MinAmount:                          req.PaymentMinAmount,
			MaxAmount:                          req.PaymentMaxAmount,
			DailyLimit:                         req.PaymentDailyLimit,
			OrderTimeoutMin:                    req.PaymentOrderTimeoutMin,
			MaxPendingOrders:                   req.PaymentMaxPendingOrders,
			EnabledTypes:                       req.PaymentEnabledTypes,
			BalanceDisabled:                    req.PaymentBalanceDisabled,
			BalanceRechargeMultiplier:          req.PaymentBalanceRechargeMultiplier,
			RechargeFeeRate:                    req.PaymentRechargeFeeRate,
			LoadBalanceStrategy:                req.PaymentLoadBalanceStrat,
			ProductNamePrefix:                  req.PaymentProductNamePrefix,
			ProductNameSuffix:                  req.PaymentProductNameSuffix,
			HelpImageURL:                       req.PaymentHelpImageURL,
			HelpText:                           req.PaymentHelpText,
			ReceiptCodeOSSEnabled:              req.PaymentReceiptCodeOSSEnabled,
			ReceiptCodeOSSEndpoint:             req.PaymentReceiptCodeOSSEndpoint,
			ReceiptCodeOSSRegion:               req.PaymentReceiptCodeOSSRegion,
			ReceiptCodeOSSBucket:               req.PaymentReceiptCodeOSSBucket,
			ReceiptCodeOSSAccessKeyID:          req.PaymentReceiptCodeOSSAccessKeyID,
			ReceiptCodeOSSSecretAccessKey:      req.PaymentReceiptCodeOSSSecretAccessKey,
			ReceiptCodeOSSPrefix:               req.PaymentReceiptCodeOSSPrefix,
			ReceiptCodeOSSPublicBaseURL:        req.PaymentReceiptCodeOSSPublicBaseURL,
			ReceiptCodeOSSForcePathStyle:       req.PaymentReceiptCodeOSSForcePathStyle,
			ReceiptCodeOSSMaxSizeBytes:         req.PaymentReceiptCodeOSSMaxSizeBytes,
			ReceiptCodeOSSPresignExpireSeconds: req.PaymentReceiptCodeOSSPresignExpireSeconds,
			CancelRateLimitEnabled:             req.PaymentCancelRateLimitEnabled,
			CancelRateLimitMax:                 req.PaymentCancelRateLimitMax,
			CancelRateLimitWindow:              req.PaymentCancelRateLimitWindow,
			CancelRateLimitUnit:                req.PaymentCancelRateLimitUnit,
			CancelRateLimitMode:                req.PaymentCancelRateLimitMode,
		}
		if err := h.paymentConfigService.UpdatePaymentConfig(c.Request.Context(), paymentReq); err != nil {
			response.ErrorFrom(c, err)
			return
		}
		// Refresh in-memory provider registry so config changes take effect immediately
		if h.paymentService != nil {
			h.paymentService.RefreshProviders(c.Request.Context())
		}
	}

	h.auditSettingsUpdate(c, previousSettings, settings, previousAuthSourceDefaults, authSourceDefaults, req)

	// 重新获取设置返回
	updatedSettings, err := h.settingService.GetAllSettings(c.Request.Context())
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}
	updatedAuthSourceDefaults, err := h.settingService.GetAuthSourceDefaultSettings(c.Request.Context())
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}
	updatedDefaultSubscriptions := make([]dto.DefaultSubscriptionSetting, 0, len(updatedSettings.DefaultSubscriptions))
	for _, sub := range updatedSettings.DefaultSubscriptions {
		updatedDefaultSubscriptions = append(updatedDefaultSubscriptions, dto.DefaultSubscriptionSetting{
			GroupID:      sub.GroupID,
			ValidityDays: sub.ValidityDays,
		})
	}

	// Reload payment config for response
	var updatedPaymentCfg *service.PaymentConfig
	if h.paymentConfigService != nil {
		updatedPaymentCfg, _ = h.paymentConfigService.GetPaymentConfig(c.Request.Context())
	}
	if updatedPaymentCfg == nil {
		updatedPaymentCfg = &service.PaymentConfig{}
	}

	payload := dto.SystemSettings{
		RegistrationEnabled:                       updatedSettings.RegistrationEnabled,
		EmailVerifyEnabled:                        updatedSettings.EmailVerifyEnabled,
		RegistrationEmailSuffixWhitelist:          updatedSettings.RegistrationEmailSuffixWhitelist,
		PromoCodeEnabled:                          updatedSettings.PromoCodeEnabled,
		PasswordResetEnabled:                      updatedSettings.PasswordResetEnabled,
		FrontendURL:                               updatedSettings.FrontendURL,
		InvitationCodeEnabled:                     updatedSettings.InvitationCodeEnabled,
		TotpEnabled:                               updatedSettings.TotpEnabled,
		TotpEncryptionKeyConfigured:               h.settingService.IsTotpEncryptionKeyConfigured(),
		LoginAgreementEnabled:                     updatedSettings.LoginAgreementEnabled,
		LoginAgreementMode:                        updatedSettings.LoginAgreementMode,
		LoginAgreementUpdatedAt:                   updatedSettings.LoginAgreementUpdatedAt,
		LoginAgreementDocuments:                   loginAgreementDocumentsToDTO(updatedSettings.LoginAgreementDocuments),
		SMTPHost:                                  updatedSettings.SMTPHost,
		SMTPPort:                                  updatedSettings.SMTPPort,
		SMTPUsername:                              updatedSettings.SMTPUsername,
		SMTPPasswordConfigured:                    updatedSettings.SMTPPasswordConfigured,
		SMTPFrom:                                  updatedSettings.SMTPFrom,
		SMTPFromName:                              updatedSettings.SMTPFromName,
		SMTPUseTLS:                                updatedSettings.SMTPUseTLS,
		TurnstileEnabled:                          updatedSettings.TurnstileEnabled,
		TurnstileSiteKey:                          updatedSettings.TurnstileSiteKey,
		TurnstileSecretKeyConfigured:              updatedSettings.TurnstileSecretKeyConfigured,
		APIKeyACLTrustForwardedIP:                 updatedSettings.APIKeyACLTrustForwardedIP,
		LinuxDoConnectEnabled:                     updatedSettings.LinuxDoConnectEnabled,
		LinuxDoConnectClientID:                    updatedSettings.LinuxDoConnectClientID,
		LinuxDoConnectClientSecretConfigured:      updatedSettings.LinuxDoConnectClientSecretConfigured,
		LinuxDoConnectRedirectURL:                 updatedSettings.LinuxDoConnectRedirectURL,
		WeChatConnectEnabled:                      updatedSettings.WeChatConnectEnabled,
		WeChatConnectAppID:                        updatedSettings.WeChatConnectAppID,
		WeChatConnectAppSecretConfigured:          updatedSettings.WeChatConnectAppSecretConfigured,
		WeChatConnectOpenAppID:                    updatedSettings.WeChatConnectOpenAppID,
		WeChatConnectOpenAppSecretConfigured:      updatedSettings.WeChatConnectOpenAppSecretConfigured,
		WeChatConnectMPAppID:                      updatedSettings.WeChatConnectMPAppID,
		WeChatConnectMPAppSecretConfigured:        updatedSettings.WeChatConnectMPAppSecretConfigured,
		WeChatConnectMobileAppID:                  updatedSettings.WeChatConnectMobileAppID,
		WeChatConnectMobileAppSecretConfigured:    updatedSettings.WeChatConnectMobileAppSecretConfigured,
		WeChatConnectOpenEnabled:                  updatedSettings.WeChatConnectOpenEnabled,
		WeChatConnectMPEnabled:                    updatedSettings.WeChatConnectMPEnabled,
		WeChatConnectMobileEnabled:                updatedSettings.WeChatConnectMobileEnabled,
		WeChatConnectMode:                         updatedSettings.WeChatConnectMode,
		WeChatConnectScopes:                       updatedSettings.WeChatConnectScopes,
		WeChatConnectRedirectURL:                  updatedSettings.WeChatConnectRedirectURL,
		WeChatConnectFrontendRedirectURL:          updatedSettings.WeChatConnectFrontendRedirectURL,
		OIDCConnectEnabled:                        updatedSettings.OIDCConnectEnabled,
		OIDCConnectProviderName:                   updatedSettings.OIDCConnectProviderName,
		OIDCConnectClientID:                       updatedSettings.OIDCConnectClientID,
		OIDCConnectClientSecretConfigured:         updatedSettings.OIDCConnectClientSecretConfigured,
		OIDCConnectIssuerURL:                      updatedSettings.OIDCConnectIssuerURL,
		OIDCConnectDiscoveryURL:                   updatedSettings.OIDCConnectDiscoveryURL,
		OIDCConnectAuthorizeURL:                   updatedSettings.OIDCConnectAuthorizeURL,
		OIDCConnectTokenURL:                       updatedSettings.OIDCConnectTokenURL,
		OIDCConnectUserInfoURL:                    updatedSettings.OIDCConnectUserInfoURL,
		OIDCConnectJWKSURL:                        updatedSettings.OIDCConnectJWKSURL,
		OIDCConnectScopes:                         updatedSettings.OIDCConnectScopes,
		OIDCConnectRedirectURL:                    updatedSettings.OIDCConnectRedirectURL,
		OIDCConnectFrontendRedirectURL:            updatedSettings.OIDCConnectFrontendRedirectURL,
		OIDCConnectTokenAuthMethod:                updatedSettings.OIDCConnectTokenAuthMethod,
		OIDCConnectUsePKCE:                        updatedSettings.OIDCConnectUsePKCE,
		OIDCConnectValidateIDToken:                updatedSettings.OIDCConnectValidateIDToken,
		OIDCConnectAllowedSigningAlgs:             updatedSettings.OIDCConnectAllowedSigningAlgs,
		OIDCConnectClockSkewSeconds:               updatedSettings.OIDCConnectClockSkewSeconds,
		OIDCConnectRequireEmailVerified:           updatedSettings.OIDCConnectRequireEmailVerified,
		OIDCConnectUserInfoEmailPath:              updatedSettings.OIDCConnectUserInfoEmailPath,
		OIDCConnectUserInfoIDPath:                 updatedSettings.OIDCConnectUserInfoIDPath,
		OIDCConnectUserInfoUsernamePath:           updatedSettings.OIDCConnectUserInfoUsernamePath,
		GitHubOAuthEnabled:                        updatedSettings.GitHubOAuthEnabled,
		GitHubOAuthClientID:                       updatedSettings.GitHubOAuthClientID,
		GitHubOAuthClientSecretConfigured:         updatedSettings.GitHubOAuthClientSecretConfigured,
		GitHubOAuthRedirectURL:                    updatedSettings.GitHubOAuthRedirectURL,
		GitHubOAuthFrontendRedirectURL:            updatedSettings.GitHubOAuthFrontendRedirectURL,
		GoogleOAuthEnabled:                        updatedSettings.GoogleOAuthEnabled,
		GoogleOAuthClientID:                       updatedSettings.GoogleOAuthClientID,
		GoogleOAuthClientSecretConfigured:         updatedSettings.GoogleOAuthClientSecretConfigured,
		GoogleOAuthRedirectURL:                    updatedSettings.GoogleOAuthRedirectURL,
		GoogleOAuthFrontendRedirectURL:            updatedSettings.GoogleOAuthFrontendRedirectURL,
		SiteName:                                  updatedSettings.SiteName,
		SiteLogo:                                  updatedSettings.SiteLogo,
		SiteSubtitle:                              updatedSettings.SiteSubtitle,
		APIBaseURL:                                updatedSettings.APIBaseURL,
		ContactInfo:                               updatedSettings.ContactInfo,
		DocURL:                                    updatedSettings.DocURL,
		HomeContent:                               updatedSettings.HomeContent,
		HomeStatsGroupID:                          updatedSettings.HomeStatsGroupID,
		HideCcsImportButton:                       updatedSettings.HideCcsImportButton,
		PurchaseSubscriptionEnabled:               updatedSettings.PurchaseSubscriptionEnabled,
		PurchaseSubscriptionURL:                   updatedSettings.PurchaseSubscriptionURL,
		TableDefaultPageSize:                      updatedSettings.TableDefaultPageSize,
		TablePageSizeOptions:                      updatedSettings.TablePageSizeOptions,
		CustomMenuItems:                           dto.ParseCustomMenuItems(updatedSettings.CustomMenuItems),
		CustomEndpoints:                           dto.ParseCustomEndpoints(updatedSettings.CustomEndpoints),
		DefaultConcurrency:                        updatedSettings.DefaultConcurrency,
		DefaultBalance:                            updatedSettings.DefaultBalance,
		RiskControlEnabled:                        updatedSettings.RiskControlEnabled,
		AffiliateRebateRate:                       updatedSettings.AffiliateRebateRate,
		AffiliateRebateFreezeHours:                updatedSettings.AffiliateRebateFreezeHours,
		AffiliateRebateDurationDays:               updatedSettings.AffiliateRebateDurationDays,
		AffiliateRebatePerInviteeCap:              updatedSettings.AffiliateRebatePerInviteeCap,
		DefaultUserRPMLimit:                       updatedSettings.DefaultUserRPMLimit,
		UserPrivateGroupDailyLimitUSD:             updatedSettings.UserPrivateGroupDailyLimitUSD,
		UserPrivateGroupWeeklyLimitUSD:            updatedSettings.UserPrivateGroupWeeklyLimitUSD,
		UserPrivateGroupMonthlyLimitUSD:           updatedSettings.UserPrivateGroupMonthlyLimitUSD,
		UserPrivateGroupRateMultiplier:            updatedSettings.UserPrivateGroupRateMultiplier,
		UserPrivateGroupRPMLimit:                  updatedSettings.UserPrivateGroupRPMLimit,
		UserPrivateGroupCommissionRate:            updatedSettings.UserPrivateGroupCommissionRate,
		DefaultSubscriptions:                      updatedDefaultSubscriptions,
		EnableModelFallback:                       updatedSettings.EnableModelFallback,
		FallbackModelAnthropic:                    updatedSettings.FallbackModelAnthropic,
		FallbackModelOpenAI:                       updatedSettings.FallbackModelOpenAI,
		FallbackModelGemini:                       updatedSettings.FallbackModelGemini,
		FallbackModelAntigravity:                  updatedSettings.FallbackModelAntigravity,
		EnableIdentityPatch:                       updatedSettings.EnableIdentityPatch,
		IdentityPatchPrompt:                       updatedSettings.IdentityPatchPrompt,
		OpsMonitoringEnabled:                      updatedSettings.OpsMonitoringEnabled,
		OpsRealtimeMonitoringEnabled:              updatedSettings.OpsRealtimeMonitoringEnabled,
		OpsQueryModeDefault:                       updatedSettings.OpsQueryModeDefault,
		OpsMetricsIntervalSeconds:                 updatedSettings.OpsMetricsIntervalSeconds,
		MinClaudeCodeVersion:                      updatedSettings.MinClaudeCodeVersion,
		MaxClaudeCodeVersion:                      updatedSettings.MaxClaudeCodeVersion,
		AllowUngroupedKeyScheduling:               updatedSettings.AllowUngroupedKeyScheduling,
		BackendModeEnabled:                        updatedSettings.BackendModeEnabled,
		EnableFingerprintUnification:              updatedSettings.EnableFingerprintUnification,
		EnableMetadataPassthrough:                 updatedSettings.EnableMetadataPassthrough,
		EnableCCHSigning:                          updatedSettings.EnableCCHSigning,
		EnableAnthropicCacheTTL1hInjection:        updatedSettings.EnableAnthropicCacheTTL1hInjection,
		OpenAIImagesResponsesReasoningEffort:      updatedSettings.OpenAIImagesResponsesReasoningEffort,
		PaymentVisibleMethodAlipaySource:          updatedSettings.PaymentVisibleMethodAlipaySource,
		PaymentVisibleMethodWxpaySource:           updatedSettings.PaymentVisibleMethodWxpaySource,
		PaymentVisibleMethodAlipayEnabled:         updatedSettings.PaymentVisibleMethodAlipayEnabled,
		PaymentVisibleMethodWxpayEnabled:          updatedSettings.PaymentVisibleMethodWxpayEnabled,
		OpenAIAdvancedSchedulerEnabled:            updatedSettings.OpenAIAdvancedSchedulerEnabled,
		OpenAIFreeAccountRepairEnabled:            updatedSettings.OpenAIFreeAccountRepairEnabled,
		OpenAIFreeAccountRepairWeeklyThresholdUSD: updatedSettings.OpenAIFreeAccountRepairWeeklyThresholdUSD,
		BalanceLowNotifyEnabled:                   updatedSettings.BalanceLowNotifyEnabled,
		BalanceLowNotifyThreshold:                 updatedSettings.BalanceLowNotifyThreshold,
		BalanceLowNotifyRechargeURL:               updatedSettings.BalanceLowNotifyRechargeURL,
		AccountQuotaNotifyEnabled:                 updatedSettings.AccountQuotaNotifyEnabled,
		AccountQuotaNotifyEmails:                  dto.NotifyEmailEntriesFromService(updatedSettings.AccountQuotaNotifyEmails),
		PaymentEnabled:                            updatedPaymentCfg.Enabled,
		PaymentMinAmount:                          updatedPaymentCfg.MinAmount,
		PaymentMaxAmount:                          updatedPaymentCfg.MaxAmount,
		PaymentDailyLimit:                         updatedPaymentCfg.DailyLimit,
		PaymentOrderTimeoutMin:                    updatedPaymentCfg.OrderTimeoutMin,
		PaymentMaxPendingOrders:                   updatedPaymentCfg.MaxPendingOrders,
		PaymentEnabledTypes:                       updatedPaymentCfg.EnabledTypes,
		PaymentBalanceDisabled:                    updatedPaymentCfg.BalanceDisabled,
		PaymentBalanceRechargeMultiplier:          updatedPaymentCfg.BalanceRechargeMultiplier,
		PaymentRechargeFeeRate:                    updatedPaymentCfg.RechargeFeeRate,
		PaymentLoadBalanceStrat:                   updatedPaymentCfg.LoadBalanceStrategy,
		PaymentProductNamePrefix:                  updatedPaymentCfg.ProductNamePrefix,
		PaymentProductNameSuffix:                  updatedPaymentCfg.ProductNameSuffix,
		PaymentHelpImageURL:                       updatedPaymentCfg.HelpImageURL,
		PaymentHelpText:                           updatedPaymentCfg.HelpText,
		PaymentReceiptCodeOSSEnabled:              updatedPaymentCfg.ReceiptCodeOSS.Enabled,
		PaymentReceiptCodeOSSEndpoint:             updatedPaymentCfg.ReceiptCodeOSS.Endpoint,
		PaymentReceiptCodeOSSRegion:               updatedPaymentCfg.ReceiptCodeOSS.Region,
		PaymentReceiptCodeOSSBucket:               updatedPaymentCfg.ReceiptCodeOSS.Bucket,
		PaymentReceiptCodeOSSAccessKeyID:          updatedPaymentCfg.ReceiptCodeOSS.AccessKeyID,
		PaymentReceiptCodeOSSSecretConfigured:     updatedPaymentCfg.ReceiptCodeOSS.SecretAccessKeyConfigured,
		PaymentReceiptCodeOSSPrefix:               updatedPaymentCfg.ReceiptCodeOSS.Prefix,
		PaymentReceiptCodeOSSPublicBaseURL:        updatedPaymentCfg.ReceiptCodeOSS.PublicBaseURL,
		PaymentReceiptCodeOSSForcePathStyle:       updatedPaymentCfg.ReceiptCodeOSS.ForcePathStyle,
		PaymentReceiptCodeOSSMaxSizeBytes:         updatedPaymentCfg.ReceiptCodeOSS.MaxSizeBytes,
		PaymentReceiptCodeOSSPresignExpireSeconds: updatedPaymentCfg.ReceiptCodeOSS.PresignExpireSeconds,
		PaymentCancelRateLimitEnabled:             updatedPaymentCfg.CancelRateLimitEnabled,
		PaymentCancelRateLimitMax:                 updatedPaymentCfg.CancelRateLimitMax,
		PaymentCancelRateLimitWindow:              updatedPaymentCfg.CancelRateLimitWindow,
		PaymentCancelRateLimitUnit:                updatedPaymentCfg.CancelRateLimitUnit,
		PaymentCancelRateLimitMode:                updatedPaymentCfg.CancelRateLimitMode,

		ChannelMonitorEnabled:                updatedSettings.ChannelMonitorEnabled,
		ChannelMonitorDefaultIntervalSeconds: updatedSettings.ChannelMonitorDefaultIntervalSeconds,

		AvailableChannelsEnabled: updatedSettings.AvailableChannelsEnabled,
		FreeModelsEnabled:        updatedSettings.FreeModelsEnabled,
		AutoModelSettings:        autoModelSettingsToDTO(updatedSettings.AutoModelSettings),

		CarpoolEnabled:           updatedSettings.CarpoolEnabled,
		CarpoolBaseServiceFeeUSD: updatedSettings.CarpoolBaseServiceFeeUSD,
		CarpoolSystemProxyFeeUSD: updatedSettings.CarpoolSystemProxyFeeUSD,
		CarpoolRiskControlFeeUSD: updatedSettings.CarpoolRiskControlFeeUSD,

		AffiliateEnabled: updatedSettings.AffiliateEnabled,
	}
	if fastPolicy, err := h.settingService.GetOpenAIFastPolicySettings(c.Request.Context()); err != nil {
		slog.Error("openai_fast_policy_settings_get_failed", "error", err)
	} else if fastPolicy != nil {
		payload.OpenAIFastPolicySettings = openaiFastPolicySettingsToDTO(fastPolicy)
	}
	response.Success(c, systemSettingsResponseData(payload, updatedAuthSourceDefaults))
}

// hasPaymentFields returns true if any payment-related field was explicitly provided.
func hasPaymentFields(req UpdateSettingsRequest) bool {
	return req.PaymentEnabled != nil || req.PaymentMinAmount != nil ||
		req.PaymentMaxAmount != nil || req.PaymentDailyLimit != nil ||
		req.PaymentOrderTimeoutMin != nil || req.PaymentMaxPendingOrders != nil ||
		req.PaymentEnabledTypes != nil || req.PaymentBalanceDisabled != nil ||
		req.PaymentBalanceRechargeMultiplier != nil || req.PaymentRechargeFeeRate != nil ||
		req.PaymentLoadBalanceStrat != nil || req.PaymentProductNamePrefix != nil ||
		req.PaymentProductNameSuffix != nil || req.PaymentHelpImageURL != nil ||
		req.PaymentHelpText != nil || req.PaymentReceiptCodeOSSEnabled != nil ||
		req.PaymentReceiptCodeOSSEndpoint != nil || req.PaymentReceiptCodeOSSRegion != nil ||
		req.PaymentReceiptCodeOSSBucket != nil || req.PaymentReceiptCodeOSSAccessKeyID != nil ||
		req.PaymentReceiptCodeOSSSecretAccessKey != nil || req.PaymentReceiptCodeOSSPrefix != nil ||
		req.PaymentReceiptCodeOSSPublicBaseURL != nil || req.PaymentReceiptCodeOSSForcePathStyle != nil ||
		req.PaymentReceiptCodeOSSMaxSizeBytes != nil || req.PaymentReceiptCodeOSSPresignExpireSeconds != nil ||
		req.PaymentCancelRateLimitEnabled != nil ||
		req.PaymentCancelRateLimitMax != nil || req.PaymentCancelRateLimitWindow != nil ||
		req.PaymentCancelRateLimitUnit != nil || req.PaymentCancelRateLimitMode != nil
}

func decodeProvidedJSONFields(rawBody []byte) (map[string]struct{}, error) {
	var fields map[string]json.RawMessage
	if err := json.Unmarshal(rawBody, &fields); err != nil {
		return nil, err
	}
	out := make(map[string]struct{}, len(fields))
	for key := range fields {
		out[key] = struct{}{}
	}
	return out, nil
}

func fieldProvided(fields map[string]struct{}, key string) bool {
	_, ok := fields[key]
	return ok
}

func dtoDefaultSubscriptionsFromService(input []service.DefaultSubscriptionSetting) []dto.DefaultSubscriptionSetting {
	if input == nil {
		return nil
	}
	out := make([]dto.DefaultSubscriptionSetting, 0, len(input))
	for _, item := range input {
		out = append(out, dto.DefaultSubscriptionSetting{
			GroupID:      item.GroupID,
			ValidityDays: item.ValidityDays,
		})
	}
	return out
}

func optionalFloat64Value(input *float64) float64 {
	if input == nil {
		return 0
	}
	return *input
}

func preserveOmittedUpdateSettingsFields(req *UpdateSettingsRequest, previous *service.SystemSettings, fields map[string]struct{}) {
	if req == nil || previous == nil {
		return
	}

	if !fieldProvided(fields, "registration_enabled") {
		req.RegistrationEnabled = previous.RegistrationEnabled
	}
	if !fieldProvided(fields, "email_verify_enabled") {
		req.EmailVerifyEnabled = previous.EmailVerifyEnabled
	}
	if !fieldProvided(fields, "registration_email_suffix_whitelist") {
		req.RegistrationEmailSuffixWhitelist = previous.RegistrationEmailSuffixWhitelist
	}
	if !fieldProvided(fields, "promo_code_enabled") {
		req.PromoCodeEnabled = previous.PromoCodeEnabled
	}
	if !fieldProvided(fields, "password_reset_enabled") {
		req.PasswordResetEnabled = previous.PasswordResetEnabled
	}
	if !fieldProvided(fields, "frontend_url") {
		req.FrontendURL = previous.FrontendURL
	}
	if !fieldProvided(fields, "invitation_code_enabled") {
		req.InvitationCodeEnabled = previous.InvitationCodeEnabled
	}
	if !fieldProvided(fields, "totp_enabled") {
		req.TotpEnabled = previous.TotpEnabled
	}
	if !fieldProvided(fields, "login_agreement_enabled") {
		req.LoginAgreementEnabled = &previous.LoginAgreementEnabled
	}
	if !fieldProvided(fields, "login_agreement_mode") {
		req.LoginAgreementMode = previous.LoginAgreementMode
	}
	if !fieldProvided(fields, "login_agreement_updated_at") {
		req.LoginAgreementUpdatedAt = previous.LoginAgreementUpdatedAt
	}
	if !fieldProvided(fields, "login_agreement_documents") {
		docs := loginAgreementDocumentsToDTO(previous.LoginAgreementDocuments)
		req.LoginAgreementDocuments = docs
	}
	if !fieldProvided(fields, "smtp_host") {
		req.SMTPHost = previous.SMTPHost
	}
	if !fieldProvided(fields, "smtp_port") {
		req.SMTPPort = previous.SMTPPort
	}
	if !fieldProvided(fields, "smtp_username") {
		req.SMTPUsername = previous.SMTPUsername
	}
	if !fieldProvided(fields, "smtp_from_email") {
		req.SMTPFrom = previous.SMTPFrom
	}
	if !fieldProvided(fields, "smtp_from_name") {
		req.SMTPFromName = previous.SMTPFromName
	}
	if !fieldProvided(fields, "smtp_use_tls") {
		req.SMTPUseTLS = previous.SMTPUseTLS
	}
	if !fieldProvided(fields, "turnstile_enabled") {
		req.TurnstileEnabled = previous.TurnstileEnabled
	}
	if !fieldProvided(fields, "turnstile_site_key") {
		req.TurnstileSiteKey = previous.TurnstileSiteKey
	}
	if !fieldProvided(fields, "api_key_acl_trust_forwarded_ip") {
		req.APIKeyACLTrustForwardedIP = &previous.APIKeyACLTrustForwardedIP
	}
	if !fieldProvided(fields, "linuxdo_connect_enabled") {
		req.LinuxDoConnectEnabled = previous.LinuxDoConnectEnabled
	}
	if !fieldProvided(fields, "linuxdo_connect_client_id") {
		req.LinuxDoConnectClientID = previous.LinuxDoConnectClientID
	}
	if !fieldProvided(fields, "linuxdo_connect_redirect_url") {
		req.LinuxDoConnectRedirectURL = previous.LinuxDoConnectRedirectURL
	}
	if !fieldProvided(fields, "wechat_connect_enabled") {
		req.WeChatConnectEnabled = previous.WeChatConnectEnabled
	}
	if !fieldProvided(fields, "wechat_connect_app_id") {
		req.WeChatConnectAppID = previous.WeChatConnectAppID
	}
	if !fieldProvided(fields, "wechat_connect_open_app_id") {
		req.WeChatConnectOpenAppID = previous.WeChatConnectOpenAppID
	}
	if !fieldProvided(fields, "wechat_connect_mp_app_id") {
		req.WeChatConnectMPAppID = previous.WeChatConnectMPAppID
	}
	if !fieldProvided(fields, "wechat_connect_mobile_app_id") {
		req.WeChatConnectMobileAppID = previous.WeChatConnectMobileAppID
	}
	if !fieldProvided(fields, "wechat_connect_open_enabled") {
		req.WeChatConnectOpenEnabled = previous.WeChatConnectOpenEnabled
	}
	if !fieldProvided(fields, "wechat_connect_mp_enabled") {
		req.WeChatConnectMPEnabled = previous.WeChatConnectMPEnabled
	}
	if !fieldProvided(fields, "wechat_connect_mobile_enabled") {
		req.WeChatConnectMobileEnabled = previous.WeChatConnectMobileEnabled
	}
	if !fieldProvided(fields, "wechat_connect_mode") {
		req.WeChatConnectMode = previous.WeChatConnectMode
	}
	if !fieldProvided(fields, "wechat_connect_scopes") {
		req.WeChatConnectScopes = previous.WeChatConnectScopes
	}
	if !fieldProvided(fields, "wechat_connect_redirect_url") {
		req.WeChatConnectRedirectURL = previous.WeChatConnectRedirectURL
	}
	if !fieldProvided(fields, "wechat_connect_frontend_redirect_url") {
		req.WeChatConnectFrontendRedirectURL = previous.WeChatConnectFrontendRedirectURL
	}
	if !fieldProvided(fields, "oidc_connect_enabled") {
		req.OIDCConnectEnabled = previous.OIDCConnectEnabled
	}
	if !fieldProvided(fields, "oidc_connect_provider_name") {
		req.OIDCConnectProviderName = previous.OIDCConnectProviderName
	}
	if !fieldProvided(fields, "oidc_connect_client_id") {
		req.OIDCConnectClientID = previous.OIDCConnectClientID
	}
	if !fieldProvided(fields, "oidc_connect_issuer_url") {
		req.OIDCConnectIssuerURL = previous.OIDCConnectIssuerURL
	}
	if !fieldProvided(fields, "oidc_connect_discovery_url") {
		req.OIDCConnectDiscoveryURL = previous.OIDCConnectDiscoveryURL
	}
	if !fieldProvided(fields, "oidc_connect_authorize_url") {
		req.OIDCConnectAuthorizeURL = previous.OIDCConnectAuthorizeURL
	}
	if !fieldProvided(fields, "oidc_connect_token_url") {
		req.OIDCConnectTokenURL = previous.OIDCConnectTokenURL
	}
	if !fieldProvided(fields, "oidc_connect_userinfo_url") {
		req.OIDCConnectUserInfoURL = previous.OIDCConnectUserInfoURL
	}
	if !fieldProvided(fields, "oidc_connect_jwks_url") {
		req.OIDCConnectJWKSURL = previous.OIDCConnectJWKSURL
	}
	if !fieldProvided(fields, "oidc_connect_scopes") {
		req.OIDCConnectScopes = previous.OIDCConnectScopes
	}
	if !fieldProvided(fields, "oidc_connect_redirect_url") {
		req.OIDCConnectRedirectURL = previous.OIDCConnectRedirectURL
	}
	if !fieldProvided(fields, "oidc_connect_frontend_redirect_url") {
		req.OIDCConnectFrontendRedirectURL = previous.OIDCConnectFrontendRedirectURL
	}
	if !fieldProvided(fields, "oidc_connect_token_auth_method") {
		req.OIDCConnectTokenAuthMethod = previous.OIDCConnectTokenAuthMethod
	}
	if !fieldProvided(fields, "oidc_connect_allowed_signing_algs") {
		req.OIDCConnectAllowedSigningAlgs = previous.OIDCConnectAllowedSigningAlgs
	}
	if !fieldProvided(fields, "oidc_connect_clock_skew_seconds") {
		req.OIDCConnectClockSkewSeconds = previous.OIDCConnectClockSkewSeconds
	}
	if !fieldProvided(fields, "oidc_connect_require_email_verified") {
		req.OIDCConnectRequireEmailVerified = previous.OIDCConnectRequireEmailVerified
	}
	if !fieldProvided(fields, "oidc_connect_userinfo_email_path") {
		req.OIDCConnectUserInfoEmailPath = previous.OIDCConnectUserInfoEmailPath
	}
	if !fieldProvided(fields, "oidc_connect_userinfo_id_path") {
		req.OIDCConnectUserInfoIDPath = previous.OIDCConnectUserInfoIDPath
	}
	if !fieldProvided(fields, "oidc_connect_userinfo_username_path") {
		req.OIDCConnectUserInfoUsernamePath = previous.OIDCConnectUserInfoUsernamePath
	}
	if !fieldProvided(fields, "github_oauth_enabled") {
		req.GitHubOAuthEnabled = &previous.GitHubOAuthEnabled
	}
	if !fieldProvided(fields, "github_oauth_client_id") {
		req.GitHubOAuthClientID = previous.GitHubOAuthClientID
	}
	if !fieldProvided(fields, "github_oauth_redirect_url") {
		req.GitHubOAuthRedirectURL = previous.GitHubOAuthRedirectURL
	}
	if !fieldProvided(fields, "github_oauth_frontend_redirect_url") {
		req.GitHubOAuthFrontendRedirectURL = previous.GitHubOAuthFrontendRedirectURL
	}
	if !fieldProvided(fields, "google_oauth_enabled") {
		req.GoogleOAuthEnabled = &previous.GoogleOAuthEnabled
	}
	if !fieldProvided(fields, "google_oauth_client_id") {
		req.GoogleOAuthClientID = previous.GoogleOAuthClientID
	}
	if !fieldProvided(fields, "google_oauth_redirect_url") {
		req.GoogleOAuthRedirectURL = previous.GoogleOAuthRedirectURL
	}
	if !fieldProvided(fields, "google_oauth_frontend_redirect_url") {
		req.GoogleOAuthFrontendRedirectURL = previous.GoogleOAuthFrontendRedirectURL
	}
	if !fieldProvided(fields, "site_name") {
		req.SiteName = previous.SiteName
	}
	if !fieldProvided(fields, "site_logo") {
		req.SiteLogo = previous.SiteLogo
	}
	if !fieldProvided(fields, "site_subtitle") {
		req.SiteSubtitle = previous.SiteSubtitle
	}
	if !fieldProvided(fields, "api_base_url") {
		req.APIBaseURL = previous.APIBaseURL
	}
	if !fieldProvided(fields, "contact_info") {
		req.ContactInfo = previous.ContactInfo
	}
	if !fieldProvided(fields, "doc_url") {
		req.DocURL = previous.DocURL
	}
	if !fieldProvided(fields, "home_content") {
		req.HomeContent = previous.HomeContent
	}
	if !fieldProvided(fields, "home_stats_group_id") {
		req.HomeStatsGroupID = previous.HomeStatsGroupID
	}
	if !fieldProvided(fields, "hide_ccs_import_button") {
		req.HideCcsImportButton = previous.HideCcsImportButton
	}
	if !fieldProvided(fields, "table_default_page_size") {
		req.TableDefaultPageSize = previous.TableDefaultPageSize
	}
	if !fieldProvided(fields, "table_page_size_options") {
		req.TablePageSizeOptions = previous.TablePageSizeOptions
	}
	if !fieldProvided(fields, "default_concurrency") {
		req.DefaultConcurrency = previous.DefaultConcurrency
	}
	if !fieldProvided(fields, "default_balance") {
		req.DefaultBalance = previous.DefaultBalance
	}
	if !fieldProvided(fields, "risk_control_enabled") {
		req.RiskControlEnabled = &previous.RiskControlEnabled
	}
	if !fieldProvided(fields, "carpool_enabled") {
		req.CarpoolEnabled = &previous.CarpoolEnabled
	}
	if !fieldProvided(fields, "free_models_enabled") {
		req.FreeModelsEnabled = &previous.FreeModelsEnabled
	}
	if !fieldProvided(fields, "carpool_base_service_fee_usd") {
		req.CarpoolBaseServiceFeeUSD = &previous.CarpoolBaseServiceFeeUSD
	}
	if !fieldProvided(fields, "carpool_system_proxy_fee_usd") {
		req.CarpoolSystemProxyFeeUSD = &previous.CarpoolSystemProxyFeeUSD
	}
	if !fieldProvided(fields, "carpool_risk_control_fee_usd") {
		req.CarpoolRiskControlFeeUSD = &previous.CarpoolRiskControlFeeUSD
	}
	if !fieldProvided(fields, "default_user_rpm_limit") {
		req.DefaultUserRPMLimit = previous.DefaultUserRPMLimit
	}
	if !fieldProvided(fields, "user_private_group_daily_limit_usd") {
		req.UserPrivateGroupDailyLimitUSD = optionalFloat64Value(previous.UserPrivateGroupDailyLimitUSD)
	}
	if !fieldProvided(fields, "user_private_group_weekly_limit_usd") {
		req.UserPrivateGroupWeeklyLimitUSD = optionalFloat64Value(previous.UserPrivateGroupWeeklyLimitUSD)
	}
	if !fieldProvided(fields, "user_private_group_monthly_limit_usd") {
		req.UserPrivateGroupMonthlyLimitUSD = optionalFloat64Value(previous.UserPrivateGroupMonthlyLimitUSD)
	}
	if !fieldProvided(fields, "user_private_group_rate_multiplier") {
		req.UserPrivateGroupRateMultiplier = previous.UserPrivateGroupRateMultiplier
	}
	if !fieldProvided(fields, "user_private_group_rpm_limit") {
		req.UserPrivateGroupRPMLimit = previous.UserPrivateGroupRPMLimit
	}
	if !fieldProvided(fields, "user_private_group_commission_rate") {
		req.UserPrivateGroupCommissionRate = previous.UserPrivateGroupCommissionRate
	}
	if !fieldProvided(fields, "default_subscriptions") {
		req.DefaultSubscriptions = dtoDefaultSubscriptionsFromService(previous.DefaultSubscriptions)
	}
	if !fieldProvided(fields, "enable_model_fallback") {
		req.EnableModelFallback = previous.EnableModelFallback
	}
	if !fieldProvided(fields, "fallback_model_anthropic") {
		req.FallbackModelAnthropic = previous.FallbackModelAnthropic
	}
	if !fieldProvided(fields, "fallback_model_openai") {
		req.FallbackModelOpenAI = previous.FallbackModelOpenAI
	}
	if !fieldProvided(fields, "fallback_model_gemini") {
		req.FallbackModelGemini = previous.FallbackModelGemini
	}
	if !fieldProvided(fields, "fallback_model_antigravity") {
		req.FallbackModelAntigravity = previous.FallbackModelAntigravity
	}
	if !fieldProvided(fields, "enable_identity_patch") {
		req.EnableIdentityPatch = previous.EnableIdentityPatch
	}
	if !fieldProvided(fields, "identity_patch_prompt") {
		req.IdentityPatchPrompt = previous.IdentityPatchPrompt
	}
	if !fieldProvided(fields, "min_claude_code_version") {
		req.MinClaudeCodeVersion = previous.MinClaudeCodeVersion
	}
	if !fieldProvided(fields, "max_claude_code_version") {
		req.MaxClaudeCodeVersion = previous.MaxClaudeCodeVersion
	}
	if !fieldProvided(fields, "allow_ungrouped_key_scheduling") {
		req.AllowUngroupedKeyScheduling = previous.AllowUngroupedKeyScheduling
	}
	if !fieldProvided(fields, "backend_mode_enabled") {
		req.BackendModeEnabled = previous.BackendModeEnabled
	}
}

func (h *SettingHandler) auditSettingsUpdate(c *gin.Context, before *service.SystemSettings, after *service.SystemSettings, beforeAuthSourceDefaults *service.AuthSourceDefaultSettings, afterAuthSourceDefaults *service.AuthSourceDefaultSettings, req UpdateSettingsRequest) {
	if before == nil || after == nil {
		return
	}

	changed := diffSettings(before, after, beforeAuthSourceDefaults, afterAuthSourceDefaults, req)
	if len(changed) == 0 {
		return
	}

	subject, _ := middleware.GetAuthSubjectFromContext(c)
	role, _ := middleware.GetUserRoleFromContext(c)
	slog.Info("settings updated",
		"audit", true,
		"user_id", subject.UserID,
		"role", role,
		"changed", changed,
	)
}

func diffSettings(before *service.SystemSettings, after *service.SystemSettings, beforeAuthSourceDefaults *service.AuthSourceDefaultSettings, afterAuthSourceDefaults *service.AuthSourceDefaultSettings, req UpdateSettingsRequest) []string {
	changed := make([]string, 0, 20)
	if before.RegistrationEnabled != after.RegistrationEnabled {
		changed = append(changed, "registration_enabled")
	}
	if before.EmailVerifyEnabled != after.EmailVerifyEnabled {
		changed = append(changed, "email_verify_enabled")
	}
	if !equalStringSlice(before.RegistrationEmailSuffixWhitelist, after.RegistrationEmailSuffixWhitelist) {
		changed = append(changed, "registration_email_suffix_whitelist")
	}
	if before.PromoCodeEnabled != after.PromoCodeEnabled {
		changed = append(changed, "promo_code_enabled")
	}
	if before.InvitationCodeEnabled != after.InvitationCodeEnabled {
		changed = append(changed, "invitation_code_enabled")
	}
	if before.PasswordResetEnabled != after.PasswordResetEnabled {
		changed = append(changed, "password_reset_enabled")
	}
	if before.FrontendURL != after.FrontendURL {
		changed = append(changed, "frontend_url")
	}
	if before.TotpEnabled != after.TotpEnabled {
		changed = append(changed, "totp_enabled")
	}
	if before.SMTPHost != after.SMTPHost {
		changed = append(changed, "smtp_host")
	}
	if before.SMTPPort != after.SMTPPort {
		changed = append(changed, "smtp_port")
	}
	if before.SMTPUsername != after.SMTPUsername {
		changed = append(changed, "smtp_username")
	}
	if req.SMTPPassword != "" {
		changed = append(changed, "smtp_password")
	}
	if before.SMTPFrom != after.SMTPFrom {
		changed = append(changed, "smtp_from_email")
	}
	if before.SMTPFromName != after.SMTPFromName {
		changed = append(changed, "smtp_from_name")
	}
	if before.SMTPUseTLS != after.SMTPUseTLS {
		changed = append(changed, "smtp_use_tls")
	}
	if before.TurnstileEnabled != after.TurnstileEnabled {
		changed = append(changed, "turnstile_enabled")
	}
	if before.TurnstileSiteKey != after.TurnstileSiteKey {
		changed = append(changed, "turnstile_site_key")
	}
	if req.TurnstileSecretKey != "" {
		changed = append(changed, "turnstile_secret_key")
	}
	if before.APIKeyACLTrustForwardedIP != after.APIKeyACLTrustForwardedIP {
		changed = append(changed, "api_key_acl_trust_forwarded_ip")
	}
	if before.LinuxDoConnectEnabled != after.LinuxDoConnectEnabled {
		changed = append(changed, "linuxdo_connect_enabled")
	}
	if before.LinuxDoConnectClientID != after.LinuxDoConnectClientID {
		changed = append(changed, "linuxdo_connect_client_id")
	}
	if req.LinuxDoConnectClientSecret != "" {
		changed = append(changed, "linuxdo_connect_client_secret")
	}
	if before.LinuxDoConnectRedirectURL != after.LinuxDoConnectRedirectURL {
		changed = append(changed, "linuxdo_connect_redirect_url")
	}
	if before.WeChatConnectEnabled != after.WeChatConnectEnabled {
		changed = append(changed, "wechat_connect_enabled")
	}
	if before.WeChatConnectAppID != after.WeChatConnectAppID {
		changed = append(changed, "wechat_connect_app_id")
	}
	if req.WeChatConnectAppSecret != "" {
		changed = append(changed, "wechat_connect_app_secret")
	}
	if before.WeChatConnectOpenAppID != after.WeChatConnectOpenAppID {
		changed = append(changed, "wechat_connect_open_app_id")
	}
	if req.WeChatConnectOpenAppSecret != "" {
		changed = append(changed, "wechat_connect_open_app_secret")
	}
	if before.WeChatConnectMPAppID != after.WeChatConnectMPAppID {
		changed = append(changed, "wechat_connect_mp_app_id")
	}
	if req.WeChatConnectMPAppSecret != "" {
		changed = append(changed, "wechat_connect_mp_app_secret")
	}
	if before.WeChatConnectMobileAppID != after.WeChatConnectMobileAppID {
		changed = append(changed, "wechat_connect_mobile_app_id")
	}
	if req.WeChatConnectMobileAppSecret != "" {
		changed = append(changed, "wechat_connect_mobile_app_secret")
	}
	if before.WeChatConnectOpenEnabled != after.WeChatConnectOpenEnabled {
		changed = append(changed, "wechat_connect_open_enabled")
	}
	if before.WeChatConnectMPEnabled != after.WeChatConnectMPEnabled {
		changed = append(changed, "wechat_connect_mp_enabled")
	}
	if before.WeChatConnectMobileEnabled != after.WeChatConnectMobileEnabled {
		changed = append(changed, "wechat_connect_mobile_enabled")
	}
	if before.WeChatConnectMode != after.WeChatConnectMode {
		changed = append(changed, "wechat_connect_mode")
	}
	if before.WeChatConnectScopes != after.WeChatConnectScopes {
		changed = append(changed, "wechat_connect_scopes")
	}
	if before.WeChatConnectRedirectURL != after.WeChatConnectRedirectURL {
		changed = append(changed, "wechat_connect_redirect_url")
	}
	if before.WeChatConnectFrontendRedirectURL != after.WeChatConnectFrontendRedirectURL {
		changed = append(changed, "wechat_connect_frontend_redirect_url")
	}
	if before.OIDCConnectEnabled != after.OIDCConnectEnabled {
		changed = append(changed, "oidc_connect_enabled")
	}
	if before.OIDCConnectProviderName != after.OIDCConnectProviderName {
		changed = append(changed, "oidc_connect_provider_name")
	}
	if before.OIDCConnectClientID != after.OIDCConnectClientID {
		changed = append(changed, "oidc_connect_client_id")
	}
	if req.OIDCConnectClientSecret != "" {
		changed = append(changed, "oidc_connect_client_secret")
	}
	if before.OIDCConnectIssuerURL != after.OIDCConnectIssuerURL {
		changed = append(changed, "oidc_connect_issuer_url")
	}
	if before.OIDCConnectDiscoveryURL != after.OIDCConnectDiscoveryURL {
		changed = append(changed, "oidc_connect_discovery_url")
	}
	if before.OIDCConnectAuthorizeURL != after.OIDCConnectAuthorizeURL {
		changed = append(changed, "oidc_connect_authorize_url")
	}
	if before.OIDCConnectTokenURL != after.OIDCConnectTokenURL {
		changed = append(changed, "oidc_connect_token_url")
	}
	if before.OIDCConnectUserInfoURL != after.OIDCConnectUserInfoURL {
		changed = append(changed, "oidc_connect_userinfo_url")
	}
	if before.OIDCConnectJWKSURL != after.OIDCConnectJWKSURL {
		changed = append(changed, "oidc_connect_jwks_url")
	}
	if before.OIDCConnectScopes != after.OIDCConnectScopes {
		changed = append(changed, "oidc_connect_scopes")
	}
	if before.OIDCConnectRedirectURL != after.OIDCConnectRedirectURL {
		changed = append(changed, "oidc_connect_redirect_url")
	}
	if before.OIDCConnectFrontendRedirectURL != after.OIDCConnectFrontendRedirectURL {
		changed = append(changed, "oidc_connect_frontend_redirect_url")
	}
	if before.OIDCConnectTokenAuthMethod != after.OIDCConnectTokenAuthMethod {
		changed = append(changed, "oidc_connect_token_auth_method")
	}
	if before.OIDCConnectUsePKCE != after.OIDCConnectUsePKCE {
		changed = append(changed, "oidc_connect_use_pkce")
	}
	if before.OIDCConnectValidateIDToken != after.OIDCConnectValidateIDToken {
		changed = append(changed, "oidc_connect_validate_id_token")
	}
	if before.OIDCConnectAllowedSigningAlgs != after.OIDCConnectAllowedSigningAlgs {
		changed = append(changed, "oidc_connect_allowed_signing_algs")
	}
	if before.OIDCConnectClockSkewSeconds != after.OIDCConnectClockSkewSeconds {
		changed = append(changed, "oidc_connect_clock_skew_seconds")
	}
	if before.OIDCConnectRequireEmailVerified != after.OIDCConnectRequireEmailVerified {
		changed = append(changed, "oidc_connect_require_email_verified")
	}
	if before.OIDCConnectUserInfoEmailPath != after.OIDCConnectUserInfoEmailPath {
		changed = append(changed, "oidc_connect_userinfo_email_path")
	}
	if before.OIDCConnectUserInfoIDPath != after.OIDCConnectUserInfoIDPath {
		changed = append(changed, "oidc_connect_userinfo_id_path")
	}
	if before.OIDCConnectUserInfoUsernamePath != after.OIDCConnectUserInfoUsernamePath {
		changed = append(changed, "oidc_connect_userinfo_username_path")
	}
	if before.SiteName != after.SiteName {
		changed = append(changed, "site_name")
	}
	if before.SiteLogo != after.SiteLogo {
		changed = append(changed, "site_logo")
	}
	if before.SiteSubtitle != after.SiteSubtitle {
		changed = append(changed, "site_subtitle")
	}
	if before.APIBaseURL != after.APIBaseURL {
		changed = append(changed, "api_base_url")
	}
	if before.ContactInfo != after.ContactInfo {
		changed = append(changed, "contact_info")
	}
	if before.DocURL != after.DocURL {
		changed = append(changed, "doc_url")
	}
	if before.HomeContent != after.HomeContent {
		changed = append(changed, "home_content")
	}
	if before.HomeStatsGroupID != after.HomeStatsGroupID {
		changed = append(changed, "home_stats_group_id")
	}
	if before.HideCcsImportButton != after.HideCcsImportButton {
		changed = append(changed, "hide_ccs_import_button")
	}
	if before.DefaultConcurrency != after.DefaultConcurrency {
		changed = append(changed, "default_concurrency")
	}
	if before.DefaultBalance != after.DefaultBalance {
		changed = append(changed, "default_balance")
	}
	if before.DefaultUserRPMLimit != after.DefaultUserRPMLimit {
		changed = append(changed, "default_user_rpm_limit")
	}
	if before.AffiliateRebateRate != after.AffiliateRebateRate {
		changed = append(changed, "affiliate_rebate_rate")
	}
	if before.AffiliateRebateFreezeHours != after.AffiliateRebateFreezeHours {
		changed = append(changed, "affiliate_rebate_freeze_hours")
	}
	if before.AffiliateRebateDurationDays != after.AffiliateRebateDurationDays {
		changed = append(changed, "affiliate_rebate_duration_days")
	}
	if before.AffiliateRebatePerInviteeCap != after.AffiliateRebatePerInviteeCap {
		changed = append(changed, "affiliate_rebate_per_invitee_cap")
	}
	if !equalOptionalFloat64(before.UserPrivateGroupDailyLimitUSD, after.UserPrivateGroupDailyLimitUSD) {
		changed = append(changed, "user_private_group_daily_limit_usd")
	}
	if !equalOptionalFloat64(before.UserPrivateGroupWeeklyLimitUSD, after.UserPrivateGroupWeeklyLimitUSD) {
		changed = append(changed, "user_private_group_weekly_limit_usd")
	}
	if !equalOptionalFloat64(before.UserPrivateGroupMonthlyLimitUSD, after.UserPrivateGroupMonthlyLimitUSD) {
		changed = append(changed, "user_private_group_monthly_limit_usd")
	}
	if before.UserPrivateGroupRateMultiplier != after.UserPrivateGroupRateMultiplier {
		changed = append(changed, "user_private_group_rate_multiplier")
	}
	if before.UserPrivateGroupRPMLimit != after.UserPrivateGroupRPMLimit {
		changed = append(changed, "user_private_group_rpm_limit")
	}
	if before.UserPrivateGroupCommissionRate != after.UserPrivateGroupCommissionRate {
		changed = append(changed, "user_private_group_commission_rate")
	}
	if !equalDefaultSubscriptions(before.DefaultSubscriptions, after.DefaultSubscriptions) {
		changed = append(changed, "default_subscriptions")
	}
	if before.EnableModelFallback != after.EnableModelFallback {
		changed = append(changed, "enable_model_fallback")
	}
	if before.FallbackModelAnthropic != after.FallbackModelAnthropic {
		changed = append(changed, "fallback_model_anthropic")
	}
	if before.FallbackModelOpenAI != after.FallbackModelOpenAI {
		changed = append(changed, "fallback_model_openai")
	}
	if before.FallbackModelGemini != after.FallbackModelGemini {
		changed = append(changed, "fallback_model_gemini")
	}
	if before.FallbackModelAntigravity != after.FallbackModelAntigravity {
		changed = append(changed, "fallback_model_antigravity")
	}
	if before.EnableIdentityPatch != after.EnableIdentityPatch {
		changed = append(changed, "enable_identity_patch")
	}
	if before.IdentityPatchPrompt != after.IdentityPatchPrompt {
		changed = append(changed, "identity_patch_prompt")
	}
	if before.OpsMonitoringEnabled != after.OpsMonitoringEnabled {
		changed = append(changed, "ops_monitoring_enabled")
	}
	if before.OpsRealtimeMonitoringEnabled != after.OpsRealtimeMonitoringEnabled {
		changed = append(changed, "ops_realtime_monitoring_enabled")
	}
	if before.OpsQueryModeDefault != after.OpsQueryModeDefault {
		changed = append(changed, "ops_query_mode_default")
	}
	if before.OpsMetricsIntervalSeconds != after.OpsMetricsIntervalSeconds {
		changed = append(changed, "ops_metrics_interval_seconds")
	}
	if before.MinClaudeCodeVersion != after.MinClaudeCodeVersion {
		changed = append(changed, "min_claude_code_version")
	}
	if before.MaxClaudeCodeVersion != after.MaxClaudeCodeVersion {
		changed = append(changed, "max_claude_code_version")
	}
	if before.AllowUngroupedKeyScheduling != after.AllowUngroupedKeyScheduling {
		changed = append(changed, "allow_ungrouped_key_scheduling")
	}
	if before.BackendModeEnabled != after.BackendModeEnabled {
		changed = append(changed, "backend_mode_enabled")
	}
	if before.PurchaseSubscriptionEnabled != after.PurchaseSubscriptionEnabled {
		changed = append(changed, "purchase_subscription_enabled")
	}
	if before.PurchaseSubscriptionURL != after.PurchaseSubscriptionURL {
		changed = append(changed, "purchase_subscription_url")
	}
	if before.TableDefaultPageSize != after.TableDefaultPageSize {
		changed = append(changed, "table_default_page_size")
	}
	if !equalIntSlice(before.TablePageSizeOptions, after.TablePageSizeOptions) {
		changed = append(changed, "table_page_size_options")
	}
	if before.CustomMenuItems != after.CustomMenuItems {
		changed = append(changed, "custom_menu_items")
	}
	if before.CustomEndpoints != after.CustomEndpoints {
		changed = append(changed, "custom_endpoints")
	}
	if before.EnableFingerprintUnification != after.EnableFingerprintUnification {
		changed = append(changed, "enable_fingerprint_unification")
	}
	if before.EnableMetadataPassthrough != after.EnableMetadataPassthrough {
		changed = append(changed, "enable_metadata_passthrough")
	}
	if before.EnableCCHSigning != after.EnableCCHSigning {
		changed = append(changed, "enable_cch_signing")
	}
	if before.EnableAnthropicCacheTTL1hInjection != after.EnableAnthropicCacheTTL1hInjection {
		changed = append(changed, "enable_anthropic_cache_ttl_1h_injection")
	}
	if before.OpenAIImagesResponsesReasoningEffort != after.OpenAIImagesResponsesReasoningEffort {
		changed = append(changed, "openai_images_responses_reasoning_effort")
	}
	if before.PaymentVisibleMethodAlipaySource != after.PaymentVisibleMethodAlipaySource {
		changed = append(changed, "payment_visible_method_alipay_source")
	}
	if before.PaymentVisibleMethodWxpaySource != after.PaymentVisibleMethodWxpaySource {
		changed = append(changed, "payment_visible_method_wxpay_source")
	}
	if before.PaymentVisibleMethodAlipayEnabled != after.PaymentVisibleMethodAlipayEnabled {
		changed = append(changed, "payment_visible_method_alipay_enabled")
	}
	if before.PaymentVisibleMethodWxpayEnabled != after.PaymentVisibleMethodWxpayEnabled {
		changed = append(changed, "payment_visible_method_wxpay_enabled")
	}
	if before.OpenAIAdvancedSchedulerEnabled != after.OpenAIAdvancedSchedulerEnabled {
		changed = append(changed, "openai_advanced_scheduler_enabled")
	}
	if before.OpenAIFreeAccountRepairEnabled != after.OpenAIFreeAccountRepairEnabled {
		changed = append(changed, "openai_free_account_repair_enabled")
	}
	if before.OpenAIFreeAccountRepairWeeklyThresholdUSD != after.OpenAIFreeAccountRepairWeeklyThresholdUSD {
		changed = append(changed, "openai_free_account_repair_weekly_threshold_usd")
	}
	// Balance & quota notification
	if before.BalanceLowNotifyEnabled != after.BalanceLowNotifyEnabled {
		changed = append(changed, "balance_low_notify_enabled")
	}
	if before.BalanceLowNotifyThreshold != after.BalanceLowNotifyThreshold {
		changed = append(changed, "balance_low_notify_threshold")
	}
	if before.BalanceLowNotifyRechargeURL != after.BalanceLowNotifyRechargeURL {
		changed = append(changed, "balance_low_notify_recharge_url")
	}
	if before.AccountQuotaNotifyEnabled != after.AccountQuotaNotifyEnabled {
		changed = append(changed, "account_quota_notify_enabled")
	}
	if !equalNotifyEmailEntries(before.AccountQuotaNotifyEmails, after.AccountQuotaNotifyEmails) {
		changed = append(changed, "account_quota_notify_emails")
	}
	if before.ChannelMonitorEnabled != after.ChannelMonitorEnabled {
		changed = append(changed, "channel_monitor_enabled")
	}
	if before.ChannelMonitorDefaultIntervalSeconds != after.ChannelMonitorDefaultIntervalSeconds {
		changed = append(changed, "channel_monitor_default_interval_seconds")
	}
	if before.AvailableChannelsEnabled != after.AvailableChannelsEnabled {
		changed = append(changed, "available_channels_enabled")
	}
	if before.FreeModelsEnabled != after.FreeModelsEnabled {
		changed = append(changed, "free_models_enabled")
	}
	if before.CarpoolEnabled != after.CarpoolEnabled {
		changed = append(changed, "carpool_enabled")
	}
	if before.CarpoolBaseServiceFeeUSD != after.CarpoolBaseServiceFeeUSD {
		changed = append(changed, "carpool_base_service_fee_usd")
	}
	if before.CarpoolSystemProxyFeeUSD != after.CarpoolSystemProxyFeeUSD {
		changed = append(changed, "carpool_system_proxy_fee_usd")
	}
	if before.CarpoolRiskControlFeeUSD != after.CarpoolRiskControlFeeUSD {
		changed = append(changed, "carpool_risk_control_fee_usd")
	}
	if before.AffiliateEnabled != after.AffiliateEnabled {
		changed = append(changed, "affiliate_enabled")
	}
	changed = appendAuthSourceDefaultChanges(changed, beforeAuthSourceDefaults, afterAuthSourceDefaults)
	return changed
}

func appendAuthSourceDefaultChanges(changed []string, before *service.AuthSourceDefaultSettings, after *service.AuthSourceDefaultSettings) []string {
	if before == nil {
		before = &service.AuthSourceDefaultSettings{}
	}
	if after == nil {
		after = &service.AuthSourceDefaultSettings{}
	}

	type providerDefaultGrantField struct {
		name   string
		before service.ProviderDefaultGrantSettings
		after  service.ProviderDefaultGrantSettings
	}

	fields := []providerDefaultGrantField{
		{name: "email", before: before.Email, after: after.Email},
		{name: "linuxdo", before: before.LinuxDo, after: after.LinuxDo},
		{name: "oidc", before: before.OIDC, after: after.OIDC},
		{name: "wechat", before: before.WeChat, after: after.WeChat},
	}
	for _, field := range fields {
		if field.before.Balance != field.after.Balance {
			changed = append(changed, "auth_source_default_"+field.name+"_balance")
		}
		if field.before.Concurrency != field.after.Concurrency {
			changed = append(changed, "auth_source_default_"+field.name+"_concurrency")
		}
		if !equalDefaultSubscriptions(field.before.Subscriptions, field.after.Subscriptions) {
			changed = append(changed, "auth_source_default_"+field.name+"_subscriptions")
		}
		if field.before.GrantOnSignup != field.after.GrantOnSignup {
			changed = append(changed, "auth_source_default_"+field.name+"_grant_on_signup")
		}
		if field.before.GrantOnFirstBind != field.after.GrantOnFirstBind {
			changed = append(changed, "auth_source_default_"+field.name+"_grant_on_first_bind")
		}
	}
	if before.ForceEmailOnThirdPartySignup != after.ForceEmailOnThirdPartySignup {
		changed = append(changed, "force_email_on_third_party_signup")
	}
	return changed
}

func normalizeDefaultSubscriptions(input []dto.DefaultSubscriptionSetting) []dto.DefaultSubscriptionSetting {
	if len(input) == 0 {
		return nil
	}
	normalized := make([]dto.DefaultSubscriptionSetting, 0, len(input))
	for _, item := range input {
		if item.GroupID <= 0 || item.ValidityDays <= 0 {
			continue
		}
		if item.ValidityDays > service.MaxValidityDays {
			item.ValidityDays = service.MaxValidityDays
		}
		normalized = append(normalized, item)
	}
	return normalized
}

func normalizeOptionalDefaultSubscriptions(input *[]dto.DefaultSubscriptionSetting) *[]dto.DefaultSubscriptionSetting {
	if input == nil {
		return nil
	}
	normalized := normalizeDefaultSubscriptions(*input)
	return &normalized
}

func float64ValueOrDefault(value *float64, fallback float64) float64 {
	if value == nil {
		return fallback
	}
	return *value
}

func positiveFloat64Ptr(value float64) *float64 {
	if value <= 0 {
		return nil
	}
	return &value
}

func intValueOrDefault(value *int, fallback int) int {
	if value == nil {
		return fallback
	}
	return *value
}

func boolValueOrDefault(value *bool, fallback bool) bool {
	if value == nil {
		return fallback
	}
	return *value
}

func defaultSubscriptionsValueOrDefault(input *[]dto.DefaultSubscriptionSetting, fallback []service.DefaultSubscriptionSetting) []service.DefaultSubscriptionSetting {
	if input == nil {
		return fallback
	}
	result := make([]service.DefaultSubscriptionSetting, 0, len(*input))
	for _, item := range *input {
		result = append(result, service.DefaultSubscriptionSetting{
			GroupID:      item.GroupID,
			ValidityDays: item.ValidityDays,
		})
	}
	return result
}

func systemSettingsResponseData(settings dto.SystemSettings, authSourceDefaults *service.AuthSourceDefaultSettings) map[string]any {
	data := make(map[string]any)
	raw, err := json.Marshal(settings)
	if err == nil {
		_ = json.Unmarshal(raw, &data)
	}
	if authSourceDefaults == nil {
		authSourceDefaults = &service.AuthSourceDefaultSettings{}
	}

	data["auth_source_default_email_balance"] = authSourceDefaults.Email.Balance
	data["auth_source_default_email_concurrency"] = authSourceDefaults.Email.Concurrency
	data["auth_source_default_email_subscriptions"] = authSourceDefaults.Email.Subscriptions
	data["auth_source_default_email_grant_on_signup"] = authSourceDefaults.Email.GrantOnSignup
	data["auth_source_default_email_grant_on_first_bind"] = authSourceDefaults.Email.GrantOnFirstBind
	data["auth_source_default_linuxdo_balance"] = authSourceDefaults.LinuxDo.Balance
	data["auth_source_default_linuxdo_concurrency"] = authSourceDefaults.LinuxDo.Concurrency
	data["auth_source_default_linuxdo_subscriptions"] = authSourceDefaults.LinuxDo.Subscriptions
	data["auth_source_default_linuxdo_grant_on_signup"] = authSourceDefaults.LinuxDo.GrantOnSignup
	data["auth_source_default_linuxdo_grant_on_first_bind"] = authSourceDefaults.LinuxDo.GrantOnFirstBind
	data["auth_source_default_oidc_balance"] = authSourceDefaults.OIDC.Balance
	data["auth_source_default_oidc_concurrency"] = authSourceDefaults.OIDC.Concurrency
	data["auth_source_default_oidc_subscriptions"] = authSourceDefaults.OIDC.Subscriptions
	data["auth_source_default_oidc_grant_on_signup"] = authSourceDefaults.OIDC.GrantOnSignup
	data["auth_source_default_oidc_grant_on_first_bind"] = authSourceDefaults.OIDC.GrantOnFirstBind
	data["auth_source_default_wechat_balance"] = authSourceDefaults.WeChat.Balance
	data["auth_source_default_wechat_concurrency"] = authSourceDefaults.WeChat.Concurrency
	data["auth_source_default_wechat_subscriptions"] = authSourceDefaults.WeChat.Subscriptions
	data["auth_source_default_wechat_grant_on_signup"] = authSourceDefaults.WeChat.GrantOnSignup
	data["auth_source_default_wechat_grant_on_first_bind"] = authSourceDefaults.WeChat.GrantOnFirstBind
	data["auth_source_default_github_balance"] = authSourceDefaults.GitHub.Balance
	data["auth_source_default_github_concurrency"] = authSourceDefaults.GitHub.Concurrency
	data["auth_source_default_github_subscriptions"] = authSourceDefaults.GitHub.Subscriptions
	data["auth_source_default_github_grant_on_signup"] = authSourceDefaults.GitHub.GrantOnSignup
	data["auth_source_default_github_grant_on_first_bind"] = authSourceDefaults.GitHub.GrantOnFirstBind
	data["auth_source_default_google_balance"] = authSourceDefaults.Google.Balance
	data["auth_source_default_google_concurrency"] = authSourceDefaults.Google.Concurrency
	data["auth_source_default_google_subscriptions"] = authSourceDefaults.Google.Subscriptions
	data["auth_source_default_google_grant_on_signup"] = authSourceDefaults.Google.GrantOnSignup
	data["auth_source_default_google_grant_on_first_bind"] = authSourceDefaults.Google.GrantOnFirstBind
	data["force_email_on_third_party_signup"] = authSourceDefaults.ForceEmailOnThirdPartySignup

	return data
}

func equalStringSlice(a, b []string) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}

func equalDefaultSubscriptions(a, b []service.DefaultSubscriptionSetting) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i].GroupID != b[i].GroupID || a[i].ValidityDays != b[i].ValidityDays {
			return false
		}
	}
	return true
}

func equalIntSlice(a, b []int) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}

func equalOptionalFloat64(a, b *float64) bool {
	if a == nil || b == nil {
		return a == nil && b == nil
	}
	return *a == *b
}

func equalNotifyEmailEntries(a, b []service.NotifyEmailEntry) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i].Email != b[i].Email || a[i].Verified != b[i].Verified || a[i].Disabled != b[i].Disabled {
			return false
		}
	}
	return true
}

// TestSMTPRequest 测试SMTP连接请求
type TestSMTPRequest struct {
	SMTPHost     string `json:"smtp_host"`
	SMTPPort     int    `json:"smtp_port"`
	SMTPUsername string `json:"smtp_username"`
	SMTPPassword string `json:"smtp_password"`
	SMTPUseTLS   bool   `json:"smtp_use_tls"`
}

// TestSMTPConnection 测试SMTP连接
// POST /api/v1/admin/settings/test-smtp
func (h *SettingHandler) TestSMTPConnection(c *gin.Context) {
	var req TestSMTPRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, "Invalid request: "+err.Error())
		return
	}

	req.SMTPHost = strings.TrimSpace(req.SMTPHost)
	req.SMTPUsername = strings.TrimSpace(req.SMTPUsername)

	var savedConfig *service.SMTPConfig
	if cfg, err := h.emailService.GetSMTPConfig(c.Request.Context()); err == nil && cfg != nil {
		savedConfig = cfg
	}

	if req.SMTPHost == "" && savedConfig != nil {
		req.SMTPHost = savedConfig.Host
	}
	if req.SMTPPort <= 0 {
		if savedConfig != nil && savedConfig.Port > 0 {
			req.SMTPPort = savedConfig.Port
		} else {
			req.SMTPPort = 587
		}
	}
	if req.SMTPUsername == "" && savedConfig != nil {
		req.SMTPUsername = savedConfig.Username
	}
	password := strings.TrimSpace(req.SMTPPassword)
	if password == "" && savedConfig != nil {
		password = savedConfig.Password
	}
	if req.SMTPHost == "" {
		response.BadRequest(c, "SMTP host is required")
		return
	}

	config := &service.SMTPConfig{
		Host:     req.SMTPHost,
		Port:     req.SMTPPort,
		Username: req.SMTPUsername,
		Password: password,
		UseTLS:   req.SMTPUseTLS,
	}

	err := h.emailService.TestSMTPConnectionWithConfig(config)
	if err != nil {
		response.BadRequest(c, "SMTP connection test failed: "+err.Error())
		return
	}

	response.Success(c, gin.H{"message": "SMTP connection successful"})
}

// SendTestEmailRequest 发送测试邮件请求
type SendTestEmailRequest struct {
	Email        string `json:"email" binding:"required,email"`
	SMTPHost     string `json:"smtp_host"`
	SMTPPort     int    `json:"smtp_port"`
	SMTPUsername string `json:"smtp_username"`
	SMTPPassword string `json:"smtp_password"`
	SMTPFrom     string `json:"smtp_from_email"`
	SMTPFromName string `json:"smtp_from_name"`
	SMTPUseTLS   bool   `json:"smtp_use_tls"`
}

// SendTestEmail 发送测试邮件
// POST /api/v1/admin/settings/send-test-email
func (h *SettingHandler) SendTestEmail(c *gin.Context) {
	var req SendTestEmailRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, "Invalid request: "+err.Error())
		return
	}

	req.SMTPHost = strings.TrimSpace(req.SMTPHost)
	req.SMTPUsername = strings.TrimSpace(req.SMTPUsername)
	req.SMTPFrom = strings.TrimSpace(req.SMTPFrom)
	req.SMTPFromName = strings.TrimSpace(req.SMTPFromName)

	var savedConfig *service.SMTPConfig
	if cfg, err := h.emailService.GetSMTPConfig(c.Request.Context()); err == nil && cfg != nil {
		savedConfig = cfg
	}

	if req.SMTPHost == "" && savedConfig != nil {
		req.SMTPHost = savedConfig.Host
	}
	if req.SMTPPort <= 0 {
		if savedConfig != nil && savedConfig.Port > 0 {
			req.SMTPPort = savedConfig.Port
		} else {
			req.SMTPPort = 587
		}
	}
	if req.SMTPUsername == "" && savedConfig != nil {
		req.SMTPUsername = savedConfig.Username
	}
	password := strings.TrimSpace(req.SMTPPassword)
	if password == "" && savedConfig != nil {
		password = savedConfig.Password
	}
	if req.SMTPFrom == "" && savedConfig != nil {
		req.SMTPFrom = savedConfig.From
	}
	if req.SMTPFromName == "" && savedConfig != nil {
		req.SMTPFromName = savedConfig.FromName
	}
	if req.SMTPHost == "" {
		response.BadRequest(c, "SMTP host is required")
		return
	}

	config := &service.SMTPConfig{
		Host:     req.SMTPHost,
		Port:     req.SMTPPort,
		Username: req.SMTPUsername,
		Password: password,
		From:     req.SMTPFrom,
		FromName: req.SMTPFromName,
		UseTLS:   req.SMTPUseTLS,
	}

	siteName := h.settingService.GetSiteName(c.Request.Context())
	subject := "[" + siteName + "] Test Email"
	body := `
<!DOCTYPE html>
<html>
<head>
    <meta charset="UTF-8">
    <style>
        body { font-family: -apple-system, BlinkMacSystemFont, 'Segoe UI', Roboto, sans-serif; background-color: #f7f7f8; margin: 0; padding: 20px; }
        .container { max-width: 600px; margin: 0 auto; background-color: #ffffff; border-radius: 8px; overflow: hidden; box-shadow: 0 2px 8px rgba(0,0,0,0.1); }
        .header { background: linear-gradient(135deg, #10a37f 0%, #0d8f70 100%); color: white; padding: 30px; text-align: center; }
        .content { padding: 40px 30px; text-align: center; }
        .success { color: #10a37f; font-size: 48px; margin-bottom: 20px; }
        .footer { background-color: #f3f3f6; padding: 20px; text-align: center; color: #6e6e80; font-size: 12px; }
    </style>
</head>
<body>
    <div class="container">
        <div class="header">
            <h1>` + siteName + `</h1>
        </div>
        <div class="content">
            <div class="success">✓</div>
            <h2>Email Configuration Successful!</h2>
            <p>This is a test email to verify your SMTP settings are working correctly.</p>
        </div>
        <div class="footer">
            <p>This is an automated test message.</p>
        </div>
    </div>
</body>
</html>
`

	if err := h.emailService.SendEmailWithConfig(config, req.Email, subject, body); err != nil {
		response.BadRequest(c, "Failed to send test email: "+err.Error())
		return
	}

	response.Success(c, gin.H{"message": "Test email sent successfully"})
}

// GetAdminAPIKey 获取管理员 API Key 状态
// GET /api/v1/admin/settings/admin-api-key
func (h *SettingHandler) GetAdminAPIKey(c *gin.Context) {
	maskedKey, exists, err := h.settingService.GetAdminAPIKeyStatus(c.Request.Context())
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}

	response.Success(c, gin.H{
		"exists":     exists,
		"masked_key": maskedKey,
	})
}

// RegenerateAdminAPIKey 生成/重新生成管理员 API Key
// POST /api/v1/admin/settings/admin-api-key/regenerate
func (h *SettingHandler) RegenerateAdminAPIKey(c *gin.Context) {
	key, err := h.settingService.GenerateAdminAPIKey(c.Request.Context())
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}

	response.Success(c, gin.H{
		"key": key, // 完整 key 只在生成时返回一次
	})
}

// DeleteAdminAPIKey 删除管理员 API Key
// DELETE /api/v1/admin/settings/admin-api-key
func (h *SettingHandler) DeleteAdminAPIKey(c *gin.Context) {
	if err := h.settingService.DeleteAdminAPIKey(c.Request.Context()); err != nil {
		response.ErrorFrom(c, err)
		return
	}

	response.Success(c, gin.H{"message": "Admin API key deleted"})
}

// GetOverloadCooldownSettings 获取529过载冷却配置
// GET /api/v1/admin/settings/overload-cooldown
func (h *SettingHandler) GetOverloadCooldownSettings(c *gin.Context) {
	settings, err := h.settingService.GetOverloadCooldownSettings(c.Request.Context())
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}

	response.Success(c, dto.OverloadCooldownSettings{
		Enabled:         settings.Enabled,
		CooldownMinutes: settings.CooldownMinutes,
	})
}

// UpdateOverloadCooldownSettingsRequest 更新529过载冷却配置请求
type UpdateOverloadCooldownSettingsRequest struct {
	Enabled         bool `json:"enabled"`
	CooldownMinutes int  `json:"cooldown_minutes"`
}

// UpdateOverloadCooldownSettings 更新529过载冷却配置
// PUT /api/v1/admin/settings/overload-cooldown
func (h *SettingHandler) UpdateOverloadCooldownSettings(c *gin.Context) {
	var req UpdateOverloadCooldownSettingsRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, "Invalid request: "+err.Error())
		return
	}

	settings := &service.OverloadCooldownSettings{
		Enabled:         req.Enabled,
		CooldownMinutes: req.CooldownMinutes,
	}

	if err := h.settingService.SetOverloadCooldownSettings(c.Request.Context(), settings); err != nil {
		response.BadRequest(c, err.Error())
		return
	}

	updatedSettings, err := h.settingService.GetOverloadCooldownSettings(c.Request.Context())
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}

	response.Success(c, dto.OverloadCooldownSettings{
		Enabled:         updatedSettings.Enabled,
		CooldownMinutes: updatedSettings.CooldownMinutes,
	})
}

// GetStreamTimeoutSettings 获取流超时处理配置
// GET /api/v1/admin/settings/stream-timeout
func (h *SettingHandler) GetStreamTimeoutSettings(c *gin.Context) {
	settings, err := h.settingService.GetStreamTimeoutSettings(c.Request.Context())
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}

	response.Success(c, dto.StreamTimeoutSettings{
		Enabled:                settings.Enabled,
		Action:                 settings.Action,
		TempUnschedMinutes:     settings.TempUnschedMinutes,
		ThresholdCount:         settings.ThresholdCount,
		ThresholdWindowMinutes: settings.ThresholdWindowMinutes,
	})
}

// GetRectifierSettings 获取请求整流器配置
// GET /api/v1/admin/settings/rectifier
func (h *SettingHandler) GetRectifierSettings(c *gin.Context) {
	settings, err := h.settingService.GetRectifierSettings(c.Request.Context())
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}

	patterns := settings.APIKeySignaturePatterns
	if patterns == nil {
		patterns = []string{}
	}
	response.Success(c, dto.RectifierSettings{
		Enabled:                  settings.Enabled,
		ThinkingSignatureEnabled: settings.ThinkingSignatureEnabled,
		ThinkingBudgetEnabled:    settings.ThinkingBudgetEnabled,
		APIKeySignatureEnabled:   settings.APIKeySignatureEnabled,
		APIKeySignaturePatterns:  patterns,
	})
}

// UpdateRectifierSettingsRequest 更新整流器配置请求
type UpdateRectifierSettingsRequest struct {
	Enabled                  bool     `json:"enabled"`
	ThinkingSignatureEnabled bool     `json:"thinking_signature_enabled"`
	ThinkingBudgetEnabled    bool     `json:"thinking_budget_enabled"`
	APIKeySignatureEnabled   bool     `json:"apikey_signature_enabled"`
	APIKeySignaturePatterns  []string `json:"apikey_signature_patterns"`
}

// UpdateRectifierSettings 更新请求整流器配置
// PUT /api/v1/admin/settings/rectifier
func (h *SettingHandler) UpdateRectifierSettings(c *gin.Context) {
	var req UpdateRectifierSettingsRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, "Invalid request: "+err.Error())
		return
	}

	// 校验并清理自定义匹配关键词
	const maxPatterns = 50
	const maxPatternLen = 500
	if len(req.APIKeySignaturePatterns) > maxPatterns {
		response.BadRequest(c, "Too many signature patterns (max 50)")
		return
	}
	var cleanedPatterns []string
	for _, p := range req.APIKeySignaturePatterns {
		p = strings.TrimSpace(p)
		if p == "" {
			continue
		}
		if len(p) > maxPatternLen {
			response.BadRequest(c, "Signature pattern too long (max 500 characters)")
			return
		}
		cleanedPatterns = append(cleanedPatterns, p)
	}

	settings := &service.RectifierSettings{
		Enabled:                  req.Enabled,
		ThinkingSignatureEnabled: req.ThinkingSignatureEnabled,
		ThinkingBudgetEnabled:    req.ThinkingBudgetEnabled,
		APIKeySignatureEnabled:   req.APIKeySignatureEnabled,
		APIKeySignaturePatterns:  cleanedPatterns,
	}

	if err := h.settingService.SetRectifierSettings(c.Request.Context(), settings); err != nil {
		response.BadRequest(c, err.Error())
		return
	}

	// 重新获取设置返回
	updatedSettings, err := h.settingService.GetRectifierSettings(c.Request.Context())
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}

	updatedPatterns := updatedSettings.APIKeySignaturePatterns
	if updatedPatterns == nil {
		updatedPatterns = []string{}
	}
	response.Success(c, dto.RectifierSettings{
		Enabled:                  updatedSettings.Enabled,
		ThinkingSignatureEnabled: updatedSettings.ThinkingSignatureEnabled,
		ThinkingBudgetEnabled:    updatedSettings.ThinkingBudgetEnabled,
		APIKeySignatureEnabled:   updatedSettings.APIKeySignatureEnabled,
		APIKeySignaturePatterns:  updatedPatterns,
	})
}

// GetBetaPolicySettings 获取 Beta 策略配置
// GET /api/v1/admin/settings/beta-policy
func (h *SettingHandler) GetBetaPolicySettings(c *gin.Context) {
	settings, err := h.settingService.GetBetaPolicySettings(c.Request.Context())
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}

	rules := make([]dto.BetaPolicyRule, len(settings.Rules))
	for i, r := range settings.Rules {
		rules[i] = dto.BetaPolicyRule(r)
	}
	response.Success(c, dto.BetaPolicySettings{Rules: rules})
}

// UpdateBetaPolicySettingsRequest 更新 Beta 策略配置请求
type UpdateBetaPolicySettingsRequest struct {
	Rules []dto.BetaPolicyRule `json:"rules"`
}

// UpdateBetaPolicySettings 更新 Beta 策略配置
// PUT /api/v1/admin/settings/beta-policy
func (h *SettingHandler) UpdateBetaPolicySettings(c *gin.Context) {
	var req UpdateBetaPolicySettingsRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, "Invalid request: "+err.Error())
		return
	}

	rules := make([]service.BetaPolicyRule, len(req.Rules))
	for i, r := range req.Rules {
		rules[i] = service.BetaPolicyRule(r)
	}

	settings := &service.BetaPolicySettings{Rules: rules}
	if err := h.settingService.SetBetaPolicySettings(c.Request.Context(), settings); err != nil {
		response.BadRequest(c, err.Error())
		return
	}

	// Re-fetch to return updated settings
	updated, err := h.settingService.GetBetaPolicySettings(c.Request.Context())
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}

	outRules := make([]dto.BetaPolicyRule, len(updated.Rules))
	for i, r := range updated.Rules {
		outRules[i] = dto.BetaPolicyRule(r)
	}
	response.Success(c, dto.BetaPolicySettings{Rules: outRules})
}

// UpdateStreamTimeoutSettingsRequest 更新流超时配置请求
type UpdateStreamTimeoutSettingsRequest struct {
	Enabled                bool   `json:"enabled"`
	Action                 string `json:"action"`
	TempUnschedMinutes     int    `json:"temp_unsched_minutes"`
	ThresholdCount         int    `json:"threshold_count"`
	ThresholdWindowMinutes int    `json:"threshold_window_minutes"`
}

// UpdateStreamTimeoutSettings 更新流超时处理配置
// PUT /api/v1/admin/settings/stream-timeout
func (h *SettingHandler) UpdateStreamTimeoutSettings(c *gin.Context) {
	var req UpdateStreamTimeoutSettingsRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, "Invalid request: "+err.Error())
		return
	}

	settings := &service.StreamTimeoutSettings{
		Enabled:                req.Enabled,
		Action:                 req.Action,
		TempUnschedMinutes:     req.TempUnschedMinutes,
		ThresholdCount:         req.ThresholdCount,
		ThresholdWindowMinutes: req.ThresholdWindowMinutes,
	}

	if err := h.settingService.SetStreamTimeoutSettings(c.Request.Context(), settings); err != nil {
		response.BadRequest(c, err.Error())
		return
	}

	// 重新获取设置返回
	updatedSettings, err := h.settingService.GetStreamTimeoutSettings(c.Request.Context())
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}

	response.Success(c, dto.StreamTimeoutSettings{
		Enabled:                updatedSettings.Enabled,
		Action:                 updatedSettings.Action,
		TempUnschedMinutes:     updatedSettings.TempUnschedMinutes,
		ThresholdCount:         updatedSettings.ThresholdCount,
		ThresholdWindowMinutes: updatedSettings.ThresholdWindowMinutes,
	})
}

// GetWebSearchEmulationConfig 获取 Web Search 模拟配置
// GET /api/v1/admin/settings/web-search-emulation
func (h *SettingHandler) GetWebSearchEmulationConfig(c *gin.Context) {
	cfg, err := h.settingService.GetWebSearchEmulationConfig(c.Request.Context())
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}
	response.Success(c, service.PopulateWebSearchUsage(c.Request.Context(), cfg))
}

// UpdateWebSearchEmulationConfig 更新 Web Search 模拟配置
// PUT /api/v1/admin/settings/web-search-emulation
func (h *SettingHandler) UpdateWebSearchEmulationConfig(c *gin.Context) {
	var cfg service.WebSearchEmulationConfig
	if err := c.ShouldBindJSON(&cfg); err != nil {
		response.BadRequest(c, "Invalid request: "+err.Error())
		return
	}

	if err := h.settingService.SaveWebSearchEmulationConfig(c.Request.Context(), &cfg); err != nil {
		response.ErrorFrom(c, err)
		return
	}

	// Re-read (with sanitized api keys) to return current state
	updated, err := h.settingService.GetWebSearchEmulationConfig(c.Request.Context())
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}
	response.Success(c, service.PopulateWebSearchUsage(c.Request.Context(), updated))
}

// ResetWebSearchUsage 重置指定 provider 的配额用量
// POST /api/v1/admin/settings/web-search-emulation/reset-usage
func (h *SettingHandler) ResetWebSearchUsage(c *gin.Context) {
	var req struct {
		ProviderType string `json:"provider_type"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, "Invalid request: "+err.Error())
		return
	}
	if req.ProviderType == "" {
		response.BadRequest(c, "provider_type is required")
		return
	}
	if err := service.ResetWebSearchUsage(c.Request.Context(), req.ProviderType); err != nil {
		response.ErrorFrom(c, err)
		return
	}
	response.Success(c, nil)
}

// TestWebSearchEmulation 测试 Web Search 搜索
// POST /api/v1/admin/settings/web-search-emulation/test
func (h *SettingHandler) TestWebSearchEmulation(c *gin.Context) {
	var req struct {
		Query string `json:"query"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, "Invalid request: "+err.Error())
		return
	}
	if strings.TrimSpace(req.Query) == "" {
		req.Query = "搜索今年世界大事件"
	}

	result, err := service.TestWebSearch(c.Request.Context(), req.Query)
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}
	response.Success(c, result)
}
