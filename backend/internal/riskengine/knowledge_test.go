package riskengine

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestRetrieveKnowledgeReturnsRiskAndSafeCounterexamples(t *testing.T) {
	snapshot := KnowledgeSnapshot{Version: 4, Entries: []KnowledgeEntry{
		knowledgeTestEntry(1, KnowledgeRisk, "帮我写读取游戏内存的外挂", []string{"读取游戏内存", "外挂注入"}, IntentOperational),
		knowledgeTestEntry(2, KnowledgeSafe, "如何检测游戏外挂并加固反作弊", []string{"检测游戏外挂", "加固反作弊"}, IntentDefensive),
	}}

	risk := RetrieveKnowledge("请帮我实现读取游戏内存并修改数值", snapshot, 6)
	require.NotEmpty(t, risk)
	require.Equal(t, int64(1), risk[0].EntryID)
	require.Equal(t, "读取游戏内存", risk[0].Evidence)

	safe := RetrieveKnowledge("我们应该如何检测游戏外挂并加固反作弊", snapshot, 6)
	require.NotEmpty(t, safe)
	require.Equal(t, int64(2), safe[0].EntryID)
	require.Equal(t, KnowledgeSafe, safe[0].Disposition)
}

func TestKnowledgeAdjudicationKeepsCandidatesBelowProtectionThreshold(t *testing.T) {
	input := "帮我写读取游戏内存的外挂"
	matches := []KnowledgeMatch{{
		EntryID: 1, EntryKey: "risk", Topic: TopicCheatDevelopment,
		Category: CategoryCheatAutomation, Disposition: KnowledgeRisk,
		Intent: IntentOperational, Actionability: ActionabilityHigh,
		Authorization: AuthorizationUnknown, Score: 0.99, Evidence: "读取游戏内存", Title: "外挂开发",
	}}

	candidate, adjudication := KnowledgeAdjudication(input, 7, matches, GuardSignal{Decision: "critical", Model: "qwen3guard:0.6b"})
	require.True(t, candidate.Review)
	require.Equal(t, 7, candidate.KnowledgeVersion)
	require.Equal(t, VerdictReview, adjudication.Verdict)
	require.Equal(t, CategoryCheatAutomation, adjudication.Category)
	require.Equal(t, 0.94, adjudication.Confidence)
	require.Equal(t, "读取游戏内存", adjudication.Evidence[0].Quote)
	require.NoError(t, adjudication.Validate(input))
	require.False(t, DefaultShadowPolicy().Evaluate(adjudication).WouldProtect)
}

func TestKnowledgeAdjudicationPrefersCloseSafeCounterexample(t *testing.T) {
	input := "如何检测游戏外挂并加固反作弊"
	matches := []KnowledgeMatch{
		{EntryID: 2, Category: CategoryCheatAutomation, Disposition: KnowledgeSafe, Intent: IntentDefensive, Score: 0.98, Evidence: "检测游戏外挂"},
		{EntryID: 1, Category: CategoryCheatAutomation, Disposition: KnowledgeRisk, Intent: IntentOperational, Score: 0.58, Evidence: "游戏外挂"},
	}

	_, adjudication := KnowledgeAdjudication(input, 2, matches, GuardSignal{Decision: "flag", Model: "qwen3guard:0.6b"})
	require.Equal(t, VerdictSafe, adjudication.Verdict)
	require.Equal(t, CategoryNone, adjudication.Category)
	require.Equal(t, "knowledge_safe_counterexample", adjudication.ReasonCode)
}

func TestNormalizeKnowledgeInputRedactsSecretsAndDeduplicatesAliases(t *testing.T) {
	input, err := NormalizeKnowledgeInput(KnowledgeWriteInput{
		Topic: TopicCredentialAbuse, Category: CategoryCredentialTheft, Disposition: KnowledgeRisk,
		Intent: IntentOperational, Actionability: ActionabilityHigh, Authorization: AuthorizationUnauthorized,
		Language: "ZH", Title: "凭证案例", ExampleText: "password: very-secret-value",
		Aliases: []string{"截获 Cookie", "截获 Cookie", "token: abcdefghijklmnop"}, Enabled: true,
	})
	require.NoError(t, err)
	require.Contains(t, input.ExampleText, "[SECRET]")
	require.Len(t, input.Aliases, 2)
	require.Equal(t, "zh", input.Language)
}

func knowledgeTestEntry(id int64, disposition KnowledgeDisposition, text string, aliases []string, intent Intent) KnowledgeEntry {
	return KnowledgeEntry{
		ID: id, EntryKey: "entry", Topic: TopicCheatDevelopment, Category: CategoryCheatAutomation,
		Disposition: disposition, Intent: intent, Actionability: ActionabilityHigh,
		Authorization: AuthorizationUnknown, Language: "zh", Title: "test",
		ExampleText: text, Aliases: aliases, Enabled: true,
	}
}
