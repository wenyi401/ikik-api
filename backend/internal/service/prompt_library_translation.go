package service

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"log/slog"
	"sort"
	"strings"
	"time"

	"golang.org/x/sync/singleflight"
	infraerrors "ikik-api/internal/pkg/errors"
)

const (
	SettingKeyPromptLibraryTranslationConfig = "prompt_library_translation_config"
	promptLibraryTranslationLocale           = "zh-CN"
	promptLibraryTranslationTimeout          = 45 * time.Second
)

var ErrPromptLibraryTranslationConfigInvalid = infraerrors.BadRequest(
	"PROMPT_LIBRARY_TRANSLATION_CONFIG_INVALID",
	"prompt library translation configuration is invalid",
)

type PromptLibraryTranslation struct {
	PromptID    string
	Locale      string
	SourceHash  string
	Title       string
	Description string
	Category    string
}

type PromptLibraryTranslationRepository interface {
	GetByPromptIDs(ctx context.Context, locale string, promptIDs []string) (map[string]PromptLibraryTranslation, error)
	Upsert(ctx context.Context, items []PromptLibraryTranslation) error
	Count(ctx context.Context, locale string) (int64, error)
}

type PromptLibraryTranslationConfig struct {
	Enabled      bool   `json:"enabled"`
	GroupID      int64  `json:"group_id"`
	Model        string `json:"model"`
	TargetLocale string `json:"target_locale"`
}

type PromptLibraryTranslationConfigStatus struct {
	PromptLibraryTranslationConfig
	TranslatedCount int64 `json:"translated_count"`
}

type PromptLibraryTranslationService struct {
	repo        PromptLibraryTranslationRepository
	settingRepo SettingRepository
	groupRepo   GroupRepository
	gateway     ContentModerationClassifierGateway
	flight      singleflight.Group
}

func NewPromptLibraryTranslationService(
	repo PromptLibraryTranslationRepository,
	settingRepo SettingRepository,
	groupRepo GroupRepository,
	gateway *OpenAIGatewayService,
) *PromptLibraryTranslationService {
	return &PromptLibraryTranslationService{
		repo:        repo,
		settingRepo: settingRepo,
		groupRepo:   groupRepo,
		gateway:     gateway,
	}
}

func (s *PromptLibraryTranslationService) GetConfig(ctx context.Context) (PromptLibraryTranslationConfigStatus, error) {
	config, err := s.loadConfig(ctx)
	if err != nil {
		return PromptLibraryTranslationConfigStatus{}, err
	}
	count, err := s.repo.Count(ctx, config.TargetLocale)
	if err != nil {
		return PromptLibraryTranslationConfigStatus{}, err
	}
	return PromptLibraryTranslationConfigStatus{
		PromptLibraryTranslationConfig: config,
		TranslatedCount:                count,
	}, nil
}

func (s *PromptLibraryTranslationService) UpdateConfig(
	ctx context.Context,
	config PromptLibraryTranslationConfig,
) (PromptLibraryTranslationConfigStatus, error) {
	config.Model = strings.TrimSpace(config.Model)
	config.TargetLocale = promptLibraryTranslationLocale
	if config.Enabled {
		if err := s.validateConfig(ctx, config); err != nil {
			return PromptLibraryTranslationConfigStatus{}, err
		}
	}
	raw, err := json.Marshal(config)
	if err != nil {
		return PromptLibraryTranslationConfigStatus{}, fmt.Errorf("encode prompt library translation config: %w", err)
	}
	if err := s.settingRepo.Set(ctx, SettingKeyPromptLibraryTranslationConfig, string(raw)); err != nil {
		return PromptLibraryTranslationConfigStatus{}, err
	}
	return s.GetConfig(ctx)
}

func (s *PromptLibraryTranslationService) Localize(
	ctx context.Context,
	body json.RawMessage,
	requestedLocale string,
) (json.RawMessage, error) {
	locale := normalizePromptLibraryLocale(requestedLocale)
	if len(body) == 0 {
		return append(json.RawMessage(nil), body...), nil
	}

	page, err := parsePromptLibraryTranslationPage(body)
	if err != nil || len(page.documents) == 0 {
		return append(json.RawMessage(nil), body...), err
	}
	config, err := s.loadConfig(ctx)
	if err != nil {
		return append(json.RawMessage(nil), body...), err
	}

	promptIDs := make([]string, 0, len(page.documents))
	for _, document := range page.documents {
		promptIDs = append(promptIDs, document.ID)
	}
	translations, err := s.repo.GetByPromptIDs(ctx, promptLibraryTranslationLocale, promptIDs)
	if err != nil {
		return append(json.RawMessage(nil), body...), err
	}

	missing := make([]promptLibraryTranslationDocument, 0, len(page.documents))
	for _, document := range page.documents {
		translation, ok := translations[document.ID]
		if ok && translation.SourceHash == document.SourceHash {
			page.apply(translation, locale)
			continue
		}
		missing = append(missing, document)
	}

	if config.Enabled && len(missing) > 0 {
		key := promptLibraryTranslationFlightKey(locale, missing)
		value, translateErr, _ := s.flight.Do(key, func() (any, error) {
			translateCtx, cancel := context.WithTimeout(context.WithoutCancel(ctx), promptLibraryTranslationTimeout)
			defer cancel()
			return s.translateDocuments(translateCtx, config, missing)
		})
		if translateErr != nil {
			slog.Warn("prompt library translation failed", "error", translateErr)
		} else if generated, ok := value.(map[string]PromptLibraryTranslation); ok {
			for _, translation := range generated {
				page.apply(translation, locale)
			}
		}
	}

	localized, err := json.Marshal(page.root)
	if err != nil {
		return append(json.RawMessage(nil), body...), fmt.Errorf("encode localized prompt library page: %w", err)
	}
	return localized, nil
}

func (s *PromptLibraryTranslationService) loadConfig(ctx context.Context) (PromptLibraryTranslationConfig, error) {
	config := PromptLibraryTranslationConfig{TargetLocale: promptLibraryTranslationLocale}
	raw, err := s.settingRepo.GetValue(ctx, SettingKeyPromptLibraryTranslationConfig)
	if err != nil && !infraerrors.IsNotFound(err) {
		return PromptLibraryTranslationConfig{}, err
	}
	if strings.TrimSpace(raw) != "" {
		if err := json.Unmarshal([]byte(raw), &config); err != nil {
			return PromptLibraryTranslationConfig{}, fmt.Errorf("decode prompt library translation config: %w", err)
		}
	}
	config.Model = strings.TrimSpace(config.Model)
	config.TargetLocale = normalizePromptLibraryLocale(config.TargetLocale)
	return config, nil
}

func (s *PromptLibraryTranslationService) validateConfig(
	ctx context.Context,
	config PromptLibraryTranslationConfig,
) error {
	if config.GroupID <= 0 || config.Model == "" {
		return ErrPromptLibraryTranslationConfigInvalid
	}
	group, err := s.groupRepo.GetByID(ctx, config.GroupID)
	if err != nil {
		return ErrPromptLibraryTranslationConfigInvalid
	}
	if group == nil || group.Status != StatusActive || group.OwnerUserID != nil ||
		NormalizeGroupScope(group.Scope) != GroupScopePublic ||
		!isContentModerationClassifierPlatform(group.Platform) {
		return ErrPromptLibraryTranslationConfigInvalid
	}
	return nil
}

type promptLibraryTranslationDocument struct {
	ID               string `json:"id"`
	Title            string `json:"title"`
	Description      string `json:"description"`
	Type             string `json:"type"`
	SourceCategory   string `json:"source_category"`
	ContentExcerpt   string `json:"content_excerpt"`
	FallbackCategory string `json:"-"`
	SourceHash       string `json:"-"`
}

type promptLibraryTranslationPage struct {
	root      map[string]any
	documents []promptLibraryTranslationDocument
	items     map[string]map[string]any
}

func parsePromptLibraryTranslationPage(body json.RawMessage) (*promptLibraryTranslationPage, error) {
	var root map[string]any
	if err := json.Unmarshal(body, &root); err != nil {
		return nil, fmt.Errorf("decode prompt library page: %w", err)
	}
	rawPrompts, ok := root["prompts"].([]any)
	if !ok {
		return nil, fmt.Errorf("prompt library page has no prompts")
	}
	page := &promptLibraryTranslationPage{
		root:  root,
		items: make(map[string]map[string]any, len(rawPrompts)),
	}
	for _, rawPrompt := range rawPrompts {
		prompt, ok := rawPrompt.(map[string]any)
		if !ok {
			continue
		}
		id, _ := prompt["id"].(string)
		title, _ := prompt["title"].(string)
		description, _ := prompt["description"].(string)
		content, _ := prompt["content"].(string)
		promptType, _ := prompt["type"].(string)
		id = strings.TrimSpace(id)
		title = strings.TrimSpace(title)
		if id == "" || title == "" {
			continue
		}
		sourceCategory := promptLibrarySourceCategory(prompt)
		promptType = strings.ToUpper(strings.TrimSpace(promptType))
		contentExcerpt := truncatePromptLibraryTranslation(strings.TrimSpace(content), 700)
		fallbackCategory := fallbackPromptLibraryCategory(promptType, sourceCategory)
		document := promptLibraryTranslationDocument{
			ID:               id,
			Title:            title,
			Description:      strings.TrimSpace(description),
			Type:             promptType,
			SourceCategory:   sourceCategory,
			ContentExcerpt:   contentExcerpt,
			FallbackCategory: fallbackCategory,
			SourceHash: promptLibraryTranslationSourceHash(
				title,
				description,
				promptType,
				sourceCategory,
				content,
			),
		}
		page.documents = append(page.documents, document)
		page.items[id] = prompt
		if fallbackCategory != "" {
			prompt["ikikCategory"] = fallbackCategory
		}
	}
	return page, nil
}

func (p *promptLibraryTranslationPage) apply(translation PromptLibraryTranslation, locale string) {
	item := p.items[translation.PromptID]
	if item == nil {
		return
	}
	if category := normalizePromptLibraryCategory(translation.Category); category != "" {
		item["ikikCategory"] = category
	}
	if locale == promptLibraryTranslationLocale && strings.TrimSpace(translation.Title) != "" {
		item["title"] = translation.Title
		if strings.TrimSpace(translation.Description) != "" {
			item["description"] = translation.Description
		}
	}
}

func (s *PromptLibraryTranslationService) translateDocuments(
	ctx context.Context,
	config PromptLibraryTranslationConfig,
	documents []promptLibraryTranslationDocument,
) (map[string]PromptLibraryTranslation, error) {
	if err := s.validateConfig(ctx, config); err != nil {
		return nil, err
	}
	requestDocuments := make([]promptLibraryTranslationDocument, 0, len(documents))
	known := make(map[string]promptLibraryTranslationDocument, len(documents))
	for _, document := range documents {
		known[document.ID] = document
		requestDocuments = append(requestDocuments, document)
	}
	userPayload, err := json.Marshal(map[string]any{"prompts": requestDocuments})
	if err != nil {
		return nil, fmt.Errorf("encode prompt translation input: %w", err)
	}
	requestBody, err := json.Marshal(map[string]any{
		"model":  config.Model,
		"stream": false,
		"messages": []map[string]string{
			{
				"role":    "system",
				"content": `Translate each prompt title and description into concise, natural Simplified Chinese and classify it. Preserve code, placeholders, model names, product names, and proper nouns. Do not translate the prompt content. Choose exactly one category: coding, writing, business, creative, education, workflow, productivity, image, video. Use coding for software engineering and programming; writing for editing, copy, and long-form text; business for sales, marketing, finance, and management; creative for ideation, roleplay, design, and non-media creative work; education for teaching, learning, and research; workflow for multi-step processes, agents, automation, and operations; productivity for general planning, organization, and personal efficiency; image only when type is IMAGE; video only when type is VIDEO. Return JSON only in this exact shape: {"translations":[{"id":"...","title":"...","description":"...","category":"coding"}]}`,
			},
			{"role": "user", "content": string(userPayload)},
		},
	})
	if err != nil {
		return nil, fmt.Errorf("encode prompt translation request: %w", err)
	}
	group, err := s.groupRepo.GetByID(ctx, config.GroupID)
	if err != nil || group == nil {
		return nil, ErrPromptLibraryTranslationConfigInvalid
	}
	response, err := s.gateway.ForwardContentModerationClassifier(ctx, ContentModerationClassifierGatewayInput{
		GroupID:  group.ID,
		Platform: group.Platform,
		Model:    config.Model,
		Body:     requestBody,
	})
	if err != nil {
		return nil, err
	}

	content, err := promptLibraryTranslationResponseContent(response.Body)
	if err != nil {
		return nil, err
	}
	var decoded struct {
		Translations []struct {
			ID          string `json:"id"`
			Title       string `json:"title"`
			Description string `json:"description"`
			Category    string `json:"category"`
		} `json:"translations"`
	}
	if err := json.Unmarshal(extractPromptLibraryJSONObject(content), &decoded); err != nil {
		return nil, fmt.Errorf("decode prompt translation result: %w", err)
	}

	items := make([]PromptLibraryTranslation, 0, len(decoded.Translations))
	result := make(map[string]PromptLibraryTranslation, len(decoded.Translations))
	for _, translated := range decoded.Translations {
		source, ok := known[strings.TrimSpace(translated.ID)]
		if !ok {
			continue
		}
		title := truncatePromptLibraryTranslation(strings.TrimSpace(translated.Title), 200)
		if title == "" {
			continue
		}
		category := normalizePromptLibraryCategory(translated.Category)
		if source.Type == "IMAGE" {
			category = "image"
		} else if source.Type == "VIDEO" {
			category = "video"
		} else if category == "image" || category == "video" || category == "" {
			category = source.FallbackCategory
		}
		if category == "" {
			category = "productivity"
		}
		item := PromptLibraryTranslation{
			PromptID:    source.ID,
			Locale:      promptLibraryTranslationLocale,
			SourceHash:  source.SourceHash,
			Title:       title,
			Description: truncatePromptLibraryTranslation(strings.TrimSpace(translated.Description), 500),
			Category:    category,
		}
		items = append(items, item)
		result[item.PromptID] = item
	}
	if len(items) == 0 {
		return nil, fmt.Errorf("prompt translation result is empty")
	}
	if err := s.repo.Upsert(ctx, items); err != nil {
		return nil, err
	}
	return result, nil
}

func promptLibraryTranslationResponseContent(body []byte) (string, error) {
	var response struct {
		Choices []struct {
			Message struct {
				Content string `json:"content"`
			} `json:"message"`
		} `json:"choices"`
	}
	if err := json.Unmarshal(body, &response); err != nil {
		return "", fmt.Errorf("decode prompt translation gateway response: %w", err)
	}
	if len(response.Choices) == 0 || strings.TrimSpace(response.Choices[0].Message.Content) == "" {
		return "", fmt.Errorf("prompt translation gateway returned empty content")
	}
	return response.Choices[0].Message.Content, nil
}

func extractPromptLibraryJSONObject(content string) []byte {
	content = strings.TrimSpace(content)
	start := strings.Index(content, "{")
	end := strings.LastIndex(content, "}")
	if start < 0 || end < start {
		return []byte(content)
	}
	return []byte(content[start : end+1])
}

func normalizePromptLibraryLocale(locale string) string {
	if strings.HasPrefix(strings.ToLower(strings.TrimSpace(locale)), "zh") {
		return promptLibraryTranslationLocale
	}
	return "en"
}

func promptLibraryTranslationSourceHash(parts ...string) string {
	normalized := make([]string, 0, len(parts))
	for _, part := range parts {
		normalized = append(normalized, strings.TrimSpace(part))
	}
	hash := sha256.Sum256([]byte(strings.Join(normalized, "\x00")))
	return hex.EncodeToString(hash[:])
}

func promptLibrarySourceCategory(prompt map[string]any) string {
	category, _ := prompt["category"].(map[string]any)
	if category == nil {
		return ""
	}
	if slug, ok := category["slug"].(string); ok && strings.TrimSpace(slug) != "" {
		return strings.ToLower(strings.TrimSpace(slug))
	}
	name, _ := category["name"].(string)
	return strings.ToLower(strings.TrimSpace(name))
}

func fallbackPromptLibraryCategory(promptType, sourceCategory string) string {
	switch strings.ToUpper(strings.TrimSpace(promptType)) {
	case "IMAGE":
		return "image"
	case "VIDEO":
		return "video"
	}

	category := strings.ToLower(strings.TrimSpace(sourceCategory))
	switch {
	case strings.Contains(category, "program"), strings.Contains(category, "cod"), strings.Contains(category, "develop"), strings.Contains(category, "software"):
		return "coding"
	case strings.Contains(category, "writ"), strings.Contains(category, "copy"), strings.Contains(category, "content"):
		return "writing"
	case strings.Contains(category, "business"), strings.Contains(category, "market"), strings.Contains(category, "sales"), strings.Contains(category, "finance"), strings.Contains(category, "management"):
		return "business"
	case strings.Contains(category, "education"), strings.Contains(category, "learn"), strings.Contains(category, "teach"), strings.Contains(category, "research"):
		return "education"
	case strings.Contains(category, "workflow"), strings.Contains(category, "automation"), strings.Contains(category, "agent"), strings.Contains(category, "operation"):
		return "workflow"
	case strings.Contains(category, "productiv"), strings.Contains(category, "planning"), strings.Contains(category, "organization"):
		return "productivity"
	case strings.Contains(category, "creative"), strings.Contains(category, "design"), strings.Contains(category, "roleplay"), strings.Contains(category, "idea"):
		return "creative"
	default:
		return ""
	}
}

func normalizePromptLibraryCategory(value string) string {
	category := strings.ToLower(strings.TrimSpace(value))
	switch category {
	case "coding", "writing", "business", "creative", "education", "workflow", "productivity", "image", "video":
		return category
	default:
		return ""
	}
}

func promptLibraryTranslationFlightKey(locale string, documents []promptLibraryTranslationDocument) string {
	keys := make([]string, 0, len(documents))
	for _, document := range documents {
		keys = append(keys, document.ID+":"+document.SourceHash)
	}
	sort.Strings(keys)
	return locale + ":" + strings.Join(keys, ",")
}

func truncatePromptLibraryTranslation(value string, maxRunes int) string {
	runes := []rune(value)
	if len(runes) <= maxRunes {
		return value
	}
	return string(runes[:maxRunes])
}
