package service

import (
	"context"
	"encoding/json"
	"log/slog"
	"math"
	"sort"
	"strconv"
	"time"

	dbent "ikik-api/ent"
	"ikik-api/ent/paymentauditlog"
	"ikik-api/ent/paymentorder"

	"entgo.io/ent/dialect"
)

// --- Dashboard & Analytics ---

func (s *PaymentService) GetDashboardStats(ctx context.Context, days int) (*DashboardStats, error) {
	if days <= 0 {
		days = 30
	}
	now := time.Now()
	since := now.AddDate(0, 0, -days)
	todayStart := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, now.Location())

	paidStatuses := []string{OrderStatusCompleted, OrderStatusPaid, OrderStatusRecharging}

	orders, err := s.entClient.PaymentOrder.Query().
		Where(
			paymentorder.StatusIn(paidStatuses...),
			paymentorder.PaidAtGTE(since),
		).
		All(ctx)
	if err != nil {
		return nil, err
	}

	entries, err := s.dashboardRedeemEntries(ctx, since, now)
	if err != nil {
		return nil, err
	}
	entries = append(entries, dashboardEntriesFromOrders(orders)...)

	st := &DashboardStats{}
	computeBasicStats(st, entries, todayStart)

	st.PendingOrders, err = s.entClient.PaymentOrder.Query().
		Where(paymentorder.StatusEQ(OrderStatusPending)).
		Count(ctx)
	if err != nil {
		return nil, err
	}

	st.DailySeries = buildDailySeries(entries, since, days)
	st.PaymentMethods = buildMethodDistribution(entries)
	st.TopUsers = buildTopUsers(entries)

	return st, nil
}

type dashboardEntry struct {
	Amount float64
	At     time.Time
	Method string
	UserID int64
	Email  string
}

func dashboardEntriesFromOrders(orders []*dbent.PaymentOrder) []dashboardEntry {
	entries := make([]dashboardEntry, 0, len(orders))
	for _, o := range orders {
		if o == nil || o.PaidAt == nil {
			continue
		}
		entries = append(entries, dashboardEntry{
			Amount: o.PayAmount,
			At:     *o.PaidAt,
			Method: normalizePaymentDashboardMethod(o.PaymentType),
			UserID: o.UserID,
			Email:  o.UserEmail,
		})
	}
	return entries
}

func (s *PaymentService) dashboardRedeemEntries(ctx context.Context, since, until time.Time) ([]dashboardEntry, error) {
	if s == nil || s.entClient == nil {
		return nil, nil
	}

	typePredicate := "(rc.type = $5 OR rc.type = $6)"
	args := []any{since, until, StatusUsed, maxRevenueCashAdjustmentAmount, RedeemTypeBalance, AdjustmentTypeAdminBalance}
	query := `
		SELECT
			rc.value::double precision AS amount,
			rc.used_at,
			rc.type,
			COALESCE(rc.used_by, 0) AS user_id,
			COALESCE(u.email, '') AS email
		FROM redeem_codes rc
		LEFT JOIN users u ON u.id = rc.used_by
		WHERE rc.used_at >= $1
			AND rc.used_at < $2
			AND rc.status = $3
			AND rc.value > 0
			AND rc.value <= $4
			AND ` + typePredicate
	if s.entClient.Driver().Dialect() != dialect.Postgres {
		typePredicate = "(rc.type = ? OR rc.type = ?)"
		args = []any{since, until, StatusUsed, maxRevenueCashAdjustmentAmount, RedeemTypeBalance, AdjustmentTypeAdminBalance}
		query = `
			SELECT
				rc.value AS amount,
				rc.used_at,
				rc.type,
				COALESCE(rc.used_by, 0) AS user_id,
				COALESCE(u.email, '') AS email
			FROM redeem_codes rc
			LEFT JOIN users u ON u.id = rc.used_by
			WHERE rc.used_at >= ?
				AND rc.used_at < ?
				AND rc.status = ?
				AND rc.value > 0
				AND rc.value <= ?
				AND ` + typePredicate
	}

	rows, err := s.entClient.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	defer func() { _ = rows.Close() }()

	entries := make([]dashboardEntry, 0)
	for rows.Next() {
		var entry dashboardEntry
		var codeType string
		if err := rows.Scan(&entry.Amount, &entry.At, &codeType, &entry.UserID, &entry.Email); err != nil {
			return nil, err
		}
		entry.Method = redeemDashboardMethod(codeType)
		entries = append(entries, entry)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	return entries, nil
}

func redeemDashboardMethod(codeType string) string {
	if codeType == AdjustmentTypeAdminBalance {
		return "admin_balance"
	}
	return "redeem_code"
}

func normalizePaymentDashboardMethod(method string) string {
	if method == "manual" {
		return "admin_balance"
	}
	return method
}

func computeBasicStats(st *DashboardStats, entries []dashboardEntry, todayStart time.Time) {
	var totalAmount, todayAmount float64
	var todayCount int
	for _, entry := range entries {
		totalAmount += entry.Amount
		if !entry.At.Before(todayStart) {
			todayAmount += entry.Amount
			todayCount++
		}
	}
	st.TotalAmount = math.Round(totalAmount*100) / 100
	st.TodayAmount = math.Round(todayAmount*100) / 100
	st.TotalCount = len(entries)
	st.TodayCount = todayCount
	if st.TotalCount > 0 {
		st.AvgAmount = math.Round(totalAmount/float64(st.TotalCount)*100) / 100
	}
}

func buildDailySeries(entries []dashboardEntry, since time.Time, days int) []DailyStats {
	dailyMap := make(map[string]*DailyStats)
	for _, entry := range entries {
		date := entry.At.Format("2006-01-02")
		ds, ok := dailyMap[date]
		if !ok {
			ds = &DailyStats{Date: date}
			dailyMap[date] = ds
		}
		ds.Amount += entry.Amount
		ds.Count++
	}
	series := make([]DailyStats, 0, days)
	for i := 0; i < days; i++ {
		date := since.AddDate(0, 0, i+1).Format("2006-01-02")
		if ds, ok := dailyMap[date]; ok {
			ds.Amount = math.Round(ds.Amount*100) / 100
			series = append(series, *ds)
		} else {
			series = append(series, DailyStats{Date: date})
		}
	}
	return series
}

func buildMethodDistribution(entries []dashboardEntry) []PaymentMethodStat {
	methodMap := make(map[string]*PaymentMethodStat)
	for _, entry := range entries {
		ms, ok := methodMap[entry.Method]
		if !ok {
			ms = &PaymentMethodStat{Type: entry.Method}
			methodMap[entry.Method] = ms
		}
		ms.Amount += entry.Amount
		ms.Count++
	}
	methods := make([]PaymentMethodStat, 0, len(methodMap))
	for _, ms := range methodMap {
		ms.Amount = math.Round(ms.Amount*100) / 100
		methods = append(methods, *ms)
	}
	return methods
}

func buildTopUsers(entries []dashboardEntry) []TopUserStat {
	userMap := make(map[int64]*TopUserStat)
	for _, entry := range entries {
		us, ok := userMap[entry.UserID]
		if !ok {
			us = &TopUserStat{UserID: entry.UserID, Email: entry.Email}
			userMap[entry.UserID] = us
		}
		us.Amount += entry.Amount
	}
	userList := make([]*TopUserStat, 0, len(userMap))
	for _, us := range userMap {
		us.Amount = math.Round(us.Amount*100) / 100
		userList = append(userList, us)
	}
	sort.Slice(userList, func(i, j int) bool {
		return userList[i].Amount > userList[j].Amount
	})
	limit := topUsersLimit
	if len(userList) < limit {
		limit = len(userList)
	}
	result := make([]TopUserStat, 0, limit)
	for i := 0; i < limit; i++ {
		result = append(result, *userList[i])
	}
	return result
}

// --- Audit Logs ---

func (s *PaymentService) writeAuditLog(ctx context.Context, oid int64, action, op string, detail map[string]any) {
	dj, _ := json.Marshal(detail)
	_, err := s.entClient.PaymentAuditLog.Create().SetOrderID(strconv.FormatInt(oid, 10)).SetAction(action).SetDetail(string(dj)).SetOperator(op).Save(ctx)
	if err != nil {
		slog.Error("audit log failed", "orderID", oid, "action", action, "error", err)
	}
}

func (s *PaymentService) GetOrderAuditLogs(ctx context.Context, oid int64) ([]*dbent.PaymentAuditLog, error) {
	return s.entClient.PaymentAuditLog.Query().Where(paymentauditlog.OrderIDEQ(strconv.FormatInt(oid, 10))).Order(paymentauditlog.ByCreatedAt()).All(ctx)
}
