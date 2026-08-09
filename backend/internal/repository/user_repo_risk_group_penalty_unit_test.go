package repository

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	"ikik-api/internal/pkg/pagination"
	"ikik-api/internal/service"
)

func TestUserRepositoryListAttachesActiveRiskGroupBlocks(t *testing.T) {
	apiKeyRepo, client := newAPIKeyRepoSQLite(t)
	ctx := context.Background()
	user := mustCreateAPIKeyRepoUser(t, ctx, client, "user-list-risk-block@test.com")
	group, err := client.Group.Create().
		SetName("user-list-risk-block").
		SetPlatform(service.PlatformOpenAI).
		SetStatus(service.StatusActive).
		SetSubscriptionType(service.SubscriptionTypeStandard).
		SetRateMultiplier(1).
		Save(ctx)
	require.NoError(t, err)

	blockedUntil := time.Now().Add(24 * time.Hour)
	_, err = apiKeyRepo.sql.ExecContext(ctx, `
INSERT INTO content_moderation_user_group_penalties (user_id, group_id, blocked_until, permanent)
VALUES ($1, $2, $3, FALSE)
`, user.ID, group.ID, blockedUntil)
	require.NoError(t, err)

	repo := newUserRepositoryWithSQL(client, apiKeyRepo.sql)
	users, _, err := repo.ListWithFilters(ctx, pagination.PaginationParams{Page: 1, PageSize: 20}, service.UserListFilters{
		Search:               user.Email,
		IncludeSubscriptions: riskGroupBlockBoolPtr(false),
	})
	require.NoError(t, err)
	require.Len(t, users, 1)
	require.Len(t, users[0].RiskGroupBlocks, 1)
	require.Equal(t, group.ID, users[0].RiskGroupBlocks[0].GroupID)
	require.True(t, users[0].IsGroupBlocked(group.ID))
}

func riskGroupBlockBoolPtr(value bool) *bool { return &value }
