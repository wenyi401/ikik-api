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
