package service

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

func TestProviderStatusServiceAggregatesOfficialSources(t *testing.T) {
	mux := http.NewServeMux()
	mux.HandleFunc("/claude", func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{
          "page":{"updated_at":"2026-07-29T01:00:00Z"},
          "components":[
            {"id":"web","name":"claude.ai","status":"operational"},
            {"id":"api","name":"Claude API (api.anthropic.com)","status":"degraded_performance"},
            {"id":"code","name":"Claude Code","status":"operational"},
            {"id":"other","name":"Claude for Government","status":"major_outage"}
          ],
          "incidents":[{
            "id":"claude-api","name":"Elevated Claude API errors","status":"investigating","impact":"minor",
            "updated_at":"2026-07-29T01:02:00Z","components":[{"id":"api","name":"Claude API (api.anthropic.com)","status":"degraded_performance"}],
            "incident_updates":[{"body":"We are investigating elevated errors."}]
          }]
        }`))
	})
	mux.HandleFunc("/grok", func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/xml")
		_, _ = w.Write([]byte(`<?xml version="1.0"?><rss><channel>
          <lastBuildDate>Wed, 29 Jul 2026 01:05:00 GMT</lastBuildDate>
          <item><title>[API (us-east-1.api.x.ai)] Grok requests unavailable</title><link>https://status.x.ai/api-us-east-1/INC1</link><pubDate>Wed, 29 Jul 2026 01:04:00 GMT</pubDate><description><![CDATA[Status: INVESTIGATING Severity: unavailable Updates: We are investigating.]]></description></item>
          <item><title>[API (us-east-1.api.x.ai)] Imagine Video errors</title><link>https://status.x.ai/api-us-east-1/INC2</link><pubDate>Wed, 29 Jul 2026 01:03:00 GMT</pubDate><description><![CDATA[Status: INVESTIGATING Severity: unavailable Updates: Video only.]]></description></item>
          <item><title>Grok Web recovered</title><link>https://status.x.ai/grok-web/INC3</link><pubDate>Wed, 29 Jul 2026 01:02:00 GMT</pubDate><description><![CDATA[Status: RESOLVED Severity: available Resolved: Wed, 29 Jul 2026 01:02:00 GMT]]></description></item>
        </channel></rss>`))
	})
	mux.HandleFunc("/gemini", func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`[{
          "id":"gemini-active","external_desc":"Vertex Gemini API elevated errors","modified":"2026-07-29T01:06:00Z",
          "status_impact":"SERVICE_DISRUPTION","severity":"high","affected_products":[{"title":"Vertex Gemini API"}],
          "most_recent_update":{"when":"2026-07-29T01:06:00Z","text":"We are investigating elevated errors.","status":"SERVICE_DISRUPTION"}
        },{
          "id":"imagen-only","external_desc":"Imagen issue","status_impact":"SERVICE_OUTAGE","severity":"high",
          "affected_products":[{"title":"Vertex Imagen API"}],"most_recent_update":{"status":"SERVICE_OUTAGE"}
        }]`))
	})
	server := httptest.NewServer(mux)
	defer server.Close()

	svc := NewProviderStatusService()
	svc.claudeURL = server.URL + "/claude"
	svc.xAIURL = server.URL + "/grok"
	svc.geminiURL = server.URL + "/gemini"
	svc.now = func() time.Time { return time.Date(2026, 7, 29, 1, 10, 0, 0, time.UTC) }

	snapshot, err := svc.Get(context.Background())
	require.NoError(t, err)
	require.False(t, snapshot.Stale)
	require.Len(t, snapshot.Providers, 3)
	require.Equal(t, "degraded", snapshot.Providers[0].Status)
	require.Len(t, snapshot.Providers[0].Components, 3)
	require.Len(t, snapshot.Providers[0].ActiveIncidents, 1)
	require.Equal(t, "major_outage", snapshot.Providers[1].Status)
	require.Len(t, snapshot.Providers[1].ActiveIncidents, 1)
	require.Equal(t, "INC1", snapshot.Providers[1].ActiveIncidents[0].ID)
	require.Equal(t, "partial_outage", snapshot.Providers[2].Status)
	require.Len(t, snapshot.Providers[2].ActiveIncidents, 1)
}

func TestProviderStatusServiceKeepsProviderCacheOnPartialRefreshFailure(t *testing.T) {
	failClaude := false
	mux := http.NewServeMux()
	mux.HandleFunc("/claude", func(w http.ResponseWriter, _ *http.Request) {
		if failClaude {
			http.Error(w, "failed", http.StatusBadGateway)
			return
		}
		_, _ = w.Write([]byte(`{"page":{"updated_at":"2026-07-29T01:00:00Z"},"components":[{"id":"api","name":"Claude API (api.anthropic.com)","status":"operational"}]}`))
	})
	mux.HandleFunc("/grok", func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write([]byte(`<?xml version="1.0"?><rss><channel><lastBuildDate>Wed, 29 Jul 2026 01:00:00 GMT</lastBuildDate></channel></rss>`))
	})
	mux.HandleFunc("/gemini", func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write([]byte(`[]`))
	})
	server := httptest.NewServer(mux)
	defer server.Close()

	now := time.Date(2026, 7, 29, 1, 0, 0, 0, time.UTC)
	svc := NewProviderStatusService()
	svc.claudeURL = server.URL + "/claude"
	svc.xAIURL = server.URL + "/grok"
	svc.geminiURL = server.URL + "/gemini"
	svc.now = func() time.Time { return now }

	first, err := svc.Get(context.Background())
	require.NoError(t, err)
	require.False(t, first.Stale)

	now = now.Add(3 * time.Minute)
	failClaude = true
	second, err := svc.Get(context.Background())
	require.NoError(t, err)
	require.True(t, second.Stale)
	require.True(t, second.Providers[0].Stale)
	require.Equal(t, "operational", second.Providers[0].Status)
	require.False(t, second.Providers[1].Stale)
	require.False(t, second.Providers[2].Stale)
}

func TestProviderStatusServiceReturnsUnknownWithoutCache(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		http.Error(w, "failed", http.StatusBadGateway)
	}))
	defer server.Close()

	svc := NewProviderStatusService()
	svc.claudeURL = server.URL
	svc.xAIURL = server.URL
	svc.geminiURL = server.URL

	snapshot, err := svc.Get(context.Background())
	require.NoError(t, err)
	require.True(t, snapshot.Stale)
	require.Len(t, snapshot.Providers, 3)
	for _, provider := range snapshot.Providers {
		require.Equal(t, "unknown", provider.Status)
	}
}

func TestCloneProviderStatusSnapshotPreservesEmptyArrays(t *testing.T) {
	snapshot := &ProviderStatusSnapshot{
		Providers: []ProviderStatusProvider{{
			ID:              "claude",
			Components:      []ProviderStatusComponent{},
			ActiveIncidents: []ProviderStatusIncident{},
		}},
	}

	clone := cloneProviderStatusSnapshot(snapshot)
	require.NotNil(t, clone.Providers[0].Components)
	require.NotNil(t, clone.Providers[0].ActiveIncidents)

	payload, err := json.Marshal(clone)
	require.NoError(t, err)
	require.JSONEq(t, `{
		"fetched_at":"0001-01-01T00:00:00Z",
		"stale":false,
		"providers":[{
			"id":"claude",
			"status":"",
			"source_updated_at":"",
			"stale":false,
			"components":[],
			"active_incidents":[]
		}]
	}`, string(payload))
}
