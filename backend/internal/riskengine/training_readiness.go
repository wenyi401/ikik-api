package riskengine

import "sort"

type TrainingGate struct {
	MinimumTotal              int
	MinimumProduction         int
	MinimumSafe               int
	MinimumReview             int
	MinimumConfirmed          int
	MinimumConfirmedPerTarget int
	MinimumTest               int
	RequireConfirmedEvidence  bool
	TargetCategories          []Category
}

type TrainingReadinessReport struct {
	Ready                    bool             `json:"ready"`
	Total                    int              `json:"total"`
	Production               int              `json:"production"`
	Safe                     int              `json:"safe"`
	Review                   int              `json:"review"`
	Confirmed                int              `json:"confirmed"`
	Test                     int              `json:"test"`
	ConfirmedMissingEvidence int              `json:"confirmed_missing_evidence"`
	ConfirmedByCategory      map[Category]int `json:"confirmed_by_category"`
	Reasons                  []string         `json:"reasons"`
}

func DefaultTrainingGate() TrainingGate {
	return TrainingGate{
		MinimumTotal: 500, MinimumProduction: 200, MinimumSafe: 200, MinimumReview: 50,
		MinimumConfirmed: 100, MinimumConfirmedPerTarget: 20, MinimumTest: 100,
		RequireConfirmedEvidence: true,
		TargetCategories: []Category{
			CategoryCheatAutomation, CategoryAuthReverseEngineering, CategoryExploitReverseEngineering,
			CategoryCredentialTheft, CategorySafetyBypass, CategoryAccountAutomation, CategoryCyberAbuse,
		},
	}
}

func AssessTrainingReadiness(items []DatasetExample, gate TrainingGate) TrainingReadinessReport {
	report := TrainingReadinessReport{Total: len(items), ConfirmedByCategory: map[Category]int{}}
	for _, item := range items {
		if item.Source != "synthetic" {
			report.Production++
		}
		if item.Split == "test" {
			report.Test++
		}
		switch item.Label.Verdict {
		case VerdictSafe:
			report.Safe++
		case VerdictReview, VerdictAbstain:
			report.Review++
		case VerdictConfirmed:
			report.Confirmed++
			report.ConfirmedByCategory[item.Label.Category]++
			if len(item.Label.Evidence) == 0 {
				report.ConfirmedMissingEvidence++
			}
		}
	}
	if report.Total < gate.MinimumTotal {
		report.Reasons = append(report.Reasons, "insufficient_total_examples")
	}
	if report.Production < gate.MinimumProduction {
		report.Reasons = append(report.Reasons, "insufficient_production_examples")
	}
	if report.Safe < gate.MinimumSafe {
		report.Reasons = append(report.Reasons, "insufficient_safe_examples")
	}
	if report.Review < gate.MinimumReview {
		report.Reasons = append(report.Reasons, "insufficient_review_examples")
	}
	if report.Confirmed < gate.MinimumConfirmed {
		report.Reasons = append(report.Reasons, "insufficient_confirmed_examples")
	}
	if report.Test < gate.MinimumTest {
		report.Reasons = append(report.Reasons, "insufficient_test_examples")
	}
	if gate.RequireConfirmedEvidence && report.ConfirmedMissingEvidence > 0 {
		report.Reasons = append(report.Reasons, "confirmed_examples_missing_evidence")
	}
	targets := append([]Category(nil), gate.TargetCategories...)
	sort.Slice(targets, func(i, j int) bool { return targets[i] < targets[j] })
	for _, category := range targets {
		if report.ConfirmedByCategory[category] < gate.MinimumConfirmedPerTarget {
			report.Reasons = append(report.Reasons, "insufficient_confirmed_"+string(category))
		}
	}
	report.Ready = len(report.Reasons) == 0
	return report
}
