package service

import (
	"context"
	"math"
	"testing"
	"time"
)

func TestComputeCarpoolExternalUsageDeltaBaselinesFirstSample(t *testing.T) {
	resetAt := time.Now().Add(5 * time.Hour).UTC()

	got := computeCarpoolExternalUsageDelta(0, nil, nil, 100, &resetAt, 25, 5)

	if !got.Valid {
		t.Fatal("expected valid computation")
	}
	if diff := math.Abs(got.CurrentExternalUSD - 20); diff > 0.000001 {
		t.Fatalf("current external = %v, want 20", got.CurrentExternalUSD)
	}
	if got.DeltaUSD != 0 {
		t.Fatalf("first sample delta = %v, want 0", got.DeltaUSD)
	}
	if got.ResetChanged {
		t.Fatal("first sample should not mark reset changed")
	}
}

func TestComputeCarpoolExternalUsageDeltaCanChargeFirstSample(t *testing.T) {
	resetAt := time.Now().Add(7 * 24 * time.Hour).UTC()

	got := computeCarpoolExternalUsageDelta(0, nil, nil, 100, &resetAt, 33.5, 0, true)

	if !got.Valid {
		t.Fatal("expected valid computation")
	}
	if diff := math.Abs(got.CurrentExternalUSD - 33.5); diff > 0.000001 {
		t.Fatalf("current external = %v, want 33.5", got.CurrentExternalUSD)
	}
	if diff := math.Abs(got.DeltaUSD - 33.5); diff > 0.000001 {
		t.Fatalf("delta = %v, want 33.5", got.DeltaUSD)
	}
	if got.ResetChanged {
		t.Fatal("first sample should not mark reset changed")
	}
}

func TestComputeCarpoolExternalUsageDeltaUsesPointLimitWhenNoUSDLimit(t *testing.T) {
	resetAt := time.Now().Add(5 * time.Hour).UTC()
	checkedAt := time.Now().Add(-time.Hour).UTC()

	got := computeCarpoolExternalUsageDelta(
		0,
		&resetAt,
		&checkedAt,
		carpoolExternalUsageLimit(0),
		&resetAt,
		3,
		carpoolExternalUsageInternal(0, 99),
	)

	if !got.Valid {
		t.Fatal("expected valid point-based computation")
	}
	if diff := math.Abs(got.CurrentExternalUSD - 3); diff > 0.000001 {
		t.Fatalf("current external = %v, want 3", got.CurrentExternalUSD)
	}
	if diff := math.Abs(got.DeltaUSD - 3); diff > 0.000001 {
		t.Fatalf("delta = %v, want 3", got.DeltaUSD)
	}
}

func TestComputeCarpoolExternalUsageDeltaOnlyChargesIncreaseInSameWindow(t *testing.T) {
	resetAt := time.Now().Add(5 * time.Hour).UTC()
	checkedAt := time.Now().Add(-time.Hour).UTC()

	got := computeCarpoolExternalUsageDelta(10, &resetAt, &checkedAt, 100, &resetAt, 35, 20)

	if diff := math.Abs(got.CurrentExternalUSD - 15); diff > 0.000001 {
		t.Fatalf("current external = %v, want 15", got.CurrentExternalUSD)
	}
	if diff := math.Abs(got.DeltaUSD - 5); diff > 0.000001 {
		t.Fatalf("delta = %v, want 5", got.DeltaUSD)
	}
	if got.ResetChanged {
		t.Fatal("same window should not mark reset changed")
	}
}

func TestComputeCarpoolExternalUsageDeltaResetsBaselineWhenWindowChanges(t *testing.T) {
	oldResetAt := time.Now().Add(-time.Hour).UTC()
	newResetAt := time.Now().Add(5 * time.Hour).UTC()
	checkedAt := time.Now().Add(-time.Hour).UTC()

	got := computeCarpoolExternalUsageDelta(30, &oldResetAt, &checkedAt, 100, &newResetAt, 12, 2)

	if diff := math.Abs(got.CurrentExternalUSD - 10); diff > 0.000001 {
		t.Fatalf("current external = %v, want 10", got.CurrentExternalUSD)
	}
	if diff := math.Abs(got.DeltaUSD - 10); diff > 0.000001 {
		t.Fatalf("delta = %v, want 10", got.DeltaUSD)
	}
	if !got.ResetChanged {
		t.Fatal("changed window should mark reset changed")
	}
}

func TestNormalizeCarpoolMemberUsageWindowClearsExpiredFiveHourWindow(t *testing.T) {
	start := time.Date(2026, 6, 12, 8, 0, 0, 0, time.UTC)
	member := CarpoolMember{
		FiveHourUsedUSD:     8,
		FiveHourWindowStart: &start,
	}

	got := normalizeCarpoolMemberUsageWindow(member, start.Add(5*time.Hour))

	if got.FiveHourUsedUSD != 0 {
		t.Fatalf("five-hour usage = %v, want 0", got.FiveHourUsedUSD)
	}
	if got.FiveHourWindowStart != nil {
		t.Fatal("expired five-hour window start should be cleared")
	}
}

func TestAttachCarpoolMemberUsageWindowsUsesMemberScopedUsage(t *testing.T) {
	start := time.Date(2026, 6, 13, 1, 0, 0, 0, time.UTC)
	weeklyResetAt := start.Add(7 * 24 * time.Hour)
	service := &CarpoolService{}
	profiles := []CarpoolMemberProfile{
		{
			Member: CarpoolMember{
				UserID:              466,
				FiveHourUsedUSD:     1.25,
				FiveHourLimitUSD:    5,
				FiveHourWindowStart: &start,
			},
			WeeklyUsageUSD: 2.5,
			WeeklyLimitUSD: 10,
			WeeklyResetAt:  &weeklyResetAt,
		},
	}

	service.attachCarpoolMemberUsageWindows(
		context.Background(),
		&CarpoolPool{ID: 12, TargetSeats: 3},
		[]CarpoolPoolAccount{{PoolID: 12, AccountID: 6459}},
		profiles,
	)

	if len(profiles[0].UsageWindows) != 2 {
		t.Fatalf("usage windows = %d, want 2", len(profiles[0].UsageWindows))
	}
	fiveHour := profiles[0].UsageWindows[0]
	if fiveHour.Window != "5h" || fiveHour.UsedPoints != 1.25 || fiveHour.LimitPoints != 100 {
		t.Fatalf("5h window = %+v, want member-scoped used=1.25 display limit=100", fiveHour)
	}
	weekly := profiles[0].UsageWindows[1]
	if weekly.Window != "7d" || weekly.UsedPoints != 2.5 || weekly.LimitPoints != 100 {
		t.Fatalf("7d window = %+v, want member-scoped used=2.5 display limit=100", weekly)
	}
}

func TestCarpoolNormalizedMemberUsedPointsMapsPoolShareToUserDisplay(t *testing.T) {
	got := carpoolNormalizedMemberUsedPoints(
		10,
		100,
		100.0/3.0,
		30,
		100,
		true,
	)

	if diff := math.Abs(got - 100); diff > 0.000001 {
		t.Fatalf("normalized usage = %v, want 100", got)
	}
}
