package riskengine

import "sort"

type BinaryMetrics struct {
	TruePositive  int     `json:"true_positive"`
	FalsePositive int     `json:"false_positive"`
	FalseNegative int     `json:"false_negative"`
	TrueNegative  int     `json:"true_negative"`
	Precision     float64 `json:"precision"`
	Recall        float64 `json:"recall"`
	F1            float64 `json:"f1"`
}

type CategoryMetrics struct {
	Category  Category `json:"category"`
	Support   int      `json:"support"`
	Predicted int      `json:"predicted"`
	Correct   int      `json:"correct"`
	Precision float64  `json:"precision"`
	Recall    float64  `json:"recall"`
}

type EvaluationReport struct {
	Total             int               `json:"total"`
	Missing           int               `json:"missing"`
	ReviewOrAbstain   int               `json:"review_or_abstain"`
	Confirmed         BinaryMetrics     `json:"confirmed"`
	Categories        []CategoryMetrics `json:"categories"`
	AutoProtectReady  bool              `json:"auto_protect_ready"`
	AutoStrikeReady   bool              `json:"auto_strike_ready"`
	ReadinessMessages []string          `json:"readiness_messages"`
}

type ReadinessGate struct {
	MinimumExamples         int
	MinimumPositiveExamples int
	ProtectPrecision        float64
	ProtectRecall           float64
	StrikePrecision         float64
}

type categoryCounter struct {
	support   int
	predicted int
	correct   int
}

func DefaultReadinessGate() ReadinessGate {
	return ReadinessGate{
		MinimumExamples: 500, MinimumPositiveExamples: 100,
		ProtectPrecision: 0.98, ProtectRecall: 0.90, StrikePrecision: 0.995,
	}
}

func EvaluateDataset(gold []DatasetExample, predictions []Prediction, gate ReadinessGate) EvaluationReport {
	predictionByID := make(map[string]Prediction, len(predictions))
	for _, prediction := range predictions {
		predictionByID[prediction.ID] = prediction
	}
	report := EvaluationReport{Total: len(gold)}
	categories := map[Category]*categoryCounter{}
	positiveSupport := 0
	for _, example := range gold {
		prediction, exists := predictionByID[example.ID]
		if !exists {
			report.Missing++
			if example.Label.Verdict == VerdictConfirmed {
				report.Confirmed.FalseNegative++
				positiveSupport++
			}
			continue
		}
		expectedPositive := example.Label.Verdict == VerdictConfirmed
		predictedPositive := prediction.Adjudication.Verdict == VerdictConfirmed
		if expectedPositive {
			positiveSupport++
		}
		switch {
		case expectedPositive && predictedPositive:
			report.Confirmed.TruePositive++
		case !expectedPositive && predictedPositive:
			report.Confirmed.FalsePositive++
		case expectedPositive && !predictedPositive:
			report.Confirmed.FalseNegative++
		default:
			report.Confirmed.TrueNegative++
		}
		if prediction.Adjudication.Verdict == VerdictReview || prediction.Adjudication.Verdict == VerdictAbstain {
			report.ReviewOrAbstain++
		}
		if example.Label.Category != CategoryNone {
			counter := ensureCategoryCounter(categories, example.Label.Category)
			counter.support++
		}
		if prediction.Adjudication.Category != CategoryNone {
			counter := ensureCategoryCounter(categories, prediction.Adjudication.Category)
			counter.predicted++
			if prediction.Adjudication.Category == example.Label.Category {
				counter.correct++
			}
		}
	}
	report.Confirmed.Precision = ratio(report.Confirmed.TruePositive, report.Confirmed.TruePositive+report.Confirmed.FalsePositive)
	report.Confirmed.Recall = ratio(report.Confirmed.TruePositive, report.Confirmed.TruePositive+report.Confirmed.FalseNegative)
	report.Confirmed.F1 = harmonicMean(report.Confirmed.Precision, report.Confirmed.Recall)
	keys := make([]Category, 0, len(categories))
	for category := range categories {
		keys = append(keys, category)
	}
	sort.Slice(keys, func(i, j int) bool { return keys[i] < keys[j] })
	for _, category := range keys {
		counter := categories[category]
		report.Categories = append(report.Categories, CategoryMetrics{
			Category: category, Support: counter.support, Predicted: counter.predicted, Correct: counter.correct,
			Precision: ratio(counter.correct, counter.predicted), Recall: ratio(counter.correct, counter.support),
		})
	}
	report.AutoProtectReady = report.Total >= gate.MinimumExamples && positiveSupport >= gate.MinimumPositiveExamples &&
		report.Missing == 0 && report.Confirmed.Precision >= gate.ProtectPrecision && report.Confirmed.Recall >= gate.ProtectRecall
	report.AutoStrikeReady = report.AutoProtectReady && report.Confirmed.Precision >= gate.StrikePrecision
	if report.Total < gate.MinimumExamples {
		report.ReadinessMessages = append(report.ReadinessMessages, "insufficient_total_examples")
	}
	if positiveSupport < gate.MinimumPositiveExamples {
		report.ReadinessMessages = append(report.ReadinessMessages, "insufficient_confirmed_examples")
	}
	if report.Missing > 0 {
		report.ReadinessMessages = append(report.ReadinessMessages, "missing_predictions")
	}
	if report.Confirmed.Precision < gate.ProtectPrecision {
		report.ReadinessMessages = append(report.ReadinessMessages, "protection_precision_below_gate")
	}
	if report.Confirmed.Recall < gate.ProtectRecall {
		report.ReadinessMessages = append(report.ReadinessMessages, "protection_recall_below_gate")
	}
	if report.Confirmed.Precision < gate.StrikePrecision {
		report.ReadinessMessages = append(report.ReadinessMessages, "strike_precision_below_gate")
	}
	return report
}

func ensureCategoryCounter(values map[Category]*categoryCounter, category Category) *categoryCounter {
	if values[category] == nil {
		values[category] = &categoryCounter{}
	}
	return values[category]
}

func ratio(numerator, denominator int) float64 {
	if denominator == 0 {
		return 0
	}
	return float64(numerator) / float64(denominator)
}

func harmonicMean(left, right float64) float64 {
	if left+right == 0 {
		return 0
	}
	return 2 * left * right / (left + right)
}
