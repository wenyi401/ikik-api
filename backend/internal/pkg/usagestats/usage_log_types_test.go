package usagestats

import (
	"encoding/json"
	"testing"
)

func TestIsValidModelSource(t *testing.T) {
	tests := []struct {
		name   string
		source string
		want   bool
	}{
		{name: "requested", source: ModelSourceRequested, want: true},
		{name: "upstream", source: ModelSourceUpstream, want: true},
		{name: "mapping", source: ModelSourceMapping, want: true},
		{name: "invalid", source: "foobar", want: false},
		{name: "empty", source: "", want: false},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			if got := IsValidModelSource(tc.source); got != tc.want {
				t.Fatalf("IsValidModelSource(%q)=%v want %v", tc.source, got, tc.want)
			}
		})
	}
}

func TestNormalizeModelSource(t *testing.T) {
	tests := []struct {
		name   string
		source string
		want   string
	}{
		{name: "requested", source: ModelSourceRequested, want: ModelSourceRequested},
		{name: "upstream", source: ModelSourceUpstream, want: ModelSourceUpstream},
		{name: "mapping", source: ModelSourceMapping, want: ModelSourceMapping},
		{name: "invalid falls back", source: "foobar", want: ModelSourceRequested},
		{name: "empty falls back", source: "", want: ModelSourceRequested},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			if got := NormalizeModelSource(tc.source); got != tc.want {
				t.Fatalf("NormalizeModelSource(%q)=%q want %q", tc.source, got, tc.want)
			}
		})
	}
}

func TestUserDashboardStatsJSONKeepsPlatformContracts(t *testing.T) {
	stats := UserDashboardStats{
		TodayPlatforms: []DashboardPlatformUsage{{
			Platform:            "openai",
			Requests:            2,
			InputTokens:         10,
			OutputTokens:        20,
			CacheCreationTokens: 30,
			CacheReadTokens:     40,
			TotalTokens:         100,
			Cost:                1.25,
			ActualCost:          0.75,
		}},
		ByPlatform: []PlatformDashboardStats{{
			Platform:        "openai",
			TotalRequests:   4,
			TotalTokens:     200,
			TotalActualCost: 1.5,
			TodayRequests:   2,
			TodayTokens:     100,
			TodayActualCost: 0.75,
		}},
	}

	encoded, err := json.Marshal(stats)
	if err != nil {
		t.Fatalf("marshal UserDashboardStats: %v", err)
	}

	var payload map[string]json.RawMessage
	if err := json.Unmarshal(encoded, &payload); err != nil {
		t.Fatalf("unmarshal UserDashboardStats payload: %v", err)
	}

	var todayRows []map[string]json.RawMessage
	if err := json.Unmarshal(payload["today_platforms"], &todayRows); err != nil {
		t.Fatalf("decode today_platforms: %v", err)
	}
	if len(todayRows) != 1 {
		t.Fatalf("today_platforms length=%d want 1", len(todayRows))
	}
	for _, field := range []string{
		"platform",
		"requests",
		"input_tokens",
		"output_tokens",
		"cache_creation_tokens",
		"cache_read_tokens",
		"total_tokens",
		"cost",
		"actual_cost",
	} {
		if _, ok := todayRows[0][field]; !ok {
			t.Errorf("today_platforms row missing %q", field)
		}
	}

	var platformRows []map[string]json.RawMessage
	if err := json.Unmarshal(payload["by_platform"], &platformRows); err != nil {
		t.Fatalf("decode by_platform: %v", err)
	}
	if len(platformRows) != 1 {
		t.Fatalf("by_platform length=%d want 1", len(platformRows))
	}
}
