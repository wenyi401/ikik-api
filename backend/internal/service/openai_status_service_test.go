package service

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

func TestOpenAIStatusServiceFiltersChatGPTAndCodexIncidents(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{
          "page":{"updated_at":"2026-07-24T00:00:00Z"},
          "components":[
            {"name":"Conversations","status":"operational"},
            {"name":"Codex Web","status":"partial_outage"}
          ],
          "incidents":[
            {"id":"chat","name":"Elevated errors affecting ChatGPT","status":"monitoring","impact":"minor","created_at":"2026-07-23T10:00:00Z","updated_at":"2026-07-23T11:00:00Z","incident_updates":[{"body":"Monitoring recovery for ChatGPT."}]},
            {"id":"codex","name":"Some users are unable to access Codex","status":"identified","impact":"major","created_at":"2026-07-23T09:00:00Z","updated_at":"2026-07-23T12:00:00Z","incident_updates":[{"body":"Codex CLI is affected."}]},
            {"id":"api","name":"Elevated API image errors","status":"investigating","impact":"major","created_at":"2026-07-23T08:00:00Z","updated_at":"2026-07-23T10:00:00Z","incident_updates":[{"body":"Image API requests are affected."}]},
            {"id":"resolved","name":"ChatGPT login issue","status":"resolved","impact":"minor","created_at":"2026-07-22T08:00:00Z","updated_at":"2026-07-22T09:00:00Z","resolved_at":"2026-07-22T09:00:00Z","incident_updates":[{"body":"ChatGPT recovered."}]}
          ]
        }`))
	}))
	defer server.Close()

	svc := NewOpenAIStatusService()
	svc.url = server.URL
	svc.now = func() time.Time { return time.Date(2026, 7, 24, 0, 0, 0, 0, time.UTC) }

	snapshot, err := svc.Get(context.Background())
	require.NoError(t, err)
	require.Len(t, snapshot.ActiveIncidents, 2)
	require.Equal(t, "codex", snapshot.ActiveIncidents[0].ID)
	require.Len(t, snapshot.RecentResolved, 1)
	require.Equal(t, []OpenAIStatusProduct{
		{ID: "chatgpt", Status: "operational", ActiveIncidents: 1},
		{ID: "codex", Status: "partial_outage", ActiveIncidents: 1},
	}, snapshot.Products)
}

func TestOpenAIStatusServiceKeepsAmbiguousActiveIncidentVisible(t *testing.T) {
	source := openAIStatusResponse{}
	source.Incidents = []openAIStatusIncident{{
		ID:        "global",
		Name:      "Elevated Error Rates",
		Status:    "monitoring",
		Impact:    "minor",
		CreatedAt: "2026-07-23T10:00:00Z",
		UpdatedAt: "2026-07-23T11:00:00Z",
		IncidentUpdates: []openAIStatusIncidentUpdate{{
			Body: "We are monitoring recovery for the listed services.",
		}},
	}}

	snapshot := buildOpenAIStatusSnapshot(source, time.Date(2026, 7, 24, 0, 0, 0, 0, time.UTC))
	require.Len(t, snapshot.ActiveIncidents, 1)
	require.Empty(t, snapshot.ActiveIncidents[0].Products)
	require.Equal(t, []OpenAIStatusProduct{
		{ID: "chatgpt", Status: "operational", ActiveIncidents: 0},
		{ID: "codex", Status: "operational", ActiveIncidents: 0},
	}, snapshot.Products)
}

func TestOpenAIStatusServiceReturnsStaleCacheWhenRefreshFails(t *testing.T) {
	fail := false
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if fail {
			http.Error(w, "failed", http.StatusBadGateway)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"page":{"updated_at":"2026-07-24T00:00:00Z"},"incidents":[]}`))
	}))
	defer server.Close()

	now := time.Date(2026, 7, 24, 0, 0, 0, 0, time.UTC)
	svc := NewOpenAIStatusService()
	svc.url = server.URL
	svc.now = func() time.Time { return now }

	first, err := svc.Get(context.Background())
	require.NoError(t, err)
	require.False(t, first.Stale)

	now = now.Add(2 * time.Minute)
	fail = true
	second, err := svc.Get(context.Background())
	require.NoError(t, err)
	require.True(t, second.Stale)
}
