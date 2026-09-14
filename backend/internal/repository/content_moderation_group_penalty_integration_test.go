//go:build integration

package repository

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"ikik-api/internal/service"
)

func TestContentModerationGroupPenaltyProgressionIntegration(t *testing.T) {
	ctx := context.Background()
	suffix := time.Now().UnixNano()
	user, err := integrationEntClient.User.Create().
		SetEmail(fmt.Sprintf("risk-penalty-%d@example.com", suffix)).
		SetPasswordHash("test-password-hash").
		SetRole(service.RoleUser).
		SetStatus(service.StatusActive).
		Save(ctx)
	require.NoError(t, err)
	group, err := integrationEntClient.Group.Create().
		SetName(fmt.Sprintf("risk-penalty-%d", suffix)).
		SetPlatform(service.PlatformOpenAI).
		SetRateMultiplier(1).
		SetStatus(service.StatusActive).
		Save(ctx)
	require.NoError(t, err)
	t.Cleanup(func() {
		_, _ = integrationDB.ExecContext(ctx, "DELETE FROM content_moderation_user_group_penalty_events WHERE user_id = $1", user.ID)
		_, _ = integrationDB.ExecContext(ctx, "DELETE FROM content_moderation_user_group_penalties WHERE user_id = $1", user.ID)
		_, _ = integrationDB.ExecContext(ctx, "DELETE FROM groups WHERE id = $1", group.ID)
		_, _ = integrationDB.ExecContext(ctx, "DELETE FROM users WHERE id = $1", user.ID)
	})

	repo := &contentModerationRepository{db: integrationDB}
	base := time.Date(2026, 7, 30, 8, 0, 0, 0, time.UTC)
	apply := func(requestID string, at time.Time) (*service.ContentModerationGroupPenalty, bool) {
		penalty, applied, applyErr := repo.ApplyUserGroupPenaltyForRisk(ctx, service.ContentModerationRiskEvent{
			RequestID: requestID,
			UserID:    user.ID,
			GroupID:   group.ID,
			Category:  service.ContentModerationRiskCategoryCheatAutomation,
			Score:     0.96,
			CreatedAt: at,
		}, 24, 36)
		require.NoError(t, applyErr)
		return penalty, applied
	}

	first, applied := apply("req-1", base)
	require.True(t, applied)
	require.Equal(t, 1, first.StrikeCount)
	require.Equal(t, base.Add(24*time.Hour), *first.BlockedUntil)

	duplicate, applied := apply("req-1", base)
	require.False(t, applied)
	require.Equal(t, 1, duplicate.StrikeCount)

	second, applied := apply("req-2", base.Add(time.Minute))
	require.True(t, applied)
	require.Equal(t, 2, second.StrikeCount)
	require.Equal(t, base.Add(time.Minute+36*time.Hour), *second.BlockedUntil)

	third, applied := apply("req-3", base.Add(2*time.Minute))
	require.True(t, applied)
	require.Equal(t, 3, third.StrikeCount)
	require.True(t, third.Permanent)
	require.Nil(t, third.BlockedUntil)

	admin, err := integrationEntClient.User.Create().
		SetEmail(fmt.Sprintf("risk-penalty-admin-%d@example.com", suffix)).
		SetPasswordHash("test-password-hash").
		SetRole(service.RoleAdmin).
		SetStatus(service.StatusActive).
		Save(ctx)
	require.NoError(t, err)
	t.Cleanup(func() { _, _ = integrationDB.ExecContext(ctx, "DELETE FROM users WHERE id = $1", admin.ID) })
	adminPenalty, adminApplied, err := repo.ApplyUserGroupPenaltyForRisk(ctx, service.ContentModerationRiskEvent{
		RequestID: "req-admin",
		UserID:    admin.ID,
		GroupID:   group.ID,
		Category:  service.ContentModerationRiskCategoryCheatAutomation,
		Score:     0.99,
		CreatedAt: base,
	}, 24, 36)
	require.NoError(t, err)
	require.False(t, adminApplied)
	require.Nil(t, adminPenalty)
}
