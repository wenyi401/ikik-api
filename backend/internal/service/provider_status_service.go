package service

import (
	"context"
	"encoding/json"
	"encoding/xml"
	"errors"
	"fmt"
	"html"
	"io"
	"net/http"
	"path"
	"regexp"
	"sort"
	"strings"
	"sync"
	"time"

	"golang.org/x/sync/singleflight"
)

const (
	claudeStatusURL        = "https://status.claude.com/api/v2/summary.json"
	xAIStatusFeedURL       = "https://status.x.ai/feed.xml"
	geminiStatusHistoryURL = "https://status.cloud.google.com/incidents.json"
	providerStatusCacheTTL = 2 * time.Minute
	providerStatusMaxBody  = 8 << 20
)

var providerStatusHTMLTagPattern = regexp.MustCompile(`<[^>]+>`)

type ProviderStatusComponent struct {
	ID     string `json:"id"`
	Name   string `json:"name"`
	Status string `json:"status"`
}

type ProviderStatusIncident struct {
	ID         string `json:"id"`
	Name       string `json:"name"`
	Status     string `json:"status"`
	Impact     string `json:"impact"`
	UpdatedAt  string `json:"updated_at"`
	LatestBody string `json:"latest_body"`
}

type ProviderStatusProvider struct {
	ID              string                    `json:"id"`
	Status          string                    `json:"status"`
	Components      []ProviderStatusComponent `json:"components"`
	ActiveIncidents []ProviderStatusIncident  `json:"active_incidents"`
	SourceUpdatedAt string                    `json:"source_updated_at"`
	Stale           bool                      `json:"stale"`
}

type ProviderStatusSnapshot struct {
	Providers []ProviderStatusProvider `json:"providers"`
	FetchedAt time.Time                `json:"fetched_at"`
	Stale     bool                     `json:"stale"`
}

type ProviderStatusService struct {
	client    *http.Client
	claudeURL string
	xAIURL    string
	geminiURL string
	now       func() time.Time

	mu        sync.RWMutex
	cached    *ProviderStatusSnapshot
	expiresAt time.Time
	refresh   singleflight.Group
}

func NewProviderStatusService() *ProviderStatusService {
	return &ProviderStatusService{
		client:    &http.Client{Timeout: 8 * time.Second},
		claudeURL: claudeStatusURL,
		xAIURL:    xAIStatusFeedURL,
		geminiURL: geminiStatusHistoryURL,
		now:       time.Now,
	}
}

func (s *ProviderStatusService) Get(ctx context.Context) (*ProviderStatusSnapshot, error) {
	if snapshot := s.freshSnapshot(); snapshot != nil {
		return snapshot, nil
	}

	value, err, _ := s.refresh.Do("provider-status", func() (any, error) {
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
		s.cached = cloneProviderStatusSnapshot(snapshot)
		s.expiresAt = s.now().Add(providerStatusCacheTTL)
		s.mu.Unlock()
		return cloneProviderStatusSnapshot(snapshot), nil
	})
	if err != nil {
		return nil, err
	}
	snapshot, ok := value.(*ProviderStatusSnapshot)
	if !ok || snapshot == nil {
		return nil, errors.New("invalid provider status response")
	}
	return snapshot, nil
}

func (s *ProviderStatusService) freshSnapshot() *ProviderStatusSnapshot {
	s.mu.RLock()
	defer s.mu.RUnlock()
	if s.cached == nil || !s.now().Before(s.expiresAt) {
		return nil
	}
	return cloneProviderStatusSnapshot(s.cached)
}

func (s *ProviderStatusService) staleSnapshot() *ProviderStatusSnapshot {
	s.mu.RLock()
	defer s.mu.RUnlock()
	if s.cached == nil {
		return nil
	}
	result := cloneProviderStatusSnapshot(s.cached)
	result.Stale = true
	for i := range result.Providers {
		result.Providers[i].Stale = true
	}
	return result
}

type providerStatusFetchResult struct {
	id       string
	provider ProviderStatusProvider
	err      error
}

func (s *ProviderStatusService) fetch(ctx context.Context) (*ProviderStatusSnapshot, error) {
	fetchers := []struct {
		id    string
		fetch func(context.Context) (ProviderStatusProvider, error)
	}{
		{id: "claude", fetch: s.fetchClaude},
		{id: "grok", fetch: s.fetchGrok},
		{id: "gemini", fetch: s.fetchGemini},
	}

	results := make(chan providerStatusFetchResult, len(fetchers))
	var wg sync.WaitGroup
	for _, item := range fetchers {
		item := item
		wg.Add(1)
		go func() {
			defer wg.Done()
			provider, err := item.fetch(ctx)
			results <- providerStatusFetchResult{id: item.id, provider: provider, err: err}
		}()
	}
	wg.Wait()
	close(results)

	if err := ctx.Err(); err != nil {
		return nil, err
	}

	byID := make(map[string]ProviderStatusProvider, len(fetchers))
	failed := make(map[string]struct{}, len(fetchers))
	for result := range results {
		if result.err != nil {
			failed[result.id] = struct{}{}
			continue
		}
		byID[result.id] = result.provider
	}

	cachedByID := make(map[string]ProviderStatusProvider)
	if cached := s.staleSnapshot(); cached != nil {
		for _, provider := range cached.Providers {
			cachedByID[provider.ID] = provider
		}
	}

	orderedIDs := []string{"claude", "grok", "gemini"}
	providers := make([]ProviderStatusProvider, 0, len(orderedIDs))
	stale := false
	for _, id := range orderedIDs {
		if provider, ok := byID[id]; ok {
			providers = append(providers, provider)
			continue
		}
		if provider, ok := cachedByID[id]; ok {
			provider.Stale = true
			providers = append(providers, provider)
			stale = true
			continue
		}
		providers = append(providers, unavailableProviderStatus(id))
	}
	return &ProviderStatusSnapshot{
		Providers: providers,
		FetchedAt: s.now().UTC(),
		Stale:     stale || len(failed) > 0,
	}, nil
}

func unavailableProviderStatus(id string) ProviderStatusProvider {
	components := map[string][]ProviderStatusComponent{
		"claude": {
			{ID: "claude_web", Name: "claude.ai", Status: "unknown"},
			{ID: "claude_api", Name: "Claude API", Status: "unknown"},
			{ID: "claude_code", Name: "Claude Code", Status: "unknown"},
		},
		"grok": {
			{ID: "grok_web", Name: "Grok Web", Status: "unknown"},
			{ID: "xai_api", Name: "xAI API", Status: "unknown"},
		},
		"gemini": {
			{ID: "gemini_api", Name: "Gemini API", Status: "unknown"},
		},
	}
	return ProviderStatusProvider{ID: id, Status: "unknown", Components: components[id]}
}

type providerStatuspageComponent struct {
	ID     string `json:"id"`
	Name   string `json:"name"`
	Status string `json:"status"`
}

type providerStatuspageIncident struct {
	ID              string                        `json:"id"`
	Name            string                        `json:"name"`
	Status          string                        `json:"status"`
	Impact          string                        `json:"impact"`
	UpdatedAt       string                        `json:"updated_at"`
	Components      []providerStatuspageComponent `json:"components"`
	IncidentUpdates []openAIStatusIncidentUpdate  `json:"incident_updates"`
}

type providerStatuspageResponse struct {
	Page struct {
		UpdatedAt string `json:"updated_at"`
	} `json:"page"`
	Components []providerStatuspageComponent `json:"components"`
	Incidents  []providerStatuspageIncident  `json:"incidents"`
}

func (s *ProviderStatusService) fetchClaude(ctx context.Context) (ProviderStatusProvider, error) {
	var source providerStatuspageResponse
	if err := s.fetchJSON(ctx, s.claudeURL, &source); err != nil {
		return ProviderStatusProvider{}, fmt.Errorf("fetch Claude status: %w", err)
	}

	componentIDs := map[string]string{
		"claude.ai":                      "claude_web",
		"claude api (api.anthropic.com)": "claude_api",
		"claude code":                    "claude_code",
	}
	components := make([]ProviderStatusComponent, 0, len(componentIDs))
	relevantComponentIDs := make(map[string]struct{}, len(componentIDs))
	severity := 0
	for _, component := range source.Components {
		id, ok := componentIDs[strings.ToLower(strings.TrimSpace(component.Name))]
		if !ok {
			continue
		}
		relevantComponentIDs[component.ID] = struct{}{}
		status := normalizeProviderComponentStatus(component.Status)
		components = append(components, ProviderStatusComponent{ID: id, Name: component.Name, Status: status})
		severity = maxProviderStatusSeverity(severity, providerStatusSeverity(status))
	}
	components = orderProviderComponents(components, []string{"claude_web", "claude_api", "claude_code"})
	if len(components) == 0 {
		components = unavailableProviderStatus("claude").Components
		severity = providerStatusSeverity("unknown")
	}

	incidents := make([]ProviderStatusIncident, 0)
	for _, incident := range source.Incidents {
		if strings.EqualFold(strings.TrimSpace(incident.Status), "resolved") ||
			!statuspageIncidentAffectsComponents(incident, relevantComponentIDs) {
			continue
		}
		latestBody := ""
		if len(incident.IncidentUpdates) > 0 {
			latestBody = strings.TrimSpace(incident.IncidentUpdates[0].Body)
		}
		incidents = append(incidents, ProviderStatusIncident{
			ID:         incident.ID,
			Name:       strings.TrimSpace(incident.Name),
			Status:     strings.TrimSpace(incident.Status),
			Impact:     strings.TrimSpace(incident.Impact),
			UpdatedAt:  incident.UpdatedAt,
			LatestBody: latestBody,
		})
		if !strings.EqualFold(strings.TrimSpace(incident.Status), "monitoring") {
			severity = maxProviderStatusSeverity(severity, openAIImpactSeverity(incident.Impact))
		}
	}
	sort.SliceStable(incidents, func(i, j int) bool { return incidents[i].UpdatedAt > incidents[j].UpdatedAt })

	return ProviderStatusProvider{
		ID:              "claude",
		Status:          providerStatusFromSeverity(severity),
		Components:      components,
		ActiveIncidents: incidents,
		SourceUpdatedAt: source.Page.UpdatedAt,
	}, nil
}

func statuspageIncidentAffectsComponents(incident providerStatuspageIncident, relevant map[string]struct{}) bool {
	if len(incident.Components) == 0 || len(relevant) == 0 {
		return true
	}
	for _, component := range incident.Components {
		if _, ok := relevant[component.ID]; ok {
			return true
		}
	}
	return false
}

type xAIStatusFeed struct {
	Channel struct {
		LastBuildDate string              `xml:"lastBuildDate"`
		Items         []xAIStatusFeedItem `xml:"item"`
	} `xml:"channel"`
}

type xAIStatusFeedItem struct {
	Title       string `xml:"title"`
	Link        string `xml:"link"`
	PubDate     string `xml:"pubDate"`
	Description string `xml:"description"`
}

func (s *ProviderStatusService) fetchGrok(ctx context.Context) (ProviderStatusProvider, error) {
	request, err := http.NewRequestWithContext(ctx, http.MethodGet, s.xAIURL, nil)
	if err != nil {
		return ProviderStatusProvider{}, err
	}
	request.Header.Set("Accept", "application/xml, application/rss+xml;q=0.9")
	request.Header.Set("User-Agent", "ikik-api-status/1.0")
	response, err := s.client.Do(request)
	if err != nil {
		return ProviderStatusProvider{}, err
	}
	defer response.Body.Close()
	if response.StatusCode != http.StatusOK {
		_, _ = io.Copy(io.Discard, io.LimitReader(response.Body, 4096))
		return ProviderStatusProvider{}, fmt.Errorf("HTTP %d", response.StatusCode)
	}

	var source xAIStatusFeed
	decoder := xml.NewDecoder(io.LimitReader(response.Body, providerStatusMaxBody))
	if err := decoder.Decode(&source); err != nil {
		return ProviderStatusProvider{}, err
	}

	components := []ProviderStatusComponent{
		{ID: "grok_web", Name: "Grok Web", Status: "operational"},
		{ID: "xai_api", Name: "xAI API", Status: "operational"},
	}
	severityByComponent := map[string]int{"grok_web": 0, "xai_api": 0}
	incidents := make([]ProviderStatusIncident, 0)
	for _, item := range source.Channel.Items {
		description := normalizeProviderStatusText(item.Description)
		if !isRelevantXAIStatusIncident(item.Title) || isResolvedXAIStatusIncident(description) {
			continue
		}
		componentID := classifyXAIStatusComponent(item.Title)
		itemSeverity := xAIStatusIncidentSeverity(description)
		severityByComponent[componentID] = maxProviderStatusSeverity(severityByComponent[componentID], itemSeverity)
		incidents = append(incidents, ProviderStatusIncident{
			ID:         path.Base(strings.TrimSpace(item.Link)),
			Name:       strings.TrimSpace(item.Title),
			Status:     xAIStatusField(description, "status", "severity"),
			Impact:     xAIStatusField(description, "severity", "updates"),
			UpdatedAt:  normalizeRSSDate(item.PubDate),
			LatestBody: truncateProviderStatusText(description, 420),
		})
	}
	severity := 0
	for i := range components {
		componentSeverity := severityByComponent[components[i].ID]
		components[i].Status = providerStatusFromSeverity(componentSeverity)
		severity = maxProviderStatusSeverity(severity, componentSeverity)
	}
	sort.SliceStable(incidents, func(i, j int) bool { return incidents[i].UpdatedAt > incidents[j].UpdatedAt })

	return ProviderStatusProvider{
		ID:              "grok",
		Status:          providerStatusFromSeverity(severity),
		Components:      components,
		ActiveIncidents: incidents,
		SourceUpdatedAt: normalizeRSSDate(source.Channel.LastBuildDate),
	}, nil
}

func isRelevantXAIStatusIncident(title string) bool {
	lower := strings.ToLower(title)
	for _, marker := range []string{"image generation", "imagine", "video", "voice"} {
		if strings.Contains(lower, marker) {
			return false
		}
	}
	return strings.Contains(lower, "grok") || strings.Contains(lower, "api")
}

func isResolvedXAIStatusIncident(description string) bool {
	lower := strings.ToLower(description)
	status := xAIStatusField(lower, "status", "severity")
	return strings.EqualFold(strings.TrimSpace(status), "resolved")
}

func classifyXAIStatusComponent(title string) string {
	if strings.Contains(strings.ToLower(title), "[api (") {
		return "xai_api"
	}
	return "grok_web"
}

func xAIStatusIncidentSeverity(description string) int {
	lower := strings.ToLower(description)
	severity := strings.TrimSpace(xAIStatusField(lower, "severity", "updates"))
	switch severity {
	case "unavailable", "critical", "major":
		return 3
	case "partial_outage", "partial outage":
		return 2
	case "degraded", "minor", "warning":
		return 1
	default:
		return 1
	}
}

func xAIStatusField(text, field, nextField string) string {
	lower := strings.ToLower(text)
	startMarker := strings.ToLower(field) + ":"
	start := strings.Index(lower, startMarker)
	if start < 0 {
		return ""
	}
	start += len(startMarker)
	end := len(text)
	if nextField != "" {
		if offset := strings.Index(lower[start:], strings.ToLower(nextField)+":"); offset >= 0 {
			end = start + offset
		}
	}
	return strings.TrimSpace(text[start:end])
}

type geminiStatusProduct struct {
	Title string `json:"title"`
}

type geminiStatusUpdate struct {
	When   string `json:"when"`
	Text   string `json:"text"`
	Status string `json:"status"`
}

type geminiStatusIncident struct {
	ID               string                `json:"id"`
	Begin            string                `json:"begin"`
	End              string                `json:"end"`
	Modified         string                `json:"modified"`
	ExternalDesc     string                `json:"external_desc"`
	StatusImpact     string                `json:"status_impact"`
	Severity         string                `json:"severity"`
	AffectedProducts []geminiStatusProduct `json:"affected_products"`
	MostRecentUpdate geminiStatusUpdate    `json:"most_recent_update"`
}

func (s *ProviderStatusService) fetchGemini(ctx context.Context) (ProviderStatusProvider, error) {
	var source []geminiStatusIncident
	if err := s.fetchJSON(ctx, s.geminiURL, &source); err != nil {
		return ProviderStatusProvider{}, fmt.Errorf("fetch Gemini status: %w", err)
	}

	incidents := make([]ProviderStatusIncident, 0)
	severity := 0
	sourceUpdatedAt := ""
	for _, incident := range source {
		if !geminiStatusIncidentAffectsAPI(incident) {
			continue
		}
		if incident.Modified > sourceUpdatedAt {
			sourceUpdatedAt = incident.Modified
		}
		if !isActiveGeminiStatusIncident(incident) {
			continue
		}
		incidentSeverity := geminiStatusIncidentSeverity(incident)
		severity = maxProviderStatusSeverity(severity, incidentSeverity)
		body := strings.TrimSpace(incident.MostRecentUpdate.Text)
		if body == "" {
			body = strings.TrimSpace(incident.ExternalDesc)
		}
		incidents = append(incidents, ProviderStatusIncident{
			ID:         incident.ID,
			Name:       strings.TrimSpace(incident.ExternalDesc),
			Status:     strings.TrimSpace(incident.MostRecentUpdate.Status),
			Impact:     strings.TrimSpace(incident.StatusImpact),
			UpdatedAt:  incident.MostRecentUpdate.When,
			LatestBody: truncateProviderStatusText(normalizeProviderStatusText(body), 420),
		})
	}
	sort.SliceStable(incidents, func(i, j int) bool { return incidents[i].UpdatedAt > incidents[j].UpdatedAt })
	return ProviderStatusProvider{
		ID:     "gemini",
		Status: providerStatusFromSeverity(severity),
		Components: []ProviderStatusComponent{
			{ID: "gemini_api", Name: "Gemini API", Status: providerStatusFromSeverity(severity)},
		},
		ActiveIncidents: incidents,
		SourceUpdatedAt: sourceUpdatedAt,
	}, nil
}

func geminiStatusIncidentAffectsAPI(incident geminiStatusIncident) bool {
	for _, product := range incident.AffectedProducts {
		name := strings.ToLower(strings.TrimSpace(product.Title))
		if strings.Contains(name, "gemini") && strings.Contains(name, "api") {
			return true
		}
	}
	return false
}

func isActiveGeminiStatusIncident(incident geminiStatusIncident) bool {
	if strings.TrimSpace(incident.End) != "" {
		return false
	}
	return !strings.EqualFold(strings.TrimSpace(incident.MostRecentUpdate.Status), "AVAILABLE")
}

func geminiStatusIncidentSeverity(incident geminiStatusIncident) int {
	impact := strings.ToUpper(strings.TrimSpace(incident.StatusImpact))
	switch impact {
	case "SERVICE_OUTAGE":
		return 3
	case "SERVICE_DISRUPTION":
		if strings.EqualFold(strings.TrimSpace(incident.Severity), "high") {
			return 2
		}
		return 1
	default:
		return 1
	}
}

func (s *ProviderStatusService) fetchJSON(ctx context.Context, url string, target any) error {
	request, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return err
	}
	request.Header.Set("Accept", "application/json")
	request.Header.Set("User-Agent", "ikik-api-status/1.0")
	response, err := s.client.Do(request)
	if err != nil {
		return err
	}
	defer response.Body.Close()
	if response.StatusCode != http.StatusOK {
		_, _ = io.Copy(io.Discard, io.LimitReader(response.Body, 4096))
		return fmt.Errorf("HTTP %d", response.StatusCode)
	}
	decoder := json.NewDecoder(io.LimitReader(response.Body, providerStatusMaxBody))
	return decoder.Decode(target)
}

func normalizeProviderComponentStatus(status string) string {
	switch strings.ToLower(strings.TrimSpace(status)) {
	case "operational", "available":
		return "operational"
	case "degraded", "degraded_performance", "under_maintenance":
		return "degraded"
	case "partial_outage":
		return "partial_outage"
	case "major_outage", "unavailable":
		return "major_outage"
	default:
		return "unknown"
	}
}

func providerStatusSeverity(status string) int {
	switch status {
	case "degraded":
		return 1
	case "partial_outage":
		return 2
	case "major_outage":
		return 3
	case "unknown":
		return -1
	default:
		return 0
	}
}

func providerStatusFromSeverity(severity int) string {
	switch {
	case severity < 0:
		return "unknown"
	case severity == 1:
		return "degraded"
	case severity == 2:
		return "partial_outage"
	case severity >= 3:
		return "major_outage"
	default:
		return "operational"
	}
}

func maxProviderStatusSeverity(left, right int) int {
	if left < 0 || right < 0 {
		if left == 0 || right == 0 {
			return -1
		}
	}
	if right > left {
		return right
	}
	return left
}

func orderProviderComponents(components []ProviderStatusComponent, ids []string) []ProviderStatusComponent {
	byID := make(map[string]ProviderStatusComponent, len(components))
	for _, component := range components {
		byID[component.ID] = component
	}
	result := make([]ProviderStatusComponent, 0, len(ids))
	for _, id := range ids {
		if component, ok := byID[id]; ok {
			result = append(result, component)
		}
	}
	return result
}

func normalizeProviderStatusText(value string) string {
	withoutTags := providerStatusHTMLTagPattern.ReplaceAllString(html.UnescapeString(value), " ")
	return strings.Join(strings.Fields(withoutTags), " ")
}

func truncateProviderStatusText(value string, limit int) string {
	value = strings.TrimSpace(value)
	if limit <= 0 || len([]rune(value)) <= limit {
		return value
	}
	runes := []rune(value)
	return strings.TrimSpace(string(runes[:limit])) + "..."
}

func normalizeRSSDate(value string) string {
	for _, layout := range []string{time.RFC1123, time.RFC1123Z, time.RFC822, time.RFC822Z} {
		if parsed, err := time.Parse(layout, strings.TrimSpace(value)); err == nil {
			return parsed.UTC().Format(time.RFC3339)
		}
	}
	return strings.TrimSpace(value)
}

func cloneProviderStatusSnapshot(snapshot *ProviderStatusSnapshot) *ProviderStatusSnapshot {
	if snapshot == nil {
		return nil
	}
	clone := *snapshot
	clone.Providers = make([]ProviderStatusProvider, len(snapshot.Providers))
	copy(clone.Providers, snapshot.Providers)
	for i := range clone.Providers {
		clone.Providers[i].Components = make([]ProviderStatusComponent, len(snapshot.Providers[i].Components))
		copy(clone.Providers[i].Components, snapshot.Providers[i].Components)
		clone.Providers[i].ActiveIncidents = make([]ProviderStatusIncident, len(snapshot.Providers[i].ActiveIncidents))
		copy(clone.Providers[i].ActiveIncidents, snapshot.Providers[i].ActiveIncidents)
	}
	return &clone
}
