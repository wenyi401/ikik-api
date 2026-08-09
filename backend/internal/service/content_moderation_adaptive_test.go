package service

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

type contentModerationGroupPenaltyRepoStub struct {
	*contentModerationTestRepo
	penalty     *ContentModerationGroupPenalty
	applied     bool
	applyErr    error
	event       ContentModerationRiskEvent
	events      []ContentModerationRiskEvent
	firstHours  int
	secondHours int
	applyCalls  int
}

func (r *contentModerationGroupPenaltyRepoStub) ApplyUserGroupPenaltyForRisk(_ context.Context, event ContentModerationRiskEvent, firstBlockHours int, secondBlockHours int) (*ContentModerationGroupPenalty, bool, error) {
	r.applyCalls++
	r.event = event
	r.events = append(r.events, event)
	r.firstHours = firstBlockHours
	r.secondHours = secondBlockHours
	return r.penalty, r.applied, r.applyErr
}

type contentModerationAdaptiveRiskPenaltyRepoStub struct {
	*contentModerationTestRepo
	ContentModerationRiskRepository
	recordApplied bool
	recordedEvent ContentModerationRiskEvent
	profile       *ContentModerationRiskProfile
	penalty       *ContentModerationGroupPenalty
	penaltyEvent  ContentModerationRiskEvent
	calls         []string
	firstHours    int
	secondHours   int
}

func (r *contentModerationAdaptiveRiskPenaltyRepoStub) RecordRiskEvent(_ context.Context, event ContentModerationRiskEvent, _ ContentModerationAdaptivePolicy) (*ContentModerationRiskProfile, bool, error) {
	r.calls = append(r.calls, "record")
	r.recordedEvent = event
	return r.profile, r.recordApplied, nil
}

func (r *contentModerationAdaptiveRiskPenaltyRepoStub) ApplyUserGroupPenaltyForRisk(_ context.Context, event ContentModerationRiskEvent, firstBlockHours int, secondBlockHours int) (*ContentModerationGroupPenalty, bool, error) {
	r.calls = append(r.calls, "penalty")
	r.penaltyEvent = event
	r.firstHours = firstBlockHours
	r.secondHours = secondBlockHours
	return r.penalty, true, nil
}

func TestContentModerationAdaptivePolicySampleRate(t *testing.T) {
	policy := DefaultContentModerationAdaptivePolicy()
	require.Equal(t, 100, policy.SampleRate(nil))

	profile := &ContentModerationRiskProfile{AuditedRequests: 99, ManualLevel: ContentModerationManualLevelAuto}
	require.Equal(t, 100, policy.SampleRate(profile))

	profile.AuditedRequests = 150
	require.Equal(t, 30, policy.SampleRate(profile))

	profile.AuditedRequests = 300
	require.Equal(t, 5, policy.SampleRate(profile))

	profile.RiskScore = 45
	require.Equal(t, 50, policy.SampleRate(profile))

	profile.RiskScore = 65
	require.Equal(t, 100, policy.SampleRate(profile))

	profile.ManualLevel = ContentModerationRiskLevelTrusted
	require.Equal(t, 5, policy.SampleRate(profile))
}

func TestContentModerationAdaptivePolicyRiskLevels(t *testing.T) {
	policy := DefaultContentModerationAdaptivePolicy()
	profile := &ContentModerationRiskProfile{AuditedRequests: 50, ManualLevel: ContentModerationManualLevelAuto}
	require.Equal(t, ContentModerationRiskLevelNew, policy.RiskLevel(profile))

	profile.AuditedRequests = 150
	require.Equal(t, ContentModerationRiskLevelNormal, policy.RiskLevel(profile))

	profile.AuditedRequests = 300
	require.Equal(t, ContentModerationRiskLevelTrusted, policy.RiskLevel(profile))

	profile.RiskScore = 40
	require.Equal(t, ContentModerationRiskLevelWatch, policy.RiskLevel(profile))
	profile.RiskScore = 60
	require.Equal(t, ContentModerationRiskLevelHigh, policy.RiskLevel(profile))
	profile.RiskScore = 80
	require.Equal(t, ContentModerationRiskLevelCritical, policy.RiskLevel(profile))
}

func TestContentModerationGroupPenaltyCategories(t *testing.T) {
	policy := DefaultContentModerationGroupPenaltyPolicy()
	require.False(t, policy.Enabled)
	require.Equal(t, 24, policy.FirstBlockHours)
	require.Equal(t, 36, policy.SecondBlockHours)

	blocked := []string{
		ContentModerationRiskCategorySafetyBypass,
		ContentModerationRiskCategoryCredentialTheft,
		ContentModerationRiskCategoryAccountAutomation,
		ContentModerationRiskCategoryAuthReverseEngineering,
		ContentModerationRiskCategoryExploitReverseEngineering,
		ContentModerationRiskCategoryCheatAutomation,
		ContentModerationPolicyCategoryViolence,
		ContentModerationPolicyCategoryWeapons,
		ContentModerationPolicyCategoryCyberAbuse,
	}
	for _, category := range blocked {
		require.True(t, policy.includesCategory(category), category)
	}
	require.False(t, policy.includesCategory(ContentModerationRiskCategoryOther))
	require.False(t, policy.includesCategory(ContentModerationPolicyCategoryPrivacy))
	require.InDelta(t, 0.82, policy.CategoryThresholds[ContentModerationRiskCategoryCheatAutomation], 0.0001)
	_, _, matched := policy.matchingCategory(ContentModerationRiskCategoryCheatAutomation, 0.8199, nil)
	require.False(t, matched)
	category, score, matched := policy.matchingCategory(ContentModerationRiskCategoryCheatAutomation, 0.82, nil)
	require.True(t, matched)
	require.Equal(t, ContentModerationRiskCategoryCheatAutomation, category)
	require.InDelta(t, 0.82, score, 0.0001)
	for _, option := range ContentModerationGroupPenaltyCategoryOptions() {
		require.NotEmpty(t, option.Category)
		require.NotEmpty(t, option.LabelZH)
		require.NotEmpty(t, option.LabelEN)
	}
}

func TestAdaptiveSampleDecisionCapturesCurrentGroup(t *testing.T) {
	groupID := int64(16)
	svc := &ContentModerationService{}
	cfg := defaultContentModerationConfig()
	event, sampled := svc.adaptiveSampleDecision(ContentModerationCheckInput{
		RequestID: "req-1",
		UserID:    7,
		GroupID:   &groupID,
	}, cfg, "hash")

	require.True(t, sampled)
	require.NotNil(t, event)
	require.Equal(t, groupID, event.GroupID)
}

func TestApplyUserGroupPenaltyForRiskInvalidatesAuthCache(t *testing.T) {
	blockedUntil := time.Now().Add(24 * time.Hour)
	repo := &contentModerationGroupPenaltyRepoStub{
		contentModerationTestRepo: &contentModerationTestRepo{},
		penalty: &ContentModerationGroupPenalty{
			UserID:       7,
			GroupID:      16,
			StrikeCount:  1,
			BlockedUntil: &blockedUntil,
		},
		applied: true,
	}
	invalidator := &contentModerationTestAuthCacheInvalidator{}
	svc := &ContentModerationService{
		repo:                 repo,
		userRepo:             &contentModerationTestUserRepo{user: &User{ID: 7, Role: RoleUser, Status: StatusActive}},
		authCacheInvalidator: invalidator,
	}

	svc.applyUserGroupPenaltyForRisk(context.Background(), &ContentModerationRiskEvent{
		RequestID: "req-1",
		UserID:    7,
		GroupID:   16,
		Category:  ContentModerationRiskCategoryCheatAutomation,
	}, ContentModerationGroupPenaltyPolicy{
		TargetGroupIDs:   []int64{16, 17},
		FirstBlockHours:  12,
		SecondBlockHours: 48,
	})

	require.Equal(t, 2, repo.applyCalls)
	require.Equal(t, int64(7), repo.event.UserID)
	require.Equal(t, []int64{16, 17}, []int64{repo.events[0].GroupID, repo.events[1].GroupID})
	require.Equal(t, 12, repo.firstHours)
	require.Equal(t, 48, repo.secondHours)
	require.Equal(t, []int64{7}, invalidator.userIDs)
}

func TestApplyUserGroupPenaltyForRiskSkipsAdmin(t *testing.T) {
	repo := &contentModerationGroupPenaltyRepoStub{
		contentModerationTestRepo: &contentModerationTestRepo{},
		penalty:                   &ContentModerationGroupPenalty{UserID: 1, GroupID: 16, Permanent: true},
		applied:                   true,
	}
	svc := &ContentModerationService{
		repo:     repo,
		userRepo: &contentModerationTestUserRepo{user: &User{ID: 1, Role: RoleAdmin, Status: StatusActive}},
	}

	svc.applyUserGroupPenaltyForRisk(context.Background(), &ContentModerationRiskEvent{
		RequestID: "req-admin",
		UserID:    1,
		GroupID:   16,
		Category:  ContentModerationRiskCategoryCheatAutomation,
	}, ContentModerationGroupPenaltyPolicy{
		TargetGroupIDs:   []int64{16},
		FirstBlockHours:  24,
		SecondBlockHours: 36,
	})

	require.Zero(t, repo.applyCalls)
}

func TestAdaptiveRiskEventRecordsScoreBeforeApplyingGroupPenalty(t *testing.T) {
	blockedUntil := time.Now().Add(24 * time.Hour)
	repo := &contentModerationAdaptiveRiskPenaltyRepoStub{
		contentModerationTestRepo: &contentModerationTestRepo{},
		recordApplied:             true,
		profile:                   &ContentModerationRiskProfile{UserID: 7, RiskLevel: ContentModerationRiskLevelWatch},
		penalty: &ContentModerationGroupPenalty{
			UserID:       7,
			GroupID:      16,
			StrikeCount:  1,
			BlockedUntil: &blockedUntil,
		},
	}
	svc := &ContentModerationService{repo: repo}
	cfg := defaultContentModerationConfig()
	cfg.AdaptivePolicy.EnforcementMode = ContentModerationEnforcementEnforce
	cfg.GroupPenalty.Enabled = true
	cfg.GroupPenalty.TargetGroupIDs = []int64{16}
	event := &ContentModerationRiskEvent{RequestID: "req-score", UserID: 7, GroupID: 16}
	applyAdaptiveDecisionToRiskEvent(event, &ContentModerationDecision{
		Audited:         true,
		Flagged:         true,
		HighestCategory: ContentModerationRiskCategoryCheatAutomation,
		HighestScore:    0.96,
		RiskSeverity:    ContentModerationSeverityMedium,
	}, cfg.AdaptivePolicy)

	svc.recordAdaptiveRiskEvent(context.Background(), cfg, event)

	require.Equal(t, []string{"record", "penalty"}, repo.calls)
	require.Greater(t, repo.recordedEvent.ScoreDelta, float64(0))
	require.Equal(t, 24, repo.firstHours)
	require.Equal(t, 36, repo.secondHours)

	repo.calls = nil
	repo.recordApplied = false
	svc.recordAdaptiveRiskEvent(context.Background(), cfg, event)
	require.Equal(t, []string{"record"}, repo.calls, "a duplicate risk event must not add another strike")
}

func TestAdaptiveRiskEventOnlyPenalizesConfiguredCategories(t *testing.T) {
	repo := &contentModerationAdaptiveRiskPenaltyRepoStub{
		contentModerationTestRepo: &contentModerationTestRepo{},
		recordApplied:             true,
		profile:                   &ContentModerationRiskProfile{UserID: 7},
		penalty:                   &ContentModerationGroupPenalty{UserID: 7, GroupID: 16},
	}
	svc := &ContentModerationService{repo: repo}
	cfg := defaultContentModerationConfig()
	cfg.AdaptivePolicy.EnforcementMode = ContentModerationEnforcementEnforce
	cfg.GroupPenalty.Enabled = true
	cfg.GroupPenalty.TargetGroupIDs = []int64{16}
	cfg.GroupPenalty.Categories = []string{ContentModerationRiskCategorySafetyBypass}
	event := &ContentModerationRiskEvent{
		RequestID: "req-filtered",
		UserID:    7,
		Flagged:   true,
		Category:  ContentModerationRiskCategoryCheatAutomation,
	}

	svc.recordAdaptiveRiskEvent(context.Background(), cfg, event)

	require.Equal(t, []string{"record"}, repo.calls)
}

func TestAdaptiveRiskEventOnlyPenalizesAtConfiguredCategoryThreshold(t *testing.T) {
	repo := &contentModerationAdaptiveRiskPenaltyRepoStub{
		contentModerationTestRepo: &contentModerationTestRepo{},
		recordApplied:             true,
		profile:                   &ContentModerationRiskProfile{UserID: 7},
		penalty:                   &ContentModerationGroupPenalty{UserID: 7, GroupID: 16},
	}
	svc := &ContentModerationService{repo: repo}
	cfg := defaultContentModerationConfig()
	cfg.AdaptivePolicy.EnforcementMode = ContentModerationEnforcementEnforce
	cfg.GroupPenalty.Enabled = true
	cfg.GroupPenalty.TargetGroupIDs = []int64{16}
	cfg.GroupPenalty.Categories = []string{ContentModerationRiskCategoryCheatAutomation}
	cfg.GroupPenalty.CategoryThresholds[ContentModerationRiskCategoryCheatAutomation] = 0.9
	event := &ContentModerationRiskEvent{
		RequestID: "req-threshold",
		UserID:    7,
		Flagged:   true,
		Category:  ContentModerationRiskCategoryCheatAutomation,
		Score:     0.89,
	}

	svc.recordAdaptiveRiskEvent(context.Background(), cfg, event)
	require.Equal(t, []string{"record"}, repo.calls)

	repo.calls = nil
	event.RequestID = "req-threshold-hit"
	event.Score = 0.9
	svc.recordAdaptiveRiskEvent(context.Background(), cfg, event)
	require.Equal(t, []string{"record", "penalty"}, repo.calls)
}

func TestAdaptiveRiskEventEvaluatesEachCategoryIndependently(t *testing.T) {
	repo := &contentModerationAdaptiveRiskPenaltyRepoStub{
		contentModerationTestRepo: &contentModerationTestRepo{},
		recordApplied:             true,
		profile:                   &ContentModerationRiskProfile{UserID: 7},
		penalty:                   &ContentModerationGroupPenalty{UserID: 7, GroupID: 16},
	}
	svc := &ContentModerationService{repo: repo}
	cfg := defaultContentModerationConfig()
	cfg.AdaptivePolicy.EnforcementMode = ContentModerationEnforcementEnforce
	cfg.GroupPenalty.Enabled = true
	cfg.GroupPenalty.TargetGroupIDs = []int64{16}
	cfg.GroupPenalty.Categories = []string{ContentModerationRiskCategoryCheatAutomation}
	cfg.GroupPenalty.CategoryThresholds[ContentModerationRiskCategoryCheatAutomation] = 0.9
	event := &ContentModerationRiskEvent{
		RequestID: "req-independent-threshold",
		UserID:    7,
		Flagged:   true,
		Category:  ContentModerationRiskCategorySafetyBypass,
		Score:     0.97,
		CategoryScores: map[string]float64{
			ContentModerationRiskCategorySafetyBypass:    0.97,
			ContentModerationRiskCategoryCheatAutomation: 0.91,
		},
	}

	svc.recordAdaptiveRiskEvent(context.Background(), cfg, event)

	require.Equal(t, []string{"record", "penalty"}, repo.calls)
	require.Equal(t, ContentModerationRiskCategoryCheatAutomation, repo.penaltyEvent.Category)
	require.InDelta(t, 0.91, repo.penaltyEvent.Score, 0.0001)
}

func TestAdaptiveRiskEventShadowModeNeverAppliesGroupPenalty(t *testing.T) {
	repo := &contentModerationAdaptiveRiskPenaltyRepoStub{
		contentModerationTestRepo: &contentModerationTestRepo{},
		recordApplied:             true,
		profile:                   &ContentModerationRiskProfile{UserID: 7},
		penalty:                   &ContentModerationGroupPenalty{UserID: 7, GroupID: 16},
	}
	svc := &ContentModerationService{repo: repo}
	cfg := defaultContentModerationConfig()
	cfg.AdaptivePolicy.EnforcementMode = ContentModerationEnforcementShadow
	cfg.GroupPenalty.Enabled = true
	cfg.GroupPenalty.TargetGroupIDs = []int64{16}
	cfg.GroupPenalty.Categories = []string{ContentModerationRiskCategoryCheatAutomation}
	event := &ContentModerationRiskEvent{
		RequestID: "req-shadow", UserID: 7, Flagged: true,
		Category: ContentModerationRiskCategoryCheatAutomation, Score: 1,
	}

	svc.recordAdaptiveRiskEvent(context.Background(), cfg, event)

	require.Equal(t, []string{"record"}, repo.calls)
}

func TestDecayContentModerationRiskScore(t *testing.T) {
	now := time.Date(2026, 7, 12, 12, 0, 0, 0, time.UTC)
	require.InDelta(t, 90, DecayContentModerationRiskScore(100, now.Add(-24*time.Hour), now, 10), 0.001)
	require.InDelta(t, 81, DecayContentModerationRiskScore(100, now.Add(-48*time.Hour), now, 10), 0.001)
}

func TestAdaptiveRiskSeverityIgnoresGeneralContentCategories(t *testing.T) {
	require.Equal(t, ContentModerationSeverityNone, adaptiveRiskSeverity("harassment", 1))
	require.Equal(t, ContentModerationSeverityNone, adaptiveRiskSeverity("sexual", 1))
	require.Equal(t, ContentModerationSeverityMedium, adaptiveRiskSeverity(ContentModerationRiskCategorySafetyBypass, 0.85))
	require.Equal(t, ContentModerationSeverityNone, adaptiveRiskSeverity(ContentModerationRiskCategoryCredentialTheft, 0.95))
}

func TestParseModelClassifierDecision(t *testing.T) {
	decision, err := parseModelClassifierDecision("```json\n{\"decision\":\"high_risk\",\"category\":\"credential_theft\",\"confidence\":0.97,\"severity\":3}\n```")
	require.NoError(t, err)
	require.Equal(t, "high_risk", decision.Decision)
	require.Equal(t, "credential_theft", decision.Category)
	require.InDelta(t, 0.97, decision.Confidence, 0.0001)

	result := modelClassifierModerationResult(decision)
	require.InDelta(t, 0.97, result.CategoryScores[ContentModerationRiskCategoryCredentialTheft], 0.0001)
	require.Equal(t, ContentModerationSeverityNone, result.RiskSeverity)
}

func TestModelClassifierPromptUsesLatestEndUserInputAndSafeMaintenanceRules(t *testing.T) {
	prompt := contentModerationClassifierSystemPrompt(nil)
	require.Contains(t, prompt, "SSH/SFTP/rsync")
	require.Contains(t, prompt, "installer/package")
	require.Contains(t, prompt, "latest end-user input")
	require.Contains(t, prompt, "excluded system and developer instructions")
	require.NotContains(t, prompt, "[tool_result]")
}

func TestModelClassifierMapsCheatAutomationSeverity(t *testing.T) {
	decision, err := parseModelClassifierDecision(`{"decision":"high_risk","category":"cheat_automation","confidence":0.96,"severity":3}`)
	require.NoError(t, err)
	result := modelClassifierModerationResult(decision)
	require.Equal(t, ContentModerationSeverityMedium, result.RiskSeverity)
	require.InDelta(t, 0.96, result.CategoryScores[ContentModerationRiskCategoryCheatAutomation], 0.0001)
}

func TestModelClassifierMapsUniversalPolicyCategory(t *testing.T) {
	decision, err := parseModelClassifierDecision(`{"decision":"high_risk","category":"privacy_or_sensitive_data","confidence":0.93,"severity":2}`)
	require.NoError(t, err)
	result := modelClassifierModerationResult(decision)
	require.Equal(t, ContentModerationSeverityNone, result.RiskSeverity)
	require.InDelta(t, 0.93, result.CategoryScores[ContentModerationPolicyCategoryPrivacy], 0.0001)
}

func TestModelClassifierDoesNotScoreGenericIPFinding(t *testing.T) {
	decision, err := parseModelClassifierDecision(`{"decision":"high_risk","category":"ip_infringement","confidence":0.99,"severity":3}`)
	require.NoError(t, err)

	result := modelClassifierModerationResult(decision)
	require.True(t, result.Flagged)
	require.Equal(t, ContentModerationSeverityNone, result.RiskSeverity)
}

func TestModelClassifierSafeDecisionDoesNotProduceRiskScore(t *testing.T) {
	decision, err := parseModelClassifierDecision(`{"decision":"safe","category":"none","confidence":0.99,"severity":0}`)
	require.NoError(t, err)
	result := modelClassifierModerationResult(decision)
	flagged, _, _ := evaluateModerationScores(result.CategoryScores, ContentModerationDefaultThresholds())
	require.False(t, flagged)
}

func TestApplyAdaptiveDecisionOnlyScoresFlaggedResults(t *testing.T) {
	policy := DefaultContentModerationAdaptivePolicy()
	event := &ContentModerationRiskEvent{}
	applyAdaptiveDecisionToRiskEvent(event, &ContentModerationDecision{
		Allowed:         true,
		Audited:         true,
		Flagged:         false,
		HighestCategory: ContentModerationRiskCategorySafetyBypass,
		HighestScore:    0.79,
		RiskSeverity:    ContentModerationSeverityMedium,
	}, policy)
	require.Equal(t, ContentModerationSeverityNone, event.Severity)
	require.Zero(t, event.ScoreDelta)

	applyAdaptiveDecisionToRiskEvent(event, &ContentModerationDecision{
		Allowed:         true,
		Audited:         true,
		Flagged:         true,
		HighestCategory: ContentModerationRiskCategorySafetyBypass,
		HighestScore:    0.95,
		RiskSeverity:    ContentModerationSeveritySevere,
	}, policy)
	require.Equal(t, ContentModerationSeveritySevere, event.Severity)
	require.Equal(t, policy.SevereRiskWeight, event.ScoreDelta)
}

func TestCallModelClassifierOnceUsesCompatibleChatRequest(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		require.Equal(t, "/v1/chat/completions", r.URL.Path)
		require.Equal(t, "Bearer test-key", r.Header.Get("Authorization"))
		raw, err := io.ReadAll(r.Body)
		require.NoError(t, err)
		require.Equal(t, signContentModerationClassifierRequest(raw, "test-key"), r.Header.Get(ContentModerationInternalSignatureHeader))
		var payload map[string]any
		require.NoError(t, json.Unmarshal(raw, &payload))
		require.Equal(t, "gpt-5.3-codex-spark", payload["model"])
		require.NotContains(t, payload, "temperature")
		require.NotContains(t, payload, "response_format")
		messages := payload["messages"].([]any)
		systemMessage := messages[0].(map[string]any)["content"].(string)
		require.Contains(t, systemMessage, "custom policy boundary")
		require.Contains(t, systemMessage, "Return exactly one JSON object")
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"choices":[{"message":{"content":"{\"decision\":\"high_risk\",\"category\":\"safety_bypass\",\"confidence\":0.94,\"severity\":3}"}}]}`))
	}))
	defer server.Close()

	svc := &ContentModerationService{httpClient: server.Client()}
	cfg := defaultContentModerationConfig()
	cfg.BaseURL = server.URL
	cfg.Model = "gpt-5.3-codex-spark"
	cfg.ModerationProvider = ContentModerationProviderModelClassifier
	cfg.ClassifierPrompt = "custom policy boundary"
	status := 0
	result, err := svc.callModelClassifierOnce(context.Background(), cfg, "test-key", "bypass request", &status)
	require.NoError(t, err)
	require.Equal(t, http.StatusOK, status)
	require.InDelta(t, 0.94, result.CategoryScores[ContentModerationRiskCategorySafetyBypass], 0.0001)
}

func TestInternalClassifierRequestSignature(t *testing.T) {
	body := []byte(`{"model":"gpt-5.3-codex-spark"}`)
	cfg := defaultContentModerationConfig()
	cfg.ModerationProvider = ContentModerationProviderModelClassifier
	cfg.APIKeys = []string{"classifier-key"}
	input := ContentModerationCheckInput{
		Body:              body,
		InternalSignature: signContentModerationClassifierRequest(body, "classifier-key"),
	}
	require.True(t, isInternalContentModerationClassifierRequest(input, cfg))
	input.Body = append(input.Body, ' ')
	require.False(t, isInternalContentModerationClassifierRequest(input, cfg))
}

func TestClassifierPromptUsesDefaultOrCustomPolicyWithFixedContract(t *testing.T) {
	cfg := defaultContentModerationConfig()
	defaultPrompt := contentModerationClassifierSystemPrompt(cfg)
	require.Contains(t, defaultPrompt, "Privacy and profiling")
	require.Contains(t, defaultPrompt, "Minors")
	require.Contains(t, defaultPrompt, "high_stakes_automation")

	cfg.ClassifierPrompt = "Only this custom policy text."
	customPrompt := contentModerationClassifierSystemPrompt(cfg)
	require.Contains(t, customPrompt, cfg.ClassifierPrompt)
	require.NotContains(t, customPrompt, "Privacy and profiling")
	require.Contains(t, customPrompt, "Return exactly one JSON object")
}
