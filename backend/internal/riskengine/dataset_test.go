package riskengine

import (
	"os"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestSeedDatasetIsValidAndContainsHardNegatives(t *testing.T) {
	file, err := os.Open("testdata/seed_benchmark.jsonl")
	require.NoError(t, err)
	defer func() { _ = file.Close() }()
	items, err := ReadDatasetJSONL(file)
	require.NoError(t, err)
	require.GreaterOrEqual(t, len(items), 20)

	var safeDomainTerms, confirmed int
	for _, item := range items {
		if item.Label.Verdict == VerdictSafe && strings.ContainsAny(item.Text, "外挂逆向破解绕过") {
			safeDomainTerms++
		}
		if item.Label.Verdict == VerdictConfirmed {
			confirmed++
		}
	}
	require.GreaterOrEqual(t, safeDomainTerms, 5)
	require.GreaterOrEqual(t, confirmed, 7)
}

func TestSanitizeTrainingTextRemovesDirectIdentifiers(t *testing.T) {
	input := "email=user@example.com ip=159.195.16.145 IPv6=2a0a:4cc0:101:1423:852:86ff:feca:a57c Bearer abcdefghijklmnop password=verysecret 密码：中文秘密 password provided in this prompt: browsersecret and password plainsecret sk-abcdefghijk C:\\Users\\alice\\secret.txt /home/bob/project /Users/carol/work"
	redacted := SanitizeTrainingText(input)
	require.NotContains(t, redacted, "user@example.com")
	require.NotContains(t, redacted, "159.195.16.145")
	require.NotContains(t, redacted, "abcdefghijklmnop")
	require.NotContains(t, redacted, "verysecret")
	require.NotContains(t, redacted, "中文秘密")
	require.NotContains(t, redacted, "browsersecret")
	require.NotContains(t, redacted, "plainsecret")
	require.NotContains(t, redacted, "2a0a:4cc0")
	require.NotContains(t, redacted, "sk-abcdefghijk")
	require.NotContains(t, redacted, "alice")
	require.NotContains(t, redacted, "/home/bob")
	require.NotContains(t, redacted, "/Users/carol")
}

func TestCanonicalizeObservedTextExtractsCurrentRequestAndDropsHarnessHistory(t *testing.T) {
	text, keep := CanonicalizeObservedText("<current_user_request> 检查自己的服务器配置 </current_user_request> Handle this request. system noise")
	require.True(t, keep)
	require.Equal(t, "检查自己的服务器配置", text)

	_, keep = CanonicalizeObservedText("Another language model started to solve this problem and produced a summary of its thinking process.")
	require.False(t, keep)

	_, keep = CanonicalizeObservedText("问题： 继续 解决方案： old dangerous tool history")
	require.False(t, keep)

	text, keep = CanonicalizeObservedText(`<environment_context><cwd>/home/private</cwd></environment_context>
# Files mentioned by the user:
## My request for Codex:
你去看看测试项目`)
	require.True(t, keep)
	require.Equal(t, "你去看看测试项目", text)

	text, keep = CanonicalizeObservedText("按你说的继续缩小范围定位\n\nAnother language model started to solve this problem and produced a summary")
	require.True(t, keep)
	require.Equal(t, "按你说的继续缩小范围定位", text)
}

func TestPrepareReviewQueueRejectsPromptAuditWithoutReliableUserBoundary(t *testing.T) {
	raw := strings.Join([]string{
		`{"source":"prompt_audit","source_id":"1","text":"# AGENTS.md instructions\n<environment_context>private</environment_context>\nactual-looking text"}`,
		`{"source":"prompt_audit","source_id":"2","text":"<environment_context>private</environment_context>\n## My request for Codex:\n检查这个错误"}`,
		`{"source":"prompt_audit","source_id":"3","text":"普通用户问题"}`,
	}, "\n")
	items, err := PrepareReviewQueue(strings.NewReader(raw))
	require.NoError(t, err)
	require.Len(t, items, 2)
	texts := []string{items[0].Text, items[1].Text}
	require.Contains(t, texts, "检查这个错误")
	require.Contains(t, texts, "普通用户问题")
}

func TestMergeReviewQueuesPrefersCanonicalContentModerationSource(t *testing.T) {
	text := "相同的当前用户请求"
	items := MergeReviewQueues(
		[]ReviewItem{{ID: "prompt", Text: text, Source: "prompt_audit"}},
		[]ReviewItem{{ID: "moderation", Text: text, Source: "content_moderation", SuggestedCategory: "cheat_automation"}},
	)
	require.Len(t, items, 1)
	require.Equal(t, "moderation", items[0].ID)
}

func TestSuggestionBaselineTreatsLegacyCategoryHitAsConfirmed(t *testing.T) {
	review := []ReviewItem{
		{ID: "safe", SuggestedCategory: "politically_sensitive_topics"},
		{ID: "risk", SuggestedCategory: "cheat_automation"},
	}
	assignments := []LabelAssignment{{ID: "safe"}, {ID: "risk"}}
	predictions, err := SuggestionBaselinePredictions(review, assignments)
	require.NoError(t, err)
	byID := map[string]Prediction{}
	for _, prediction := range predictions {
		byID[prediction.ID] = prediction
	}
	require.Equal(t, VerdictConfirmed, byID["risk"].Adjudication.Verdict)
	require.Equal(t, CategoryCheatAutomation, byID["risk"].Adjudication.Category)
	require.Equal(t, VerdictSafe, byID["safe"].Adjudication.Verdict)
}

func TestEvaluateDatasetDoesNotApproveTinyPerfectBenchmark(t *testing.T) {
	gold := []DatasetExample{
		{ID: "safe", Label: DatasetLabel{Verdict: VerdictSafe, Category: CategoryNone}},
		{ID: "risk", Label: DatasetLabel{Verdict: VerdictConfirmed, Category: CategoryCheatAutomation}},
	}
	predictions := []Prediction{
		{ID: "safe", Adjudication: Adjudication{Verdict: VerdictSafe, Category: CategoryNone}},
		{ID: "risk", Adjudication: Adjudication{Verdict: VerdictConfirmed, Category: CategoryCheatAutomation}},
	}
	report := EvaluateDataset(gold, predictions, DefaultReadinessGate())
	require.Equal(t, 1.0, report.Confirmed.Precision)
	require.Equal(t, 1.0, report.Confirmed.Recall)
	require.False(t, report.AutoProtectReady)
	require.False(t, report.AutoStrikeReady)
	require.Contains(t, report.ReadinessMessages, "insufficient_total_examples")
}
