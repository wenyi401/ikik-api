package service

import (
	"context"
	"crypto/sha256"
	"encoding/binary"
	"fmt"
	"log/slog"
	"math"
	"strings"
	"time"

	"ikik-api/internal/pkg/pagination"
)

const (
	ContentModerationModeAdaptive = "adaptive"

	ContentModerationEnforcementShadow  = "shadow"
	ContentModerationEnforcementNotify  = "notify"
	ContentModerationEnforcementEnforce = "enforce"

	ContentModerationRiskLevelNew      = "new"
	ContentModerationRiskLevelNormal   = "normal"
	ContentModerationRiskLevelTrusted  = "trusted"
	ContentModerationRiskLevelWatch    = "watch"
	ContentModerationRiskLevelHigh     = "high"
	ContentModerationRiskLevelCritical = "critical"

	ContentModerationManualLevelAuto = "auto"

	ContentModerationSeverityNone   = "none"
	ContentModerationSeverityLow    = "low"
	ContentModerationSeverityMedium = "medium"
	ContentModerationSeveritySevere = "severe"

	ContentModerationRiskCategorySafetyBypass              = "gateway_abuse/safety_bypass"
	ContentModerationRiskCategoryCredentialTheft           = "gateway_abuse/credential_theft"
	ContentModerationRiskCategoryAccountAutomation         = "gateway_abuse/account_automation"
	ContentModerationRiskCategoryAuthReverseEngineering    = "gateway_abuse/auth_reverse_engineering"
	ContentModerationRiskCategoryExploitReverseEngineering = "gateway_abuse/exploit_reverse_engineering"
	ContentModerationRiskCategoryCheatAutomation           = "gateway_abuse/cheat_automation"
	ContentModerationRiskCategoryOther                     = "gateway_abuse/other"
	ContentModerationPolicyCategoryHarassment              = "policy/harassment_or_defamation"
	ContentModerationPolicyCategorySelfHarm                = "policy/self_harm"
	ContentModerationPolicyCategorySexualAbuse             = "policy/sexual_or_nonconsensual"
	ContentModerationPolicyCategoryViolence                = "policy/violence_terrorism_or_hate"
	ContentModerationPolicyCategoryWeapons                 = "policy/weapons"
	ContentModerationPolicyCategoryIllicit                 = "policy/illicit_goods_or_services"
	ContentModerationPolicyCategoryCyberAbuse              = "policy/cyber_abuse"
	ContentModerationPolicyCategoryGambling                = "policy/real_money_gambling"
	ContentModerationPolicyCategoryUnlicensedAdvice        = "policy/unlicensed_high_stakes_advice"
	ContentModerationPolicyCategoryPrivacy                 = "policy/privacy_or_sensitive_data"
	ContentModerationPolicyCategoryProfiling               = "policy/biometric_or_social_profiling"
	ContentModerationPolicyCategoryMinorExploitation       = "policy/minor_exploitation"
	ContentModerationPolicyCategoryMinorUnsafeContent      = "policy/minor_unsafe_content"
	ContentModerationPolicyCategoryFraud                   = "policy/fraud_spam_or_impersonation"
	ContentModerationPolicyCategoryAcademicDishonesty      = "policy/academic_dishonesty"
	ContentModerationPolicyCategoryPoliticalManipulation   = "policy/political_manipulation"
	ContentModerationPolicyCategoryHighStakesAutomation    = "policy/high_stakes_automation"
	ContentModerationPolicyCategoryNationalSecurity        = "policy/national_security_or_intelligence"
	ContentModerationPolicyCategoryIPInfringement          = "policy/ip_infringement"

	contentModerationRiskOverviewCacheTTL = 30 * time.Second
)

type ContentModerationAdaptivePolicy struct {
	EnforcementMode           string  `json:"enforcement_mode"`
	FullAuditRequests         int64   `json:"full_audit_requests"`
	RampAuditRequests         int64   `json:"ramp_audit_requests"`
	RampSampleRate            int     `json:"ramp_sample_rate"`
	TrustedSampleRate         int     `json:"trusted_sample_rate"`
	WatchSampleRate           int     `json:"watch_sample_rate"`
	HighRiskSampleRate        int     `json:"high_risk_sample_rate"`
	DailyDecayPercent         float64 `json:"daily_decay_percent"`
	LowRiskWeight             float64 `json:"low_risk_weight"`
	MediumRiskWeight          float64 `json:"medium_risk_weight"`
	SevereRiskWeight          float64 `json:"severe_risk_weight"`
	WatchThreshold            float64 `json:"watch_threshold"`
	HighRiskThreshold         float64 `json:"high_risk_threshold"`
	CriticalThreshold         float64 `json:"critical_threshold"`
	NotificationCooldownHours int     `json:"notification_cooldown_hours"`
}

func DefaultContentModerationAdaptivePolicy() ContentModerationAdaptivePolicy {
	return ContentModerationAdaptivePolicy{
		EnforcementMode:           ContentModerationEnforcementShadow,
		FullAuditRequests:         100,
		RampAuditRequests:         300,
		RampSampleRate:            30,
		TrustedSampleRate:         5,
		WatchSampleRate:           50,
		HighRiskSampleRate:        100,
		DailyDecayPercent:         10,
		LowRiskWeight:             4,
		MediumRiskWeight:          12,
		SevereRiskWeight:          30,
		WatchThreshold:            40,
		HighRiskThreshold:         60,
		CriticalThreshold:         80,
		NotificationCooldownHours: 24,
	}
}

func (p *ContentModerationAdaptivePolicy) normalize() {
	if p == nil {
		return
	}
	defaults := DefaultContentModerationAdaptivePolicy()
	switch strings.ToLower(strings.TrimSpace(p.EnforcementMode)) {
	case ContentModerationEnforcementNotify, ContentModerationEnforcementEnforce:
		p.EnforcementMode = strings.ToLower(strings.TrimSpace(p.EnforcementMode))
	default:
		p.EnforcementMode = ContentModerationEnforcementShadow
	}
	if p.FullAuditRequests <= 0 {
		p.FullAuditRequests = defaults.FullAuditRequests
	}
	if p.RampAuditRequests < p.FullAuditRequests {
		p.RampAuditRequests = maxInt64(defaults.RampAuditRequests, p.FullAuditRequests)
	}
	p.RampSampleRate = clampModerationPercent(p.RampSampleRate, defaults.RampSampleRate)
	p.TrustedSampleRate = clampModerationPercent(p.TrustedSampleRate, defaults.TrustedSampleRate)
	p.WatchSampleRate = clampModerationPercent(p.WatchSampleRate, defaults.WatchSampleRate)
	p.HighRiskSampleRate = clampModerationPercent(p.HighRiskSampleRate, defaults.HighRiskSampleRate)
	if p.DailyDecayPercent <= 0 || p.DailyDecayPercent >= 100 {
		p.DailyDecayPercent = defaults.DailyDecayPercent
	}
	if p.LowRiskWeight <= 0 {
		p.LowRiskWeight = defaults.LowRiskWeight
	}
	if p.MediumRiskWeight <= 0 {
		p.MediumRiskWeight = defaults.MediumRiskWeight
	}
	if p.SevereRiskWeight <= 0 {
		p.SevereRiskWeight = defaults.SevereRiskWeight
	}
	if p.WatchThreshold <= 0 {
		p.WatchThreshold = defaults.WatchThreshold
	}
	if p.HighRiskThreshold <= p.WatchThreshold {
		p.HighRiskThreshold = maxFloat(defaults.HighRiskThreshold, p.WatchThreshold+1)
	}
	if p.CriticalThreshold <= p.HighRiskThreshold {
		p.CriticalThreshold = maxFloat(defaults.CriticalThreshold, p.HighRiskThreshold+1)
	}
	if p.CriticalThreshold > 100 {
		p.CriticalThreshold = 100
	}
	if p.NotificationCooldownHours <= 0 {
		p.NotificationCooldownHours = defaults.NotificationCooldownHours
	}
}

func clampModerationPercent(value int, fallback int) int {
	if value < 0 || value > 100 {
		return fallback
	}
	return value
}

func maxFloat(a, b float64) float64 {
	if a > b {
		return a
	}
	return b
}

func maxInt64(a, b int64) int64 {
	if a > b {
		return a
	}
	return b
}

type ContentModerationRiskProfile struct {
	UserID            int64      `json:"user_id"`
	UserEmail         string     `json:"user_email"`
	UserStatus        string     `json:"user_status"`
	TotalRequests     int64      `json:"total_requests"`
	AuditedRequests   int64      `json:"audited_requests"`
	FlaggedRequests   int64      `json:"flagged_requests"`
	RiskScore         float64    `json:"risk_score"`
	RiskLevel         string     `json:"risk_level"`
	ManualLevel       string     `json:"manual_level"`
	CurrentSampleRate int        `json:"current_sample_rate"`
	LastCategory      string     `json:"last_category"`
	LastScoreDelta    float64    `json:"last_score_delta"`
	LastHitAt         *time.Time `json:"last_hit_at,omitempty"`
	LastAuditedAt     *time.Time `json:"last_audited_at,omitempty"`
	LastNotifiedAt    *time.Time `json:"last_notified_at,omitempty"`
	ScoreUpdatedAt    time.Time  `json:"score_updated_at"`
	CreatedAt         time.Time  `json:"created_at"`
	UpdatedAt         time.Time  `json:"updated_at"`
}

type ContentModerationRiskOverview struct {
	TotalProfiles    int64   `json:"total_profiles"`
	NewProfiles      int64   `json:"new_profiles"`
	TrustedProfiles  int64   `json:"trusted_profiles"`
	WatchProfiles    int64   `json:"watch_profiles"`
	HighProfiles     int64   `json:"high_profiles"`
	CriticalProfiles int64   `json:"critical_profiles"`
	AuditedRequests  int64   `json:"audited_requests"`
	FlaggedRequests  int64   `json:"flagged_requests"`
	AverageRiskScore float64 `json:"average_risk_score"`
}

type ContentModerationRiskProfileFilter struct {
	Pagination pagination.PaginationParams
	Level      string
	Search     string
}

type ContentModerationRiskProfilesPage struct {
	Items    []ContentModerationRiskProfile `json:"items"`
	Overview ContentModerationRiskOverview  `json:"overview"`
	Total    int64                          `json:"total"`
	Page     int                            `json:"page"`
	PageSize int                            `json:"page_size"`
	Pages    int                            `json:"pages"`
}

type ContentModerationRiskEvent struct {
	RequestID  string
	UserID     int64
	UserEmail  string
	APIKeyID   int64
	APIKeyName string
	GroupID    int64
	GroupName  string
	Audited    bool
	Flagged    bool
	Severity   string
	Category   string
	Score      float64
	ScoreDelta float64
	SampleRate int
	CreatedAt  time.Time
}

type UpdateContentModerationRiskProfileInput struct {
	ManualLevel *string `json:"manual_level"`
	ResetScore  bool    `json:"reset_score"`
}

type ContentModerationRiskRepository interface {
	RecordRiskEvent(ctx context.Context, event ContentModerationRiskEvent, policy ContentModerationAdaptivePolicy) (*ContentModerationRiskProfile, bool, error)
	GetRiskProfile(ctx context.Context, userID int64) (*ContentModerationRiskProfile, error)
	ListRiskProfiles(ctx context.Context, filter ContentModerationRiskProfileFilter) ([]ContentModerationRiskProfile, *pagination.PaginationResult, error)
	GetRiskOverview(ctx context.Context) (*ContentModerationRiskOverview, error)
	UpdateRiskProfile(ctx context.Context, userID int64, input UpdateContentModerationRiskProfileInput, policy ContentModerationAdaptivePolicy) (*ContentModerationRiskProfile, error)
	ReserveRiskNotification(ctx context.Context, userID int64, cooldown time.Duration) (bool, error)
	DisableAPIKeyForRisk(ctx context.Context, apiKeyID int64, userID int64) (bool, error)
}

type ContentModerationGroupPenalty struct {
	UserID        int64
	GroupID       int64
	StrikeCount   int
	BlockedUntil  *time.Time
	Permanent     bool
	LastCategory  string
	LastRequestID string
	LastScore     float64
	CreatedAt     time.Time
	UpdatedAt     time.Time
}

type ContentModerationGroupPenaltyRepository interface {
	ApplyUserGroupPenaltyForRisk(ctx context.Context, event ContentModerationRiskEvent) (*ContentModerationGroupPenalty, bool, error)
}

func (p ContentModerationAdaptivePolicy) SampleRate(profile *ContentModerationRiskProfile) int {
	p.normalize()
	if profile == nil {
		return 100
	}
	manual := normalizeContentModerationManualLevel(profile.ManualLevel)
	switch manual {
	case ContentModerationRiskLevelTrusted:
		return p.TrustedSampleRate
	case ContentModerationRiskLevelWatch:
		return p.WatchSampleRate
	case ContentModerationRiskLevelHigh, ContentModerationRiskLevelCritical:
		return p.HighRiskSampleRate
	}
	if profile.AuditedRequests < p.FullAuditRequests {
		return 100
	}
	if profile.RiskScore >= p.CriticalThreshold {
		return 100
	}
	if profile.RiskScore >= p.HighRiskThreshold {
		return p.HighRiskSampleRate
	}
	if profile.RiskScore >= p.WatchThreshold {
		return p.WatchSampleRate
	}
	if profile.AuditedRequests < p.RampAuditRequests {
		return p.RampSampleRate
	}
	return p.TrustedSampleRate
}

func (p ContentModerationAdaptivePolicy) RiskLevel(profile *ContentModerationRiskProfile) string {
	p.normalize()
	if profile == nil {
		return ContentModerationRiskLevelNew
	}
	manual := normalizeContentModerationManualLevel(profile.ManualLevel)
	if manual != ContentModerationManualLevelAuto {
		return manual
	}
	if profile.AuditedRequests < p.FullAuditRequests {
		return ContentModerationRiskLevelNew
	}
	if profile.RiskScore >= p.CriticalThreshold {
		return ContentModerationRiskLevelCritical
	}
	if profile.RiskScore >= p.HighRiskThreshold {
		return ContentModerationRiskLevelHigh
	}
	if profile.RiskScore >= p.WatchThreshold {
		return ContentModerationRiskLevelWatch
	}
	if profile.AuditedRequests < p.RampAuditRequests {
		return ContentModerationRiskLevelNormal
	}
	return ContentModerationRiskLevelTrusted
}

func normalizeContentModerationManualLevel(value string) string {
	switch strings.ToLower(strings.TrimSpace(value)) {
	case ContentModerationRiskLevelTrusted, ContentModerationRiskLevelWatch, ContentModerationRiskLevelHigh, ContentModerationRiskLevelCritical:
		return strings.ToLower(strings.TrimSpace(value))
	default:
		return ContentModerationManualLevelAuto
	}
}

func (s *ContentModerationService) cachedAdaptiveRiskProfile(userID int64) *ContentModerationRiskProfile {
	if s == nil || userID <= 0 {
		return nil
	}
	s.adaptiveRiskMu.RLock()
	profile, ok := s.adaptiveRiskProfiles[userID]
	s.adaptiveRiskMu.RUnlock()
	if !ok {
		return nil
	}
	copy := profile
	return &copy
}

func (s *ContentModerationService) cacheAdaptiveRiskProfile(profile *ContentModerationRiskProfile) {
	if s == nil || profile == nil || profile.UserID <= 0 {
		return
	}
	s.adaptiveRiskMu.Lock()
	if s.adaptiveRiskProfiles == nil {
		s.adaptiveRiskProfiles = make(map[int64]ContentModerationRiskProfile)
	}
	s.adaptiveRiskProfiles[profile.UserID] = *profile
	s.adaptiveRiskMu.Unlock()
}

func (s *ContentModerationService) adaptiveSampleDecision(input ContentModerationCheckInput, cfg *ContentModerationConfig, inputHash string) (*ContentModerationRiskEvent, bool) {
	if cfg == nil || input.UserID <= 0 {
		return nil, true
	}
	profile := s.cachedAdaptiveRiskProfile(input.UserID)
	rate := cfg.AdaptivePolicy.SampleRate(profile)
	requestID := strings.TrimSpace(input.RequestID)
	if requestID == "" {
		raw := sha256.Sum256([]byte(fmt.Sprintf("%d:%d:%s:%d", input.UserID, input.APIKeyID, inputHash, time.Now().UnixNano())))
		requestID = fmt.Sprintf("adaptive-%x", raw[:12])
	}
	event := &ContentModerationRiskEvent{
		RequestID:  requestID,
		UserID:     input.UserID,
		UserEmail:  input.UserEmail,
		APIKeyID:   input.APIKeyID,
		APIKeyName: input.APIKeyName,
		GroupID:    contentModerationLogGroupID(input.GroupID),
		GroupName:  input.GroupName,
		SampleRate: rate,
		CreatedAt:  time.Now(),
	}
	if rate >= 100 {
		return event, true
	}
	if rate <= 0 {
		return event, false
	}
	raw := sha256.Sum256([]byte(fmt.Sprintf("%d:%s:%s", input.UserID, requestID, inputHash)))
	return event, int(binary.BigEndian.Uint16(raw[:2])%100) < rate
}

func (s *ContentModerationService) enqueueAdaptiveRiskTask(input ContentModerationCheckInput, cfg *ContentModerationConfig, content ContentModerationInput, inputHash string, event *ContentModerationRiskEvent, sampled bool) {
	if s == nil || s.asyncQueue == nil || cfg == nil || event == nil {
		return
	}
	queueSize := cfg.QueueSize
	if queueSize <= 0 {
		queueSize = defaultContentModerationQueueSize
	}
	if len(s.asyncQueue) >= queueSize {
		s.asyncDropped.Add(1)
		return
	}
	task := contentModerationTask{
		input:      input,
		inputHash:  inputHash,
		riskEvent:  event,
		enqueuedAt: time.Now(),
	}
	if sampled {
		task.content = content
	}
	select {
	case s.asyncQueue <- task:
		s.asyncEnqueued.Add(1)
	default:
		s.asyncDropped.Add(1)
	}
}

func adaptiveRiskSeverity(category string, score float64) string {
	category = strings.ToLower(strings.TrimSpace(category))
	if !isContentModerationAccountRiskCategory(category) {
		return ContentModerationSeverityNone
	}
	switch category {
	case ContentModerationRiskCategoryAccountAutomation:
		if score >= 0.8 {
			return ContentModerationSeveritySevere
		}
		return ContentModerationSeverityMedium
	case ContentModerationRiskCategorySafetyBypass:
		if score >= 0.9 {
			return ContentModerationSeveritySevere
		}
		return ContentModerationSeverityMedium
	case ContentModerationRiskCategoryAuthReverseEngineering,
		ContentModerationRiskCategoryExploitReverseEngineering,
		ContentModerationRiskCategoryCheatAutomation:
		return ContentModerationSeverityMedium
	default:
		return ContentModerationSeverityNone
	}
}

func isContentModerationAccountRiskCategory(category string) bool {
	switch strings.ToLower(strings.TrimSpace(category)) {
	case ContentModerationRiskCategorySafetyBypass,
		ContentModerationRiskCategoryAccountAutomation,
		ContentModerationRiskCategoryAuthReverseEngineering,
		ContentModerationRiskCategoryExploitReverseEngineering,
		ContentModerationRiskCategoryCheatAutomation:
		return true
	default:
		return false
	}
}

func shouldApplyContentModerationGroupPenalty(category string) bool {
	switch strings.ToLower(strings.TrimSpace(category)) {
	case ContentModerationRiskCategorySafetyBypass,
		ContentModerationRiskCategoryCredentialTheft,
		ContentModerationRiskCategoryAccountAutomation,
		ContentModerationRiskCategoryAuthReverseEngineering,
		ContentModerationRiskCategoryExploitReverseEngineering,
		ContentModerationRiskCategoryCheatAutomation,
		ContentModerationPolicyCategoryViolence,
		ContentModerationPolicyCategoryWeapons,
		ContentModerationPolicyCategoryCyberAbuse:
		return true
	default:
		return false
	}
}

func adaptiveRiskScoreDelta(policy ContentModerationAdaptivePolicy, severity string) float64 {
	policy.normalize()
	switch severity {
	case ContentModerationSeveritySevere:
		return policy.SevereRiskWeight
	case ContentModerationSeverityMedium:
		return policy.MediumRiskWeight
	case ContentModerationSeverityLow:
		return policy.LowRiskWeight
	default:
		return 0
	}
}

func applyAdaptiveDecisionToRiskEvent(event *ContentModerationRiskEvent, decision *ContentModerationDecision, policy ContentModerationAdaptivePolicy) {
	if event == nil || decision == nil {
		return
	}
	event.Audited = decision.Audited
	event.Flagged = decision.Flagged
	event.Category = decision.HighestCategory
	event.Score = decision.HighestScore
	event.Severity = ContentModerationSeverityNone
	event.ScoreDelta = 0
	if !decision.Flagged {
		return
	}
	event.Severity = decision.RiskSeverity
	if event.Severity == "" || event.Severity == ContentModerationSeverityNone {
		event.Severity = adaptiveRiskSeverity(decision.HighestCategory, decision.HighestScore)
	}
	event.ScoreDelta = adaptiveRiskScoreDelta(policy, event.Severity)
}

func (s *ContentModerationService) recordAdaptiveRiskEvent(ctx context.Context, cfg *ContentModerationConfig, event *ContentModerationRiskEvent) {
	if s == nil || cfg == nil || event == nil || event.UserID <= 0 {
		return
	}
	repo, ok := s.repo.(ContentModerationRiskRepository)
	if !ok {
		return
	}
	profile, applied, err := repo.RecordRiskEvent(ctx, *event, cfg.AdaptivePolicy)
	if err != nil {
		slog.Warn("content_moderation.adaptive_profile_update_failed", "user_id", event.UserID, "request_id", event.RequestID, "error", err)
		return
	}
	s.cacheAdaptiveRiskProfile(profile)
	if !applied || !event.Flagged || profile == nil || cfg.AdaptivePolicy.EnforcementMode == ContentModerationEnforcementShadow {
		return
	}
	if cfg.AdaptivePolicy.EnforcementMode == ContentModerationEnforcementEnforce && shouldApplyContentModerationGroupPenalty(event.Category) {
		s.applyUserGroupPenaltyForRisk(ctx, event)
	}
	if cfg.EmailOnHit && s.emailService != nil && strings.TrimSpace(event.UserEmail) != "" {
		reserved, reserveErr := repo.ReserveRiskNotification(ctx, event.UserID, time.Duration(cfg.AdaptivePolicy.NotificationCooldownHours)*time.Hour)
		if reserveErr != nil {
			slog.Warn("content_moderation.adaptive_notification_reserve_failed", "user_id", event.UserID, "error", reserveErr)
		}
		if reserveErr == nil && reserved {
			log := &ContentModerationLog{
				UserID:          &event.UserID,
				UserEmail:       event.UserEmail,
				APIKeyName:      event.APIKeyName,
				GroupName:       event.GroupName,
				HighestCategory: event.Category,
				HighestScore:    event.Score,
				ViolationCount:  int(profile.FlaggedRequests),
				CreatedAt:       event.CreatedAt,
			}
			_ = s.sendViolationEmail(ctx, cfg, log)
		}
	}
	if cfg.AdaptivePolicy.EnforcementMode != ContentModerationEnforcementEnforce || profile.RiskLevel != ContentModerationRiskLevelCritical || event.Severity != ContentModerationSeveritySevere || event.APIKeyID <= 0 {
		return
	}
	if s.userRepo != nil {
		user, getErr := s.userRepo.GetByID(ctx, event.UserID)
		if getErr == nil && user.IsAdmin() {
			return
		}
	}
	disabled, disableErr := repo.DisableAPIKeyForRisk(ctx, event.APIKeyID, event.UserID)
	if disableErr != nil {
		slog.Warn("content_moderation.adaptive_api_key_disable_failed", "user_id", event.UserID, "api_key_id", event.APIKeyID, "error", disableErr)
	}
	if disableErr == nil && disabled && s.authCacheInvalidator != nil {
		s.authCacheInvalidator.InvalidateAuthCacheByUserID(ctx, event.UserID)
	}
}

func (s *ContentModerationService) applyUserGroupPenaltyForRisk(ctx context.Context, event *ContentModerationRiskEvent) {
	if s == nil || event == nil || event.UserID <= 0 || event.GroupID <= 0 || strings.TrimSpace(event.RequestID) == "" {
		return
	}
	repo, ok := s.repo.(ContentModerationGroupPenaltyRepository)
	if !ok {
		return
	}
	if s.userRepo != nil {
		user, err := s.userRepo.GetByID(ctx, event.UserID)
		if err == nil && user != nil && user.IsAdmin() {
			slog.Warn("content_moderation.adaptive_group_penalty_skipped_admin", "user_id", event.UserID, "group_id", event.GroupID, "category", event.Category)
			return
		}
	}
	penalty, applied, err := repo.ApplyUserGroupPenaltyForRisk(ctx, *event)
	if err != nil {
		slog.Warn("content_moderation.adaptive_group_penalty_failed", "user_id", event.UserID, "group_id", event.GroupID, "category", event.Category, "request_id", event.RequestID, "error", err)
		return
	}
	if !applied || penalty == nil {
		return
	}
	if s.authCacheInvalidator != nil {
		s.authCacheInvalidator.InvalidateAuthCacheByUserID(ctx, event.UserID)
	}
	slog.Warn("content_moderation.adaptive_group_penalty_applied",
		"user_id", event.UserID,
		"group_id", event.GroupID,
		"category", event.Category,
		"request_id", event.RequestID,
		"strike_count", penalty.StrikeCount,
		"blocked_until", penalty.BlockedUntil,
		"permanent", penalty.Permanent,
	)
}

func (s *ContentModerationService) ListRiskProfiles(ctx context.Context, filter ContentModerationRiskProfileFilter) (*ContentModerationRiskProfilesPage, error) {
	repo, ok := s.repo.(ContentModerationRiskRepository)
	if !ok {
		return nil, fmt.Errorf("content moderation risk repository is unavailable")
	}
	items, page, err := repo.ListRiskProfiles(ctx, filter)
	if err != nil {
		return nil, err
	}
	overview, err := s.getCachedRiskOverview(ctx, repo)
	if err != nil {
		return nil, err
	}
	for i := range items {
		s.cacheAdaptiveRiskProfile(&items[i])
	}
	return &ContentModerationRiskProfilesPage{
		Items: items, Overview: *overview, Total: page.Total, Page: page.Page, PageSize: page.PageSize, Pages: page.Pages,
	}, nil
}

func (s *ContentModerationService) getCachedRiskOverview(ctx context.Context, repo ContentModerationRiskRepository) (*ContentModerationRiskOverview, error) {
	now := time.Now()
	s.adaptiveOverviewMu.Lock()
	defer s.adaptiveOverviewMu.Unlock()
	if s.adaptiveOverview != nil && now.Before(s.adaptiveOverviewExpiry) {
		copy := *s.adaptiveOverview
		return &copy, nil
	}
	overview, err := repo.GetRiskOverview(ctx)
	if err != nil {
		return nil, err
	}
	copy := *overview
	s.adaptiveOverview = &copy
	s.adaptiveOverviewExpiry = now.Add(contentModerationRiskOverviewCacheTTL)
	return overview, nil
}

func (s *ContentModerationService) UpdateRiskProfile(ctx context.Context, userID int64, input UpdateContentModerationRiskProfileInput) (*ContentModerationRiskProfile, error) {
	if userID <= 0 {
		return nil, fmt.Errorf("invalid user id")
	}
	repo, ok := s.repo.(ContentModerationRiskRepository)
	if !ok {
		return nil, fmt.Errorf("content moderation risk repository is unavailable")
	}
	if input.ManualLevel != nil {
		normalized := normalizeContentModerationManualLevel(*input.ManualLevel)
		input.ManualLevel = &normalized
	}
	cfg, err := s.loadConfig(ctx)
	if err != nil {
		return nil, err
	}
	updated, err := repo.UpdateRiskProfile(ctx, userID, input, cfg.AdaptivePolicy)
	if err != nil {
		return nil, err
	}
	s.cacheAdaptiveRiskProfile(updated)
	return updated, nil
}

func DecayContentModerationRiskScore(score float64, updatedAt time.Time, now time.Time, dailyDecayPercent float64) float64 {
	if score <= 0 || updatedAt.IsZero() || !now.After(updatedAt) {
		return math.Max(0, math.Min(100, score))
	}
	base := 1 - dailyDecayPercent/100
	if base <= 0 || base >= 1 {
		base = 0.9
	}
	days := now.Sub(updatedAt).Hours() / 24
	return math.Max(0, math.Min(100, score*math.Pow(base, days)))
}
