package service

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"sort"
	"strings"
	"sync"
	"time"

	"golang.org/x/sync/singleflight"
)

const (
	openAIIncidentsURL      = "https://status.openai.com/api/v2/summary.json"
	openAIStatusCacheTTL    = time.Minute
	openAIStatusHistoryDays = 7
	openAIStatusMaxBody     = 2 << 20
)

type OpenAIStatusProduct struct {
	ID              string `json:"id"`
	Status          string `json:"status"`
	ActiveIncidents int    `json:"active_incidents"`
}

type OpenAIStatusIncident struct {
	ID         string   `json:"id"`
	Name       string   `json:"name"`
	Status     string   `json:"status"`
	Impact     string   `json:"impact"`
	Products   []string `json:"products"`
	CreatedAt  string   `json:"created_at"`
	UpdatedAt  string   `json:"updated_at"`
	ResolvedAt *string  `json:"resolved_at,omitempty"`
	LatestBody string   `json:"latest_body"`
	SourceURL  string   `json:"source_url"`
}

type OpenAIStatusSnapshot struct {
	Products        []OpenAIStatusProduct  `json:"products"`
	ActiveIncidents []OpenAIStatusIncident `json:"active_incidents"`
	RecentResolved  []OpenAIStatusIncident `json:"recent_resolved"`
	SourceUpdatedAt string                 `json:"source_updated_at"`
	FetchedAt       time.Time              `json:"fetched_at"`
	Stale           bool                   `json:"stale"`
}

type openAIStatusIncidentUpdate struct {
	Body      string `json:"body"`
	CreatedAt string `json:"created_at"`
	UpdatedAt string `json:"updated_at"`
}

type openAIStatusIncident struct {
	ID              string                       `json:"id"`
	Name            string                       `json:"name"`
	Status          string                       `json:"status"`
	Impact          string                       `json:"impact"`
	CreatedAt       string                       `json:"created_at"`
	UpdatedAt       string                       `json:"updated_at"`
	ResolvedAt      *string                      `json:"resolved_at"`
	IncidentUpdates []openAIStatusIncidentUpdate `json:"incident_updates"`
}

type openAIStatusResponse struct {
	Page struct {
		UpdatedAt string `json:"updated_at"`
	} `json:"page"`
	Components []openAIStatusComponent `json:"components"`
	Incidents  []openAIStatusIncident  `json:"incidents"`
}

type openAIStatusComponent struct {
	Name   string `json:"name"`
	Status string `json:"status"`
}

type OpenAIStatusService struct {
	client *http.Client
	url    string
	now    func() time.Time

	mu        sync.RWMutex
	cached    *OpenAIStatusSnapshot
	expiresAt time.Time
	refresh   singleflight.Group
}

func NewOpenAIStatusService() *OpenAIStatusService {
	return &OpenAIStatusService{
		client: &http.Client{Timeout: 8 * time.Second},
		url:    openAIIncidentsURL,
		now:    time.Now,
	}
}

func (s *OpenAIStatusService) Get(ctx context.Context) (*OpenAIStatusSnapshot, error) {
	if snapshot := s.freshSnapshot(); snapshot != nil {
		return snapshot, nil
	}

	value, err, _ := s.refresh.Do("openai-incidents", func() (any, error) {
		if snapshot := s.freshSnapshot(); snapshot != nil {
			return snapshot, nil
		}
		snapshot, fetchErr := s.fetch(ctx)
		if fetchErr != nil {
			if stale := s.staleSnapshot(); stale != nil {
				return stale, nil
			}
			return nil, fetchErr
		}

		s.mu.Lock()
		s.cached = cloneOpenAIStatusSnapshot(snapshot)
		s.expiresAt = s.now().Add(openAIStatusCacheTTL)
		s.mu.Unlock()
		return cloneOpenAIStatusSnapshot(snapshot), nil
	})
	if err != nil {
		return nil, err
	}
	snapshot, ok := value.(*OpenAIStatusSnapshot)
	if !ok || snapshot == nil {
		return nil, errors.New("invalid OpenAI status response")
	}
	return snapshot, nil
}

func (s *OpenAIStatusService) freshSnapshot() *OpenAIStatusSnapshot {
	s.mu.RLock()
	defer s.mu.RUnlock()
	if s.cached == nil || !s.now().Before(s.expiresAt) {
		return nil
	}
	return cloneOpenAIStatusSnapshot(s.cached)
}

func (s *OpenAIStatusService) staleSnapshot() *OpenAIStatusSnapshot {
	s.mu.RLock()
	defer s.mu.RUnlock()
	if s.cached == nil {
		return nil
	}
	result := cloneOpenAIStatusSnapshot(s.cached)
	result.Stale = true
	return result
}

func (s *OpenAIStatusService) fetch(ctx context.Context) (*OpenAIStatusSnapshot, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, s.url, nil)
	if err != nil {
		return nil, fmt.Errorf("create OpenAI status request: %w", err)
	}
	req.Header.Set("Accept", "application/json")
	req.Header.Set("User-Agent", "ikik-api-status/1.0")

	resp, err := s.client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("fetch OpenAI status: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		_, _ = io.Copy(io.Discard, io.LimitReader(resp.Body, 4096))
		return nil, fmt.Errorf("fetch OpenAI status: HTTP %d", resp.StatusCode)
	}

	var source openAIStatusResponse
	decoder := json.NewDecoder(io.LimitReader(resp.Body, openAIStatusMaxBody))
	if err := decoder.Decode(&source); err != nil {
		return nil, fmt.Errorf("decode OpenAI status: %w", err)
	}
	return buildOpenAIStatusSnapshot(source, s.now()), nil
}

func buildOpenAIStatusSnapshot(source openAIStatusResponse, now time.Time) *OpenAIStatusSnapshot {
	active := make([]OpenAIStatusIncident, 0)
	resolved := make([]OpenAIStatusIncident, 0)
	cutoff := now.AddDate(0, 0, -openAIStatusHistoryDays)

	for _, incident := range source.Incidents {
		products := classifyOpenAIStatusProducts(incident)
		if len(products) == 0 {
			// The official page can show a global active incident while ChatGPT and
			// Codex component groups remain operational. Preserve that incident without
			// assigning it to either product. Explicitly unrelated API incidents stay out.
			if strings.EqualFold(strings.TrimSpace(incident.Status), "resolved") ||
				isClearlyNonTargetOpenAIStatusIncident(incident) {
				continue
			}
		}
		mapped := mapOpenAIStatusIncident(incident, products)
		if !strings.EqualFold(strings.TrimSpace(incident.Status), "resolved") {
			active = append(active, mapped)
			continue
		}
		resolvedAt := firstParsedTime(incident.ResolvedAt, &incident.UpdatedAt)
		if resolvedAt.IsZero() || resolvedAt.Before(cutoff) {
			continue
		}
		resolved = append(resolved, mapped)
	}

	sort.SliceStable(active, func(i, j int) bool { return active[i].UpdatedAt > active[j].UpdatedAt })
	sort.SliceStable(resolved, func(i, j int) bool { return resolved[i].UpdatedAt > resolved[j].UpdatedAt })
	if len(resolved) > 12 {
		resolved = resolved[:12]
	}

	products := []OpenAIStatusProduct{
		buildOpenAIProductStatus("chatgpt", source.Components, active),
		buildOpenAIProductStatus("codex", source.Components, active),
	}
	return &OpenAIStatusSnapshot{
		Products:        products,
		ActiveIncidents: active,
		RecentResolved:  resolved,
		SourceUpdatedAt: source.Page.UpdatedAt,
		FetchedAt:       now.UTC(),
	}
}

func classifyOpenAIStatusProducts(incident openAIStatusIncident) []string {
	text := openAIStatusIncidentSearchText(incident)
	products := make([]string, 0, 2)
	if strings.Contains(text, "chatgpt") || strings.Contains(text, "chat gpt") {
		products = append(products, "chatgpt")
	}
	if strings.Contains(text, "codex") {
		products = append(products, "codex")
	}
	return products
}

func openAIStatusIncidentSearchText(incident openAIStatusIncident) string {
	var searchable strings.Builder
	searchable.WriteString(incident.Name)
	for _, update := range incident.IncidentUpdates {
		searchable.WriteByte('\n')
		searchable.WriteString(update.Body)
	}
	return strings.ToLower(searchable.String())
}

func isClearlyNonTargetOpenAIStatusIncident(incident openAIStatusIncident) bool {
	text := openAIStatusIncidentSearchText(incident)
	for _, marker := range []string{
		"api image", "image generation", "sora", "audio api", "realtime api",
		"moderation api", "embeddings api", "batch api", "fine-tuning api",
	} {
		if strings.Contains(text, marker) {
			return true
		}
	}
	return false
}

func mapOpenAIStatusIncident(incident openAIStatusIncident, products []string) OpenAIStatusIncident {
	latestBody := ""
	if len(incident.IncidentUpdates) > 0 {
		latestBody = strings.TrimSpace(incident.IncidentUpdates[0].Body)
	}
	return OpenAIStatusIncident{
		ID:         incident.ID,
		Name:       strings.TrimSpace(incident.Name),
		Status:     strings.TrimSpace(incident.Status),
		Impact:     strings.TrimSpace(incident.Impact),
		Products:   append([]string(nil), products...),
		CreatedAt:  incident.CreatedAt,
		UpdatedAt:  incident.UpdatedAt,
		ResolvedAt: incident.ResolvedAt,
		LatestBody: latestBody,
		SourceURL:  "https://status.openai.com/",
	}
}

func buildOpenAIProductStatus(product string, components []openAIStatusComponent, incidents []OpenAIStatusIncident) OpenAIStatusProduct {
	result := OpenAIStatusProduct{ID: product, Status: "operational"}
	severity := 0
	matchedComponent := false
	for _, component := range components {
		if !openAIComponentBelongsToProduct(product, component.Name) {
			continue
		}
		matchedComponent = true
		if candidate := openAIComponentSeverity(component.Status); candidate > severity {
			severity = candidate
		}
	}
	for _, incident := range incidents {
		if !containsString(incident.Products, product) {
			continue
		}
		result.ActiveIncidents++
		if strings.EqualFold(strings.TrimSpace(incident.Status), "monitoring") {
			continue
		}
		if !matchedComponent {
			if candidate := openAIImpactSeverity(incident.Impact); candidate > severity {
				severity = candidate
			}
		}
	}
	switch {
	case severity == 1:
		result.Status = "degraded"
	case severity == 2:
		result.Status = "partial_outage"
	case severity == 3:
		result.Status = "major_outage"
	}
	return result
}

func openAIComponentBelongsToProduct(product, name string) bool {
	component := strings.ToLower(strings.TrimSpace(name))
	var names map[string]struct{}
	switch product {
	case "chatgpt":
		names = map[string]struct{}{
			"conversations": {}, "search": {}, "gpts": {}, "chatgpt atlas": {},
			"chatgpt work": {}, "deep research": {}, "agent": {}, "voice mode": {},
			"file uploads": {}, "image generation": {},
		}
	case "codex":
		names = map[string]struct{}{
			"codex api": {}, "vs code extension": {}, "codex in chatgpt desktop": {},
			"codex web": {}, "cli": {},
		}
	default:
		return false
	}
	_, ok := names[component]
	return ok
}

func openAIComponentSeverity(status string) int {
	switch strings.ToLower(strings.TrimSpace(status)) {
	case "major_outage":
		return 3
	case "partial_outage":
		return 2
	case "degraded_performance", "under_maintenance":
		return 1
	default:
		return 0
	}
}

func openAIImpactSeverity(impact string) int {
	switch strings.ToLower(strings.TrimSpace(impact)) {
	case "critical":
		return 3
	case "major":
		return 2
	case "minor":
		return 1
	default:
		return 1
	}
}

func containsString(values []string, target string) bool {
	for _, value := range values {
		if value == target {
			return true
		}
	}
	return false
}

func firstParsedTime(values ...*string) time.Time {
	for _, value := range values {
		if value == nil || strings.TrimSpace(*value) == "" {
			continue
		}
		if parsed, err := time.Parse(time.RFC3339, *value); err == nil {
			return parsed
		}
	}
	return time.Time{}
}

func cloneOpenAIStatusSnapshot(snapshot *OpenAIStatusSnapshot) *OpenAIStatusSnapshot {
	if snapshot == nil {
		return nil
	}
	clone := *snapshot
	clone.Products = append([]OpenAIStatusProduct(nil), snapshot.Products...)
	clone.ActiveIncidents = cloneOpenAIStatusIncidents(snapshot.ActiveIncidents)
	clone.RecentResolved = cloneOpenAIStatusIncidents(snapshot.RecentResolved)
	return &clone
}

func cloneOpenAIStatusIncidents(values []OpenAIStatusIncident) []OpenAIStatusIncident {
	result := make([]OpenAIStatusIncident, len(values))
	copy(result, values)
	for i := range result {
		result[i].Products = append([]string(nil), values[i].Products...)
	}
	return result
}
