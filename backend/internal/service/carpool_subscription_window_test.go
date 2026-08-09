package service

import (
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

func TestCarpoolMemberSubscriptionWindowIsPermanent(t *testing.T) {
	paidAt := time.Now().UTC().Add(-24 * time.Hour)
	pool := &CarpoolPool{
		DurationDays: 1,
		CreatedAt:    paidAt,
	}
	member := CarpoolMember{PaidConfirmedAt: &paidAt}

	startAt, expiresAt := carpoolMemberSubscriptionWindow(pool, member)

	require.Equal(t, paidAt, startAt)
	require.Equal(t, MaxExpiresAt, expiresAt)
}
