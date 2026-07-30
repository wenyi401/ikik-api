package repository

import (
	"context"
	"database/sql"
	"fmt"
	"strings"

	"ikik-api/internal/service"
)

type riskGroupPenaltyQueryer interface {
	QueryContext(ctx context.Context, query string, args ...any) (*sql.Rows, error)
}

func attachActiveRiskGroupBlocks(ctx context.Context, queryer riskGroupPenaltyQueryer, users ...*service.User) error {
	if queryer == nil || len(users) == 0 {
		return nil
	}
	byID := make(map[int64][]*service.User, len(users))
	userIDs := make([]int64, 0, len(users))
	for _, user := range users {
		if user == nil || user.ID <= 0 {
			continue
		}
		if _, exists := byID[user.ID]; !exists {
			userIDs = append(userIDs, user.ID)
		}
		byID[user.ID] = append(byID[user.ID], user)
		user.RiskGroupBlocks = nil
	}
	if len(userIDs) == 0 {
		return nil
	}

	placeholders := make([]string, len(userIDs))
	args := make([]any, len(userIDs))
	for i, userID := range userIDs {
		placeholders[i] = fmt.Sprintf("$%d", i+1)
		args[i] = userID
	}
	rows, err := queryer.QueryContext(ctx, `
SELECT user_id, group_id, blocked_until, permanent
FROM content_moderation_user_group_penalties
WHERE user_id IN (`+strings.Join(placeholders, ", ")+`)
  AND (permanent = TRUE OR blocked_until > CURRENT_TIMESTAMP)
ORDER BY user_id, group_id
`, args...)
	if err != nil {
		if sqliteMissingRiskGroupPenaltyTable(queryer, err) {
			return nil
		}
		return fmt.Errorf("load active content moderation group penalties: %w", err)
	}
	defer func() { _ = rows.Close() }()

	for rows.Next() {
		var userID, groupID int64
		var blockedUntil sql.NullTime
		var permanent bool
		if err := rows.Scan(&userID, &groupID, &blockedUntil, &permanent); err != nil {
			return fmt.Errorf("scan active content moderation group penalty: %w", err)
		}
		block := service.UserRiskGroupBlock{GroupID: groupID, Permanent: permanent}
		if blockedUntil.Valid {
			value := blockedUntil.Time
			block.BlockedUntil = &value
		}
		for _, user := range byID[userID] {
			user.RiskGroupBlocks = append(user.RiskGroupBlocks, block)
		}
	}
	if err := rows.Err(); err != nil {
		return fmt.Errorf("iterate active content moderation group penalties: %w", err)
	}
	return nil
}

func sqliteMissingRiskGroupPenaltyTable(queryer riskGroupPenaltyQueryer, err error) bool {
	if err == nil || !strings.Contains(strings.ToLower(err.Error()), "no such table") {
		return false
	}
	db, ok := queryer.(*sql.DB)
	if !ok || db.Driver() == nil {
		return false
	}
	return strings.Contains(strings.ToLower(fmt.Sprintf("%T", db.Driver())), "sqlite")
}
