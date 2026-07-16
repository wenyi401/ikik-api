package repository

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"strings"

	"ikik-api/internal/service"
)

type withdrawalRepository struct {
	db *sql.DB
}

func NewWithdrawalRepository(db *sql.DB) service.WithdrawalRepository {
	return &withdrawalRepository{db: db}
}

func (r *withdrawalRepository) Submit(ctx context.Context, input service.WithdrawalSubmitInput) (*service.WithdrawalRequest, error) {
	tx, err := r.db.BeginTx(ctx, &sql.TxOptions{Isolation: sql.LevelReadCommitted})
	if err != nil {
		return nil, err
	}
	defer rollbackUnlessDone(tx)

	var userEmail string
	var balanceBefore float64
	if err := tx.QueryRowContext(ctx, `
SELECT email, share_income_balance::double precision
FROM users
WHERE id = $1 AND deleted_at IS NULL
FOR UPDATE`, input.UserID).Scan(&userEmail, &balanceBefore); err != nil {
		return nil, err
	}

	var pendingExists bool
	if err := tx.QueryRowContext(ctx, `
SELECT EXISTS (
	SELECT 1 FROM user_withdrawal_requests WHERE user_id = $1 AND status = $2
)`, input.UserID, service.WithdrawalStatusPending).Scan(&pendingExists); err != nil {
		return nil, err
	}
	if pendingExists {
		return nil, service.ErrWithdrawalPendingExists
	}

	var hasPriorWithdrawal bool
	if err := tx.QueryRowContext(ctx, `
SELECT EXISTS (
	SELECT 1 FROM user_withdrawal_requests
	WHERE user_id = $1
)`, input.UserID).Scan(&hasPriorWithdrawal); err != nil {
		return nil, err
	}

	feeAmount := 0.0
	if !hasPriorWithdrawal {
		feeAmount = service.WithdrawalFirstFee
	}
	totalDeducted := input.Amount + feeAmount
	if balanceBefore+1e-9 < totalDeducted {
		return nil, service.ErrWithdrawalInsufficientBalance
	}

	var receipt service.ReceiptCode
	if err := tx.QueryRowContext(ctx, `
SELECT id, user_id, payment_method, storage_provider, storage_key, url, content_type, byte_size, sha256, created_at, updated_at
FROM user_receipt_codes
WHERE user_id = $1 AND payment_method = $2`, input.UserID, input.PaymentMethod).Scan(
		&receipt.ID,
		&receipt.UserID,
		&receipt.PaymentMethod,
		&receipt.StorageProvider,
		&receipt.StorageKey,
		&receipt.URL,
		&receipt.ContentType,
		&receipt.ByteSize,
		&receipt.SHA256,
		&receipt.CreatedAt,
		&receipt.UpdatedAt,
	); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, service.ErrWithdrawalReceiptCodeRequired
		}
		return nil, err
	}

	balanceAfter := balanceBefore - totalDeducted
	if _, err := tx.ExecContext(ctx, `
UPDATE users
SET balance = balance - $1,
	share_income_balance = share_income_balance - $1,
	updated_at = NOW()
WHERE id = $2`, totalDeducted, input.UserID); err != nil {
		return nil, err
	}

	req, err := queryWithdrawalRow(ctx, tx, `
INSERT INTO user_withdrawal_requests (
	user_id, user_email, amount, fee_amount, total_deducted, balance_before, balance_after,
	payment_method, receipt_code_storage_provider, receipt_code_storage_key, receipt_code_url,
	receipt_code_content_type, receipt_code_byte_size, receipt_code_sha256, receipt_code_updated_at,
	status
) VALUES (
	$1, $2, $3, $4, $5, $6, $7,
	$8, $9, $10, $11,
	$12, $13, $14, $15,
	$16
)
RETURNING `+withdrawalColumns,
		input.UserID,
		userEmail,
		input.Amount,
		feeAmount,
		totalDeducted,
		balanceBefore,
		balanceAfter,
		input.PaymentMethod,
		receipt.StorageProvider,
		receipt.StorageKey,
		receipt.URL,
		receipt.ContentType,
		receipt.ByteSize,
		receipt.SHA256,
		receipt.UpdatedAt,
		service.WithdrawalStatusPending,
	)
	if err != nil {
		return nil, err
	}
	if err := tx.Commit(); err != nil {
		return nil, err
	}
	return req, nil
}

func (r *withdrawalRepository) Cancel(ctx context.Context, userID, id int64, reason string) (*service.WithdrawalRequest, error) {
	return r.closeWithRefund(ctx, id, userID, 0, service.WithdrawalStatusCancelled, reason, service.ErrWithdrawalCannotCancel)
}

func (r *withdrawalRepository) Reject(ctx context.Context, id, adminUserID int64, note string) (*service.WithdrawalRequest, error) {
	return r.closeWithRefund(ctx, id, 0, adminUserID, service.WithdrawalStatusRejected, note, service.ErrWithdrawalCannotReject)
}

func (r *withdrawalRepository) Settle(ctx context.Context, id, adminUserID int64, note string) (*service.WithdrawalRequest, error) {
	tx, err := r.db.BeginTx(ctx, &sql.TxOptions{Isolation: sql.LevelReadCommitted})
	if err != nil {
		return nil, err
	}
	defer rollbackUnlessDone(tx)

	current, err := getWithdrawalForUpdate(ctx, tx, id)
	if err != nil {
		return nil, err
	}
	if current.Status != service.WithdrawalStatusPending {
		return nil, service.ErrWithdrawalCannotSettle
	}

	req, err := queryWithdrawalRow(ctx, tx, `
UPDATE user_withdrawal_requests
SET status = $1,
	admin_note = NULLIF($2, ''),
	processed_by_user_id = $3,
	processed_at = NOW(),
	updated_at = NOW()
WHERE id = $4
RETURNING `+withdrawalColumns,
		service.WithdrawalStatusSettled,
		strings.TrimSpace(note),
		adminUserID,
		id,
	)
	if err != nil {
		return nil, err
	}
	if err := tx.Commit(); err != nil {
		return nil, err
	}
	return req, nil
}

func (r *withdrawalRepository) closeWithRefund(ctx context.Context, id, userID, operatorID int64, targetStatus, note string, invalidState error) (*service.WithdrawalRequest, error) {
	tx, err := r.db.BeginTx(ctx, &sql.TxOptions{Isolation: sql.LevelReadCommitted})
	if err != nil {
		return nil, err
	}
	defer rollbackUnlessDone(tx)

	current, err := getWithdrawalForUpdate(ctx, tx, id)
	if err != nil {
		return nil, err
	}
	if userID > 0 && current.UserID != userID {
		return nil, service.ErrWithdrawalNotFound
	}
	if current.Status != service.WithdrawalStatusPending {
		return nil, invalidState
	}

	if _, err := tx.ExecContext(ctx, `
UPDATE users
SET balance = balance + $1,
	share_income_balance = share_income_balance + $1,
	updated_at = NOW()
WHERE id = $2`, current.TotalDeducted, current.UserID); err != nil {
		return nil, err
	}

	req, err := updateWithdrawalClosed(ctx, tx, current.ID, targetStatus, operatorID, note)
	if err != nil {
		return nil, err
	}
	if err := tx.Commit(); err != nil {
		return nil, err
	}
	return req, nil
}

func updateWithdrawalClosed(ctx context.Context, tx *sql.Tx, id int64, targetStatus string, operatorID int64, note string) (*service.WithdrawalRequest, error) {
	note = strings.TrimSpace(note)
	if targetStatus == service.WithdrawalStatusCancelled {
		return queryWithdrawalRow(ctx, tx, `
UPDATE user_withdrawal_requests
SET status = $1,
	user_cancel_reason = NULLIF($2, ''),
	processed_at = NOW(),
	updated_at = NOW()
WHERE id = $3
RETURNING `+withdrawalColumns,
			targetStatus,
			note,
			id,
		)
	}
	return queryWithdrawalRow(ctx, tx, `
UPDATE user_withdrawal_requests
SET status = $1,
	admin_note = NULLIF($2, ''),
	processed_by_user_id = $3,
	processed_at = NOW(),
	updated_at = NOW()
WHERE id = $4
RETURNING `+withdrawalColumns,
		targetStatus,
		note,
		operatorID,
		id,
	)
}

func (r *withdrawalRepository) ListByUser(ctx context.Context, userID int64, page, pageSize int) ([]service.WithdrawalRequest, int64, error) {
	params := service.WithdrawalListParams{Page: page, PageSize: pageSize, UserID: userID}
	return r.list(ctx, params, true)
}

func (r *withdrawalRepository) ListAdmin(ctx context.Context, params service.WithdrawalListParams) ([]service.WithdrawalRequest, int64, error) {
	return r.list(ctx, params, false)
}

func (r *withdrawalRepository) GetByID(ctx context.Context, id int64) (*service.WithdrawalRequest, error) {
	if id <= 0 {
		return nil, service.ErrWithdrawalNotFound
	}
	return queryWithdrawalRow(ctx, r.db, `
SELECT `+withdrawalColumns+`
FROM user_withdrawal_requests
WHERE id = $1`, id)
}

func (r *withdrawalRepository) list(ctx context.Context, params service.WithdrawalListParams, forceUser bool) ([]service.WithdrawalRequest, int64, error) {
	page, pageSize := normalizeWithdrawalPagination(params.Page, params.PageSize)
	where, args := buildWithdrawalWhere(params, forceUser)
	countSQL := "SELECT COUNT(*) FROM user_withdrawal_requests" + where
	var total int64
	if err := r.db.QueryRowContext(ctx, countSQL, args...).Scan(&total); err != nil {
		return nil, 0, err
	}
	args = append(args, pageSize, (page-1)*pageSize)
	rows, err := r.db.QueryContext(ctx, `
SELECT `+withdrawalColumns+`
FROM user_withdrawal_requests`+where+`
ORDER BY created_at DESC, id DESC
LIMIT $`+fmt.Sprint(len(args)-1)+` OFFSET $`+fmt.Sprint(len(args)),
		args...,
	)
	if err != nil {
		return nil, 0, err
	}
	defer func() { _ = rows.Close() }()

	items := make([]service.WithdrawalRequest, 0)
	for rows.Next() {
		item, err := scanWithdrawal(rows)
		if err != nil {
			return nil, 0, err
		}
		items = append(items, *item)
	}
	return items, total, rows.Err()
}

func buildWithdrawalWhere(params service.WithdrawalListParams, forceUser bool) (string, []any) {
	clauses := make([]string, 0, 4)
	args := make([]any, 0, 4)
	if forceUser || params.UserID > 0 {
		args = append(args, params.UserID)
		clauses = append(clauses, fmt.Sprintf("user_id = $%d", len(args)))
	}
	if status := strings.TrimSpace(params.Status); status != "" {
		args = append(args, status)
		clauses = append(clauses, fmt.Sprintf("status = $%d", len(args)))
	}
	if method := strings.TrimSpace(params.PaymentMethod); method != "" {
		args = append(args, method)
		clauses = append(clauses, fmt.Sprintf("payment_method = $%d", len(args)))
	}
	if keyword := strings.TrimSpace(params.Keyword); keyword != "" {
		args = append(args, "%"+keyword+"%")
		clauses = append(clauses, fmt.Sprintf("(user_email ILIKE $%d OR CAST(id AS TEXT) ILIKE $%d)", len(args), len(args)))
	}
	if len(clauses) == 0 {
		return "", args
	}
	return " WHERE " + strings.Join(clauses, " AND "), args
}

func (r *withdrawalRepository) ReceiptCodeInUse(ctx context.Context, storageKey string) (bool, error) {
	storageKey = strings.TrimSpace(storageKey)
	if storageKey == "" {
		return false, nil
	}
	var exists bool
	err := r.db.QueryRowContext(ctx, `
SELECT EXISTS (
	SELECT 1 FROM user_withdrawal_requests WHERE receipt_code_storage_key = $1
)`, storageKey).Scan(&exists)
	return exists, err
}

func getWithdrawalForUpdate(ctx context.Context, tx *sql.Tx, id int64) (*service.WithdrawalRequest, error) {
	req, err := queryWithdrawalRow(ctx, tx, `
SELECT `+withdrawalColumns+`
FROM user_withdrawal_requests
WHERE id = $1
FOR UPDATE`, id)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, service.ErrWithdrawalNotFound
	}
	return req, err
}

func queryWithdrawalRow(ctx context.Context, q queryRower, query string, args ...any) (*service.WithdrawalRequest, error) {
	req, err := scanWithdrawal(q.QueryRowContext(ctx, query, args...))
	if errors.Is(err, sql.ErrNoRows) {
		return nil, service.ErrWithdrawalNotFound
	}
	return req, err
}

type queryRower interface {
	QueryRowContext(ctx context.Context, query string, args ...any) *sql.Row
}

type withdrawalScanner interface {
	Scan(dest ...any) error
}

func scanWithdrawal(row withdrawalScanner) (*service.WithdrawalRequest, error) {
	var req service.WithdrawalRequest
	var userCancelReason sql.NullString
	var adminNote sql.NullString
	var processedBy sql.NullInt64
	var processedAt sql.NullTime
	if err := row.Scan(
		&req.ID,
		&req.UserID,
		&req.UserEmail,
		&req.Amount,
		&req.FeeAmount,
		&req.TotalDeducted,
		&req.BalanceBefore,
		&req.BalanceAfter,
		&req.PaymentMethod,
		&req.ReceiptCodeStorageProvider,
		&req.ReceiptCodeStorageKey,
		&req.ReceiptCodeURL,
		&req.ReceiptCodeContentType,
		&req.ReceiptCodeByteSize,
		&req.ReceiptCodeSHA256,
		&req.ReceiptCodeUpdatedAt,
		&req.Status,
		&userCancelReason,
		&adminNote,
		&processedBy,
		&processedAt,
		&req.CreatedAt,
		&req.UpdatedAt,
	); err != nil {
		return nil, err
	}
	if userCancelReason.Valid {
		req.UserCancelReason = &userCancelReason.String
	}
	if adminNote.Valid {
		req.AdminNote = &adminNote.String
	}
	if processedBy.Valid {
		req.ProcessedByUserID = &processedBy.Int64
	}
	if processedAt.Valid {
		req.ProcessedAt = &processedAt.Time
	}
	return &req, nil
}

func normalizeWithdrawalPagination(page, pageSize int) (int, int) {
	if page <= 0 {
		page = 1
	}
	if pageSize <= 0 {
		pageSize = 20
	}
	if pageSize > 1000 {
		pageSize = 1000
	}
	return page, pageSize
}

func rollbackUnlessDone(tx *sql.Tx) {
	if tx != nil {
		_ = tx.Rollback()
	}
}

const withdrawalColumns = `
id, user_id, user_email, amount::double precision, fee_amount::double precision,
total_deducted::double precision, balance_before::double precision, balance_after::double precision,
payment_method, receipt_code_storage_provider, receipt_code_storage_key, receipt_code_url,
receipt_code_content_type, receipt_code_byte_size, receipt_code_sha256, receipt_code_updated_at,
status, user_cancel_reason, admin_note, processed_by_user_id, processed_at, created_at, updated_at`
