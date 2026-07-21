package service

import (
	"context"
	"encoding/json"

	"fmt"

	"strings"

	infraerrors "ikik-api/internal/pkg/errors"
)

// GetUsageRetention returns the raw usage_logs auto archive and cleanup config.
func (s *BackupService) GetUsageRetention(ctx context.Context) (*UsageRetentionConfig, error) {
	defaultCfg := s.defaultUsageRetentionConfig()
	raw, err := s.settingRepo.GetValue(ctx, settingKeyUsageRetention)
	if err != nil || strings.TrimSpace(raw) == "" {
		return &defaultCfg, nil
	}
	var cfg UsageRetentionConfig
	if err := json.Unmarshal([]byte(raw), &cfg); err != nil {
		return nil, infraerrors.InternalServer("USAGE_RETENTION_CONFIG_CORRUPT", "usage retention config data is corrupted")
	}
	if err := validateUsageRetentionConfig(cfg, s.usageRetentionMaxWindowDays()); err != nil {
		return nil, err
	}
	return &cfg, nil
}

// UpdateUsageRetention saves the raw usage_logs auto archive and cleanup config.
func (s *BackupService) UpdateUsageRetention(ctx context.Context, cfg UsageRetentionConfig) (*UsageRetentionConfig, error) {
	if err := validateUsageRetentionConfig(cfg, s.usageRetentionMaxWindowDays()); err != nil {
		return nil, err
	}
	data, err := json.Marshal(cfg)
	if err != nil {
		return nil, fmt.Errorf("marshal usage retention config: %w", err)
	}
	if err := s.settingRepo.Set(ctx, settingKeyUsageRetention, string(data)); err != nil {
		return nil, fmt.Errorf("save usage retention config: %w", err)
	}
	return &cfg, nil
}

// UsageRetentionConfig controls automatic raw usage_logs archive and cleanup.
type UsageRetentionConfig struct {
	Enabled          bool `json:"enabled"`
	RetainDays       int  `json:"retain_days"`
	RunIntervalHours int  `json:"run_interval_hours"`
	WindowDays       int  `json:"window_days"`
	BackupExpireDays int  `json:"backup_expire_days"`
}

func (s *BackupService) defaultUsageRetentionConfig() UsageRetentionConfig {
	cfg := UsageRetentionConfig{
		Enabled:          false,
		RetainDays:       3,
		RunIntervalHours: 24,
		WindowDays:       1,
		BackupExpireDays: 14,
	}
	if s == nil {
		return cfg
	}
	auto := s.usageCleanup.AutoRetention
	cfg.Enabled = auto.Enabled
	if auto.RetainDays > 0 {
		cfg.RetainDays = auto.RetainDays
	}
	if auto.RunIntervalHours > 0 {
		cfg.RunIntervalHours = auto.RunIntervalHours
	}
	if auto.WindowDays > 0 {
		cfg.WindowDays = auto.WindowDays
	}
	if auto.BackupExpireDays >= 0 {
		cfg.BackupExpireDays = auto.BackupExpireDays
	}
	return cfg
}

func (s *BackupService) usageRetentionMaxWindowDays() int {
	if s == nil || s.usageCleanup.MaxRangeDays <= 0 {
		return 31
	}
	return s.usageCleanup.MaxRangeDays
}

func validateUsageRetentionConfig(cfg UsageRetentionConfig, maxWindowDays int) error {
	if cfg.RetainDays <= 0 {
		return infraerrors.BadRequest("INVALID_USAGE_RETENTION_RETAIN_DAYS", "retain_days must be positive")
	}
	if cfg.RunIntervalHours <= 0 {
		return infraerrors.BadRequest("INVALID_USAGE_RETENTION_RUN_INTERVAL", "run_interval_hours must be positive")
	}
	if cfg.WindowDays <= 0 {
		return infraerrors.BadRequest("INVALID_USAGE_RETENTION_WINDOW_DAYS", "window_days must be positive")
	}
	if cfg.BackupExpireDays < 0 {
		return infraerrors.BadRequest("INVALID_USAGE_RETENTION_BACKUP_EXPIRE_DAYS", "backup_expire_days must be non-negative")
	}
	if maxWindowDays <= 0 {
		maxWindowDays = 31
	}
	if cfg.WindowDays > maxWindowDays {
		return infraerrors.BadRequest("INVALID_USAGE_RETENTION_WINDOW_DAYS", fmt.Sprintf("window_days must be less than or equal to %d", maxWindowDays))
	}
	return nil
}
