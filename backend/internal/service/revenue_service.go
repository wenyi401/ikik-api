package service

import (
	"context"
	"database/sql"
	"fmt"
	"math"
	"strings"
	"time"

	dbent "ikik-api/ent"
	"ikik-api/internal/payment"
	infraerrors "ikik-api/internal/pkg/errors"
)

const revenueSnapshotBusinessTimezone = "Asia/Shanghai"

const (
	RevenueGranularityDay  = "day"
	RevenueGranularityHour = "hour"

	defaultRevenueTopLimit = 10
	maxRevenueTopLimit     = 50
	maxRevenueLiveRange    = 3 * 24 * time.Hour
	maxRevenueSummaryRange = 366 * 24 * time.Hour

	revenueAffiliateActionAccrue   = "accrue"
	revenueAffiliateActionTransfer = "transfer"
	revenueShareStatusApplied      = "applied"
	maxRevenueCashAdjustmentAmount = 10000.0
)

type RevenueQueryParams struct {
	StartTime   time.Time
	EndTime     time.Time
	Granularity string
	Timezone    string
	TopLimit    int
	UserID      *int64
}

type RevenueShareSettlementQueryParams struct {
	StartTime time.Time
	EndTime   time.Time
	Page      int
	PageSize  int
	Search    string
	Status    string
}

type RevenueSummary struct {
	GeneratedAt    string                           `json:"generated_at"`
	StartDate      string                           `json:"start_date"`
	EndDate        string                           `json:"end_date"`
	Granularity    string                           `json:"granularity"`
	Cash           RevenueCashStats                 `json:"cash"`
	Usage          RevenueUsageStats                `json:"usage"`
	Adjustments    RevenueAdjustmentStats           `json:"adjustments"`
	Profit         RevenueProfitStats               `json:"profit"`
	Trend          []RevenueTrendPoint              `json:"trend"`
	TopUsers       []RevenueBreakdownItem           `json:"top_users"`
	TopGroups      []RevenueBreakdownItem           `json:"top_groups"`
	TopAccounts    []RevenueBreakdownItem           `json:"top_accounts"`
	TopModels      []RevenueBreakdownItem           `json:"top_models"`
	TopShareOwners []RevenueShareOwnerBreakdownItem `json:"top_share_owners"`
}

type RevenueCashStats struct {
	PaidAmount             float64 `json:"paid_amount"`
	BalancePaidAmount      float64 `json:"balance_paid_amount"`
	SubscriptionPaidAmount float64 `json:"subscription_paid_amount"`
	RedeemBalanceAmount    float64 `json:"redeem_balance_amount"`
	RefundAmount           float64 `json:"refund_amount"`
	NetPaidAmount          float64 `json:"net_paid_amount"`
	PendingAmount          float64 `json:"pending_amount"`
	PaidOrderCount         int64   `json:"paid_order_count"`
	RedeemBalanceCount     int64   `json:"redeem_balance_count"`
	RefundOrderCount       int64   `json:"refund_order_count"`
	PendingOrderCount      int64   `json:"pending_order_count"`
}

type RevenueUsageStats struct {
	Requests              int64   `json:"requests"`
	TotalTokens           int64   `json:"total_tokens"`
	StandardCost          float64 `json:"standard_cost"`
	ConsumedRevenue       float64 `json:"consumed_revenue"`
	BalanceConsumedAmount float64 `json:"balance_consumed_amount"`
	PointsConsumedAmount  float64 `json:"points_consumed_amount"`
	PointsIssuedAmount    float64 `json:"points_issued_amount"`
	AccountCost           float64 `json:"account_cost"`
}

type RevenueAdjustmentStats struct {
	AffiliateRebate        float64 `json:"affiliate_rebate"`
	AffiliateTransfer      float64 `json:"affiliate_transfer"`
	AffiliateRebateCount   int64   `json:"affiliate_rebate_count"`
	PrivateGroupCommission float64 `json:"private_group_commission"`
	ShareConsumerCharge    float64 `json:"share_consumer_charge"`
	ShareAccountCost       float64 `json:"share_account_cost"`
	ShareOwnerCredit       float64 `json:"share_owner_credit"`
	SharePlatformFee       float64 `json:"share_platform_fee"`
	ShareNetProfit         float64 `json:"share_net_profit"`
	ShareSettlementCount   int64   `json:"share_settlement_count"`
}

type RevenueProfitStats struct {
	UsageGrossProfit   float64 `json:"usage_gross_profit"`
	UsageGrossMargin   float64 `json:"usage_gross_margin"`
	EstimatedNetProfit float64 `json:"estimated_net_profit"`
	EstimatedNetMargin float64 `json:"estimated_net_margin"`
}

type RevenueTrendPoint struct {
	Date                   string  `json:"date"`
	PaidAmount             float64 `json:"paid_amount"`
	RedeemBalanceAmount    float64 `json:"redeem_balance_amount"`
	RefundAmount           float64 `json:"refund_amount"`
	NetPaidAmount          float64 `json:"net_paid_amount"`
	Requests               int64   `json:"requests"`
	ConsumedRevenue        float64 `json:"consumed_revenue"`
	BalanceConsumedAmount  float64 `json:"balance_consumed_amount"`
	PointsConsumedAmount   float64 `json:"points_consumed_amount"`
	PointsIssuedAmount     float64 `json:"points_issued_amount"`
	AccountCost            float64 `json:"account_cost"`
	UsageGrossProfit       float64 `json:"usage_gross_profit"`
	AffiliateRebate        float64 `json:"affiliate_rebate"`
	PrivateGroupCommission float64 `json:"private_group_commission"`
	ShareOwnerCredit       float64 `json:"share_owner_credit"`
	SharePlatformFee       float64 `json:"share_platform_fee"`
	EstimatedNetProfit     float64 `json:"estimated_net_profit"`
}

type RevenueBreakdownItem struct {
	ID               int64   `json:"id,omitempty"`
	Name             string  `json:"name"`
	Secondary        string  `json:"secondary,omitempty"`
	Requests         int64   `json:"requests"`
	TotalTokens      int64   `json:"total_tokens"`
	ConsumedRevenue  float64 `json:"consumed_revenue"`
	AccountCost      float64 `json:"account_cost"`
	ShareOwnerCredit float64 `json:"share_owner_credit"`
	GrossProfit      float64 `json:"gross_profit"`
	GrossMargin      float64 `json:"gross_margin"`
	NetProfit        float64 `json:"net_profit"`
	NetMargin        float64 `json:"net_margin"`
}

type RevenueShareOwnerBreakdownItem struct {
	ID              int64   `json:"id,omitempty"`
	Name            string  `json:"name"`
	Secondary       string  `json:"secondary,omitempty"`
	Requests        int64   `json:"requests"`
	TotalTokens     int64   `json:"total_tokens"`
	ConsumerCharge  float64 `json:"consumer_charge"`
	AccountCost     float64 `json:"account_cost"`
	OwnerCredit     float64 `json:"owner_credit"`
	PlatformFee     float64 `json:"platform_fee"`
	OwnerShareRatio float64 `json:"owner_share_ratio"`
}

type RevenueShareSettlementItem struct {
	ID                  int64      `json:"id"`
	UsageLogID          *int64     `json:"usage_log_id,omitempty"`
	RequestID           string     `json:"request_id"`
	APIKeyID            int64      `json:"api_key_id"`
	APIKeyName          string     `json:"api_key_name"`
	ConsumerUserID      int64      `json:"consumer_user_id"`
	ConsumerEmail       string     `json:"consumer_email"`
	ConsumerUsername    string     `json:"consumer_username,omitempty"`
	OwnerUserID         int64      `json:"owner_user_id"`
	OwnerEmail          string     `json:"owner_email"`
	OwnerUsername       string     `json:"owner_username,omitempty"`
	InviterUserID       *int64     `json:"inviter_user_id,omitempty"`
	InviterEmail        string     `json:"inviter_email,omitempty"`
	InviterUsername     string     `json:"inviter_username,omitempty"`
	AccountID           int64      `json:"account_id"`
	AccountName         string     `json:"account_name"`
	AccountPlatform     string     `json:"account_platform"`
	GroupID             *int64     `json:"group_id,omitempty"`
	GroupName           string     `json:"group_name,omitempty"`
	PolicyID            *int64     `json:"policy_id,omitempty"`
	PolicyVersion       int        `json:"policy_version"`
	ShareModeSnapshot   string     `json:"share_mode_snapshot"`
	ShareStatusSnapshot string     `json:"share_status_snapshot"`
	ConsumerCharge      float64    `json:"consumer_charge"`
	AccountCost         float64    `json:"account_cost"`
	OwnerShareRatio     float64    `json:"owner_share_ratio"`
	OwnerCredit         float64    `json:"owner_credit"`
	InviteBoundAt       *time.Time `json:"invite_bound_at,omitempty"`
	InviteExpiresAt     *time.Time `json:"invite_expires_at,omitempty"`
	InviteShareRatio    float64    `json:"invite_share_ratio"`
	InviteCredit        float64    `json:"invite_credit"`
	PlatformShareRatio  float64    `json:"platform_share_ratio"`
	PlatformFee         float64    `json:"platform_fee"`
	PlatformNetProfit   float64    `json:"platform_net_profit"`
	Status              string     `json:"status"`
	CreatedAt           time.Time  `json:"created_at"`
}

type RevenueService struct {
	entClient *dbent.Client
}

func NewRevenueService(entClient *dbent.Client) *RevenueService {
	return &RevenueService{entClient: entClient}
}

func (s *RevenueService) GetSummary(ctx context.Context, params RevenueQueryParams) (*RevenueSummary, error) {
	if s == nil || s.entClient == nil {
		return nil, infraerrors.New(500, "REVENUE_SERVICE_UNAVAILABLE", "revenue service is unavailable")
	}
	if err := validateRevenueQueryParams(params); err != nil {
		return nil, err
	}
	params.TopLimit = normalizeRevenueTopLimit(params.TopLimit)

	loc := loadRevenueLocation(params.Timezone)
	points, pointIndex := buildRevenueTrendSkeleton(params.StartTime, params.EndTime, params.Granularity, loc)
	out := &RevenueSummary{
		GeneratedAt: time.Now().UTC().Format(time.RFC3339),
		StartDate:   params.StartTime.In(loc).Format("2006-01-02"),
		EndDate:     params.EndTime.Add(-time.Nanosecond).In(loc).Format("2006-01-02"),
		Granularity: params.Granularity,
		Trend:       points,
	}

	if err := s.fillRevenueCashStats(ctx, params, out, pointIndex); err != nil {
		return nil, err
	}
	if err := s.fillRevenueUsageStats(ctx, params, out, pointIndex); err != nil {
		return nil, err
	}
	if err := s.fillRevenueWalletBreakdownStats(ctx, params, out, pointIndex); err != nil {
		return nil, err
	}
	if err := s.fillRevenueAffiliateStats(ctx, params, out, pointIndex); err != nil {
		return nil, err
	}
	if err := s.fillRevenuePrivateGroupCommissionStats(ctx, params, out, pointIndex); err != nil {
		return nil, err
	}
	if err := s.fillRevenueShareStats(ctx, params, out, pointIndex); err != nil {
		return nil, err
	}

	finalizeRevenueSummary(out)

	var err error
	if out.TopUsers, err = s.queryRevenueBreakdown(ctx, params, revenueBreakdownUsers); err != nil {
		return nil, err
	}
	if out.TopGroups, err = s.queryRevenueBreakdown(ctx, params, revenueBreakdownGroups); err != nil {
		return nil, err
	}
	if out.TopAccounts, err = s.queryRevenueBreakdown(ctx, params, revenueBreakdownAccounts); err != nil {
		return nil, err
	}
	if out.TopModels, err = s.queryRevenueBreakdown(ctx, params, revenueBreakdownModels); err != nil {
		return nil, err
	}
	if out.TopShareOwners, err = s.queryRevenueShareOwnerBreakdown(ctx, params); err != nil {
		return nil, err
	}

	return out, nil
}

func (s *RevenueService) ListShareSettlements(ctx context.Context, params RevenueShareSettlementQueryParams) ([]RevenueShareSettlementItem, int64, error) {
	if s == nil || s.entClient == nil {
		return nil, 0, infraerrors.New(500, "REVENUE_SERVICE_UNAVAILABLE", "revenue service is unavailable")
	}
	if params.StartTime.IsZero() || params.EndTime.IsZero() {
		return nil, 0, infraerrors.BadRequest("REVENUE_TIME_RANGE_REQUIRED", "start_time and end_time are required")
	}
	if !params.EndTime.After(params.StartTime) {
		return nil, 0, infraerrors.BadRequest("REVENUE_TIME_RANGE_INVALID", "end_time must be after start_time")
	}
	if params.EndTime.Sub(params.StartTime) > maxRevenueLiveRange {
		return nil, 0, infraerrors.BadRequest("REVENUE_TIME_RANGE_TOO_LARGE", "date range exceeds 3 days")
	}
	params.Page, params.PageSize = normalizeRevenueSettlementPagination(params.Page, params.PageSize)
	status, err := normalizeRevenueSettlementStatus(params.Status)
	if err != nil {
		return nil, 0, infraerrors.BadRequest("REVENUE_SHARE_STATUS_INVALID", "invalid share settlement status")
	}

	where, args := buildRevenueShareSettlementWhere(params, status)
	countQuery := `
		SELECT COUNT(*)
		FROM account_share_settlement_entries ase
		LEFT JOIN users cu ON cu.id = ase.consumer_user_id
		LEFT JOIN users ou ON ou.id = ase.owner_user_id
		LEFT JOIN users iu ON iu.id = ase.inviter_user_id
		LEFT JOIN accounts a ON a.id = ase.account_id
		LEFT JOIN api_keys ak ON ak.id = ase.api_key_id
		LEFT JOIN groups g ON g.id = ase.group_id
		WHERE ` + where
	var total int64
	if err := s.querySingle(ctx, countQuery, args, &total); err != nil {
		return nil, 0, fmt.Errorf("query revenue share settlement count: %w", err)
	}

	limitArg := len(args) + 1
	offsetArg := len(args) + 2
	query := fmt.Sprintf(`
		SELECT
			ase.id,
			ase.usage_log_id,
			ase.request_id,
			ase.api_key_id,
			COALESCE(NULLIF(ak.name, ''), '') AS api_key_name,
			ase.consumer_user_id,
			COALESCE(NULLIF(cu.email, ''), 'unknown') AS consumer_email,
			COALESCE(NULLIF(cu.username, ''), '') AS consumer_username,
			ase.owner_user_id,
			COALESCE(NULLIF(ou.email, ''), 'unknown') AS owner_email,
			COALESCE(NULLIF(ou.username, ''), '') AS owner_username,
			ase.inviter_user_id,
			COALESCE(NULLIF(iu.email, ''), '') AS inviter_email,
			COALESCE(NULLIF(iu.username, ''), '') AS inviter_username,
			ase.account_id,
			COALESCE(NULLIF(a.name, ''), CONCAT('Account #', ase.account_id::text)) AS account_name,
			COALESCE(NULLIF(a.platform, ''), '') AS account_platform,
			ase.group_id,
			COALESCE(NULLIF(g.name, ''), '') AS group_name,
			ase.policy_id,
			ase.policy_version,
			ase.share_mode_snapshot,
			ase.share_status_snapshot,
			ase.consumer_charge::double precision,
			ase.account_cost::double precision,
			ase.owner_share_ratio::double precision,
			ase.owner_credit::double precision,
			ase.invite_bound_at_snapshot,
			ase.invite_expires_at_snapshot,
			ase.invite_share_ratio::double precision,
			ase.invite_credit::double precision,
			ase.platform_share_ratio::double precision,
			ase.platform_fee::double precision,
			(ase.platform_fee - ase.account_cost)::double precision AS platform_net_profit,
			ase.status,
			ase.created_at
		FROM account_share_settlement_entries ase
		LEFT JOIN users cu ON cu.id = ase.consumer_user_id
		LEFT JOIN users ou ON ou.id = ase.owner_user_id
		LEFT JOIN users iu ON iu.id = ase.inviter_user_id
		LEFT JOIN accounts a ON a.id = ase.account_id
		LEFT JOIN api_keys ak ON ak.id = ase.api_key_id
		LEFT JOIN groups g ON g.id = ase.group_id
		WHERE %s
		ORDER BY ase.created_at DESC, ase.id DESC
		LIMIT $%d OFFSET $%d
	`, where, limitArg, offsetArg)
	args = append(args, params.PageSize, (params.Page-1)*params.PageSize)
	rows, err := s.entClient.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, 0, fmt.Errorf("query revenue share settlements: %w", err)
	}
	defer func() { _ = rows.Close() }()

	items := make([]RevenueShareSettlementItem, 0, params.PageSize)
	for rows.Next() {
		var (
			item       RevenueShareSettlementItem
			usageLogID sql.NullInt64
			groupID    sql.NullInt64
			policyID   sql.NullInt64
			inviterID  sql.NullInt64
			boundAt    sql.NullTime
			expiresAt  sql.NullTime
		)
		if err := rows.Scan(
			&item.ID,
			&usageLogID,
			&item.RequestID,
			&item.APIKeyID,
			&item.APIKeyName,
			&item.ConsumerUserID,
			&item.ConsumerEmail,
			&item.ConsumerUsername,
			&item.OwnerUserID,
			&item.OwnerEmail,
			&item.OwnerUsername,
			&inviterID,
			&item.InviterEmail,
			&item.InviterUsername,
			&item.AccountID,
			&item.AccountName,
			&item.AccountPlatform,
			&groupID,
			&item.GroupName,
			&policyID,
			&item.PolicyVersion,
			&item.ShareModeSnapshot,
			&item.ShareStatusSnapshot,
			&item.ConsumerCharge,
			&item.AccountCost,
			&item.OwnerShareRatio,
			&item.OwnerCredit,
			&boundAt,
			&expiresAt,
			&item.InviteShareRatio,
			&item.InviteCredit,
			&item.PlatformShareRatio,
			&item.PlatformFee,
			&item.PlatformNetProfit,
			&item.Status,
			&item.CreatedAt,
		); err != nil {
			return nil, 0, fmt.Errorf("scan revenue share settlement: %w", err)
		}
		if usageLogID.Valid {
			v := usageLogID.Int64
			item.UsageLogID = &v
		}
		if groupID.Valid {
			v := groupID.Int64
			item.GroupID = &v
		}
		if policyID.Valid {
			v := policyID.Int64
			item.PolicyID = &v
		}
		if inviterID.Valid {
			v := inviterID.Int64
			item.InviterUserID = &v
		}
		if boundAt.Valid {
			v := boundAt.Time
			item.InviteBoundAt = &v
		}
		if expiresAt.Valid {
			v := expiresAt.Time
			item.InviteExpiresAt = &v
		}
		item.ConsumerCharge = roundRevenue(item.ConsumerCharge)
		item.AccountCost = roundRevenue(item.AccountCost)
		item.OwnerShareRatio = roundRevenue(item.OwnerShareRatio)
		item.OwnerCredit = roundRevenue(item.OwnerCredit)
		item.InviteShareRatio = roundRevenue(item.InviteShareRatio)
		item.InviteCredit = roundRevenue(item.InviteCredit)
		item.PlatformShareRatio = roundRevenue(item.PlatformShareRatio)
		item.PlatformFee = roundRevenue(item.PlatformFee)
		item.PlatformNetProfit = roundRevenue(item.PlatformNetProfit)
		items = append(items, item)
	}
	if err := rows.Err(); err != nil {
		return nil, 0, fmt.Errorf("iterate revenue share settlements: %w", err)
	}
	return items, total, nil
}

func validateRevenueQueryParams(params RevenueQueryParams) error {
	if params.StartTime.IsZero() || params.EndTime.IsZero() {
		return infraerrors.BadRequest("REVENUE_TIME_RANGE_REQUIRED", "start_time and end_time are required")
	}
	if !params.EndTime.After(params.StartTime) {
		return infraerrors.BadRequest("REVENUE_TIME_RANGE_INVALID", "end_time must be after start_time")
	}
	switch params.Granularity {
	case RevenueGranularityDay, RevenueGranularityHour:
	default:
		return infraerrors.BadRequest("REVENUE_GRANULARITY_INVALID", "granularity must be day or hour")
	}
	window := params.EndTime.Sub(params.StartTime)
	if window > maxRevenueSummaryRange {
		return infraerrors.BadRequest("REVENUE_TIME_RANGE_TOO_LARGE", "date range exceeds 366 days")
	}
	if params.Granularity == RevenueGranularityHour && window > maxRevenueLiveRange {
		return infraerrors.BadRequest("REVENUE_TIME_RANGE_TOO_LARGE", "hour granularity supports at most 3 days")
	}
	if params.UserID != nil && *params.UserID <= 0 {
		return infraerrors.BadRequest("REVENUE_USER_ID_INVALID", "user_id must be a positive integer")
	}
	return nil
}

func revenueUserFilter(column string, userID *int64, placeholder int) (string, []any) {
	if userID == nil {
		return "", nil
	}
	return fmt.Sprintf(" AND %s = $%d", column, placeholder), []any{*userID}
}

func normalizeRevenueTopLimit(limit int) int {
	if limit <= 0 {
		return defaultRevenueTopLimit
	}
	if limit > maxRevenueTopLimit {
		return maxRevenueTopLimit
	}
	return limit
}

func normalizeRevenueSettlementPagination(page, pageSize int) (int, int) {
	if page <= 0 {
		page = 1
	}
	if pageSize <= 0 {
		pageSize = 20
	}
	if pageSize > 100 {
		pageSize = 100
	}
	return page, pageSize
}

func normalizeRevenueSettlementStatus(raw string) (string, error) {
	switch strings.ToLower(strings.TrimSpace(raw)) {
	case "", "all":
		return "", nil
	case "applied", "reversed", "frozen":
		return strings.ToLower(strings.TrimSpace(raw)), nil
	default:
		return "", fmt.Errorf("invalid settlement status: %s", raw)
	}
}

func buildRevenueShareSettlementWhere(params RevenueShareSettlementQueryParams, status string) (string, []any) {
	conditions := []string{
		"ase.created_at >= $1",
		"ase.created_at < $2",
		"ase.consumer_user_id <> ase.owner_user_id",
	}
	args := []any{params.StartTime, params.EndTime}
	if status != "" {
		args = append(args, status)
		conditions = append(conditions, fmt.Sprintf("ase.status = $%d", len(args)))
	}
	if search := strings.TrimSpace(params.Search); search != "" {
		args = append(args, "%"+search+"%")
		placeholder := fmt.Sprintf("$%d", len(args))
		conditions = append(conditions, fmt.Sprintf(`(
			ase.request_id ILIKE %s
			OR COALESCE(cu.email, '') ILIKE %s
			OR COALESCE(cu.username, '') ILIKE %s
			OR COALESCE(ou.email, '') ILIKE %s
			OR COALESCE(ou.username, '') ILIKE %s
			OR COALESCE(a.name, '') ILIKE %s
			OR COALESCE(ak.name, '') ILIKE %s
		)`, placeholder, placeholder, placeholder, placeholder, placeholder, placeholder, placeholder))
	}
	return strings.Join(conditions, " AND "), args
}

func loadRevenueLocation(name string) *time.Location {
	if name == "" {
		return time.Local
	}
	loc, err := time.LoadLocation(name)
	if err != nil {
		return time.Local
	}
	return loc
}

func buildRevenueTrendSkeleton(start, end time.Time, granularity string, loc *time.Location) ([]RevenueTrendPoint, map[string]int) {
	if loc == nil {
		loc = time.Local
	}
	startLocal := start.In(loc)
	endLocal := end.In(loc)
	if granularity == RevenueGranularityHour {
		startLocal = startLocal.Truncate(time.Hour)
	} else {
		startLocal = time.Date(startLocal.Year(), startLocal.Month(), startLocal.Day(), 0, 0, 0, 0, loc)
		endLocal = time.Date(endLocal.Year(), endLocal.Month(), endLocal.Day(), 0, 0, 0, 0, loc)
	}

	points := make([]RevenueTrendPoint, 0)
	index := make(map[string]int)
	for cur := startLocal; cur.Before(endLocal); {
		label := formatRevenueBucket(cur, granularity)
		index[label] = len(points)
		points = append(points, RevenueTrendPoint{Date: label})
		if granularity == RevenueGranularityHour {
			cur = cur.Add(time.Hour)
		} else {
			cur = cur.AddDate(0, 0, 1)
		}
	}
	return points, index
}

func formatRevenueBucket(t time.Time, granularity string) string {
	if granularity == RevenueGranularityHour {
		return t.Format("2006-01-02 15:00")
	}
	return t.Format("2006-01-02")
}

func revenueBucketExpression(column, granularity string) string {
	if granularity == RevenueGranularityHour {
		return fmt.Sprintf("TO_CHAR(date_trunc('hour', %s AT TIME ZONE $3), 'YYYY-MM-DD HH24:00')", column)
	}
	return fmt.Sprintf("TO_CHAR(%s AT TIME ZONE $3, 'YYYY-MM-DD')", column)
}

func revenueSnapshotDateRange(params RevenueQueryParams) (string, string) {
	return revenueSnapshotBusinessDate(params.StartTime), revenueSnapshotBusinessDate(params.EndTime)
}

func shouldUseRevenueDailySnapshots(params RevenueQueryParams) bool {
	return params.Granularity == RevenueGranularityDay &&
		strings.TrimSpace(params.Timezone) == revenueSnapshotBusinessTimezone &&
		isRevenueSnapshotBusinessFullDayRange(params.StartTime, params.EndTime)
}

func revenueSnapshotBusinessLocation() *time.Location {
	loc, err := time.LoadLocation(revenueSnapshotBusinessTimezone)
	if err != nil {
		return time.FixedZone(revenueSnapshotBusinessTimezone, 8*60*60)
	}
	return loc
}

func revenueSnapshotBusinessDate(t time.Time) string {
	return t.In(revenueSnapshotBusinessLocation()).Format("2006-01-02")
}

func isRevenueSnapshotBusinessFullDayRange(start, end time.Time) bool {
	if !end.After(start) {
		return false
	}
	loc := revenueSnapshotBusinessLocation()
	startLocal := start.In(loc)
	endLocal := end.In(loc)
	return startLocal.Hour() == 0 &&
		startLocal.Minute() == 0 &&
		startLocal.Second() == 0 &&
		startLocal.Nanosecond() == 0 &&
		endLocal.Hour() == 0 &&
		endLocal.Minute() == 0 &&
		endLocal.Second() == 0 &&
		endLocal.Nanosecond() == 0
}

func revenueSnapshotUserFilter(column string, userID *int64, placeholder int) string {
	if userID == nil {
		return ""
	}
	return fmt.Sprintf(" AND %s = $%d", column, placeholder)
}

func (s *RevenueService) fillRevenueCashStats(ctx context.Context, params RevenueQueryParams, out *RevenueSummary, pointIndex map[string]int) error {
	userFilter, userArgs := revenueUserFilter("user_id", params.UserID, 6)
	query := `
		SELECT
			COALESCE(SUM(pay_amount) FILTER (WHERE paid_at >= $1 AND paid_at < $2), 0)::double precision AS paid_amount,
			COALESCE(SUM(pay_amount) FILTER (WHERE paid_at >= $1 AND paid_at < $2 AND order_type = $3), 0)::double precision AS balance_paid_amount,
			COALESCE(SUM(pay_amount) FILTER (WHERE paid_at >= $1 AND paid_at < $2 AND order_type = $4), 0)::double precision AS subscription_paid_amount,
			COUNT(*) FILTER (WHERE paid_at >= $1 AND paid_at < $2) AS paid_order_count,
			COALESCE(SUM(refund_amount) FILTER (WHERE refund_at >= $1 AND refund_at < $2 AND refund_amount > 0), 0)::double precision AS refund_amount,
			COUNT(*) FILTER (WHERE refund_at >= $1 AND refund_at < $2 AND refund_amount > 0) AS refund_order_count,
			COALESCE(SUM(pay_amount) FILTER (WHERE status = $5 AND created_at >= $1 AND created_at < $2), 0)::double precision AS pending_amount,
			COUNT(*) FILTER (WHERE status = $5 AND created_at >= $1 AND created_at < $2) AS pending_order_count
		FROM payment_orders
		WHERE ((paid_at >= $1 AND paid_at < $2)
			OR (refund_at >= $1 AND refund_at < $2)
			OR (status = $5 AND created_at >= $1 AND created_at < $2))
	`
	if userFilter != "" {
		query += userFilter
	}
	args := []any{params.StartTime, params.EndTime, payment.OrderTypeBalance, payment.OrderTypeSubscription, OrderStatusPending}
	args = append(args, userArgs...)
	if err := s.querySingle(ctx, query, args,
		&out.Cash.PaidAmount,
		&out.Cash.BalancePaidAmount,
		&out.Cash.SubscriptionPaidAmount,
		&out.Cash.PaidOrderCount,
		&out.Cash.RefundAmount,
		&out.Cash.RefundOrderCount,
		&out.Cash.PendingAmount,
		&out.Cash.PendingOrderCount,
	); err != nil {
		return fmt.Errorf("query revenue cash stats: %w", err)
	}
	if err := s.fillRevenueRedeemCashStats(ctx, params, out, pointIndex); err != nil {
		return err
	}

	bucketPaid := revenueBucketExpression("paid_at", params.Granularity)
	bucketRefund := revenueBucketExpression("refund_at", params.Granularity)
	trendUserFilter, trendUserArgs := revenueUserFilter("user_id", params.UserID, 4)
	trendQuery := fmt.Sprintf(`
		SELECT bucket,
			COALESCE(SUM(paid_amount), 0)::double precision,
			COALESCE(SUM(refund_amount), 0)::double precision
		FROM (
			SELECT %s AS bucket, COALESCE(SUM(pay_amount), 0) AS paid_amount, 0::numeric AS refund_amount
			FROM payment_orders
			WHERE paid_at >= $1 AND paid_at < $2
				%s
			GROUP BY 1
			UNION ALL
			SELECT %s AS bucket, 0::numeric AS paid_amount, COALESCE(SUM(refund_amount), 0) AS refund_amount
			FROM payment_orders
			WHERE refund_at >= $1 AND refund_at < $2 AND refund_amount > 0
				%s
			GROUP BY 1
		) s
		GROUP BY bucket
		ORDER BY bucket
	`, bucketPaid, trendUserFilter, bucketRefund, trendUserFilter)
	trendArgs := []any{params.StartTime, params.EndTime, params.Timezone}
	trendArgs = append(trendArgs, trendUserArgs...)
	rows, err := s.entClient.QueryContext(ctx, trendQuery, trendArgs...)
	if err != nil {
		return fmt.Errorf("query revenue cash trend: %w", err)
	}
	defer func() { _ = rows.Close() }()
	for rows.Next() {
		var bucket string
		var paidAmount, refundAmount float64
		if err := rows.Scan(&bucket, &paidAmount, &refundAmount); err != nil {
			return fmt.Errorf("scan revenue cash trend: %w", err)
		}
		if idx, ok := pointIndex[bucket]; ok {
			out.Trend[idx].PaidAmount = paidAmount
			out.Trend[idx].RefundAmount = refundAmount
		}
	}
	if err := rows.Err(); err != nil {
		return fmt.Errorf("iterate revenue cash trend: %w", err)
	}
	return nil
}

func (s *RevenueService) fillRevenueRedeemCashStats(ctx context.Context, params RevenueQueryParams, out *RevenueSummary, pointIndex map[string]int) error {
	hasRevenueFlag := s.redeemCodeRevenueFlagAvailable(ctx)
	userFilter, userArgs := revenueUserFilter("used_by", params.UserID, 6)
	typePredicate := "type = $5"
	args := []any{params.StartTime, params.EndTime, StatusUsed, maxRevenueCashAdjustmentAmount, RedeemTypeBalance}
	if hasRevenueFlag {
		typePredicate = "(type = $5 OR (type = $6 AND COALESCE(count_as_revenue, FALSE)))"
		args = append(args, AdjustmentTypeAdminBalance)
		userFilter, userArgs = revenueUserFilter("used_by", params.UserID, 7)
	}
	query := fmt.Sprintf(`
		SELECT
			COALESCE(SUM(value), 0)::double precision AS redeem_balance_amount,
			COUNT(*) AS redeem_balance_count
		FROM redeem_codes
		WHERE used_at >= $1
			AND used_at < $2
			AND status = $3
			AND value > 0
			AND value <= $4
			AND %s
			%s
	`, typePredicate, userFilter)
	args = append(args, userArgs...)
	if err := s.querySingle(ctx, query, args, &out.Cash.RedeemBalanceAmount, &out.Cash.RedeemBalanceCount); err != nil {
		return fmt.Errorf("query revenue redeem cash stats: %w", err)
	}

	bucketUsed := revenueBucketExpression("used_at", params.Granularity)
	trendUserFilter, trendUserArgs := revenueUserFilter("used_by", params.UserID, 6)
	trendTypePredicate := "type = $5"
	trendArgs := []any{params.StartTime, params.EndTime, params.Timezone, StatusUsed, RedeemTypeBalance}
	if hasRevenueFlag {
		trendTypePredicate = "(type = $5 OR (type = $6 AND COALESCE(count_as_revenue, FALSE)))"
		trendArgs = append(trendArgs, AdjustmentTypeAdminBalance)
		trendUserFilter, trendUserArgs = revenueUserFilter("used_by", params.UserID, 7)
	}
	trendQuery := fmt.Sprintf(`
		SELECT %s AS bucket,
			COALESCE(SUM(value), 0)::double precision AS redeem_balance_amount
		FROM redeem_codes
		WHERE used_at >= $1
			AND used_at < $2
			AND status = $4
			AND value > 0
			AND value <= %f
			AND %s
			%s
		GROUP BY 1
		ORDER BY 1
	`, bucketUsed, maxRevenueCashAdjustmentAmount, trendTypePredicate, trendUserFilter)
	trendArgs = append(trendArgs, trendUserArgs...)
	rows, err := s.entClient.QueryContext(ctx, trendQuery, trendArgs...)
	if err != nil {
		return fmt.Errorf("query revenue redeem cash trend: %w", err)
	}
	defer func() { _ = rows.Close() }()
	for rows.Next() {
		var bucket string
		var redeemAmount float64
		if err := rows.Scan(&bucket, &redeemAmount); err != nil {
			return fmt.Errorf("scan revenue redeem cash trend: %w", err)
		}
		if idx, ok := pointIndex[bucket]; ok {
			out.Trend[idx].RedeemBalanceAmount = redeemAmount
		}
	}
	if err := rows.Err(); err != nil {
		return fmt.Errorf("iterate revenue redeem cash trend: %w", err)
	}
	return nil
}

func (s *RevenueService) redeemCodeRevenueFlagAvailable(ctx context.Context) bool {
	if s == nil || s.entClient == nil {
		return false
	}
	var exists bool
	query := `
		SELECT EXISTS (
			SELECT 1
			FROM information_schema.columns
			WHERE table_name = 'redeem_codes'
				AND column_name = 'count_as_revenue'
		)`
	if s.entClient.Driver().Dialect() != "postgres" {
		query = "SELECT COUNT(*) > 0 FROM pragma_table_info('redeem_codes') WHERE name = 'count_as_revenue'"
	}
	return s.querySingle(ctx, query, nil, &exists) == nil && exists
}

func (s *RevenueService) fillRevenueUsageStats(ctx context.Context, params RevenueQueryParams, out *RevenueSummary, pointIndex map[string]int) error {
	if shouldUseRevenueDailySnapshots(params) {
		return s.fillRevenueUsageStatsFromSnapshots(ctx, params, out, pointIndex)
	}

	accountCostExpr := "COALESCE(account_stats_cost, total_cost) * COALESCE(account_rate_multiplier, 1)"
	userFilter, userArgs := revenueUserFilter("user_id", params.UserID, 3)
	query := fmt.Sprintf(`
		SELECT
			COUNT(*) AS requests,
			COALESCE(SUM(input_tokens + output_tokens + cache_creation_tokens + cache_read_tokens), 0) AS total_tokens,
			COALESCE(SUM(total_cost), 0)::double precision AS standard_cost,
			COALESCE(SUM(actual_cost), 0)::double precision AS consumed_revenue,
			COALESCE(SUM(%s), 0)::double precision AS account_cost
		FROM usage_logs
		WHERE created_at >= $1 AND created_at < $2
			%s
	`, accountCostExpr, userFilter)
	args := []any{params.StartTime, params.EndTime}
	args = append(args, userArgs...)
	if err := s.querySingle(ctx, query, args,
		&out.Usage.Requests,
		&out.Usage.TotalTokens,
		&out.Usage.StandardCost,
		&out.Usage.ConsumedRevenue,
		&out.Usage.AccountCost,
	); err != nil {
		return fmt.Errorf("query revenue usage stats: %w", err)
	}

	bucketExpr := revenueBucketExpression("created_at", params.Granularity)
	trendUserFilter, trendUserArgs := revenueUserFilter("user_id", params.UserID, 4)
	trendQuery := fmt.Sprintf(`
		SELECT
			%s AS bucket,
			COUNT(*) AS requests,
			COALESCE(SUM(actual_cost), 0)::double precision AS consumed_revenue,
			COALESCE(SUM(%s), 0)::double precision AS account_cost
		FROM usage_logs
		WHERE created_at >= $1 AND created_at < $2
			%s
		GROUP BY 1
		ORDER BY 1
	`, bucketExpr, accountCostExpr, trendUserFilter)
	trendArgs := []any{params.StartTime, params.EndTime, params.Timezone}
	trendArgs = append(trendArgs, trendUserArgs...)
	rows, err := s.entClient.QueryContext(ctx, trendQuery, trendArgs...)
	if err != nil {
		return fmt.Errorf("query revenue usage trend: %w", err)
	}
	defer func() { _ = rows.Close() }()
	for rows.Next() {
		var bucket string
		var requests int64
		var consumedRevenue, accountCost float64
		if err := rows.Scan(&bucket, &requests, &consumedRevenue, &accountCost); err != nil {
			return fmt.Errorf("scan revenue usage trend: %w", err)
		}
		if idx, ok := pointIndex[bucket]; ok {
			out.Trend[idx].Requests = requests
			out.Trend[idx].ConsumedRevenue = consumedRevenue
			out.Trend[idx].AccountCost = accountCost
		}
	}
	if err := rows.Err(); err != nil {
		return fmt.Errorf("iterate revenue usage trend: %w", err)
	}
	return nil
}

func (s *RevenueService) fillRevenueUsageStatsFromSnapshots(ctx context.Context, params RevenueQueryParams, out *RevenueSummary, pointIndex map[string]int) error {
	startDate, endDate := revenueSnapshotDateRange(params)
	statsSnapshotUserFilter := revenueSnapshotUserFilter("s.user_id", params.UserID, 5)
	statsLiveUserFilter := revenueSnapshotUserFilter("ul.user_id", params.UserID, 5)
	query := fmt.Sprintf(`
		WITH snapshot_days AS (
			SELECT DISTINCT bucket_date
			FROM revenue_daily_dimension_snapshots
			WHERE bucket_date >= $1::date AND bucket_date < $2::date
				%s
		),
		combined AS (
			SELECT
				s.bucket_date,
				SUM(s.total_requests)::bigint AS requests,
				SUM(s.total_tokens)::bigint AS total_tokens,
				SUM(s.standard_cost)::double precision AS standard_cost,
				SUM(s.consumed_revenue)::double precision AS consumed_revenue,
				SUM(s.account_cost)::double precision AS account_cost
			FROM revenue_daily_dimension_snapshots s
			WHERE s.bucket_date >= $1::date AND s.bucket_date < $2::date
				%s
			GROUP BY s.bucket_date
			UNION ALL
			SELECT
				(ul.created_at AT TIME ZONE 'Asia/Shanghai')::date AS bucket_date,
				COUNT(*)::bigint AS requests,
				COALESCE(SUM(ul.input_tokens + ul.output_tokens + ul.cache_creation_tokens + ul.cache_read_tokens), 0)::bigint AS total_tokens,
				COALESCE(SUM(ul.total_cost), 0)::double precision AS standard_cost,
				COALESCE(SUM(ul.actual_cost), 0)::double precision AS consumed_revenue,
				COALESCE(SUM(COALESCE(ul.account_stats_cost, ul.total_cost) * COALESCE(ul.account_rate_multiplier, 1)), 0)::double precision AS account_cost
			FROM usage_logs ul
			WHERE ul.created_at >= $3 AND ul.created_at < $4
				AND NOT EXISTS (
					SELECT 1
					FROM snapshot_days sd
					WHERE sd.bucket_date = (ul.created_at AT TIME ZONE 'Asia/Shanghai')::date
				)
				%s
			GROUP BY 1
		)
		SELECT
			COALESCE(SUM(requests), 0)::bigint,
			COALESCE(SUM(total_tokens), 0)::bigint,
			COALESCE(SUM(standard_cost), 0)::double precision,
			COALESCE(SUM(consumed_revenue), 0)::double precision,
			COALESCE(SUM(account_cost), 0)::double precision
		FROM combined
	`, statsSnapshotUserFilter, statsSnapshotUserFilter, statsLiveUserFilter)
	args := []any{startDate, endDate, params.StartTime, params.EndTime}
	if params.UserID != nil {
		args = append(args, *params.UserID)
	}
	if err := s.querySingle(ctx, query, args,
		&out.Usage.Requests,
		&out.Usage.TotalTokens,
		&out.Usage.StandardCost,
		&out.Usage.ConsumedRevenue,
		&out.Usage.AccountCost,
	); err != nil {
		return fmt.Errorf("query revenue usage snapshot stats: %w", err)
	}

	trendSnapshotUserFilter := revenueSnapshotUserFilter("s.user_id", params.UserID, 6)
	trendLiveUserFilter := revenueSnapshotUserFilter("ul.user_id", params.UserID, 6)
	trendQuery := fmt.Sprintf(`
		WITH snapshot_days AS (
			SELECT DISTINCT bucket_date
			FROM revenue_daily_dimension_snapshots
			WHERE bucket_date >= $1::date AND bucket_date < $2::date
				%s
		),
		combined AS (
			SELECT
				TO_CHAR(s.bucket_date::timestamp, 'YYYY-MM-DD') AS bucket,
				SUM(s.total_requests)::bigint AS requests,
				SUM(s.consumed_revenue)::double precision AS consumed_revenue,
				SUM(s.account_cost)::double precision AS account_cost
			FROM revenue_daily_dimension_snapshots s
			WHERE s.bucket_date >= $1::date AND s.bucket_date < $2::date
				%s
			GROUP BY 1
			UNION ALL
			SELECT
				TO_CHAR(ul.created_at AT TIME ZONE $5, 'YYYY-MM-DD') AS bucket,
				COUNT(*)::bigint AS requests,
				COALESCE(SUM(ul.actual_cost), 0)::double precision AS consumed_revenue,
				COALESCE(SUM(COALESCE(ul.account_stats_cost, ul.total_cost) * COALESCE(ul.account_rate_multiplier, 1)), 0)::double precision AS account_cost
			FROM usage_logs ul
			WHERE ul.created_at >= $3 AND ul.created_at < $4
				AND NOT EXISTS (
					SELECT 1
					FROM snapshot_days sd
					WHERE sd.bucket_date = (ul.created_at AT TIME ZONE 'Asia/Shanghai')::date
				)
				%s
			GROUP BY 1
		)
		SELECT
			bucket,
			COALESCE(SUM(requests), 0)::bigint,
			COALESCE(SUM(consumed_revenue), 0)::double precision,
			COALESCE(SUM(account_cost), 0)::double precision
		FROM combined
		GROUP BY bucket
		ORDER BY bucket
	`, trendSnapshotUserFilter, trendSnapshotUserFilter, trendLiveUserFilter)
	trendArgs := []any{startDate, endDate, params.StartTime, params.EndTime, params.Timezone}
	if params.UserID != nil {
		trendArgs = append(trendArgs, *params.UserID)
	}
	rows, err := s.entClient.QueryContext(ctx, trendQuery, trendArgs...)
	if err != nil {
		return fmt.Errorf("query revenue usage snapshot trend: %w", err)
	}
	defer func() { _ = rows.Close() }()
	for rows.Next() {
		var bucket string
		var requests int64
		var consumedRevenue, accountCost float64
		if err := rows.Scan(&bucket, &requests, &consumedRevenue, &accountCost); err != nil {
			return fmt.Errorf("scan revenue usage snapshot trend: %w", err)
		}
		if idx, ok := pointIndex[bucket]; ok {
			out.Trend[idx].Requests = requests
			out.Trend[idx].ConsumedRevenue = consumedRevenue
			out.Trend[idx].AccountCost = accountCost
		}
	}
	if err := rows.Err(); err != nil {
		return fmt.Errorf("iterate revenue usage snapshot trend: %w", err)
	}
	return nil
}

func (s *RevenueService) fillRevenueWalletBreakdownStats(ctx context.Context, params RevenueQueryParams, out *RevenueSummary, pointIndex map[string]int) error {
	userBalanceFilter, userBalanceArgs := revenueUserFilter("user_id", params.UserID, 4)
	balanceQuery := `
		SELECT COALESCE(SUM(amount), 0)::double precision
		FROM user_balance_ledger
		WHERE created_at >= $1 AND created_at < $2
			AND direction = 'debit'
			AND reason = $3
	`
	balanceQuery += userBalanceFilter
	balanceArgs := []any{params.StartTime, params.EndTime, "usage_charge"}
	balanceArgs = append(balanceArgs, userBalanceArgs...)
	if err := s.querySingle(ctx, balanceQuery, balanceArgs, &out.Usage.BalanceConsumedAmount); err != nil {
		return fmt.Errorf("query revenue balance consumed stats: %w", err)
	}

	userPointsFilter, userPointsArgs := revenueUserFilter("user_id", params.UserID, 8)
	pointsQuery := `
		SELECT
			COALESCE(SUM(amount) FILTER (WHERE direction = 'debit' AND reason IN ($3, $4)), 0)::double precision AS points_consumed,
			COALESCE(SUM(amount) FILTER (WHERE direction = 'credit' AND reason IN ($5, $6, $7)), 0)::double precision AS points_issued
		FROM points_ledger
		WHERE created_at >= $1 AND created_at < $2
	`
	pointsQuery += userPointsFilter
	pointsArgs := []any{
		params.StartTime,
		params.EndTime,
		"usage_charge",
		"shop_order",
		"redeem_code",
		"admin_adjustment",
		"shop_draw_reward",
	}
	pointsArgs = append(pointsArgs, userPointsArgs...)
	if err := s.querySingle(ctx, pointsQuery, pointsArgs, &out.Usage.PointsConsumedAmount, &out.Usage.PointsIssuedAmount); err != nil {
		return fmt.Errorf("query revenue points stats: %w", err)
	}

	bucketExpr := revenueBucketExpression("created_at", params.Granularity)
	trendBalanceUserFilter, trendBalanceUserArgs := revenueUserFilter("user_id", params.UserID, 5)
	trendBalanceQuery := fmt.Sprintf(`
		SELECT %s AS bucket, COALESCE(SUM(amount), 0)::double precision AS balance_consumed
		FROM user_balance_ledger
		WHERE created_at >= $1 AND created_at < $2
			AND direction = 'debit'
			AND reason = $4
			%s
		GROUP BY 1
		ORDER BY 1
	`, bucketExpr, trendBalanceUserFilter)
	trendBalanceArgs := []any{params.StartTime, params.EndTime, params.Timezone, "usage_charge"}
	trendBalanceArgs = append(trendBalanceArgs, trendBalanceUserArgs...)
	rows, err := s.entClient.QueryContext(ctx, trendBalanceQuery, trendBalanceArgs...)
	if err != nil {
		return fmt.Errorf("query revenue balance consumed trend: %w", err)
	}
	for rows.Next() {
		var bucket string
		var amount float64
		if err := rows.Scan(&bucket, &amount); err != nil {
			_ = rows.Close()
			return fmt.Errorf("scan revenue balance consumed trend: %w", err)
		}
		if idx, ok := pointIndex[bucket]; ok {
			out.Trend[idx].BalanceConsumedAmount = amount
		}
	}
	if err := rows.Err(); err != nil {
		_ = rows.Close()
		return fmt.Errorf("iterate revenue balance consumed trend: %w", err)
	}
	_ = rows.Close()

	trendPointsUserFilter, trendPointsUserArgs := revenueUserFilter("user_id", params.UserID, 9)
	trendPointsQuery := fmt.Sprintf(`
		SELECT
			%s AS bucket,
			COALESCE(SUM(amount) FILTER (WHERE direction = 'debit' AND reason IN ($4, $5)), 0)::double precision AS points_consumed,
			COALESCE(SUM(amount) FILTER (WHERE direction = 'credit' AND reason IN ($6, $7, $8)), 0)::double precision AS points_issued
		FROM points_ledger
		WHERE created_at >= $1 AND created_at < $2
			%s
		GROUP BY 1
		ORDER BY 1
	`, bucketExpr, trendPointsUserFilter)
	trendPointsArgs := []any{
		params.StartTime,
		params.EndTime,
		params.Timezone,
		"usage_charge",
		"shop_order",
		"redeem_code",
		"admin_adjustment",
		"shop_draw_reward",
	}
	trendPointsArgs = append(trendPointsArgs, trendPointsUserArgs...)
	rows, err = s.entClient.QueryContext(ctx, trendPointsQuery, trendPointsArgs...)
	if err != nil {
		return fmt.Errorf("query revenue points trend: %w", err)
	}
	defer func() { _ = rows.Close() }()
	for rows.Next() {
		var bucket string
		var consumed, issued float64
		if err := rows.Scan(&bucket, &consumed, &issued); err != nil {
			return fmt.Errorf("scan revenue points trend: %w", err)
		}
		if idx, ok := pointIndex[bucket]; ok {
			out.Trend[idx].PointsConsumedAmount = consumed
			out.Trend[idx].PointsIssuedAmount = issued
		}
	}
	if err := rows.Err(); err != nil {
		return fmt.Errorf("iterate revenue points trend: %w", err)
	}
	return nil
}

func (s *RevenueService) fillRevenueAffiliateStats(ctx context.Context, params RevenueQueryParams, out *RevenueSummary, pointIndex map[string]int) error {
	userFilter, userArgs := revenueUserFilter("user_id", params.UserID, 5)
	query := `
		SELECT
			COALESCE(SUM(amount) FILTER (WHERE action = $3), 0)::double precision AS affiliate_rebate,
			COALESCE(SUM(amount) FILTER (WHERE action = $4), 0)::double precision AS affiliate_transfer,
			COUNT(*) FILTER (WHERE action = $3) AS affiliate_rebate_count
		FROM user_affiliate_ledger
		WHERE created_at >= $1 AND created_at < $2
	`
	query += userFilter
	args := []any{params.StartTime, params.EndTime, revenueAffiliateActionAccrue, revenueAffiliateActionTransfer}
	args = append(args, userArgs...)
	if err := s.querySingle(ctx, query, args,
		&out.Adjustments.AffiliateRebate,
		&out.Adjustments.AffiliateTransfer,
		&out.Adjustments.AffiliateRebateCount,
	); err != nil {
		return fmt.Errorf("query revenue affiliate stats: %w", err)
	}

	bucketExpr := revenueBucketExpression("created_at", params.Granularity)
	trendUserFilter, trendUserArgs := revenueUserFilter("user_id", params.UserID, 5)
	trendQuery := fmt.Sprintf(`
		SELECT %s AS bucket, COALESCE(SUM(amount), 0)::double precision AS affiliate_rebate
		FROM user_affiliate_ledger
		WHERE created_at >= $1 AND created_at < $2 AND action = $4
			%s
		GROUP BY 1
		ORDER BY 1
	`, bucketExpr, trendUserFilter)
	trendArgs := []any{params.StartTime, params.EndTime, params.Timezone, revenueAffiliateActionAccrue}
	trendArgs = append(trendArgs, trendUserArgs...)
	rows, err := s.entClient.QueryContext(ctx, trendQuery, trendArgs...)
	if err != nil {
		return fmt.Errorf("query revenue affiliate trend: %w", err)
	}
	defer func() { _ = rows.Close() }()
	for rows.Next() {
		var bucket string
		var rebate float64
		if err := rows.Scan(&bucket, &rebate); err != nil {
			return fmt.Errorf("scan revenue affiliate trend: %w", err)
		}
		if idx, ok := pointIndex[bucket]; ok {
			out.Trend[idx].AffiliateRebate = rebate
		}
	}
	if err := rows.Err(); err != nil {
		return fmt.Errorf("iterate revenue affiliate trend: %w", err)
	}
	return nil
}

func (s *RevenueService) fillRevenueShareStats(ctx context.Context, params RevenueQueryParams, out *RevenueSummary, pointIndex map[string]int) error {
	if shouldUseRevenueDailySnapshots(params) {
		return s.fillRevenueShareStatsFromSnapshots(ctx, params, out, pointIndex)
	}

	userFilter, userArgs := revenueUserFilter("consumer_user_id", params.UserID, 4)
	query := `
		SELECT
			COALESCE(SUM(consumer_charge), 0)::double precision AS consumer_charge,
			COALESCE(SUM(account_cost), 0)::double precision AS account_cost,
			COALESCE(SUM(owner_credit), 0)::double precision AS owner_credit,
			COALESCE(SUM(platform_fee), 0)::double precision AS platform_fee,
			COUNT(*) AS settlement_count
		FROM account_share_settlement_entries
		WHERE created_at >= $1 AND created_at < $2 AND status = $3
			AND consumer_user_id <> owner_user_id
	`
	query += userFilter
	args := []any{params.StartTime, params.EndTime, revenueShareStatusApplied}
	args = append(args, userArgs...)
	if err := s.querySingle(ctx, query, args,
		&out.Adjustments.ShareConsumerCharge,
		&out.Adjustments.ShareAccountCost,
		&out.Adjustments.ShareOwnerCredit,
		&out.Adjustments.SharePlatformFee,
		&out.Adjustments.ShareSettlementCount,
	); err != nil {
		return fmt.Errorf("query revenue share stats: %w", err)
	}

	bucketExpr := revenueBucketExpression("created_at", params.Granularity)
	trendUserFilter, trendUserArgs := revenueUserFilter("consumer_user_id", params.UserID, 5)
	trendQuery := fmt.Sprintf(`
		SELECT
			%s AS bucket,
			COALESCE(SUM(owner_credit), 0)::double precision AS owner_credit,
			COALESCE(SUM(platform_fee), 0)::double precision AS platform_fee
		FROM account_share_settlement_entries
		WHERE created_at >= $1 AND created_at < $2 AND status = $4
			AND consumer_user_id <> owner_user_id
			%s
		GROUP BY 1
		ORDER BY 1
	`, bucketExpr, trendUserFilter)
	trendArgs := []any{params.StartTime, params.EndTime, params.Timezone, revenueShareStatusApplied}
	trendArgs = append(trendArgs, trendUserArgs...)
	rows, err := s.entClient.QueryContext(ctx, trendQuery, trendArgs...)
	if err != nil {
		return fmt.Errorf("query revenue share trend: %w", err)
	}
	defer func() { _ = rows.Close() }()
	for rows.Next() {
		var bucket string
		var ownerCredit, platformFee float64
		if err := rows.Scan(&bucket, &ownerCredit, &platformFee); err != nil {
			return fmt.Errorf("scan revenue share trend: %w", err)
		}
		if idx, ok := pointIndex[bucket]; ok {
			out.Trend[idx].ShareOwnerCredit = ownerCredit
			out.Trend[idx].SharePlatformFee = platformFee
		}
	}
	if err := rows.Err(); err != nil {
		return fmt.Errorf("iterate revenue share trend: %w", err)
	}
	return nil
}

func (s *RevenueService) fillRevenueShareStatsFromSnapshots(ctx context.Context, params RevenueQueryParams, out *RevenueSummary, pointIndex map[string]int) error {
	startDate, endDate := revenueSnapshotDateRange(params)
	statsSnapshotUserFilter := revenueSnapshotUserFilter("s.user_id", params.UserID, 5)
	statsLiveUserFilter := revenueSnapshotUserFilter("ase.consumer_user_id", params.UserID, 5)
	statsStatusPlaceholder := nextRevenuePlaceholder(params.UserID, 5)
	query := fmt.Sprintf(`
		WITH snapshot_days AS (
			SELECT DISTINCT bucket_date
			FROM revenue_daily_dimension_snapshots
			WHERE bucket_date >= $1::date AND bucket_date < $2::date
				%s
		),
		combined AS (
			SELECT
				s.bucket_date,
				SUM(s.share_consumer_charge)::double precision AS consumer_charge,
				SUM(s.share_account_cost)::double precision AS account_cost,
				SUM(s.share_owner_credit)::double precision AS owner_credit,
				SUM(s.share_platform_fee)::double precision AS platform_fee,
				SUM(CASE WHEN s.share_owner_credit > 0 OR s.share_platform_fee > 0 THEN s.total_requests ELSE 0 END)::bigint AS settlement_count
			FROM revenue_daily_dimension_snapshots s
			WHERE s.bucket_date >= $1::date AND s.bucket_date < $2::date
				%s
			GROUP BY s.bucket_date
			UNION ALL
			SELECT
				(ase.created_at AT TIME ZONE 'Asia/Shanghai')::date AS bucket_date,
				COALESCE(SUM(ase.consumer_charge), 0)::double precision AS consumer_charge,
				COALESCE(SUM(ase.account_cost), 0)::double precision AS account_cost,
				COALESCE(SUM(ase.owner_credit), 0)::double precision AS owner_credit,
				COALESCE(SUM(ase.platform_fee), 0)::double precision AS platform_fee,
				COUNT(*)::bigint AS settlement_count
			FROM account_share_settlement_entries ase
			WHERE ase.created_at >= $3 AND ase.created_at < $4 AND ase.status = $%d
				AND ase.consumer_user_id <> ase.owner_user_id
				AND NOT EXISTS (
					SELECT 1
					FROM snapshot_days sd
					WHERE sd.bucket_date = (ase.created_at AT TIME ZONE 'Asia/Shanghai')::date
				)
				%s
			GROUP BY 1
		)
		SELECT
			COALESCE(SUM(consumer_charge), 0)::double precision,
			COALESCE(SUM(account_cost), 0)::double precision,
			COALESCE(SUM(owner_credit), 0)::double precision,
			COALESCE(SUM(platform_fee), 0)::double precision,
			COALESCE(SUM(settlement_count), 0)::bigint
		FROM combined
	`, statsSnapshotUserFilter, statsSnapshotUserFilter, statsStatusPlaceholder, statsLiveUserFilter)
	args := []any{startDate, endDate, params.StartTime, params.EndTime}
	if params.UserID != nil {
		args = append(args, *params.UserID)
	}
	args = append(args, revenueShareStatusApplied)
	if err := s.querySingle(ctx, query, args,
		&out.Adjustments.ShareConsumerCharge,
		&out.Adjustments.ShareAccountCost,
		&out.Adjustments.ShareOwnerCredit,
		&out.Adjustments.SharePlatformFee,
		&out.Adjustments.ShareSettlementCount,
	); err != nil {
		return fmt.Errorf("query revenue share snapshot stats: %w", err)
	}

	trendSnapshotUserFilter := revenueSnapshotUserFilter("s.user_id", params.UserID, 6)
	trendLiveUserFilter := revenueSnapshotUserFilter("ase.consumer_user_id", params.UserID, 6)
	trendStatusPlaceholder := nextRevenuePlaceholder(params.UserID, 6)
	trendQuery := fmt.Sprintf(`
		WITH snapshot_days AS (
			SELECT DISTINCT bucket_date
			FROM revenue_daily_dimension_snapshots
			WHERE bucket_date >= $1::date AND bucket_date < $2::date
				%s
		),
		combined AS (
			SELECT
				TO_CHAR(s.bucket_date::timestamp, 'YYYY-MM-DD') AS bucket,
				SUM(s.share_owner_credit)::double precision AS owner_credit,
				SUM(s.share_platform_fee)::double precision AS platform_fee
			FROM revenue_daily_dimension_snapshots s
			WHERE s.bucket_date >= $1::date AND s.bucket_date < $2::date
				%s
			GROUP BY 1
			UNION ALL
			SELECT
				TO_CHAR(ase.created_at AT TIME ZONE $5, 'YYYY-MM-DD') AS bucket,
				COALESCE(SUM(ase.owner_credit), 0)::double precision AS owner_credit,
				COALESCE(SUM(ase.platform_fee), 0)::double precision AS platform_fee
			FROM account_share_settlement_entries ase
			WHERE ase.created_at >= $3 AND ase.created_at < $4 AND ase.status = $%d
				AND ase.consumer_user_id <> ase.owner_user_id
				AND NOT EXISTS (
					SELECT 1
					FROM snapshot_days sd
					WHERE sd.bucket_date = (ase.created_at AT TIME ZONE 'Asia/Shanghai')::date
				)
				%s
			GROUP BY 1
		)
		SELECT
			bucket,
			COALESCE(SUM(owner_credit), 0)::double precision,
			COALESCE(SUM(platform_fee), 0)::double precision
		FROM combined
		GROUP BY bucket
		ORDER BY bucket
	`, trendSnapshotUserFilter, trendSnapshotUserFilter, trendStatusPlaceholder, trendLiveUserFilter)
	trendArgs := []any{startDate, endDate, params.StartTime, params.EndTime, params.Timezone}
	if params.UserID != nil {
		trendArgs = append(trendArgs, *params.UserID)
	}
	trendArgs = append(trendArgs, revenueShareStatusApplied)
	rows, err := s.entClient.QueryContext(ctx, trendQuery, trendArgs...)
	if err != nil {
		return fmt.Errorf("query revenue share snapshot trend: %w", err)
	}
	defer func() { _ = rows.Close() }()
	for rows.Next() {
		var bucket string
		var ownerCredit, platformFee float64
		if err := rows.Scan(&bucket, &ownerCredit, &platformFee); err != nil {
			return fmt.Errorf("scan revenue share snapshot trend: %w", err)
		}
		if idx, ok := pointIndex[bucket]; ok {
			out.Trend[idx].ShareOwnerCredit = ownerCredit
			out.Trend[idx].SharePlatformFee = platformFee
		}
	}
	if err := rows.Err(); err != nil {
		return fmt.Errorf("iterate revenue share snapshot trend: %w", err)
	}
	return nil
}

func (s *RevenueService) fillRevenuePrivateGroupCommissionStats(ctx context.Context, params RevenueQueryParams, out *RevenueSummary, pointIndex map[string]int) error {
	userFilter, userArgs := revenueUserFilter("user_id", params.UserID, 4)
	query := `
		SELECT COALESCE(SUM(amount), 0)::double precision
		FROM user_balance_ledger
		WHERE created_at >= $1 AND created_at < $2 AND reason = $3
	`
	query += userFilter
	args := []any{params.StartTime, params.EndTime, "private_group_commission"}
	args = append(args, userArgs...)
	if err := s.querySingle(ctx, query, args,
		&out.Adjustments.PrivateGroupCommission,
	); err != nil {
		return fmt.Errorf("query revenue private group commission stats: %w", err)
	}

	bucketExpr := revenueBucketExpression("created_at", params.Granularity)
	trendUserFilter, trendUserArgs := revenueUserFilter("user_id", params.UserID, 5)
	trendQuery := fmt.Sprintf(`
		SELECT %s AS bucket, COALESCE(SUM(amount), 0)::double precision AS commission
		FROM user_balance_ledger
		WHERE created_at >= $1 AND created_at < $2 AND reason = $4
			%s
		GROUP BY 1
		ORDER BY 1
	`, bucketExpr, trendUserFilter)
	trendArgs := []any{params.StartTime, params.EndTime, params.Timezone, "private_group_commission"}
	trendArgs = append(trendArgs, trendUserArgs...)
	rows, err := s.entClient.QueryContext(ctx, trendQuery, trendArgs...)
	if err != nil {
		return fmt.Errorf("query revenue private group commission trend: %w", err)
	}
	defer func() { _ = rows.Close() }()
	for rows.Next() {
		var bucket string
		var commission float64
		if err := rows.Scan(&bucket, &commission); err != nil {
			return fmt.Errorf("scan revenue private group commission trend: %w", err)
		}
		if idx, ok := pointIndex[bucket]; ok {
			out.Trend[idx].PrivateGroupCommission = commission
		}
	}
	if err := rows.Err(); err != nil {
		return fmt.Errorf("iterate revenue private group commission trend: %w", err)
	}
	return nil
}

func finalizeRevenueSummary(out *RevenueSummary) {
	out.Cash.PaidAmount = roundRevenue(out.Cash.PaidAmount)
	out.Cash.BalancePaidAmount = roundRevenue(out.Cash.BalancePaidAmount)
	out.Cash.SubscriptionPaidAmount = roundRevenue(out.Cash.SubscriptionPaidAmount)
	out.Cash.RedeemBalanceAmount = roundRevenue(out.Cash.RedeemBalanceAmount)
	out.Cash.RefundAmount = roundRevenue(out.Cash.RefundAmount)
	out.Cash.PendingAmount = roundRevenue(out.Cash.PendingAmount)
	out.Cash.NetPaidAmount = roundRevenue(out.Cash.PaidAmount + out.Cash.RedeemBalanceAmount - out.Cash.RefundAmount)

	out.Usage.StandardCost = roundRevenue(out.Usage.StandardCost)
	out.Usage.ConsumedRevenue = roundRevenue(out.Usage.ConsumedRevenue)
	out.Usage.BalanceConsumedAmount = roundRevenue(out.Usage.BalanceConsumedAmount)
	out.Usage.PointsConsumedAmount = roundRevenue(out.Usage.PointsConsumedAmount)
	out.Usage.PointsIssuedAmount = roundRevenue(out.Usage.PointsIssuedAmount)
	out.Usage.AccountCost = roundRevenue(out.Usage.AccountCost)

	out.Adjustments.AffiliateRebate = roundRevenue(out.Adjustments.AffiliateRebate)
	out.Adjustments.AffiliateTransfer = roundRevenue(out.Adjustments.AffiliateTransfer)
	out.Adjustments.PrivateGroupCommission = roundRevenue(out.Adjustments.PrivateGroupCommission)
	out.Adjustments.ShareConsumerCharge = roundRevenue(out.Adjustments.ShareConsumerCharge)
	out.Adjustments.ShareAccountCost = roundRevenue(out.Adjustments.ShareAccountCost)
	out.Adjustments.ShareOwnerCredit = roundRevenue(out.Adjustments.ShareOwnerCredit)
	out.Adjustments.SharePlatformFee = roundRevenue(out.Adjustments.SharePlatformFee)
	out.Adjustments.ShareNetProfit = roundRevenue(out.Adjustments.SharePlatformFee - out.Adjustments.ShareAccountCost)

	out.Profit.UsageGrossProfit = roundRevenue(out.Usage.ConsumedRevenue - out.Usage.AccountCost)
	out.Profit.UsageGrossMargin = marginRatio(out.Profit.UsageGrossProfit, out.Usage.ConsumedRevenue)
	out.Profit.EstimatedNetProfit = roundRevenue(out.Usage.ConsumedRevenue - out.Usage.AccountCost - out.Adjustments.AffiliateRebate - out.Adjustments.ShareOwnerCredit + out.Adjustments.PrivateGroupCommission)
	out.Profit.EstimatedNetMargin = marginRatio(out.Profit.EstimatedNetProfit, out.Usage.ConsumedRevenue)

	for i := range out.Trend {
		p := &out.Trend[i]
		p.PaidAmount = roundRevenue(p.PaidAmount)
		p.RedeemBalanceAmount = roundRevenue(p.RedeemBalanceAmount)
		p.RefundAmount = roundRevenue(p.RefundAmount)
		p.NetPaidAmount = roundRevenue(p.PaidAmount + p.RedeemBalanceAmount - p.RefundAmount)
		p.ConsumedRevenue = roundRevenue(p.ConsumedRevenue)
		p.BalanceConsumedAmount = roundRevenue(p.BalanceConsumedAmount)
		p.PointsConsumedAmount = roundRevenue(p.PointsConsumedAmount)
		p.PointsIssuedAmount = roundRevenue(p.PointsIssuedAmount)
		p.AccountCost = roundRevenue(p.AccountCost)
		p.UsageGrossProfit = roundRevenue(p.ConsumedRevenue - p.AccountCost)
		p.AffiliateRebate = roundRevenue(p.AffiliateRebate)
		p.PrivateGroupCommission = roundRevenue(p.PrivateGroupCommission)
		p.ShareOwnerCredit = roundRevenue(p.ShareOwnerCredit)
		p.SharePlatformFee = roundRevenue(p.SharePlatformFee)
		p.EstimatedNetProfit = roundRevenue(p.ConsumedRevenue - p.AccountCost - p.AffiliateRebate - p.ShareOwnerCredit + p.PrivateGroupCommission)
	}
}

func marginRatio(numerator, denominator float64) float64 {
	if denominator <= 0 {
		return 0
	}
	return roundRevenue(numerator / denominator)
}

func roundRevenue(v float64) float64 {
	return math.Round(v*1_000_000) / 1_000_000
}

func (s *RevenueService) querySingle(ctx context.Context, query string, args []any, dest ...any) error {
	rows, err := s.entClient.QueryContext(ctx, query, args...)
	if err != nil {
		return err
	}
	defer func() { _ = rows.Close() }()
	if !rows.Next() {
		if err := rows.Err(); err != nil {
			return err
		}
		return sql.ErrNoRows
	}
	if err := rows.Scan(dest...); err != nil {
		return err
	}
	if err := rows.Err(); err != nil {
		return err
	}
	return nil
}

type revenueBreakdownKind int

const (
	revenueBreakdownUsers revenueBreakdownKind = iota
	revenueBreakdownGroups
	revenueBreakdownAccounts
	revenueBreakdownModels
)

func (s *RevenueService) queryRevenueBreakdown(ctx context.Context, params RevenueQueryParams, kind revenueBreakdownKind) ([]RevenueBreakdownItem, error) {
	if shouldUseRevenueDailySnapshots(params) {
		return s.queryRevenueBreakdownFromSnapshots(ctx, params, kind)
	}

	const accountCostExpr = "COALESCE(ul.account_stats_cost, ul.total_cost) * COALESCE(ul.account_rate_multiplier, 1)"
	var selectExpr string
	var joinExpr string
	var groupExpr string
	var orderExpr = "consumed_revenue DESC, requests DESC"

	switch kind {
	case revenueBreakdownUsers:
		selectExpr = "ul.user_id AS id, COALESCE(NULLIF(u.email, ''), 'unknown') AS name, COALESCE(NULLIF(u.username, ''), '') AS secondary"
		joinExpr = "LEFT JOIN users u ON u.id = ul.user_id"
		groupExpr = "ul.user_id, u.email, u.username"
	case revenueBreakdownGroups:
		selectExpr = "COALESCE(ul.group_id, 0) AS id, COALESCE(NULLIF(g.name, ''), 'No Group') AS name, COALESCE(NULLIF(g.platform, ''), '') AS secondary"
		joinExpr = "LEFT JOIN groups g ON g.id = ul.group_id"
		groupExpr = "ul.group_id, g.name, g.platform"
	case revenueBreakdownAccounts:
		selectExpr = "ul.account_id AS id, COALESCE(NULLIF(a.name, ''), CONCAT('Account #', ul.account_id::text)) AS name, COALESCE(NULLIF(a.platform, ''), '') AS secondary"
		joinExpr = "LEFT JOIN accounts a ON a.id = ul.account_id"
		groupExpr = "ul.account_id, a.name, a.platform"
	case revenueBreakdownModels:
		selectExpr = "0 AS id, COALESCE(NULLIF(TRIM(ul.requested_model), ''), NULLIF(TRIM(ul.model), ''), 'unknown') AS name, '' AS secondary"
		groupExpr = "COALESCE(NULLIF(TRIM(ul.requested_model), ''), NULLIF(TRIM(ul.model), ''), 'unknown')"
	default:
		return nil, infraerrors.BadRequest("REVENUE_BREAKDOWN_INVALID", "invalid breakdown kind")
	}

	userFilter, userArgs := revenueUserFilter("ul.user_id", params.UserID, 5)
	query := fmt.Sprintf(`
		SELECT
			%s,
			COUNT(*) AS requests,
			COALESCE(SUM(ul.input_tokens + ul.output_tokens + ul.cache_creation_tokens + ul.cache_read_tokens), 0) AS total_tokens,
			COALESCE(SUM(ul.actual_cost), 0)::double precision AS consumed_revenue,
			COALESCE(SUM(%s), 0)::double precision AS account_cost,
			COALESCE(SUM(ase.owner_credit), 0)::double precision AS share_owner_credit
		FROM usage_logs ul
		%s
		LEFT JOIN account_share_settlement_entries ase ON ase.usage_log_id = ul.id
			AND ase.status = $4
			AND ase.consumer_user_id <> ase.owner_user_id
		WHERE ul.created_at >= $1 AND ul.created_at < $2
			%s
		GROUP BY %s
		ORDER BY %s
		LIMIT $3
	`, selectExpr, accountCostExpr, joinExpr, userFilter, groupExpr, orderExpr)

	args := []any{params.StartTime, params.EndTime, params.TopLimit, revenueShareStatusApplied}
	args = append(args, userArgs...)
	rows, err := s.entClient.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, fmt.Errorf("query revenue breakdown: %w", err)
	}
	defer func() { _ = rows.Close() }()

	items := make([]RevenueBreakdownItem, 0)
	for rows.Next() {
		var item RevenueBreakdownItem
		if err := rows.Scan(
			&item.ID,
			&item.Name,
			&item.Secondary,
			&item.Requests,
			&item.TotalTokens,
			&item.ConsumedRevenue,
			&item.AccountCost,
			&item.ShareOwnerCredit,
		); err != nil {
			return nil, fmt.Errorf("scan revenue breakdown: %w", err)
		}
		item.ConsumedRevenue = roundRevenue(item.ConsumedRevenue)
		item.AccountCost = roundRevenue(item.AccountCost)
		item.ShareOwnerCredit = roundRevenue(item.ShareOwnerCredit)
		item.GrossProfit = roundRevenue(item.ConsumedRevenue - item.AccountCost)
		item.GrossMargin = marginRatio(item.GrossProfit, item.ConsumedRevenue)
		item.NetProfit = roundRevenue(item.ConsumedRevenue - item.AccountCost - item.ShareOwnerCredit)
		item.NetMargin = marginRatio(item.NetProfit, item.ConsumedRevenue)
		items = append(items, item)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate revenue breakdown: %w", err)
	}
	return items, nil
}

func (s *RevenueService) queryRevenueBreakdownFromSnapshots(ctx context.Context, params RevenueQueryParams, kind revenueBreakdownKind) ([]RevenueBreakdownItem, error) {
	startDate, endDate := revenueSnapshotDateRange(params)
	var snapshotSelectExpr string
	var snapshotGroupExpr string
	var liveSelectExpr string
	var liveGroupExpr string
	var joinExpr string

	switch kind {
	case revenueBreakdownUsers:
		snapshotSelectExpr = "s.user_id AS id"
		snapshotGroupExpr = "s.user_id"
		liveSelectExpr = "ul.user_id AS id"
		liveGroupExpr = "ul.user_id"
		joinExpr = "LEFT JOIN users u ON u.id = rolled.id"
	case revenueBreakdownGroups:
		snapshotSelectExpr = "COALESCE(s.group_id, 0) AS id"
		snapshotGroupExpr = "COALESCE(s.group_id, 0)"
		liveSelectExpr = "COALESCE(ul.group_id, 0) AS id"
		liveGroupExpr = "COALESCE(ul.group_id, 0)"
		joinExpr = "LEFT JOIN groups g ON g.id = rolled.id"
	case revenueBreakdownAccounts:
		snapshotSelectExpr = "s.account_id AS id"
		snapshotGroupExpr = "s.account_id"
		liveSelectExpr = "ul.account_id AS id"
		liveGroupExpr = "ul.account_id"
		joinExpr = "LEFT JOIN accounts a ON a.id = rolled.id"
	case revenueBreakdownModels:
		snapshotSelectExpr = "0 AS id, COALESCE(NULLIF(TRIM(s.requested_model), ''), NULLIF(TRIM(s.model), ''), 'unknown') AS model_name"
		snapshotGroupExpr = "COALESCE(NULLIF(TRIM(s.requested_model), ''), NULLIF(TRIM(s.model), ''), 'unknown')"
		liveSelectExpr = "0 AS id, COALESCE(NULLIF(TRIM(ul.requested_model), ''), NULLIF(TRIM(ul.model), ''), 'unknown') AS model_name"
		liveGroupExpr = "COALESCE(NULLIF(TRIM(ul.requested_model), ''), NULLIF(TRIM(ul.model), ''), 'unknown')"
	default:
		return nil, infraerrors.BadRequest("REVENUE_BREAKDOWN_INVALID", "invalid breakdown kind")
	}

	statsSnapshotUserFilter := revenueSnapshotUserFilter("s.user_id", params.UserID, 6)
	statsLiveUserFilter := revenueSnapshotUserFilter("ul.user_id", params.UserID, 6)
	var query string
	if kind == revenueBreakdownModels {
		query = fmt.Sprintf(`
			WITH snapshot_days AS (
				SELECT DISTINCT bucket_date
				FROM revenue_daily_dimension_snapshots
				WHERE bucket_date >= $1::date AND bucket_date < $2::date
					%s
			),
			combined AS (
				SELECT
					%s,
					SUM(s.total_requests)::bigint AS requests,
					SUM(s.total_tokens)::bigint AS total_tokens,
					SUM(s.consumed_revenue)::double precision AS consumed_revenue,
					SUM(s.account_cost)::double precision AS account_cost,
					SUM(s.share_owner_credit)::double precision AS share_owner_credit
				FROM revenue_daily_dimension_snapshots s
				WHERE s.bucket_date >= $1::date AND s.bucket_date < $2::date
					%s
				GROUP BY %s
				UNION ALL
				SELECT
					%s,
					COUNT(*)::bigint AS requests,
					COALESCE(SUM(ul.input_tokens + ul.output_tokens + ul.cache_creation_tokens + ul.cache_read_tokens), 0)::bigint AS total_tokens,
					COALESCE(SUM(ul.actual_cost), 0)::double precision AS consumed_revenue,
					COALESCE(SUM(COALESCE(ul.account_stats_cost, ul.total_cost) * COALESCE(ul.account_rate_multiplier, 1)), 0)::double precision AS account_cost,
					COALESCE(SUM(ase.owner_credit), 0)::double precision AS share_owner_credit
				FROM usage_logs ul
				LEFT JOIN account_share_settlement_entries ase ON ase.usage_log_id = ul.id
					AND ase.status = $5
					AND ase.consumer_user_id <> ase.owner_user_id
				WHERE ul.created_at >= $3 AND ul.created_at < $4
					AND NOT EXISTS (
						SELECT 1
						FROM snapshot_days sd
						WHERE sd.bucket_date = (ul.created_at AT TIME ZONE 'Asia/Shanghai')::date
					)
					%s
				GROUP BY %s
			)
			SELECT *
			FROM (
				SELECT
					id,
					model_name AS name,
					'' AS secondary,
					COALESCE(SUM(requests), 0)::bigint AS requests,
					COALESCE(SUM(total_tokens), 0)::bigint AS total_tokens,
					COALESCE(SUM(consumed_revenue), 0)::double precision AS consumed_revenue,
					COALESCE(SUM(account_cost), 0)::double precision AS account_cost,
					COALESCE(SUM(share_owner_credit), 0)::double precision AS share_owner_credit
				FROM combined
				GROUP BY id, model_name
			) rolled_models
			ORDER BY rolled_models.consumed_revenue DESC, rolled_models.requests DESC
			LIMIT $%d
		`, statsSnapshotUserFilter, snapshotSelectExpr, statsSnapshotUserFilter, snapshotGroupExpr, liveSelectExpr, statsLiveUserFilter, liveGroupExpr, nextRevenuePlaceholder(params.UserID, 6))
	} else {
		nameExpr, secondaryExpr := revenueBreakdownNameExpressions(kind)
		query = fmt.Sprintf(`
			WITH snapshot_days AS (
				SELECT DISTINCT bucket_date
				FROM revenue_daily_dimension_snapshots
				WHERE bucket_date >= $1::date AND bucket_date < $2::date
					%s
			),
			combined AS (
				SELECT
					%s,
					SUM(s.total_requests)::bigint AS requests,
					SUM(s.total_tokens)::bigint AS total_tokens,
					SUM(s.consumed_revenue)::double precision AS consumed_revenue,
					SUM(s.account_cost)::double precision AS account_cost,
					SUM(s.share_owner_credit)::double precision AS share_owner_credit
				FROM revenue_daily_dimension_snapshots s
				WHERE s.bucket_date >= $1::date AND s.bucket_date < $2::date
					%s
				GROUP BY %s
				UNION ALL
				SELECT
					%s,
					COUNT(*)::bigint AS requests,
					COALESCE(SUM(ul.input_tokens + ul.output_tokens + ul.cache_creation_tokens + ul.cache_read_tokens), 0)::bigint AS total_tokens,
					COALESCE(SUM(ul.actual_cost), 0)::double precision AS consumed_revenue,
					COALESCE(SUM(COALESCE(ul.account_stats_cost, ul.total_cost) * COALESCE(ul.account_rate_multiplier, 1)), 0)::double precision AS account_cost,
					COALESCE(SUM(ase.owner_credit), 0)::double precision AS share_owner_credit
				FROM usage_logs ul
				LEFT JOIN account_share_settlement_entries ase ON ase.usage_log_id = ul.id
					AND ase.status = $5
					AND ase.consumer_user_id <> ase.owner_user_id
				WHERE ul.created_at >= $3 AND ul.created_at < $4
					AND NOT EXISTS (
						SELECT 1
						FROM snapshot_days sd
						WHERE sd.bucket_date = (ul.created_at AT TIME ZONE 'Asia/Shanghai')::date
					)
					%s
				GROUP BY %s
			),
			rolled AS (
				SELECT
					id,
					COALESCE(SUM(requests), 0)::bigint AS requests,
					COALESCE(SUM(total_tokens), 0)::bigint AS total_tokens,
					COALESCE(SUM(consumed_revenue), 0)::double precision AS consumed_revenue,
					COALESCE(SUM(account_cost), 0)::double precision AS account_cost,
					COALESCE(SUM(share_owner_credit), 0)::double precision AS share_owner_credit
				FROM combined
				GROUP BY id
			)
			SELECT
				rolled.id,
				%s AS name,
				%s AS secondary,
				rolled.requests,
				rolled.total_tokens,
				rolled.consumed_revenue,
				rolled.account_cost,
				rolled.share_owner_credit
			FROM rolled
			%s
			ORDER BY rolled.consumed_revenue DESC, rolled.requests DESC
			LIMIT $%d
		`, statsSnapshotUserFilter, snapshotSelectExpr, statsSnapshotUserFilter, snapshotGroupExpr, liveSelectExpr, statsLiveUserFilter, liveGroupExpr, nameExpr, secondaryExpr, joinExpr, nextRevenuePlaceholder(params.UserID, 6))
	}

	args := []any{startDate, endDate, params.StartTime, params.EndTime, revenueShareStatusApplied}
	if params.UserID != nil {
		args = append(args, *params.UserID)
	}
	args = append(args, params.TopLimit)
	rows, err := s.entClient.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, fmt.Errorf("query revenue snapshot breakdown: %w", err)
	}
	defer func() { _ = rows.Close() }()

	items := make([]RevenueBreakdownItem, 0)
	for rows.Next() {
		var item RevenueBreakdownItem
		if err := rows.Scan(
			&item.ID,
			&item.Name,
			&item.Secondary,
			&item.Requests,
			&item.TotalTokens,
			&item.ConsumedRevenue,
			&item.AccountCost,
			&item.ShareOwnerCredit,
		); err != nil {
			return nil, fmt.Errorf("scan revenue snapshot breakdown: %w", err)
		}
		item.ConsumedRevenue = roundRevenue(item.ConsumedRevenue)
		item.AccountCost = roundRevenue(item.AccountCost)
		item.ShareOwnerCredit = roundRevenue(item.ShareOwnerCredit)
		item.GrossProfit = roundRevenue(item.ConsumedRevenue - item.AccountCost)
		item.GrossMargin = marginRatio(item.GrossProfit, item.ConsumedRevenue)
		item.NetProfit = roundRevenue(item.ConsumedRevenue - item.AccountCost - item.ShareOwnerCredit)
		item.NetMargin = marginRatio(item.NetProfit, item.ConsumedRevenue)
		items = append(items, item)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate revenue snapshot breakdown: %w", err)
	}
	return items, nil
}

func revenueBreakdownNameExpressions(kind revenueBreakdownKind) (string, string) {
	switch kind {
	case revenueBreakdownUsers:
		return "COALESCE(NULLIF(u.email, ''), 'unknown')", "COALESCE(NULLIF(u.username, ''), '')"
	case revenueBreakdownGroups:
		return "COALESCE(NULLIF(g.name, ''), 'No Group')", "COALESCE(NULLIF(g.platform, ''), '')"
	case revenueBreakdownAccounts:
		return "COALESCE(NULLIF(a.name, ''), CONCAT('Account #', rolled.id::text))", "COALESCE(NULLIF(a.platform, ''), '')"
	default:
		return "''", "''"
	}
}

func nextRevenuePlaceholder(userID *int64, base int) int {
	if userID != nil {
		return base + 1
	}
	return base
}

func (s *RevenueService) queryRevenueShareOwnerBreakdown(ctx context.Context, params RevenueQueryParams) ([]RevenueShareOwnerBreakdownItem, error) {
	if shouldUseRevenueDailySnapshots(params) {
		return s.queryRevenueShareOwnerBreakdownFromSnapshots(ctx, params)
	}

	userFilter, userArgs := revenueUserFilter("ase.consumer_user_id", params.UserID, 5)
	query := `
		SELECT
			ase.owner_user_id AS id,
			COALESCE(NULLIF(u.email, ''), 'unknown') AS name,
			COALESCE(NULLIF(u.username, ''), '') AS secondary,
			COUNT(*) AS requests,
			COALESCE(SUM(COALESCE(ul.input_tokens, 0) + COALESCE(ul.output_tokens, 0) + COALESCE(ul.cache_creation_tokens, 0) + COALESCE(ul.cache_read_tokens, 0)), 0) AS total_tokens,
			COALESCE(SUM(ase.consumer_charge), 0)::double precision AS consumer_charge,
			COALESCE(SUM(ase.account_cost), 0)::double precision AS account_cost,
			COALESCE(SUM(ase.owner_credit), 0)::double precision AS owner_credit,
			COALESCE(SUM(ase.platform_fee), 0)::double precision AS platform_fee,
			CASE
				WHEN COALESCE(SUM(ase.consumer_charge), 0) > 0
				THEN (COALESCE(SUM(ase.owner_credit), 0) / COALESCE(SUM(ase.consumer_charge), 0))::double precision
				ELSE 0::double precision
			END AS owner_share_ratio
		FROM account_share_settlement_entries ase
		LEFT JOIN users u ON u.id = ase.owner_user_id
		LEFT JOIN usage_logs ul ON ul.id = ase.usage_log_id
		WHERE ase.created_at >= $1 AND ase.created_at < $2
			AND ase.status = $3
			AND ase.consumer_user_id <> ase.owner_user_id
			%s
		GROUP BY ase.owner_user_id, u.email, u.username
		ORDER BY owner_credit DESC, requests DESC
		LIMIT $4
	`
	query = fmt.Sprintf(query, userFilter)
	args := []any{params.StartTime, params.EndTime, revenueShareStatusApplied, params.TopLimit}
	args = append(args, userArgs...)
	rows, err := s.entClient.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, fmt.Errorf("query revenue share owner breakdown: %w", err)
	}
	defer func() { _ = rows.Close() }()

	items := make([]RevenueShareOwnerBreakdownItem, 0)
	for rows.Next() {
		var item RevenueShareOwnerBreakdownItem
		if err := rows.Scan(
			&item.ID,
			&item.Name,
			&item.Secondary,
			&item.Requests,
			&item.TotalTokens,
			&item.ConsumerCharge,
			&item.AccountCost,
			&item.OwnerCredit,
			&item.PlatformFee,
			&item.OwnerShareRatio,
		); err != nil {
			return nil, fmt.Errorf("scan revenue share owner breakdown: %w", err)
		}
		item.ConsumerCharge = roundRevenue(item.ConsumerCharge)
		item.AccountCost = roundRevenue(item.AccountCost)
		item.OwnerCredit = roundRevenue(item.OwnerCredit)
		item.PlatformFee = roundRevenue(item.PlatformFee)
		item.OwnerShareRatio = marginRatio(item.OwnerCredit, item.ConsumerCharge)
		items = append(items, item)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate revenue share owner breakdown: %w", err)
	}
	return items, nil
}

func (s *RevenueService) queryRevenueShareOwnerBreakdownFromSnapshots(ctx context.Context, params RevenueQueryParams) ([]RevenueShareOwnerBreakdownItem, error) {
	startDate, endDate := revenueSnapshotDateRange(params)
	snapshotUserFilter := revenueSnapshotUserFilter("s.user_id", params.UserID, 6)
	liveUserFilter := revenueSnapshotUserFilter("ase.consumer_user_id", params.UserID, 6)
	query := fmt.Sprintf(`
		WITH snapshot_days AS (
			SELECT DISTINCT bucket_date
			FROM revenue_daily_dimension_snapshots
			WHERE bucket_date >= $1::date AND bucket_date < $2::date
				%s
		),
		combined AS (
			SELECT
				s.owner_user_id AS id,
				SUM(CASE WHEN s.share_owner_credit > 0 OR s.share_platform_fee > 0 THEN s.total_requests ELSE 0 END)::bigint AS requests,
				SUM(CASE WHEN s.share_owner_credit > 0 OR s.share_platform_fee > 0 THEN s.total_tokens ELSE 0 END)::bigint AS total_tokens,
				SUM(s.share_consumer_charge)::double precision AS consumer_charge,
				SUM(s.share_account_cost)::double precision AS account_cost,
				SUM(s.share_owner_credit)::double precision AS owner_credit,
				SUM(s.share_platform_fee)::double precision AS platform_fee
			FROM revenue_daily_dimension_snapshots s
			WHERE s.bucket_date >= $1::date AND s.bucket_date < $2::date
				AND s.owner_user_id > 0
				%s
			GROUP BY s.owner_user_id
			UNION ALL
			SELECT
				ase.owner_user_id AS id,
				COUNT(*)::bigint AS requests,
				COALESCE(SUM(COALESCE(ul.input_tokens, 0) + COALESCE(ul.output_tokens, 0) + COALESCE(ul.cache_creation_tokens, 0) + COALESCE(ul.cache_read_tokens, 0)), 0)::bigint AS total_tokens,
				COALESCE(SUM(ase.consumer_charge), 0)::double precision AS consumer_charge,
				COALESCE(SUM(ase.account_cost), 0)::double precision AS account_cost,
				COALESCE(SUM(ase.owner_credit), 0)::double precision AS owner_credit,
				COALESCE(SUM(ase.platform_fee), 0)::double precision AS platform_fee
			FROM account_share_settlement_entries ase
			LEFT JOIN usage_logs ul ON ul.id = ase.usage_log_id
			WHERE ase.created_at >= $3 AND ase.created_at < $4
				AND ase.status = $5
				AND ase.consumer_user_id <> ase.owner_user_id
				AND NOT EXISTS (
					SELECT 1
					FROM snapshot_days sd
					WHERE sd.bucket_date = (ase.created_at AT TIME ZONE 'Asia/Shanghai')::date
				)
				%s
			GROUP BY ase.owner_user_id
		),
		rolled AS (
			SELECT
				id,
				COALESCE(SUM(requests), 0)::bigint AS requests,
				COALESCE(SUM(total_tokens), 0)::bigint AS total_tokens,
				COALESCE(SUM(consumer_charge), 0)::double precision AS consumer_charge,
				COALESCE(SUM(account_cost), 0)::double precision AS account_cost,
				COALESCE(SUM(owner_credit), 0)::double precision AS owner_credit,
				COALESCE(SUM(platform_fee), 0)::double precision AS platform_fee
			FROM combined
			GROUP BY id
		)
		SELECT
			rolled.id,
			COALESCE(NULLIF(u.email, ''), 'unknown') AS name,
			COALESCE(NULLIF(u.username, ''), '') AS secondary,
			rolled.requests,
			rolled.total_tokens,
			rolled.consumer_charge,
			rolled.account_cost,
			rolled.owner_credit,
			rolled.platform_fee,
			CASE
				WHEN rolled.consumer_charge > 0 THEN (rolled.owner_credit / rolled.consumer_charge)::double precision
				ELSE 0::double precision
			END AS owner_share_ratio
		FROM rolled
		LEFT JOIN users u ON u.id = rolled.id
		ORDER BY rolled.owner_credit DESC, rolled.requests DESC
		LIMIT $%d
	`, snapshotUserFilter, snapshotUserFilter, liveUserFilter, nextRevenuePlaceholder(params.UserID, 6))
	args := []any{startDate, endDate, params.StartTime, params.EndTime, revenueShareStatusApplied}
	if params.UserID != nil {
		args = append(args, *params.UserID)
	}
	args = append(args, params.TopLimit)
	rows, err := s.entClient.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, fmt.Errorf("query revenue share owner snapshot breakdown: %w", err)
	}
	defer func() { _ = rows.Close() }()

	items := make([]RevenueShareOwnerBreakdownItem, 0)
	for rows.Next() {
		var item RevenueShareOwnerBreakdownItem
		if err := rows.Scan(
			&item.ID,
			&item.Name,
			&item.Secondary,
			&item.Requests,
			&item.TotalTokens,
			&item.ConsumerCharge,
			&item.AccountCost,
			&item.OwnerCredit,
			&item.PlatformFee,
			&item.OwnerShareRatio,
		); err != nil {
			return nil, fmt.Errorf("scan revenue share owner snapshot breakdown: %w", err)
		}
		item.ConsumerCharge = roundRevenue(item.ConsumerCharge)
		item.AccountCost = roundRevenue(item.AccountCost)
		item.OwnerCredit = roundRevenue(item.OwnerCredit)
		item.PlatformFee = roundRevenue(item.PlatformFee)
		item.OwnerShareRatio = marginRatio(item.OwnerCredit, item.ConsumerCharge)
		items = append(items, item)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate revenue share owner snapshot breakdown: %w", err)
	}
	return items, nil
}
