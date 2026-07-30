package service

import (
	"context"

	"github.com/redis/go-redis/v9"
	dbent "ikik-api/ent"
	"ikik-api/internal/config"
	"ikik-api/internal/payment"
	"ikik-api/internal/pkg/kirocooldown"
)

func ProvideAccountService(
	accountRepo AccountRepository,
	groupRepo GroupRepository,
	userRepo UserRepository,
	userSubRepo UserSubscriptionRepository,
	accountSharePolicyRepo AccountSharePolicyRepository,
	privateGroupProvisioner UserPrivateGroupProvisioner,
	proxyRepo ProxyRepository,
	proxyProber ProxyExitInfoProber,
) *AccountService {
	svc := NewAccountService(accountRepo, groupRepo, userRepo, userSubRepo)
	svc.SetAccountSharePolicyRepository(accountSharePolicyRepo)
	svc.SetUserPrivateGroupProvisioner(privateGroupProvisioner)
	svc.SetProxyRepository(proxyRepo)
	svc.SetProxyProber(proxyProber)
	return svc
}

func ProvideAdminService(
	userRepo UserRepository,
	groupRepo AdminGroupRepository,
	accountRepo AdminAccountRepository,
	proxyRepo ProxyRepository,
	apiKeyRepo APIKeyRepository,
	redeemCodeRepo RedeemCodeRepository,
	userGroupRateRepo UserGroupRateRepository,
	userRPMCache UserRPMCache,
	billingCacheService *BillingCacheService,
	proxyProber ProxyExitInfoProber,
	proxyLatencyCache ProxyLatencyCache,
	authCacheInvalidator APIKeyAuthCacheInvalidator,
	entClient *dbent.Client,
	settingService *SettingService,
	defaultSubAssigner DefaultSubscriptionAssigner,
	userSubRepo UserSubscriptionRepository,
	privacyClientFactory PrivacyClientFactory,
	runtimeBlocker AccountRuntimeBlocker,
	affiliateService *AffiliateService,
	privateGroupProvisioner UserPrivateGroupProvisioner,
	compositeRouteRepo CompositeModelRouteRepository,
	compositeResolver *CompositeRouteResolver,
) AdminService {
	svc := NewAdminService(
		userRepo,
		groupRepo,
		accountRepo,
		proxyRepo,
		apiKeyRepo,
		redeemCodeRepo,
		userGroupRateRepo,
		userRPMCache,
		billingCacheService,
		proxyProber,
		proxyLatencyCache,
		authCacheInvalidator,
		entClient,
		settingService,
		defaultSubAssigner,
		userSubRepo,
		privacyClientFactory,
		runtimeBlocker,
		affiliateService,
		compositeRouteRepo,
		compositeResolver,
	)
	return SetAdminUserPrivateGroupProvisioner(svc, privateGroupProvisioner)
}
func ProvideAuthService(
	entClient *dbent.Client,
	userRepo UserRepository,
	redeemRepo RedeemCodeRepository,
	refreshTokenCache RefreshTokenCache,
	cfg *config.Config,
	settingService *SettingService,
	emailService *EmailService,
	turnstileService *TurnstileService,
	emailQueueService *EmailQueueService,
	promoService *PromoService,
	defaultSubAssigner DefaultSubscriptionAssigner,
	affiliateService *AffiliateService,
	userPlatformQuotaRepo UserPlatformQuotaRepository,
	privateGroupProvisioner UserPrivateGroupProvisioner,
) *AuthService {
	svc := NewAuthService(
		entClient,
		userRepo,
		redeemRepo,
		refreshTokenCache,
		cfg,
		settingService,
		emailService,
		turnstileService,
		emailQueueService,
		promoService,
		defaultSubAssigner,
		affiliateService,
		userPlatformQuotaRepo,
	)
	svc.SetUserPrivateGroupProvisioner(privateGroupProvisioner)
	return svc
}

func ProvideCarpoolService(
	repo CarpoolRepository,
	groupRepo GroupRepository,
	accountRepo AccountRepository,
	proxyRepo ProxyRepository,
	userRepo UserRepository,
	userSubRepo UserSubscriptionRepository,
	subscriptionService *SubscriptionService,
	settingService *SettingService,
	billingCacheService *BillingCacheService,
	authCacheInvalidator APIKeyAuthCacheInvalidator,
	accountUsageService *AccountUsageService,
	emailService *EmailService,
	rateLimitService *RateLimitService,
) *CarpoolService {
	return NewCarpoolService(
		repo,
		groupRepo,
		accountRepo,
		proxyRepo,
		userRepo,
		userSubRepo,
		subscriptionService,
		settingService,
		billingCacheService,
		authCacheInvalidator,
		accountUsageService,
		emailService,
		rateLimitService,
	)
}
func ProvideEmailBroadcastService(
	repo EmailBroadcastRepository,
	userRepo UserRepository,
	emailService *EmailService,
	settingRepo SettingRepository,
) *EmailBroadcastService {
	svc := NewEmailBroadcastService(repo, userRepo, emailService, settingRepo)
	svc.FailInterruptedBroadcasts(context.Background())
	return svc
}

// ProvideGroupRateScheduleService creates and starts the group rate schedule worker.
func ProvideGroupRateScheduleService(
	repo GroupRateScheduleRepository,
	groupRepo GroupRepository,
	authCacheInvalidator APIKeyAuthCacheInvalidator,
) *GroupRateScheduleService {
	svc := NewGroupRateScheduleService(repo, groupRepo, authCacheInvalidator, defaultGroupRateScheduleInterval)
	svc.Start()
	return svc
}
func ProvideKiroCooldownStore(redisClient *redis.Client) KiroCooldownStore {
	return kirocooldown.NewStore(redisClient)
}
func ProvideKiroTokenProvider(
	accountRepo AccountRepository,
	tokenCache GeminiTokenCache,
	kiroOAuthService *KiroOAuthService,
	refreshAPI *OAuthRefreshAPI,
) *KiroTokenProvider {
	p := NewKiroTokenProvider(accountRepo, tokenCache, kiroOAuthService)
	executor := NewKiroTokenRefresher(kiroOAuthService)
	p.SetRefreshAPI(refreshAPI, executor)
	p.SetRefreshPolicy(GeminiProviderRefreshPolicy())
	return p
}

func ProvideShopService(
	entClient *dbent.Client,
	paymentService *PaymentService,
	authCacheInvalidator APIKeyAuthCacheInvalidator,
	billingCacheService *BillingCacheService,
	settingRepo SettingRepository,
	key payment.EncryptionKey,
	fileCardStoreFactory ShopFileCardObjectStoreFactory,
) *ShopService {
	svc := NewShopService(
		entClient,
		paymentService,
		authCacheInvalidator,
		billingCacheService,
		WithShopSettingRepository(settingRepo),
		WithShopEncryptionKey([]byte(key)),
		WithShopFileCardObjectStoreFactory(fileCardStoreFactory),
	)
	paymentService.SetShopFulfillment(svc)
	return svc
}
func ProvideUsageService(
	usageRepo UsageLogRepository,
	userRepo UserRepository,
	client *dbent.Client,
	authCacheInvalidator APIKeyAuthCacheInvalidator,
	settingRepo SettingRepository,
	groupRepo GroupRepository,
) *UsageService {
	svc := NewUsageService(usageRepo, userRepo, client, authCacheInvalidator)
	svc.SetSettingRepository(settingRepo)
	svc.SetHomeStatsGroupReader(groupRepo)
	return svc
}
