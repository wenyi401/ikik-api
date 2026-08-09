package service

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"

	dbent "ikik-api/ent"
	"ikik-api/ent/user"
	infraerrors "ikik-api/internal/pkg/errors"
)

const (
	openAIExperimentalPromptMaxLength     = 40000
	openAIExperimentalPromptMaxPriceCents = 100000000
	openAIExperimentalPromptCacheTTL      = 30 * time.Second
)

// OpenAIExperimentalPromptSettings is deliberately generic: the administrator owns
// the instruction text. The server ships no jailbreak text by default.
type OpenAIExperimentalPromptSettings struct {
	Prompt     string `json:"prompt"`
	PriceCents int64  `json:"price_cents"`
}

type OpenAIExperimentalPromptStatus struct {
	FeatureKey string `json:"feature_key"`
	Unlocked   bool   `json:"unlocked"`
	Configured bool   `json:"configured"`
	PriceCents int64  `json:"price_cents"`
}

type cachedOpenAIExperimentalPromptSettings struct {
	settings  OpenAIExperimentalPromptSettings
	expiresAt int64
}

var (
	ErrFeatureAlreadyUnlocked = infraerrors.Conflict("FEATURE_ALREADY_UNLOCKED", "feature is already unlocked")
	ErrFeatureNotConfigured   = infraerrors.BadRequest("FEATURE_NOT_CONFIGURED", "this feature is not configured")
)

func DefaultOpenAIExperimentalPromptSettings() OpenAIExperimentalPromptSettings {
	return OpenAIExperimentalPromptSettings{}
}

func normalizeOpenAIExperimentalPromptSettings(settings OpenAIExperimentalPromptSettings) (OpenAIExperimentalPromptSettings, error) {
	settings.Prompt = strings.TrimSpace(settings.Prompt)
	if len([]byte(settings.Prompt)) > openAIExperimentalPromptMaxLength {
		return OpenAIExperimentalPromptSettings{}, fmt.Errorf("prompt must be at most %d bytes", openAIExperimentalPromptMaxLength)
	}
	if settings.PriceCents < 0 || settings.PriceCents > openAIExperimentalPromptMaxPriceCents {
		return OpenAIExperimentalPromptSettings{}, fmt.Errorf("price_cents must be between 0 and %d", openAIExperimentalPromptMaxPriceCents)
	}
	return settings, nil
}

func (s *SettingService) GetOpenAIExperimentalPromptSettings(ctx context.Context) (*OpenAIExperimentalPromptSettings, error) {
	defaults := DefaultOpenAIExperimentalPromptSettings()
	if s == nil || s.settingRepo == nil {
		return &defaults, nil
	}
	if cached, ok := s.openAIExperimentalPromptCache.Load().(*cachedOpenAIExperimentalPromptSettings); ok && cached != nil && time.Now().UnixNano() < cached.expiresAt {
		settings := cached.settings
		return &settings, nil
	}

	value, err, _ := s.openAIExperimentalPromptSF.Do("load", func() (any, error) {
		if cached, ok := s.openAIExperimentalPromptCache.Load().(*cachedOpenAIExperimentalPromptSettings); ok && cached != nil && time.Now().UnixNano() < cached.expiresAt {
			return cached.settings, nil
		}

		settings := defaults
		raw, loadErr := s.settingRepo.GetValue(ctx, SettingKeyOpenAIExperimentalPromptSettings)
		if loadErr != nil && !errors.Is(loadErr, ErrSettingNotFound) {
			return nil, loadErr
		}
		if loadErr == nil && strings.TrimSpace(raw) != "" {
			if parseErr := json.Unmarshal([]byte(raw), &settings); parseErr != nil {
				return nil, fmt.Errorf("parse openai experimental prompt settings: %w", parseErr)
			}
			settings, loadErr = normalizeOpenAIExperimentalPromptSettings(settings)
			if loadErr != nil {
				return nil, loadErr
			}
		}

		s.openAIExperimentalPromptCache.Store(&cachedOpenAIExperimentalPromptSettings{
			settings:  settings,
			expiresAt: time.Now().Add(openAIExperimentalPromptCacheTTL).UnixNano(),
		})
		return settings, nil
	})
	if err != nil {
		return nil, err
	}
	settings := value.(OpenAIExperimentalPromptSettings)
	return &settings, nil
}

func (s *SettingService) SetOpenAIExperimentalPromptSettings(ctx context.Context, settings OpenAIExperimentalPromptSettings) (*OpenAIExperimentalPromptSettings, error) {
	settings, err := normalizeOpenAIExperimentalPromptSettings(settings)
	if err != nil {
		return nil, infraerrors.BadRequest("OPENAI_EXPERIMENTAL_PROMPT_SETTINGS_INVALID", err.Error())
	}
	data, err := json.Marshal(settings)
	if err != nil {
		return nil, fmt.Errorf("marshal openai experimental prompt settings: %w", err)
	}
	if s == nil || s.settingRepo == nil {
		return nil, errors.New("setting service is not configured")
	}
	if err := s.settingRepo.Set(ctx, SettingKeyOpenAIExperimentalPromptSettings, string(data)); err != nil {
		return nil, err
	}
	s.openAIExperimentalPromptCache.Store(&cachedOpenAIExperimentalPromptSettings{
		settings:  settings,
		expiresAt: time.Now().Add(openAIExperimentalPromptCacheTTL).UnixNano(),
	})
	return &settings, nil
}

func openAIExperimentalPromptForRequest(ctx context.Context, settings *SettingService, apiKey *APIKey) string {
	if apiKey == nil || !apiKey.OpenAIExperimentalPromptEnabled || apiKey.Group == nil || apiKey.User == nil || apiKey.Group.Platform != PlatformOpenAI || !apiKey.Group.OpenAIExperimentalPromptEnabled || !apiKey.User.OpenAIExperimentalPromptUnlocked || settings == nil {
		return ""
	}
	configured, err := settings.GetOpenAIExperimentalPromptSettings(ctx)
	if err != nil || configured == nil {
		return ""
	}
	return strings.TrimSpace(configured.Prompt)
}

// ApplyOpenAIExperimentalPrompt injects into only the current route's request body.
// Responses requests use instructions; Chat Completions requests receive a system
// message so both raw and Responses-compatibility upstreams preserve the instruction.
func (s *OpenAIGatewayService) ApplyOpenAIExperimentalPrompt(ctx context.Context, apiKey *APIKey, body []byte) []byte {
	prompt := openAIExperimentalPromptForRequest(ctx, s.settingService, apiKey)
	if prompt == "" || len(body) == 0 {
		return body
	}
	return injectOpenAIExperimentalPrompt(body, prompt)
}

func injectOpenAIExperimentalPrompt(body []byte, prompt string) []byte {
	var root map[string]any
	if err := json.Unmarshal(body, &root); err != nil {
		return body
	}
	if _, hasMessages := root["messages"]; hasMessages {
		messages, ok := root["messages"].([]any)
		if !ok {
			return body
		}
		messages = append([]any{map[string]any{"role": "system", "content": prompt}}, messages...)
		root["messages"] = messages
	} else {
		existing, ok := root["instructions"].(string)
		if ok && strings.TrimSpace(existing) != "" {
			root["instructions"] = prompt + "\n\n" + existing
		} else if _, exists := root["instructions"]; !exists || root["instructions"] == nil {
			root["instructions"] = prompt
		} else {
			return body
		}
	}
	encoded, err := json.Marshal(root)
	if err != nil {
		return body
	}
	return encoded
}

func (s *RedeemService) OpenAIExperimentalPromptStatus(ctx context.Context, userID int64) (*OpenAIExperimentalPromptStatus, error) {
	if s == nil || s.userRepo == nil {
		return nil, errors.New("redeem service is not configured")
	}
	current, err := s.userRepo.GetByID(ctx, userID)
	if err != nil {
		return nil, err
	}
	settings := DefaultOpenAIExperimentalPromptSettings()
	if s.settingService != nil {
		loaded, loadErr := s.settingService.GetOpenAIExperimentalPromptSettings(ctx)
		if loadErr != nil {
			return nil, loadErr
		}
		settings = *loaded
	}
	return &OpenAIExperimentalPromptStatus{
		FeatureKey: FeatureKeyOpenAIExperimentalPrompt,
		Unlocked:   current.OpenAIExperimentalPromptUnlocked,
		Configured: strings.TrimSpace(settings.Prompt) != "",
		PriceCents: settings.PriceCents,
	}, nil
}

func (s *RedeemService) PurchaseOpenAIExperimentalPrompt(ctx context.Context, userID int64) (*OpenAIExperimentalPromptStatus, error) {
	if s == nil || s.entClient == nil {
		return nil, errors.New("redeem service is not configured")
	}
	status, err := s.OpenAIExperimentalPromptStatus(ctx, userID)
	if err != nil {
		return nil, err
	}
	if status.Unlocked {
		return nil, ErrFeatureAlreadyUnlocked
	}
	if !status.Configured {
		return nil, ErrFeatureNotConfigured
	}
	price := float64(status.PriceCents) / 100
	tx, err := s.entClient.Tx(ctx)
	if err != nil {
		return nil, err
	}
	defer func() { _ = tx.Rollback() }()
	update := tx.Client().User.Update().Where(
		user.IDEQ(userID),
		user.OpenaiExperimentalPromptUnlockedEQ(false),
		user.BalanceGTE(price),
	).SetOpenaiExperimentalPromptUnlocked(true)
	if price > 0 {
		update = update.AddBalance(-price)
	}
	affected, err := update.Save(ctx)
	if err != nil {
		return nil, err
	}
	if affected == 0 {
		current, queryErr := tx.Client().User.Query().Where(user.IDEQ(userID)).Only(ctx)
		if queryErr != nil {
			return nil, queryErr
		}
		if current.OpenaiExperimentalPromptUnlocked {
			return nil, ErrFeatureAlreadyUnlocked
		}
		return nil, ErrInsufficientBalance
	}
	if err := tx.Commit(); err != nil {
		return nil, err
	}
	if s.authCacheInvalidator != nil {
		s.authCacheInvalidator.InvalidateAuthCacheByUserID(ctx, userID)
	}
	if s.billingCacheService != nil {
		_ = s.billingCacheService.InvalidateUserBalance(ctx, userID)
	}
	status.Unlocked = true
	return status, nil
}

func unlockOpenAIExperimentalPromptTx(ctx context.Context, tx *dbent.Tx, userID int64) error {
	current, err := tx.Client().User.Query().Where(user.IDEQ(userID)).Only(ctx)
	if err != nil {
		return err
	}
	if current.OpenaiExperimentalPromptUnlocked {
		return ErrFeatureAlreadyUnlocked
	}
	return tx.Client().User.UpdateOneID(userID).SetOpenaiExperimentalPromptUnlocked(true).Exec(ctx)
}
