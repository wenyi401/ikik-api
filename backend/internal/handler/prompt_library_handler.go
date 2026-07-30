package handler

import (
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"sync"
	"time"

	"ikik-api/internal/pkg/response"

	"github.com/gin-gonic/gin"
)

const (
	promptLibraryUpstream = "https://prompts.chat/api/prompts"
	promptLibraryMaxBody  = 8 << 20
	promptLibraryCacheTTL = 5 * time.Minute
)

type promptLibraryCacheEntry struct {
	body      json.RawMessage
	expiresAt time.Time
}

var promptLibraryProxy = struct {
	sync.RWMutex
	entries map[string]promptLibraryCacheEntry
}{entries: make(map[string]promptLibraryCacheEntry)}

var promptLibraryHTTPClient = &http.Client{Timeout: 15 * time.Second}

// GetPromptLibrary proxies the public prompts.chat catalog through the ikik
// backend so mobile browsers do not depend on third-party CORS behavior.
func (h *ServiceStatusHandler) GetPromptLibrary(c *gin.Context) {
	locale := c.Query("locale")
	query, cacheKey, err := normalizePromptLibraryQuery(c.Request.URL.Query())
	if err != nil {
		response.Error(c, http.StatusBadRequest, err.Error())
		return
	}

	if cached, ok := getPromptLibraryCache(cacheKey); ok {
		h.respondPromptLibrary(c, cached, locale)
		return
	}

	req, err := http.NewRequestWithContext(c.Request.Context(), http.MethodGet, promptLibraryUpstream+"?"+query.Encode(), nil)
	if err != nil {
		response.Error(c, http.StatusInternalServerError, "Failed to prepare prompt library request")
		return
	}
	req.Header.Set("Accept", "application/json")
	req.Header.Set("User-Agent", "ikik-api/prompt-library")

	upstream, err := promptLibraryHTTPClient.Do(req)
	if err != nil {
		response.Error(c, http.StatusServiceUnavailable, "Prompt library is temporarily unavailable")
		return
	}
	defer upstream.Body.Close()
	if upstream.StatusCode != http.StatusOK {
		response.Error(c, http.StatusServiceUnavailable, "Prompt library is temporarily unavailable")
		return
	}

	body, err := io.ReadAll(io.LimitReader(upstream.Body, promptLibraryMaxBody+1))
	if err != nil || len(body) > promptLibraryMaxBody || !json.Valid(body) {
		response.Error(c, http.StatusBadGateway, "Prompt library returned an invalid response")
		return
	}

	raw := json.RawMessage(body)
	setPromptLibraryCache(cacheKey, raw)
	h.respondPromptLibrary(c, raw, locale)
}

func (h *ServiceStatusHandler) respondPromptLibrary(c *gin.Context, raw json.RawMessage, locale string) {
	if h.promptLibraryTranslationService == nil {
		response.Success(c, raw)
		return
	}
	localized, err := h.promptLibraryTranslationService.Localize(c.Request.Context(), raw, locale)
	if err != nil {
		slog.Warn("prompt library localization unavailable", "error", err)
		response.Success(c, raw)
		return
	}
	response.Success(c, localized)
}

func normalizePromptLibraryQuery(input url.Values) (url.Values, string, error) {
	page, err := positiveBoundedInt(input.Get("page"), 1, 10000)
	if err != nil {
		return nil, "", fmt.Errorf("page is invalid")
	}
	perPage, err := positiveBoundedInt(input.Get("per_page"), 12, 48)
	if err != nil {
		return nil, "", fmt.Errorf("per_page is invalid")
	}

	query := strings.TrimSpace(input.Get("q"))
	if len([]rune(query)) > 200 {
		return nil, "", fmt.Errorf("query is too long")
	}
	sort := strings.TrimSpace(input.Get("sort"))
	if sort != "" && sort != "newest" && sort != "oldest" && sort != "upvotes" {
		return nil, "", fmt.Errorf("sort is invalid")
	}
	promptType := strings.ToUpper(strings.TrimSpace(input.Get("type")))
	if promptType != "" && promptType != "TEXT" && promptType != "STRUCTURED" && promptType != "IMAGE" && promptType != "VIDEO" && promptType != "AUDIO" {
		return nil, "", fmt.Errorf("type is invalid")
	}

	output := url.Values{}
	output.Set("page", strconv.Itoa(page))
	output.Set("perPage", strconv.Itoa(perPage))
	if query != "" {
		output.Set("q", query)
	}
	if sort != "" && sort != "newest" {
		output.Set("sort", sort)
	}
	if promptType != "" {
		output.Set("type", promptType)
	}
	return output, output.Encode(), nil
}

func positiveBoundedInt(raw string, fallback, max int) (int, error) {
	if strings.TrimSpace(raw) == "" {
		return fallback, nil
	}
	value, err := strconv.Atoi(raw)
	if err != nil || value < 1 || value > max {
		return 0, fmt.Errorf("invalid integer")
	}
	return value, nil
}

func getPromptLibraryCache(key string) (json.RawMessage, bool) {
	promptLibraryProxy.RLock()
	entry, ok := promptLibraryProxy.entries[key]
	promptLibraryProxy.RUnlock()
	if !ok || time.Now().After(entry.expiresAt) {
		if ok {
			promptLibraryProxy.Lock()
			delete(promptLibraryProxy.entries, key)
			promptLibraryProxy.Unlock()
		}
		return nil, false
	}
	return append(json.RawMessage(nil), entry.body...), true
}

func setPromptLibraryCache(key string, body json.RawMessage) {
	promptLibraryProxy.Lock()
	defer promptLibraryProxy.Unlock()
	if len(promptLibraryProxy.entries) >= 128 {
		for existingKey, entry := range promptLibraryProxy.entries {
			if time.Now().After(entry.expiresAt) {
				delete(promptLibraryProxy.entries, existingKey)
			}
		}
	}
	promptLibraryProxy.entries[key] = promptLibraryCacheEntry{
		body:      append(json.RawMessage(nil), body...),
		expiresAt: time.Now().Add(promptLibraryCacheTTL),
	}
}
