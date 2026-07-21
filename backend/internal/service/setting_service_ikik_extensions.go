package service

import (
	"context"

	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"math"

	"strconv"
	"strings"

	infraerrors "ikik-api/internal/pkg/errors"
)

func (s *SettingService) AddContentModerationGroup(ctx context.Context, groupID int64) error {
	return s.updateContentModerationGroupScope(ctx, groupID, true)
}

var (
	ErrHomeStatsGroupInvalid = infraerrors.BadRequest(
		"HOME_STATS_GROUP_INVALID",
		"home stats group must be an administrator public group",
	)
)

func (s *SettingService) GetOpenAIFreeAccountRepairSettings(ctx context.Context) (enabled bool, weeklyThresholdUSD float64) {
	if s == nil || s.settingRepo == nil {
		return false, 0
	}
	rawEnabled, err := s.settingRepo.GetValue(ctx, SettingKeyOpenAIFreeAccountRepairEnabled)
	if err != nil || !strings.EqualFold(strings.TrimSpace(rawEnabled), "true") {
		return false, 0
	}

	threshold := 60.0
	rawThreshold, err := s.settingRepo.GetValue(ctx, SettingKeyOpenAIFreeAccountRepairWeeklyThresholdUSD)
	if err == nil && strings.TrimSpace(rawThreshold) != "" {
		parsed, parseErr := strconv.ParseFloat(strings.TrimSpace(rawThreshold), 64)
		if parseErr != nil || parsed <= 0 || math.IsNaN(parsed) || math.IsInf(parsed, 0) {
			return false, 0
		}
		threshold = parsed
	} else if err != nil && !errors.Is(err, ErrSettingNotFound) {
		return false, 0
	}

	return true, threshold
}

// GetOpenAIImagesResponsesReasoningEffort returns the Responses API reasoning effort
// used by the OAuth images bridge. DB setting wins; missing or invalid values fall
// back to config/default so a corrupted setting cannot break image requests.
func (s *SettingService) GetOpenAIImagesResponsesReasoningEffort(ctx context.Context) string {
	fallback := s.defaultOpenAIImagesResponsesReasoningEffort()
	if s == nil || s.settingRepo == nil {
		return fallback
	}
	if ctx == nil {
		ctx = context.Background()
	}
	dbCtx, cancel := context.WithTimeout(context.WithoutCancel(ctx), gatewayForwardingDBTimeout)
	defer cancel()
	value, err := s.settingRepo.GetValue(dbCtx, SettingKeyOpenAIImagesResponsesReasoningEffort)
	if err != nil {
		if !errors.Is(err, ErrSettingNotFound) {
			slog.Warn("failed to get openai images responses reasoning effort setting", "error", err)
		}
		return fallback
	}
	trimmed := strings.TrimSpace(value)
	if trimmed == "" || !IsValidOpenAIImagesResponsesReasoningEffort(trimmed) {
		return fallback
	}
	return NormalizeOpenAIImagesResponsesReasoningEffort(trimmed)
}

// GetUserPrivateGroupTemplate returns the default quota template for newly provisioned user-private groups.
func (s *SettingService) GetUserPrivateGroupTemplate(ctx context.Context) (*UserPrivateGroupTemplate, error) {
	settings, err := s.GetAllSettings(ctx)
	if err != nil {
		return nil, err
	}
	return &UserPrivateGroupTemplate{
		DailyLimitUSD:   settings.UserPrivateGroupDailyLimitUSD,
		WeeklyLimitUSD:  settings.UserPrivateGroupWeeklyLimitUSD,
		MonthlyLimitUSD: settings.UserPrivateGroupMonthlyLimitUSD,
		RateMultiplier:  settings.UserPrivateGroupRateMultiplier,
		RPMLimit:        settings.UserPrivateGroupRPMLimit,
		CommissionRate:  settings.UserPrivateGroupCommissionRate,
	}, nil
}

// IsCarpoolEnabled 检查是否启用拼车池功能（总开关）
func (s *SettingService) IsCarpoolEnabled(ctx context.Context) bool {
	value, err := s.settingRepo.GetValue(ctx, SettingKeyCarpoolEnabled)
	if err != nil {
		return false
	}
	return value == "true"
}

func (s *SettingService) RemoveContentModerationGroup(ctx context.Context, groupID int64) error {
	return s.updateContentModerationGroupScope(ctx, groupID, false)
}
func (s *SettingService) defaultOpenAIImagesResponsesReasoningEffort() string {
	if s != nil && s.cfg != nil {
		return NormalizeOpenAIImagesResponsesReasoningEffort(s.cfg.Gateway.OpenAIImagesResponsesReasoningEffort)
	}
	return OpenAIImagesResponsesReasoningEffortDefault
}

func formatNonNegativeSettingFloat(value float64, fallback float64) string {
	if value < 0 || math.IsNaN(value) || math.IsInf(value, 0) {
		value = fallback
	}
	return strconv.FormatFloat(value, 'f', 8, 64)
}
func formatPositiveOptionalFloat(value *float64) string {
	if value == nil || *value <= 0 || math.IsNaN(*value) || math.IsInf(*value, 0) {
		return "0"
	}
	return strconv.FormatFloat(*value, 'f', 8, 64)
}
func isAdministratorPublicGroup(group *Group) bool {
	return group != nil && group.OwnerUserID == nil && NormalizeGroupScope(group.Scope) == GroupScopePublic
}

func parseNonNegativeSettingFloat(value string, fallback float64) float64 {
	parsed, err := strconv.ParseFloat(strings.TrimSpace(value), 64)
	if err != nil || parsed < 0 || math.IsNaN(parsed) || math.IsInf(parsed, 0) {
		return fallback
	}
	return parsed
}

func parseNonNegativeSettingInt64(value string) int64 {
	parsed, err := strconv.ParseInt(strings.TrimSpace(value), 10, 64)
	if err != nil || parsed < 0 {
		return 0
	}
	return parsed
}

func parsePositiveOptionalFloat(value string) *float64 {
	parsed, err := strconv.ParseFloat(strings.TrimSpace(value), 64)
	if err != nil || parsed <= 0 || math.IsNaN(parsed) || math.IsInf(parsed, 0) {
		return nil
	}
	return &parsed
}
func (s *SettingService) updateContentModerationGroupScope(ctx context.Context, groupID int64, add bool) error {
	if s == nil || s.settingRepo == nil || groupID <= 0 {
		return nil
	}

	cfg := defaultContentModerationConfig()
	raw, err := s.settingRepo.GetValue(ctx, SettingKeyContentModerationConfig)
	if err != nil && !errors.Is(err, ErrSettingNotFound) {
		return fmt.Errorf("load content moderation config: %w", err)
	}
	if err == nil && strings.TrimSpace(raw) != "" {
		if unmarshalErr := json.Unmarshal([]byte(raw), cfg); unmarshalErr != nil {
			return fmt.Errorf("parse content moderation config: %w", unmarshalErr)
		}
	}
	cfg.normalize()
	if cfg.AllGroups {
		return nil
	}

	nextIDs := append([]int64(nil), cfg.GroupIDs...)
	if add {
		nextIDs = append(nextIDs, groupID)
	} else {
		filtered := nextIDs[:0]
		for _, id := range nextIDs {
			if id != groupID {
				filtered = append(filtered, id)
			}
		}
		nextIDs = filtered
	}
	cfg.GroupIDs = normalizeInt64IDs(nextIDs)
	rawBytes, err := json.Marshal(cfg)
	if err != nil {
		return fmt.Errorf("marshal content moderation config: %w", err)
	}
	if err := s.settingRepo.Set(ctx, SettingKeyContentModerationConfig, string(rawBytes)); err != nil {
		return fmt.Errorf("save content moderation config: %w", err)
	}
	return nil
}
func (s *SettingService) validateHomeStatsGroup(ctx context.Context, groupID int64) error {
	if groupID <= 0 || s.defaultSubGroupReader == nil {
		return nil
	}
	group, err := s.defaultSubGroupReader.GetByID(ctx, groupID)
	if err != nil {
		if errors.Is(err, ErrGroupNotFound) {
			return ErrHomeStatsGroupInvalid.WithMetadata(map[string]string{
				"group_id": strconv.FormatInt(groupID, 10),
			})
		}
		return fmt.Errorf("get home stats group %d: %w", groupID, err)
	}
	if !isAdministratorPublicGroup(group) {
		return ErrHomeStatsGroupInvalid.WithMetadata(map[string]string{
			"group_id": strconv.FormatInt(groupID, 10),
		})
	}
	return nil
}
