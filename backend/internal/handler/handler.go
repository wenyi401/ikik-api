package handler

import (
	"ikik-api/internal/handler/admin"
)

// AdminHandlers contains all admin-related HTTP handlers
type AdminHandlers struct {
	Dashboard              *admin.DashboardHandler
	User                   *admin.UserHandler
	Group                  *admin.GroupHandler
	Account                *admin.AccountHandler
	AccountSharePolicy     *admin.AccountSharePolicyHandler
	Carpool                *admin.CarpoolHandler
	Announcement           *admin.AnnouncementHandler
	EmailBroadcast         *admin.EmailBroadcastHandler
	DataManagement         *admin.DataManagementHandler
	Backup                 *admin.BackupHandler
	OAuth                  *admin.OAuthHandler
	OpenAIOAuth            *admin.OpenAIOAuthHandler
	GeminiOAuth            *admin.GeminiOAuthHandler
	AntigravityOAuth       *admin.AntigravityOAuthHandler
	GrokOAuth              *admin.GrokOAuthHandler
	Proxy                  *admin.ProxyHandler
	Redeem                 *admin.RedeemHandler
	Promo                  *admin.PromoHandler
	Setting                *admin.SettingHandler
	Ops                    *admin.OpsHandler
	System                 *admin.SystemHandler
	Subscription           *admin.SubscriptionHandler
	Usage                  *admin.UsageHandler
	UserAttribute          *admin.UserAttributeHandler
	ErrorPassthrough       *admin.ErrorPassthroughHandler
	TLSFingerprintProfile  *admin.TLSFingerprintProfileHandler
	APIKey                 *admin.AdminAPIKeyHandler
	ScheduledTest          *admin.ScheduledTestHandler
	Channel                *admin.ChannelHandler
	ChannelMonitor         *admin.ChannelMonitorHandler
	ChannelMonitorTemplate *admin.ChannelMonitorRequestTemplateHandler
	ContentModeration      *admin.ContentModerationHandler
	Payment                *admin.PaymentHandler
	Revenue                *admin.RevenueHandler
	Withdrawal             *admin.WithdrawalHandler
	Shop                   *admin.ShopHandler
	Affiliate              *admin.AffiliateHandler
	Module                 *admin.ModuleHandler
}

// Handlers contains all HTTP handlers
type Handlers struct {
	Auth             *AuthHandler
	User             *UserHandler
	APIKey           *APIKeyHandler
	UserAccount      *UserAccountHandler
	Usage            *UsageHandler
	Redeem           *RedeemHandler
	Subscription     *SubscriptionHandler
	Announcement     *AnnouncementHandler
	ChannelMonitor   *ChannelMonitorUserHandler
	Admin            *AdminHandlers
	Gateway          *GatewayHandler
	OpenAIGateway    *OpenAIGatewayHandler
	Setting          *SettingHandler
	Totp             *TotpHandler
	Payment          *PaymentHandler
	PaymentWebhook   *PaymentWebhookHandler
	AvailableChannel *AvailableChannelHandler
	ReceiptCode      *ReceiptCodeHandler
	Withdrawal       *WithdrawalHandler
	Shop             *ShopHandler
}

// BuildInfo contains build-time information
type BuildInfo struct {
	Version   string
	BuildType string // "source" for manual builds, "release" for CI builds
}
