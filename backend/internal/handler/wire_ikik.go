package handler

import (
	"ikik-api/internal/gatewayhook"
	"ikik-api/internal/handler/admin"
	"ikik-api/internal/service"

	"github.com/google/wire"
)

type IkikHandlerRuntimeWiring struct{}

func ProvideIkikHandlerRuntimeWiring(
	gatewayHandler *GatewayHandler,
	openAIGatewayHandler *OpenAIGatewayHandler,
	channelMonitorUserHandler *ChannelMonitorUserHandler,
	paymentHandler *PaymentHandler,
	adminAccountHandler *admin.AccountHandler,
	adminGroupHandler *admin.GroupHandler,
	hooks *gatewayhook.Chain,
	accountService *service.AccountService,
	kiroOAuthService *service.KiroOAuthService,
	accountBatchTaskService *service.AccountBatchTaskService,
	groupRateScheduleService *service.GroupRateScheduleService,
	groupCapacityService *service.GroupCapacityService,
	channelService *service.ChannelService,
	_ *service.IkikRuntimeWiring,
) *IkikHandlerRuntimeWiring {
	gatewayHandler.SetPreFlightHooks(hooks)
	openAIGatewayHandler.SetPreFlightHooks(hooks)
	adminAccountHandler.ConfigureIkikExtensions(accountService, kiroOAuthService, accountBatchTaskService)
	adminGroupHandler.ConfigureIkikExtensions(groupRateScheduleService)
	channelMonitorUserHandler.configureIkikExtensions(groupCapacityService, accountService)
	paymentHandler.configureIkikExtensions(channelService)
	return &IkikHandlerRuntimeWiring{}
}

func ProvideUserAccountHandler(
	accountService *service.AccountService,
	accountUsageService *service.AccountUsageService,
	accountTestService *service.AccountTestService,
	rateLimitService *service.RateLimitService,
	openAIQuotaService *service.OpenAIQuotaService,
	oauthService *service.OAuthService,
	openaiOAuthService *service.OpenAIOAuthService,
	geminiOAuthService *service.GeminiOAuthService,
	antigravityOAuthService *service.AntigravityOAuthService,
	grokOAuthService *service.GrokOAuthService,
	kiroOAuthService *service.KiroOAuthService,
	accountBatchTaskService *service.AccountBatchTaskService,
	carpoolService *service.CarpoolService,
	settingService *service.SettingService,
	ollamaCloudUsage *service.OllamaCloudUsageService,
) *UserAccountHandler {
	h := NewUserAccountHandler(
		accountService,
		accountUsageService,
		accountTestService,
		oauthService,
		openaiOAuthService,
		geminiOAuthService,
		antigravityOAuthService,
		accountBatchTaskService,
	)
	h.SetCarpoolService(carpoolService)
	h.SetSettingService(settingService)
	h.SetGrokOAuthService(grokOAuthService)
	h.SetKiroOAuthService(kiroOAuthService)
	h.SetRateLimitService(rateLimitService)
	h.SetOpenAIQuotaService(openAIQuotaService)
	h.SetOllamaCloudUsageService(ollamaCloudUsage)
	return h
}

var IkikProviderSet = wire.NewSet(
	ProvideGatewayHookChain,
	ProvideIkikHandlerRuntimeWiring,
	ProvideUserAccountHandler,
	NewPlaygroundHandler,
	NewReceiptCodeHandler,
	NewWithdrawalHandler,
	NewShopHandler,
	NewPromptSubmissionHandler,
	admin.NewAccountSharePolicyHandler,
	admin.NewCarpoolHandler,
	admin.NewEmailBroadcastHandler,
	admin.NewKiroOAuthHandler,
	admin.NewRevenueHandler,
	admin.NewWithdrawalHandler,
	admin.NewShopHandler,
	admin.NewModuleHandler,
	admin.NewPromptSubmissionHandler,
)
