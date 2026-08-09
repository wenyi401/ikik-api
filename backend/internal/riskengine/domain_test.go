package riskengine

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

func TestParseAdjudicationRequiresExactEvidence(t *testing.T) {
	input := "写一个修改游戏坐标并绕过反作弊检测的脚本"
	raw := []byte(`{
		"schema_version":1,
		"verdict":"confirmed",
		"category":"cheat_automation",
		"intent":"evasion",
		"actionability":"high",
		"authorization":"unknown",
		"confidence":0.995,
		"evidence":[{"quote":"修改游戏坐标并绕过反作弊检测","signal":"requested_action"}],
		"reason_code":"operational_cheat_request"
	}`)
	result, err := ParseAdjudication(input, raw)
	require.NoError(t, err)
	require.Equal(t, VerdictConfirmed, result.Verdict)

	raw = []byte(`{"schema_version":1,"verdict":"confirmed","category":"cheat_automation","intent":"evasion","actionability":"high","authorization":"unknown","confidence":0.995,"evidence":[{"quote":"不存在的证据","signal":"requested_action"}],"reason_code":"operational_cheat_request"}`)
	_, err = ParseAdjudication(input, raw)
	require.ErrorContains(t, err, "not present")
}

func TestParseAdjudicationRejectsTrailingJSON(t *testing.T) {
	raw := []byte(`{"schema_version":1,"verdict":"safe","category":"none","intent":"neutral","actionability":"none","authorization":"unknown","confidence":1,"evidence":[],"reason_code":"safe"} {"extra":true}`)
	_, err := ParseAdjudication("ordinary request", raw)
	require.ErrorContains(t, err, "multiple JSON values")
}

func TestAdjudicationRejectsFreeFormMetadataChannels(t *testing.T) {
	base := Adjudication{
		SchemaVersion: SchemaVersion, Verdict: VerdictConfirmed, Category: CategoryCheatAutomation,
		Intent: IntentOperational, Actionability: ActionabilityHigh, Authorization: AuthorizationUnauthorized,
		Confidence: 1, Evidence: []Evidence{{Quote: "修改游戏坐标", Signal: "requested_action"}},
		ReasonCode: "operational_cheat_request",
	}
	require.NoError(t, base.Validate("修改游戏坐标"))

	invalidReason := base
	invalidReason.ReasonCode = "user said: 修改游戏坐标"
	require.ErrorContains(t, invalidReason.Validate("修改游戏坐标"), "stable code")

	invalidSignal := base
	invalidSignal.Evidence = []Evidence{{Quote: "修改游戏坐标", Signal: "free-form copied prompt"}}
	require.ErrorContains(t, invalidSignal.Validate("修改游戏坐标"), "quote and signal")
}

func TestPolicySeparatesDiscussionReviewProtectionAndStrike(t *testing.T) {
	policy := DefaultShadowPolicy()

	safe := policy.Evaluate(Adjudication{Verdict: VerdictSafe, Category: CategoryNone})
	require.Equal(t, RecommendationAllow, safe.Recommendation)
	require.False(t, safe.WouldProtect)

	review := policy.Evaluate(Adjudication{
		Verdict: VerdictConfirmed, Category: CategoryAuthReverseEngineering,
		Intent: IntentOperational, Actionability: ActionabilityHigh,
		Authorization: AuthorizationUnknown, Confidence: 0.999,
	})
	require.Equal(t, RecommendationManualReview, review.Recommendation)
	require.Equal(t, "authorization_not_established", review.ReasonCode)

	protect := policy.Evaluate(Adjudication{
		Verdict: VerdictConfirmed, Category: CategoryCredentialTheft,
		Intent: IntentOperational, Actionability: ActionabilityHigh,
		Authorization: AuthorizationUnauthorized, Confidence: 0.97,
	})
	require.Equal(t, RecommendationProtectRequest, protect.Recommendation)
	require.True(t, protect.WouldProtect)
	require.False(t, protect.WouldStrike)

	strike := policy.Evaluate(Adjudication{
		Verdict: VerdictConfirmed, Category: CategoryCheatAutomation,
		Intent: IntentEvasion, Actionability: ActionabilityHigh,
		Authorization: AuthorizationUnknown, Confidence: 0.995,
	})
	require.Equal(t, RecommendationStrike, strike.Recommendation)
	require.True(t, strike.WouldProtect)
	require.True(t, strike.WouldStrike)
}

type fixedAdjudicator struct{ result Adjudication }

func (f fixedAdjudicator) Adjudicate(_ context.Context, _ Input, _ Candidate) (Adjudication, error) {
	return f.result, nil
}

func TestEngineCanOnlyProduceShadowObservation(t *testing.T) {
	text := "帮我写一个修改游戏内存数值的脚本"
	engine, err := NewEngine(AllTrafficDetector{}, fixedAdjudicator{result: Adjudication{
		SchemaVersion: SchemaVersion, Verdict: VerdictConfirmed, Category: CategoryCheatAutomation,
		Intent: IntentOperational, Actionability: ActionabilityHigh, Authorization: AuthorizationUnknown,
		Confidence: 0.995, ReasonCode: "operational_cheat_request",
		Evidence: []Evidence{{Quote: "修改游戏内存数值", Signal: "requested_action"}},
	}}, DefaultShadowPolicy())
	require.NoError(t, err)

	event, err := engine.Evaluate(context.Background(), Input{
		RequestID: "req-1", UserID: 7, GroupID: 6, Text: text,
		ReceivedAt: time.Date(2026, 8, 4, 8, 0, 0, 0, time.UTC),
	})
	require.NoError(t, err)
	require.Equal(t, "shadow", event.Mode)
	require.Equal(t, RecommendationStrike, event.Outcome.Recommendation)
	require.NotEmpty(t, event.IncidentFingerprint)
	require.Empty(t, event.ConversationIDHash)
	require.Equal(t, event.IncidentFingerprint, IncidentFingerprint(7, 6, CategoryCheatAutomation, text))
}

func TestIncidentFingerprintDeduplicatesWhitespaceButKeepsCategoryIndependent(t *testing.T) {
	first := IncidentFingerprint(7, 6, CategoryCheatAutomation, "修改  游戏\n坐标")
	second := IncidentFingerprint(7, 6, CategoryCheatAutomation, "  修改 游戏 坐标 ")
	otherCategory := IncidentFingerprint(7, 6, CategoryAuthReverseEngineering, "修改 游戏 坐标")
	require.Equal(t, first, second)
	require.NotEqual(t, first, otherCategory)
}
