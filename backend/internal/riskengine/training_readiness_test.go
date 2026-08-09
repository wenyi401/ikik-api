package riskengine

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestTrainingReadinessRefusesSmallOrEvidenceFreeDataset(t *testing.T) {
	items := []DatasetExample{
		{Source: "production", Split: "test", Label: DatasetLabel{Verdict: VerdictConfirmed, Category: CategoryCheatAutomation}},
		{Source: "synthetic", Split: "train", Label: DatasetLabel{Verdict: VerdictSafe, Category: CategoryNone}},
	}
	report := AssessTrainingReadiness(items, DefaultTrainingGate())
	require.False(t, report.Ready)
	require.Equal(t, 1, report.Production)
	require.Equal(t, 1, report.ConfirmedMissingEvidence)
	require.Contains(t, report.Reasons, "insufficient_total_examples")
	require.Contains(t, report.Reasons, "confirmed_examples_missing_evidence")
	require.Contains(t, report.Reasons, "insufficient_confirmed_cheat_automation")
}

func TestDatasetEvidenceMustQuoteInput(t *testing.T) {
	example := DatasetExample{
		ID: "risk", Text: "修改游戏坐标", Language: "zh", Source: "synthetic", Split: "train",
		Label: DatasetLabel{
			Verdict: VerdictConfirmed, Category: CategoryCheatAutomation, Intent: IntentOperational,
			Actionability: ActionabilityHigh, Authorization: AuthorizationUnauthorized,
			Evidence: []Evidence{{Quote: "不存在的内容", Signal: "requested_action"}},
		},
	}
	require.ErrorContains(t, example.Validate(), "not present")
}
