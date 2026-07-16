package service

import (
	"archive/zip"
	"bytes"
	"context"
	"database/sql"
	"errors"
	"io"
	"strconv"
	"testing"
	"time"

	entsql "entgo.io/ent/dialect/sql"
	"github.com/stretchr/testify/require"
	dbent "ikik-api/ent"
	"ikik-api/ent/shopbalanceledger"
	"ikik-api/internal/payment"
	infraerrors "ikik-api/internal/pkg/errors"
)

func TestShopPlatformFulfillmentUsesReservedCards(t *testing.T) {
	ctx := context.Background()
	client := newPaymentConfigServiceTestClient(t)
	svc := NewShopService(client, nil, nil, nil)
	user := createShopTestUser(t, ctx, client, "reserved@example.com")
	product := createShopTestProduct(t, ctx, client, "Reserved product")
	cardA := createShopTestCard(t, ctx, client, product.ID, "CARD-A")
	cardB := createShopTestCard(t, ctx, client, product.ID, "CARD-B")
	order := createShopTestOrder(t, ctx, client, user.ID, product.ID, "platform", ShopOrderStatusPending, 1)
	paymentOrder := createShopTestPaymentOrder(t, ctx, client, user.ID, order.ID, order.TotalAmount, payment.OrderStatusPaid)
	order, err := client.ShopOrder.UpdateOneID(order.ID).SetPaymentOrderID(paymentOrder.ID).Save(ctx)
	require.NoError(t, err)
	now := time.Now()
	cardA, err = client.ShopCardKey.UpdateOneID(cardA.ID).
		SetStatus(ShopCardStatusLocked).
		SetOrderID(order.ID).
		SetLockedAt(now).
		SetLockedUntil(now.Add(30 * time.Minute)).
		Save(ctx)
	require.NoError(t, err)

	require.NoError(t, svc.ConfirmPaidAndDeliver(ctx, paymentOrder.ID))

	cardA, err = client.ShopCardKey.Get(ctx, cardA.ID)
	require.NoError(t, err)
	cardB, err = client.ShopCardKey.Get(ctx, cardB.ID)
	require.NoError(t, err)
	require.Equal(t, ShopCardStatusSold, cardA.Status)
	require.Equal(t, ShopCardStatusAvailable, cardB.Status)
	require.Nil(t, cardA.LockedAt)
	require.Nil(t, cardA.LockedUntil)
	fulfilled, err := client.ShopOrder.Get(ctx, order.ID)
	require.NoError(t, err)
	require.Equal(t, ShopOrderStatusCompleted, fulfilled.Status)
	require.Equal(t, []string{"CARD-A"}, fulfilled.DeliveredCards)
}

func TestShopCancelKeepsReservationUntilGraceCleanup(t *testing.T) {
	ctx := context.Background()
	client := newPaymentConfigServiceTestClient(t)
	svc := NewShopService(client, nil, nil, nil)
	user := createShopTestUser(t, ctx, client, "cancel@example.com")
	product := createShopTestProduct(t, ctx, client, "Cancel product")
	card := createShopTestCard(t, ctx, client, product.ID, "CARD-C")
	order := createShopTestOrder(t, ctx, client, user.ID, product.ID, "platform", ShopOrderStatusPending, 1)
	paymentOrder := createShopTestPaymentOrder(t, ctx, client, user.ID, order.ID, order.TotalAmount, payment.OrderStatusPending)
	order, err := client.ShopOrder.UpdateOneID(order.ID).SetPaymentOrderID(paymentOrder.ID).Save(ctx)
	require.NoError(t, err)
	lockedUntil := time.Now().Add(-10 * time.Minute)
	card, err = client.ShopCardKey.UpdateOneID(card.ID).
		SetStatus(ShopCardStatusLocked).
		SetOrderID(order.ID).
		SetLockedAt(lockedUntil.Add(-30 * time.Minute)).
		SetLockedUntil(lockedUntil).
		Save(ctx)
	require.NoError(t, err)

	require.NoError(t, svc.CancelPendingPayment(ctx, paymentOrder.ID, ShopOrderStatusCancelled))
	card, err = client.ShopCardKey.Get(ctx, card.ID)
	require.NoError(t, err)
	require.Equal(t, ShopCardStatusLocked, card.Status)
	require.NotNil(t, card.OrderID)

	require.NoError(t, svc.ReleaseStalePaymentReservations(ctx, time.Now()))
	card, err = client.ShopCardKey.Get(ctx, card.ID)
	require.NoError(t, err)
	require.Equal(t, ShopCardStatusAvailable, card.Status)
	require.Nil(t, card.OrderID)
	require.Nil(t, card.LockedAt)
	require.Nil(t, card.LockedUntil)
}

func TestShopFulfillmentRejectsPaymentUserMismatch(t *testing.T) {
	ctx := context.Background()
	client := newPaymentConfigServiceTestClient(t)
	svc := NewShopService(client, nil, nil, nil)
	user := createShopTestUser(t, ctx, client, "buyer@example.com")
	other := createShopTestUser(t, ctx, client, "other@example.com")
	product := createShopTestProduct(t, ctx, client, "Mismatch product")
	createShopTestCard(t, ctx, client, product.ID, "CARD-D")
	order := createShopTestOrder(t, ctx, client, user.ID, product.ID, "platform", ShopOrderStatusPending, 1)
	paymentOrder := createShopTestPaymentOrder(t, ctx, client, other.ID, order.ID, order.TotalAmount, payment.OrderStatusPaid)
	_, err := client.ShopOrder.UpdateOneID(order.ID).SetPaymentOrderID(paymentOrder.ID).Save(ctx)
	require.NoError(t, err)

	err = svc.ConfirmPaidAndDeliver(ctx, paymentOrder.ID)
	require.Error(t, err)
	require.Equal(t, "SHOP_PAYMENT_USER_MISMATCH", errorCodeForTest(err))
}

func TestAdminCannotUpdateOrDeleteLockedCardKey(t *testing.T) {
	ctx := context.Background()
	client := newPaymentConfigServiceTestClient(t)
	svc := NewShopService(client, nil, nil, nil)
	user := createShopTestUser(t, ctx, client, "locked-admin@example.com")
	product := createShopTestProduct(t, ctx, client, "Locked product")
	card := createShopTestCard(t, ctx, client, product.ID, "CARD-E")
	order := createShopTestOrder(t, ctx, client, user.ID, product.ID, "platform", ShopOrderStatusPending, 1)
	card, err := client.ShopCardKey.UpdateOneID(card.ID).
		SetStatus(ShopCardStatusLocked).
		SetOrderID(order.ID).
		SetLockedAt(time.Now()).
		SetLockedUntil(time.Now().Add(30 * time.Minute)).
		Save(ctx)
	require.NoError(t, err)

	newContent := "CARD-E-EDITED"
	_, err = svc.AdminUpdateCardKey(ctx, card.ID, ShopUpdateCardKeyRequest{Content: &newContent})
	require.Error(t, err)
	require.Equal(t, "SHOP_CARD_KEY_ALREADY_ASSIGNED", errorCodeForTest(err))

	err = svc.AdminDeleteCardKey(ctx, card.ID)
	require.Error(t, err)
	require.Equal(t, "SHOP_CARD_KEY_ALREADY_ASSIGNED", errorCodeForTest(err))
}

func TestShopBalanceDrawCompletesGuaranteedRewardCycle(t *testing.T) {
	ctx := context.Background()
	client := newPaymentConfigServiceTestClient(t)
	svc := NewShopService(client, nil, nil, nil)
	user := createShopTestUser(t, ctx, client, "draw-cycle@example.com")
	user, err := client.User.UpdateOneID(user.ID).SetBalance(60).Save(ctx)
	require.NoError(t, err)
	product := createShopTestBalanceDrawProduct(t, ctx, client, "Balance draw product")

	var totalReward float64
	for i := 0; i < 20; i++ {
		order, err := svc.CreateOrder(ctx, ShopCreateOrderRequest{
			UserID:        user.ID,
			ProductID:     product.ID,
			Quantity:      1,
			PaymentMethod: ShopPaymentMethodBalance,
		})
		require.NoError(t, err)
		require.Equal(t, ShopOrderStatusCompleted, order.Status)
		require.NotNil(t, order.DrawRewardAmount)
		require.NotNil(t, order.DrawCycleID)
		require.NotNil(t, order.DrawCycleIndex)
		require.GreaterOrEqual(t, *order.DrawRewardAmount, 1.0)
		require.LessOrEqual(t, *order.DrawRewardAmount, 5.0)
		require.Equal(t, i+1, *order.DrawCycleIndex)
		ledger, err := client.ShopBalanceLedger.Query().
			Where(shopbalanceledger.ShopOrderIDEQ(order.ID)).
			Only(ctx)
		require.NoError(t, err)
		require.Equal(t, ShopBalanceLedgerEntryNet, ledger.EntryType)
		require.Equal(t, 3.0, normalizeShopAmount(ledger.DebitAmount))
		require.Equal(t, normalizeShopAmount(*order.DrawRewardAmount), normalizeShopAmount(ledger.CreditAmount))
		require.Equal(t, *order.DrawCycleID, *ledger.DrawCycleID)
		require.Equal(t, *order.DrawCycleIndex, *ledger.DrawCycleIndex)
		require.Equal(t, normalizeShopAmount(ledger.BalanceBefore-ledger.DebitAmount+ledger.CreditAmount), normalizeShopAmount(ledger.BalanceAfter))
		totalReward = normalizeShopAmount(totalReward + *order.DrawRewardAmount)
	}

	require.Equal(t, 60.0, totalReward)
	user, err = client.User.Get(ctx, user.ID)
	require.NoError(t, err)
	require.Equal(t, 60.0, normalizeShopAmount(user.Balance))
	cycle, err := client.ShopDrawCycle.Query().Only(ctx)
	require.NoError(t, err)
	require.True(t, cycle.Completed)
	require.Equal(t, 20, cycle.DrawnCount)
	require.Equal(t, 60.0, normalizeShopAmount(cycle.DrawnAmount))
	require.Empty(t, cycle.RemainingAmounts)
	ledgerCount, err := client.ShopBalanceLedger.Query().Count(ctx)
	require.NoError(t, err)
	require.Equal(t, 20, ledgerCount)
}

func TestShopBalanceDrawBlocksEconomicsUpdateWhenCycleActive(t *testing.T) {
	ctx := context.Background()
	client := newPaymentConfigServiceTestClient(t)
	svc := NewShopService(client, nil, nil, nil)
	user := createShopTestUser(t, ctx, client, "draw-lock@example.com")
	_, err := client.User.UpdateOneID(user.ID).SetBalance(60).Save(ctx)
	require.NoError(t, err)
	product := createShopTestBalanceDrawProduct(t, ctx, client, "Locked draw product")

	_, err = svc.CreateOrder(ctx, ShopCreateOrderRequest{
		UserID:        user.ID,
		ProductID:     product.ID,
		Quantity:      1,
		PaymentMethod: ShopPaymentMethodBalance,
	})
	require.NoError(t, err)
	newPrice := 4.0
	_, err = svc.AdminUpdateProduct(ctx, product.ID, ShopUpdateProductRequest{Price: &newPrice})
	require.Error(t, err)
	require.Equal(t, "SHOP_DRAW_CYCLE_ACTIVE", errorCodeForTest(err))
}

func TestShopPointsPaymentBalanceDrawCreditsBalanceAndDeductsPoints(t *testing.T) {
	ctx := context.Background()
	client := newPaymentConfigServiceTestClient(t)
	svc := NewShopService(client, nil, nil, nil)
	user := createShopTestUser(t, ctx, client, "points-balance-draw@example.com")
	user, err := client.User.UpdateOneID(user.ID).SetPointsBalance(10).Save(ctx)
	require.NoError(t, err)
	product := createShopTestBalanceDrawProduct(t, ctx, client, "Points pay balance draw product")
	product, err = client.ShopProduct.UpdateOneID(product.ID).SetAllowPointsPayment(true).Save(ctx)
	require.NoError(t, err)

	order, err := svc.CreateOrder(ctx, ShopCreateOrderRequest{
		UserID:        user.ID,
		ProductID:     product.ID,
		Quantity:      1,
		PaymentMethod: ShopPaymentMethodPoints,
	})
	require.NoError(t, err)
	require.Equal(t, ShopOrderStatusCompleted, order.Status)
	require.Equal(t, ShopPaymentMethodPoints, order.PaymentMethod)
	require.Equal(t, ShopProductTypeBalanceDraw, order.ProductType)
	require.Equal(t, "balance", order.DrawRewardType)
	require.Equal(t, product.Price, normalizeShopAmount(order.PointsAmount))
	require.NotNil(t, order.DrawRewardAmount)
	require.NotNil(t, order.DrawCycleID)
	require.NotNil(t, order.DrawCycleIndex)

	user, err = client.User.Get(ctx, user.ID)
	require.NoError(t, err)
	require.Equal(t, 7.0, normalizeShopAmount(user.PointsBalance))
	require.Equal(t, normalizeShopAmount(*order.DrawRewardAmount), normalizeShopAmount(user.Balance))
	ledger, err := client.ShopBalanceLedger.Query().
		Where(shopbalanceledger.ShopOrderIDEQ(order.ID)).
		Only(ctx)
	require.NoError(t, err)
	require.Equal(t, 0.0, normalizeShopAmount(ledger.DebitAmount))
	require.Equal(t, normalizeShopAmount(*order.DrawRewardAmount), normalizeShopAmount(ledger.CreditAmount))
	require.Equal(t, normalizeShopAmount(*order.DrawRewardAmount), normalizeShopAmount(ledger.BalanceAfter))
	require.Equal(t, 1, countShopTestPointsLedger(t, ctx, client, order.ID, "debit", "shop_order"))
	require.Equal(t, 0, countShopTestPointsLedger(t, ctx, client, order.ID, "credit", "shop_draw_reward"))
}

func TestShopProductPaymentMethodToggles(t *testing.T) {
	ctx := context.Background()
	client := newPaymentConfigServiceTestClient(t)
	svc := NewShopService(client, nil, nil, nil)
	user := createShopTestUser(t, ctx, client, "shop-methods@example.com")
	_, err := client.User.UpdateOneID(user.ID).SetBalance(100).SetPointsBalance(100).Save(ctx)
	require.NoError(t, err)
	product := createShopTestProduct(t, ctx, client, "Method toggle product")
	createShopTestCard(t, ctx, client, product.ID, "METHOD-CARD-1")

	_, err = client.ShopProduct.UpdateOneID(product.ID).
		SetAllowBalancePayment(false).
		SetAllowPointsPayment(true).
		SetAllowPlatformPayment(false).
		Save(ctx)
	require.NoError(t, err)

	_, err = svc.CreateOrder(ctx, ShopCreateOrderRequest{
		UserID:        user.ID,
		ProductID:     product.ID,
		Quantity:      1,
		PaymentMethod: ShopPaymentMethodBalance,
	})
	require.Error(t, err)
	require.Equal(t, "SHOP_UNSUPPORTED_PAYMENT_METHOD", errorCodeForTest(err))

	order, err := svc.CreateOrder(ctx, ShopCreateOrderRequest{
		UserID:        user.ID,
		ProductID:     product.ID,
		Quantity:      1,
		PaymentMethod: ShopPaymentMethodPoints,
	})
	require.NoError(t, err)
	require.Equal(t, ShopOrderStatusCompleted, order.Status)
	require.Equal(t, ShopPaymentMethodPoints, order.PaymentMethod)
}

func TestAdminProductRejectsNoPaymentMethods(t *testing.T) {
	ctx := context.Background()
	client := newPaymentConfigServiceTestClient(t)
	svc := NewShopService(client, nil, nil, nil)
	allowBalance := false
	allowPoints := false
	allowPlatform := false

	_, err := svc.AdminCreateProduct(ctx, ShopCreateProductRequest{
		Name:                 "No payment methods",
		Price:                1,
		MinPurchase:          1,
		MaxPurchase:          1,
		ProductType:          ShopProductTypeCardKey,
		AllowBalancePayment:  &allowBalance,
		AllowPointsPayment:   &allowPoints,
		AllowPlatformPayment: &allowPlatform,
	})
	require.Error(t, err)
	require.Equal(t, "SHOP_UNSUPPORTED_PAYMENT_METHOD", errorCodeForTest(err))
}

func TestShopPointsDrawCreditsPointsReward(t *testing.T) {
	ctx := context.Background()
	client := newPaymentConfigServiceTestClient(t)
	svc := NewShopService(client, nil, nil, nil)
	user := createShopTestUser(t, ctx, client, "points-draw@example.com")
	user, err := client.User.UpdateOneID(user.ID).SetBalance(10).SetPointsBalance(10).Save(ctx)
	require.NoError(t, err)
	product := createShopTestPointsDrawProduct(t, ctx, client, "Points draw product")

	balanceOrder, err := svc.CreateOrder(ctx, ShopCreateOrderRequest{
		UserID:        user.ID,
		ProductID:     product.ID,
		Quantity:      1,
		PaymentMethod: ShopPaymentMethodBalance,
	})
	require.NoError(t, err)
	require.Equal(t, ShopOrderStatusCompleted, balanceOrder.Status)
	require.Equal(t, ShopProductTypePointsDraw, balanceOrder.ProductType)
	require.Equal(t, "points", balanceOrder.DrawRewardType)
	require.NotNil(t, balanceOrder.DrawRewardAmount)
	user, err = client.User.Get(ctx, user.ID)
	require.NoError(t, err)
	require.Equal(t, 7.0, normalizeShopAmount(user.Balance))
	require.Equal(t, normalizeShopAmount(10+*balanceOrder.DrawRewardAmount), normalizeShopAmount(user.PointsBalance))
	ledger, err := client.ShopBalanceLedger.Query().
		Where(shopbalanceledger.ShopOrderIDEQ(balanceOrder.ID)).
		Only(ctx)
	require.NoError(t, err)
	require.Equal(t, product.Price, normalizeShopAmount(ledger.DebitAmount))
	require.Equal(t, 0.0, normalizeShopAmount(ledger.CreditAmount))
	require.Equal(t, 1, countShopTestPointsLedger(t, ctx, client, balanceOrder.ID, "credit", "shop_draw_reward"))

	pointsOrder, err := svc.CreateOrder(ctx, ShopCreateOrderRequest{
		UserID:        user.ID,
		ProductID:     product.ID,
		Quantity:      1,
		PaymentMethod: ShopPaymentMethodPoints,
	})
	require.NoError(t, err)
	require.Equal(t, ShopOrderStatusCompleted, pointsOrder.Status)
	require.Equal(t, ShopProductTypePointsDraw, pointsOrder.ProductType)
	require.Equal(t, "points", pointsOrder.DrawRewardType)
	require.NotNil(t, pointsOrder.DrawRewardAmount)
	user, err = client.User.Get(ctx, user.ID)
	require.NoError(t, err)
	expectedPoints := 10 + *balanceOrder.DrawRewardAmount - product.Price + *pointsOrder.DrawRewardAmount
	require.Equal(t, normalizeShopAmount(expectedPoints), normalizeShopAmount(user.PointsBalance))
	require.Equal(t, 7.0, normalizeShopAmount(user.Balance))
	require.Equal(t, 0, countShopTestBalanceLedger(t, ctx, client, pointsOrder.ID))
	require.Equal(t, 1, countShopTestPointsLedger(t, ctx, client, pointsOrder.ID, "debit", "shop_order"))
	require.Equal(t, 1, countShopTestPointsLedger(t, ctx, client, pointsOrder.ID, "credit", "shop_draw_reward"))
}

func TestShopPlatformFulfillmentReallocatesAvailableCardAfterReservationReleased(t *testing.T) {
	ctx := context.Background()
	client := newPaymentConfigServiceTestClient(t)
	svc := NewShopService(client, nil, nil, nil)
	user := createShopTestUser(t, ctx, client, "released@example.com")
	other := createShopTestUser(t, ctx, client, "released-other@example.com")
	product := createShopTestProduct(t, ctx, client, "Released reservation product")
	oldCard := createShopTestCard(t, ctx, client, product.ID, "CARD-F")
	newCard := createShopTestCard(t, ctx, client, product.ID, "CARD-G")
	otherOrder := createShopTestOrder(t, ctx, client, other.ID, product.ID, "platform", ShopOrderStatusCompleted, 1)
	oldCard, err := client.ShopCardKey.UpdateOneID(oldCard.ID).
		SetStatus(ShopCardStatusSold).
		SetOrderID(otherOrder.ID).
		SetSoldAt(time.Now()).
		Save(ctx)
	require.NoError(t, err)
	order := createShopTestOrder(t, ctx, client, user.ID, product.ID, "platform", ShopOrderStatusFailed, 1)
	paymentOrder := createShopTestPaymentOrder(t, ctx, client, user.ID, order.ID, order.TotalAmount, payment.OrderStatusPaid)
	_, err = client.ShopOrder.UpdateOneID(order.ID).
		SetPaymentOrderID(paymentOrder.ID).
		SetFailedReason("reservation released").
		Save(ctx)
	require.NoError(t, err)

	require.NoError(t, svc.ConfirmPaidAndDeliver(ctx, paymentOrder.ID))

	oldCard, err = client.ShopCardKey.Get(ctx, oldCard.ID)
	require.NoError(t, err)
	require.Equal(t, ShopCardStatusSold, oldCard.Status)
	require.NotNil(t, oldCard.OrderID)
	require.Equal(t, otherOrder.ID, *oldCard.OrderID)
	newCard, err = client.ShopCardKey.Get(ctx, newCard.ID)
	require.NoError(t, err)
	require.Equal(t, ShopCardStatusSold, newCard.Status)
	require.NotNil(t, newCard.OrderID)
	require.Equal(t, order.ID, *newCard.OrderID)
	fulfilled, err := client.ShopOrder.Get(ctx, order.ID)
	require.NoError(t, err)
	require.Equal(t, ShopOrderStatusCompleted, fulfilled.Status)
	require.Equal(t, []string{"CARD-G"}, fulfilled.DeliveredCards)
}

func TestAdminImportFileCardKeysRollsBackBatchOnUploadFailure(t *testing.T) {
	ctx := context.Background()
	client := newPaymentConfigServiceTestClient(t)
	ensureShopFileCardTestColumns(t, ctx, client)
	product := createShopTestProduct(t, ctx, client, "File import rollback product")
	store := newMemoryShopFileCardStore()
	store.failUploadAt = 2
	svc := NewShopService(
		client,
		nil,
		nil,
		nil,
		WithShopSettingRepository(newShopFileCardTestSettingRepo()),
		WithShopFileCardObjectStoreFactory(func(context.Context, ShopFileCardStorageConfig) (ShopFileCardObjectStore, error) {
			return store, nil
		}),
	)

	_, err := svc.AdminImportFileCardKeys(ctx, product.ID, []ShopFileCardUpload{
		{Filename: "first.txt", ContentType: "text/plain", Reader: bytes.NewReader([]byte("first card"))},
		{Filename: "second.txt", ContentType: "text/plain", Reader: bytes.NewReader([]byte("second card"))},
	})
	require.Error(t, err)
	require.Empty(t, store.objects)
	require.Len(t, store.deleted, 1)
	count, err := client.ShopCardKey.Query().Count(ctx)
	require.NoError(t, err)
	require.Zero(t, count)
}

func TestAdminDeleteFileCardKeyDeletesObjectAfterDatabaseCommit(t *testing.T) {
	ctx := context.Background()
	client := newPaymentConfigServiceTestClient(t)
	ensureShopFileCardTestColumns(t, ctx, client)
	product := createShopTestProduct(t, ctx, client, "File delete product")
	card := createShopTestFileCard(t, ctx, client, product.ID, "cards/delete-me.txt")
	store := newMemoryShopFileCardStore()
	store.objects["cards/delete-me.txt"] = []byte("file body")
	store.deleteHook = func(key string) error {
		_, err := client.ShopCardKey.Get(ctx, card.ID)
		if err == nil {
			return errors.New("file object deleted before database row")
		}
		if !dbent.IsNotFound(err) {
			return err
		}
		return nil
	}
	svc := NewShopService(
		client,
		nil,
		nil,
		nil,
		WithShopSettingRepository(newShopFileCardTestSettingRepo()),
		WithShopFileCardObjectStoreFactory(func(context.Context, ShopFileCardStorageConfig) (ShopFileCardObjectStore, error) {
			return store, nil
		}),
	)

	require.NoError(t, svc.AdminDeleteCardKey(ctx, card.ID))
	_, err := client.ShopCardKey.Get(ctx, card.ID)
	require.True(t, dbent.IsNotFound(err))
	require.Equal(t, []string{"cards/delete-me.txt"}, store.deleted)
	require.Empty(t, store.objects)
}

func TestWriteOrderFileCardArchiveStreamsDeliveredFiles(t *testing.T) {
	ctx := context.Background()
	client := newPaymentConfigServiceTestClient(t)
	ensureShopFileCardTestColumns(t, ctx, client)
	user := createShopTestUser(t, ctx, client, "file-archive@example.com")
	product := createShopTestProduct(t, ctx, client, "File archive product")
	order := createShopTestOrder(t, ctx, client, user.ID, product.ID, ShopPaymentMethodBalance, ShopOrderStatusCompleted, 2)
	cardA := createShopTestFileCard(t, ctx, client, product.ID, "cards/a.txt")
	cardB := createShopTestFileCard(t, ctx, client, product.ID, "cards/b.txt")
	now := time.Now()
	for _, card := range []*dbent.ShopCardKey{cardA, cardB} {
		_, err := client.ShopCardKey.UpdateOneID(card.ID).
			SetStatus(ShopCardStatusSold).
			SetOrderID(order.ID).
			SetSoldAt(now).
			Save(ctx)
		require.NoError(t, err)
	}
	store := newMemoryShopFileCardStore()
	store.objects["cards/a.txt"] = []byte("alpha")
	store.objects["cards/b.txt"] = []byte("beta")
	svc := NewShopService(
		client,
		nil,
		nil,
		nil,
		WithShopSettingRepository(newShopFileCardTestSettingRepo()),
		WithShopFileCardObjectStoreFactory(func(context.Context, ShopFileCardStorageConfig) (ShopFileCardObjectStore, error) {
			return store, nil
		}),
	)

	var buf bytes.Buffer
	filename, err := svc.WriteOrderFileCardArchive(ctx, user.ID, order.ID, &buf)
	require.NoError(t, err)
	require.Equal(t, "shop-order-"+strconv.FormatInt(order.ID, 10)+"-files.zip", filename)
	zr, err := zip.NewReader(bytes.NewReader(buf.Bytes()), int64(buf.Len()))
	require.NoError(t, err)
	require.Len(t, zr.File, 2)
	require.Equal(t, "file.txt", zr.File[0].Name)
	require.Equal(t, "file-2.txt", zr.File[1].Name)
}

func createShopTestUser(t *testing.T, ctx context.Context, client *dbent.Client, email string) *dbent.User {
	t.Helper()
	user, err := client.User.Create().
		SetEmail(email).
		SetPasswordHash("hash").
		SetUsername(email).
		Save(ctx)
	require.NoError(t, err)
	return user
}

func createShopTestProduct(t *testing.T, ctx context.Context, client *dbent.Client, name string) *dbent.ShopProduct {
	t.Helper()
	product, err := client.ShopProduct.Create().
		SetName(name).
		SetPrice(12.34).
		SetEnabled(true).
		SetMinPurchase(1).
		SetMaxPurchase(10).
		SetAutoDelivery(true).
		Save(ctx)
	require.NoError(t, err)
	return product
}

func createShopTestBalanceDrawProduct(t *testing.T, ctx context.Context, client *dbent.Client, name string) *dbent.ShopProduct {
	t.Helper()
	product, err := client.ShopProduct.Create().
		SetName(name).
		SetPrice(3).
		SetEnabled(true).
		SetMinPurchase(1).
		SetMaxPurchase(1).
		SetAutoDelivery(true).
		SetProductType(ShopProductTypeBalanceDraw).
		SetBalanceOnly(true).
		SetAllowBalancePayment(true).
		SetAllowPlatformPayment(false).
		SetDrawEnabled(true).
		SetDrawMinAmount(1).
		SetDrawMaxAmount(5).
		SetDrawGuaranteeCount(20).
		SetDrawReturnRate(1).
		Save(ctx)
	require.NoError(t, err)
	return product
}

func createShopTestPointsDrawProduct(t *testing.T, ctx context.Context, client *dbent.Client, name string) *dbent.ShopProduct {
	t.Helper()
	product, err := client.ShopProduct.Create().
		SetName(name).
		SetPrice(3).
		SetEnabled(true).
		SetMinPurchase(1).
		SetMaxPurchase(1).
		SetAutoDelivery(true).
		SetProductType(ShopProductTypePointsDraw).
		SetBalanceOnly(true).
		SetAllowBalancePayment(true).
		SetAllowPointsPayment(true).
		SetAllowPlatformPayment(false).
		SetDrawEnabled(true).
		SetDrawMinAmount(1).
		SetDrawMaxAmount(5).
		SetDrawGuaranteeCount(20).
		SetDrawReturnRate(1).
		Save(ctx)
	require.NoError(t, err)
	return product
}

func createShopTestCard(t *testing.T, ctx context.Context, client *dbent.Client, productID int64, content string) *dbent.ShopCardKey {
	t.Helper()
	card, err := client.ShopCardKey.Create().
		SetProductID(productID).
		SetContent(content).
		SetStatus(ShopCardStatusAvailable).
		Save(ctx)
	require.NoError(t, err)
	return card
}

func createShopTestFileCard(t *testing.T, ctx context.Context, client *dbent.Client, productID int64, storageKey string) *dbent.ShopCardKey {
	t.Helper()
	card := createShopTestCard(t, ctx, client, productID, "file-placeholder")
	execShopTestSQL(t, ctx, client, `
		UPDATE shop_card_keys
		SET card_type = $1,
			storage_provider = $2,
			storage_key = $3,
			original_filename = $4,
			content_type = $5,
			byte_size = $6,
			sha256 = $7
		WHERE id = $8
	`, ShopCardTypeFile, ShopFileCardStorageProviderOSS, storageKey, "file.txt", "text/plain", 5, "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa", card.ID)
	return card
}

func ensureShopFileCardTestColumns(t *testing.T, ctx context.Context, client *dbent.Client) {
	t.Helper()
	statements := []string{
		"ALTER TABLE shop_card_keys ADD COLUMN card_type varchar(20) NOT NULL DEFAULT 'text'",
		"ALTER TABLE shop_card_keys ADD COLUMN storage_provider varchar(20)",
		"ALTER TABLE shop_card_keys ADD COLUMN storage_key text",
		"ALTER TABLE shop_card_keys ADD COLUMN original_filename varchar(255)",
		"ALTER TABLE shop_card_keys ADD COLUMN content_type varchar(120)",
		"ALTER TABLE shop_card_keys ADD COLUMN byte_size integer",
		"ALTER TABLE shop_card_keys ADD COLUMN sha256 varchar(64)",
	}
	for _, statement := range statements {
		execShopTestSQL(t, ctx, client, statement)
	}
}

func execShopTestSQL(t *testing.T, ctx context.Context, client *dbent.Client, query string, args ...any) {
	t.Helper()
	drv, ok := client.Driver().(*entsql.Driver)
	require.True(t, ok, "test client must use ent sql driver")
	_, err := drv.DB().ExecContext(ctx, query, args...)
	require.NoError(t, err)
}

func queryShopTestRow(t *testing.T, ctx context.Context, client *dbent.Client, query string, args ...any) *sql.Row {
	t.Helper()
	drv, ok := client.Driver().(*entsql.Driver)
	require.True(t, ok, "test client must use ent sql driver")
	return drv.DB().QueryRowContext(ctx, query, args...)
}

func countShopTestBalanceLedger(t *testing.T, ctx context.Context, client *dbent.Client, orderID int64) int {
	t.Helper()
	count, err := client.ShopBalanceLedger.Query().
		Where(shopbalanceledger.ShopOrderIDEQ(orderID)).
		Count(ctx)
	require.NoError(t, err)
	return count
}

func countShopTestPointsLedger(t *testing.T, ctx context.Context, client *dbent.Client, orderID int64, direction, reason string) int {
	t.Helper()
	var count int
	err := queryShopTestRow(t, ctx, client, `
		SELECT COUNT(*)
		FROM points_ledger
		WHERE ref_type = 'shop_order'
			AND ref_id = $1
			AND direction = $2
			AND reason = $3
	`, orderID, direction, reason).Scan(&count)
	require.NoError(t, err)
	return count
}

func newShopFileCardTestSettingRepo() *paymentConfigSettingRepoStub {
	return &paymentConfigSettingRepoStub{values: map[string]string{
		settingShopFileCardOSSEnabled:         "true",
		settingShopFileCardOSSEndpoint:        "https://oss.example.com",
		settingShopFileCardOSSRegion:          "oss-cn-hangzhou",
		settingShopFileCardOSSBucket:          "shop-file-cards",
		settingShopFileCardOSSAccessKeyID:     "access-key",
		settingShopFileCardOSSSecretAccessKey: `{"secret":"secret-key"}`,
		settingShopFileCardOSSPrefix:          "cards/",
		settingShopFileCardOSSForcePathStyle:  "false",
	}}
}

func TestShopFileCardStorageRequiresSecretWhenAccessKeyChanges(t *testing.T) {
	t.Parallel()

	repo := newShopFileCardTestSettingRepo()
	svc := NewShopService(nil, nil, nil, nil, WithShopSettingRepository(repo))
	newAccessKey := "new-access-key"
	emptySecret := ""

	_, err := svc.UpdateFileCardStorageConfig(context.Background(), UpdateShopFileCardStorageConfigRequest{
		AccessKeyID:     &newAccessKey,
		SecretAccessKey: &emptySecret,
	})
	require.Error(t, err)
	require.Equal(t, "SHOP_FILE_CARD_OSS_SECRET_REQUIRED_FOR_NEW_ACCESS_KEY", infraerrors.Reason(err))

	err = svc.TestFileCardStorageConfig(context.Background(), &UpdateShopFileCardStorageConfigRequest{
		AccessKeyID:     &newAccessKey,
		SecretAccessKey: &emptySecret,
	})
	require.Error(t, err)
	require.Equal(t, "SHOP_FILE_CARD_OSS_SECRET_REQUIRED_FOR_NEW_ACCESS_KEY", infraerrors.Reason(err))
}

type memoryShopFileCardStore struct {
	objects      map[string][]byte
	deleted      []string
	failUploadAt int
	uploadCount  int
	deleteHook   func(string) error
}

func newMemoryShopFileCardStore() *memoryShopFileCardStore {
	return &memoryShopFileCardStore{objects: map[string][]byte{}}
}

func (s *memoryShopFileCardStore) Upload(_ context.Context, key string, body io.Reader, _ string) error {
	s.uploadCount++
	if s.failUploadAt > 0 && s.uploadCount == s.failUploadAt {
		return errors.New("upload failed")
	}
	data, err := io.ReadAll(body)
	if err != nil {
		return err
	}
	s.objects[key] = data
	return nil
}

func (s *memoryShopFileCardStore) Download(_ context.Context, key string) (io.ReadCloser, error) {
	data, ok := s.objects[key]
	if !ok {
		return nil, errors.New("object not found")
	}
	return io.NopCloser(bytes.NewReader(data)), nil
}

func (s *memoryShopFileCardStore) Delete(_ context.Context, key string) error {
	if s.deleteHook != nil {
		if err := s.deleteHook(key); err != nil {
			return err
		}
	}
	s.deleted = append(s.deleted, key)
	delete(s.objects, key)
	return nil
}

func (s *memoryShopFileCardStore) HeadBucket(context.Context) error {
	return nil
}

func createShopTestOrder(t *testing.T, ctx context.Context, client *dbent.Client, userID, productID int64, paymentMethod, status string, quantity int) *dbent.ShopOrder {
	t.Helper()
	total := normalizeShopAmount(12.34 * float64(quantity))
	order, err := client.ShopOrder.Create().
		SetOrderNo("SHOPTEST" + generateRandomString(12)).
		SetUserID(userID).
		SetProductID(productID).
		SetProductName("test product").
		SetUnitPrice(12.34).
		SetQuantity(quantity).
		SetTotalAmount(total).
		SetPaymentMethod(paymentMethod).
		SetStatus(status).
		SetDeliveredCards([]string{}).
		Save(ctx)
	require.NoError(t, err)
	return order
}

func createShopTestPaymentOrder(t *testing.T, ctx context.Context, client *dbent.Client, userID, shopOrderID int64, amount float64, status string) *dbent.PaymentOrder {
	t.Helper()
	paymentOrder, err := client.PaymentOrder.Create().
		SetUserID(userID).
		SetUserEmail("payment@example.com").
		SetUserName("payment-user").
		SetAmount(amount).
		SetPayAmount(amount).
		SetFeeRate(0).
		SetRechargeCode("PAY-" + generateRandomString(8)).
		SetOutTradeNo("sub2_test_" + generateRandomString(12)).
		SetPaymentType(payment.TypeAlipay).
		SetPaymentTradeNo("").
		SetOrderType(payment.OrderTypeShop).
		SetShopOrderID(shopOrderID).
		SetStatus(status).
		SetExpiresAt(time.Now().Add(30 * time.Minute)).
		SetClientIP("127.0.0.1").
		SetSrcHost("example.com").
		Save(ctx)
	require.NoError(t, err)
	return paymentOrder
}

func errorCodeForTest(err error) string {
	var appErr *infraerrors.ApplicationError
	if errors.As(err, &appErr) {
		return appErr.Reason
	}
	return ""
}
