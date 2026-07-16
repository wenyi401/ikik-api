package service

import (
	"context"
	"math"
	"strings"
	"time"

	infraerrors "ikik-api/internal/pkg/errors"
)

const (
	WithdrawalStatusPending   = "PENDING"
	WithdrawalStatusSettled   = "SETTLED"
	WithdrawalStatusCancelled = "CANCELLED"
	WithdrawalStatusRejected  = "REJECTED"

	WithdrawalMinimumAmount = 1.00
	WithdrawalFirstFee      = 0.10
)

var (
	ErrWithdrawalAmountInvalid       = infraerrors.BadRequest("WITHDRAWAL_AMOUNT_INVALID", "withdrawal amount must be at least 1.00 and use at most two decimal places")
	ErrWithdrawalReceiptCodeRequired = infraerrors.BadRequest("WITHDRAWAL_RECEIPT_CODE_REQUIRED", "receipt code is required")
	ErrWithdrawalInsufficientBalance = infraerrors.Forbidden("WITHDRAWAL_INSUFFICIENT_BALANCE", "insufficient balance")
	ErrWithdrawalPendingExists       = infraerrors.Conflict("WITHDRAWAL_PENDING_EXISTS", "user already has a pending withdrawal")
	ErrWithdrawalNotFound            = infraerrors.NotFound("WITHDRAWAL_NOT_FOUND", "withdrawal request not found")
	ErrWithdrawalCannotCancel        = infraerrors.Conflict("WITHDRAWAL_CANNOT_CANCEL", "only pending withdrawals can be cancelled")
	ErrWithdrawalCannotSettle        = infraerrors.Conflict("WITHDRAWAL_CANNOT_SETTLE", "only pending withdrawals can be settled")
	ErrWithdrawalCannotReject        = infraerrors.Conflict("WITHDRAWAL_CANNOT_REJECT", "only pending withdrawals can be rejected")
)

type WithdrawalRequest struct {
	ID                         int64      `json:"id"`
	UserID                     int64      `json:"user_id"`
	UserEmail                  string     `json:"user_email"`
	Amount                     float64    `json:"amount"`
	FeeAmount                  float64    `json:"fee_amount"`
	TotalDeducted              float64    `json:"total_deducted"`
	BalanceBefore              float64    `json:"balance_before"`
	BalanceAfter               float64    `json:"balance_after"`
	PaymentMethod              string     `json:"payment_method"`
	ReceiptCodeStorageProvider string     `json:"receipt_code_storage_provider"`
	ReceiptCodeStorageKey      string     `json:"-"`
	ReceiptCodeURL             string     `json:"receipt_code_url,omitempty"`
	ReceiptCodeContentType     string     `json:"receipt_code_content_type"`
	ReceiptCodeByteSize        int        `json:"receipt_code_byte_size"`
	ReceiptCodeSHA256          string     `json:"receipt_code_sha256"`
	ReceiptCodeUpdatedAt       time.Time  `json:"receipt_code_updated_at"`
	Status                     string     `json:"status"`
	UserCancelReason           *string    `json:"user_cancel_reason,omitempty"`
	AdminNote                  *string    `json:"admin_note,omitempty"`
	ProcessedByUserID          *int64     `json:"processed_by_user_id,omitempty"`
	ProcessedAt                *time.Time `json:"processed_at,omitempty"`
	CreatedAt                  time.Time  `json:"created_at"`
	UpdatedAt                  time.Time  `json:"updated_at"`
}

type WithdrawalListParams struct {
	Page          int
	PageSize      int
	UserID        int64
	Status        string
	Keyword       string
	PaymentMethod string
}

type WithdrawalSubmitInput struct {
	UserID        int64
	Amount        float64
	PaymentMethod string
}

type WithdrawalRepository interface {
	Submit(ctx context.Context, input WithdrawalSubmitInput) (*WithdrawalRequest, error)
	Cancel(ctx context.Context, userID, id int64, reason string) (*WithdrawalRequest, error)
	GetByID(ctx context.Context, id int64) (*WithdrawalRequest, error)
	ListByUser(ctx context.Context, userID int64, page, pageSize int) ([]WithdrawalRequest, int64, error)
	ListAdmin(ctx context.Context, params WithdrawalListParams) ([]WithdrawalRequest, int64, error)
	Settle(ctx context.Context, id, adminUserID int64, note string) (*WithdrawalRequest, error)
	Reject(ctx context.Context, id, adminUserID int64, note string) (*WithdrawalRequest, error)
	ReceiptCodeInUse(ctx context.Context, storageKey string) (bool, error)
}

type WithdrawalService struct {
	repo                 WithdrawalRepository
	authCacheInvalidator APIKeyAuthCacheInvalidator
	billingCacheService  *BillingCacheService
	receiptCodeService   *ReceiptCodeService
}

func NewWithdrawalService(
	repo WithdrawalRepository,
	authCacheInvalidator APIKeyAuthCacheInvalidator,
	billingCacheService *BillingCacheService,
	receiptCodeService *ReceiptCodeService,
) *WithdrawalService {
	return &WithdrawalService{
		repo:                 repo,
		authCacheInvalidator: authCacheInvalidator,
		billingCacheService:  billingCacheService,
		receiptCodeService:   receiptCodeService,
	}
}

func (s *WithdrawalService) Submit(ctx context.Context, input WithdrawalSubmitInput) (*WithdrawalRequest, error) {
	input.PaymentMethod = normalizeReceiptCodePaymentMethod(input.PaymentMethod)
	if input.PaymentMethod == "" {
		return nil, ErrReceiptCodePaymentMethodInvalid
	}
	amount, ok := normalizeWithdrawalAmount(input.Amount)
	if !ok || amount < WithdrawalMinimumAmount {
		return nil, ErrWithdrawalAmountInvalid
	}
	input.Amount = amount

	req, err := s.repo.Submit(ctx, input)
	if err != nil {
		return nil, err
	}
	s.invalidateBalance(ctx, req.UserID)
	return s.attachReceiptURL(ctx, req)
}

func (s *WithdrawalService) Cancel(ctx context.Context, userID, id int64, reason string) (*WithdrawalRequest, error) {
	req, err := s.repo.Cancel(ctx, userID, id, strings.TrimSpace(reason))
	if err != nil {
		return nil, err
	}
	s.invalidateBalance(ctx, req.UserID)
	return s.attachReceiptURL(ctx, req)
}

func (s *WithdrawalService) ListMine(ctx context.Context, userID int64, page, pageSize int) ([]WithdrawalRequest, int64, error) {
	items, total, err := s.repo.ListByUser(ctx, userID, page, pageSize)
	if err != nil {
		return nil, 0, err
	}
	if err := s.attachReceiptURLs(ctx, items); err != nil {
		return nil, 0, err
	}
	return items, total, nil
}

func (s *WithdrawalService) AdminList(ctx context.Context, params WithdrawalListParams) ([]WithdrawalRequest, int64, error) {
	items, total, err := s.repo.ListAdmin(ctx, params)
	if err != nil {
		return nil, 0, err
	}
	if err := s.attachReceiptURLs(ctx, items); err != nil {
		return nil, 0, err
	}
	return items, total, nil
}

func (s *WithdrawalService) AdminGet(ctx context.Context, id int64) (*WithdrawalRequest, error) {
	req, err := s.repo.GetByID(ctx, id)
	if err != nil {
		return nil, err
	}
	return s.attachReceiptURL(ctx, req)
}

func (s *WithdrawalService) AdminSettle(ctx context.Context, id, adminUserID int64, note string) (*WithdrawalRequest, error) {
	req, err := s.repo.Settle(ctx, id, adminUserID, strings.TrimSpace(note))
	if err != nil {
		return nil, err
	}
	return s.attachReceiptURL(ctx, req)
}

func (s *WithdrawalService) AdminReject(ctx context.Context, id, adminUserID int64, note string) (*WithdrawalRequest, error) {
	req, err := s.repo.Reject(ctx, id, adminUserID, strings.TrimSpace(note))
	if err != nil {
		return nil, err
	}
	s.invalidateBalance(ctx, req.UserID)
	return s.attachReceiptURL(ctx, req)
}

func (s *WithdrawalService) ReceiptCodeInUse(ctx context.Context, storageKey string) (bool, error) {
	if s == nil || s.repo == nil || strings.TrimSpace(storageKey) == "" {
		return false, nil
	}
	return s.repo.ReceiptCodeInUse(ctx, storageKey)
}

func (s *WithdrawalService) invalidateBalance(ctx context.Context, userID int64) {
	if s.authCacheInvalidator != nil {
		s.authCacheInvalidator.InvalidateAuthCacheByUserID(ctx, userID)
	}
	if s.billingCacheService != nil {
		_ = s.billingCacheService.InvalidateUserBalance(ctx, userID)
	}
}

func (s *WithdrawalService) attachReceiptURLs(ctx context.Context, items []WithdrawalRequest) error {
	for i := range items {
		if _, err := s.attachReceiptURL(ctx, &items[i]); err != nil {
			return err
		}
	}
	return nil
}

func (s *WithdrawalService) attachReceiptURL(ctx context.Context, req *WithdrawalRequest) (*WithdrawalRequest, error) {
	if req == nil || s.receiptCodeService == nil || strings.TrimSpace(req.ReceiptCodeStorageKey) == "" {
		return req, nil
	}
	code := &ReceiptCode{
		StorageKey:      req.ReceiptCodeStorageKey,
		URL:             req.ReceiptCodeURL,
		StorageProvider: req.ReceiptCodeStorageProvider,
		ContentType:     req.ReceiptCodeContentType,
		ByteSize:        req.ReceiptCodeByteSize,
		SHA256:          req.ReceiptCodeSHA256,
	}
	if err := s.receiptCodeService.attachAccessURL(ctx, code); err != nil {
		return nil, err
	}
	req.ReceiptCodeURL = code.URL
	return req, nil
}

func normalizeWithdrawalAmount(amount float64) (float64, bool) {
	if math.IsNaN(amount) || math.IsInf(amount, 0) {
		return 0, false
	}
	rounded := math.Round(amount*100) / 100
	if math.Abs(amount-rounded) > 1e-9 {
		return 0, false
	}
	return rounded, true
}
