package repository

import "github.com/google/wire"

// IkikProviderSet keeps product-specific repositories out of the upstream
// provider list so upstream Wire updates remain easy to review.
var IkikProviderSet = wire.NewSet(
	NewAccountBatchTaskRepository,
	NewAccountSharePolicyRepository,
	NewCarpoolRepository,
	NewEmailBroadcastRepository,
	NewGroupRateScheduleRepository,
	NewReceiptCodeObjectStoreFactory,
	NewReceiptCodeRepository,
	NewShopFileCardObjectStoreFactory,
	NewWithdrawalRepository,
)
