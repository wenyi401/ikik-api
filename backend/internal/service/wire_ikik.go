package service

import "github.com/google/wire"

// IkikProviderSet contains product extensions that are intentionally kept out
// of the upstream service provider list.
var IkikProviderSet = wire.NewSet(
	ProvideAuthService,
	ProvideAccountService,
	ProvideAdminService,
	ProvideUsageService,
	NewAccountBatchTaskService,
	NewAccountSharePolicyService,
	ProvideCarpoolService,
	ProvideEmailBroadcastService,
	ProvideGroupRateScheduleService,
	NewKiroOAuthService,
	NewPromptLibraryTranslationService,
	NewPromptSubmissionService,
	ProvideKiroCooldownStore,
	ProvideKiroTokenProvider,
	NewUserPrivateGroupService,
	NewRevenueService,
	ProvideReceiptCodeStorageConfigProvider,
	NewReceiptCodeService,
	NewWithdrawalService,
	ProvideShopService,
	ProvideIkikRuntimeWiring,
)

func ProvideReceiptCodeStorageConfigProvider(service *PaymentConfigService) ReceiptCodeStorageConfigProvider {
	return service
}
