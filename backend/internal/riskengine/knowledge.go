package riskengine

import (
	"errors"
	"fmt"
	"math"
	"sort"
	"strings"
	"unicode"
	"unicode/utf8"
)

const (
	DefaultKnowledgeMatchLimit = 6
	minimumKnowledgeScore      = 0.24
)

type KnowledgeTopic string

const (
	TopicCheatDevelopment   KnowledgeTopic = "cheat_development"
	TopicCheatUsage         KnowledgeTopic = "cheat_usage"
	TopicReverseEngineering KnowledgeTopic = "reverse_engineering"
	TopicLicenseCracking    KnowledgeTopic = "license_cracking"
	TopicDetectionBypass    KnowledgeTopic = "detection_bypass"
	TopicAccountAutomation  KnowledgeTopic = "account_automation"
	TopicThirdPartyScripts  KnowledgeTopic = "third_party_scripts"
	TopicCredentialAbuse    KnowledgeTopic = "credential_abuse"
	TopicBenignResearch     KnowledgeTopic = "benign_research"
)

var knowledgeTopics = map[KnowledgeTopic]struct{}{
	TopicCheatDevelopment: {}, TopicCheatUsage: {}, TopicReverseEngineering: {},
	TopicLicenseCracking: {}, TopicDetectionBypass: {}, TopicAccountAutomation: {},
	TopicThirdPartyScripts: {}, TopicCredentialAbuse: {}, TopicBenignResearch: {},
}

type KnowledgeDisposition string

const (
	KnowledgeSafe   KnowledgeDisposition = "safe"
	KnowledgeReview KnowledgeDisposition = "review"
	KnowledgeRisk   KnowledgeDisposition = "risk"
)

type KnowledgeEntry struct {
	ID            int64                `json:"id"`
	EntryKey      string               `json:"entry_key"`
	Topic         KnowledgeTopic       `json:"topic"`
	Category      Category             `json:"category"`
	Disposition   KnowledgeDisposition `json:"disposition"`
	Intent        Intent               `json:"intent"`
	Actionability Actionability        `json:"actionability"`
	Authorization Authorization        `json:"authorization"`
	Language      string               `json:"language"`
	Title         string               `json:"title"`
	ExampleText   string               `json:"example_text"`
	Aliases       []string             `json:"aliases"`
	Rationale     string               `json:"rationale"`
	Enabled       bool                 `json:"enabled"`
	SourceType    string               `json:"source_type"`
	SourceEventID *int64               `json:"source_event_id,omitempty"`
	Revision      int                  `json:"revision"`
	CreatedBy     *int64               `json:"created_by,omitempty"`
	UpdatedBy     *int64               `json:"updated_by,omitempty"`
	CreatedAt     string               `json:"created_at"`
	UpdatedAt     string               `json:"updated_at"`
}

type KnowledgeWriteInput struct {
	Topic         KnowledgeTopic       `json:"topic"`
	Category      Category             `json:"category"`
	Disposition   KnowledgeDisposition `json:"disposition"`
	Intent        Intent               `json:"intent"`
	Actionability Actionability        `json:"actionability"`
	Authorization Authorization        `json:"authorization"`
	Language      string               `json:"language"`
	Title         string               `json:"title"`
	ExampleText   string               `json:"example_text"`
	Aliases       []string             `json:"aliases"`
	Rationale     string               `json:"rationale"`
	Enabled       bool                 `json:"enabled"`
	SourceType    string               `json:"source_type,omitempty"`
	SourceEventID *int64               `json:"source_event_id,omitempty"`
}

type KnowledgeSnapshot struct {
	Version int              `json:"version"`
	Entries []KnowledgeEntry `json:"entries"`
}

type KnowledgeMatch struct {
	EntryID       int64                `json:"entry_id"`
	EntryKey      string               `json:"entry_key"`
	Topic         KnowledgeTopic       `json:"topic"`
	Category      Category             `json:"category"`
	Disposition   KnowledgeDisposition `json:"disposition"`
	Intent        Intent               `json:"intent"`
	Actionability Actionability        `json:"actionability"`
	Authorization Authorization        `json:"authorization"`
	Score         float64              `json:"score"`
	Evidence      string               `json:"evidence"`
	Title         string               `json:"title"`
}

type GuardSignal struct {
	Decision   string
	Categories []string
	Model      string
}

func NormalizeKnowledgeInput(input KnowledgeWriteInput) (KnowledgeWriteInput, error) {
	input.Title = strings.TrimSpace(SanitizeTrainingText(input.Title))
	input.ExampleText = strings.TrimSpace(SanitizeTrainingText(input.ExampleText))
	input.Rationale = strings.TrimSpace(SanitizeTrainingText(input.Rationale))
	input.Language = strings.ToLower(strings.TrimSpace(input.Language))
	input.SourceType = strings.ToLower(strings.TrimSpace(input.SourceType))
	if input.SourceType == "" {
		input.SourceType = "admin"
	}
	aliases := make([]string, 0, len(input.Aliases))
	seen := map[string]struct{}{}
	for _, value := range input.Aliases {
		value = strings.TrimSpace(SanitizeTrainingText(value))
		if value == "" {
			continue
		}
		key := strings.ToLower(value)
		if _, exists := seen[key]; exists {
			continue
		}
		seen[key] = struct{}{}
		aliases = append(aliases, value)
	}
	input.Aliases = aliases
	if _, ok := knowledgeTopics[input.Topic]; !ok {
		return input, fmt.Errorf("invalid risk knowledge topic %q", input.Topic)
	}
	if _, ok := domainCategories[input.Category]; !ok {
		return input, fmt.Errorf("invalid risk knowledge category %q", input.Category)
	}
	if input.Disposition != KnowledgeSafe && input.Disposition != KnowledgeReview && input.Disposition != KnowledgeRisk {
		return input, fmt.Errorf("invalid risk knowledge disposition %q", input.Disposition)
	}
	if !validIntent(input.Intent) || !validActionability(input.Actionability) || !validAuthorization(input.Authorization) {
		return input, errors.New("invalid risk knowledge adjudication fields")
	}
	if input.Language != "zh" && input.Language != "en" && input.Language != "multilingual" {
		return input, errors.New("risk knowledge language must be zh, en, or multilingual")
	}
	if input.Title == "" || utf8.RuneCountInString(input.Title) > 160 {
		return input, errors.New("risk knowledge title is required and must not exceed 160 characters")
	}
	if input.ExampleText == "" || utf8.RuneCountInString(input.ExampleText) > 2000 {
		return input, errors.New("risk knowledge example is required and must not exceed 2000 characters")
	}
	if utf8.RuneCountInString(input.Rationale) > 1000 {
		return input, errors.New("risk knowledge rationale must not exceed 1000 characters")
	}
	if len(input.Aliases) > 24 {
		return input, errors.New("risk knowledge aliases must not exceed 24 entries")
	}
	for _, alias := range input.Aliases {
		if utf8.RuneCountInString(alias) > 96 {
			return input, errors.New("risk knowledge alias must not exceed 96 characters")
		}
	}
	if input.SourceType != "seed" && input.SourceType != "admin" && input.SourceType != "audit_review" {
		return input, errors.New("invalid risk knowledge source type")
	}
	if input.SourceEventID != nil && *input.SourceEventID <= 0 {
		return input, errors.New("risk knowledge source event id must be positive")
	}
	return input, nil
}

func RetrieveKnowledge(input string, snapshot KnowledgeSnapshot, limit int) []KnowledgeMatch {
	input = strings.TrimSpace(input)
	if input == "" || len(snapshot.Entries) == 0 {
		return nil
	}
	if limit <= 0 || limit > 20 {
		limit = DefaultKnowledgeMatchLimit
	}
	inputFolded := foldKnowledgeText(input)
	inputFeatures := knowledgeFeatures(inputFolded)
	matches := make([]KnowledgeMatch, 0, limit)
	for _, entry := range snapshot.Entries {
		if !entry.Enabled {
			continue
		}
		score, evidence := knowledgeEntryScore(input, inputFolded, inputFeatures, entry)
		if score < minimumKnowledgeScore {
			continue
		}
		matches = append(matches, KnowledgeMatch{
			EntryID: entry.ID, EntryKey: entry.EntryKey, Topic: entry.Topic, Category: entry.Category,
			Disposition: entry.Disposition, Intent: entry.Intent, Actionability: entry.Actionability,
			Authorization: entry.Authorization, Score: score, Evidence: evidence, Title: entry.Title,
		})
	}
	sort.SliceStable(matches, func(i, j int) bool {
		if math.Abs(matches[i].Score-matches[j].Score) > 0.00001 {
			return matches[i].Score > matches[j].Score
		}
		if matches[i].Disposition != matches[j].Disposition {
			return matches[i].Disposition == KnowledgeSafe
		}
		return matches[i].EntryID < matches[j].EntryID
	})
	if len(matches) > limit {
		matches = matches[:limit]
	}
	return matches
}

func KnowledgeAdjudication(input string, version int, matches []KnowledgeMatch, guard GuardSignal) (Candidate, Adjudication) {
	candidate := Candidate{KnowledgeVersion: version, KnowledgeMatches: append([]KnowledgeMatch(nil), matches...)}
	if len(matches) == 0 {
		candidate.Signals = []string{"knowledge_no_match"}
		return candidate, Adjudication{
			SchemaVersion: SchemaVersion, Verdict: VerdictSafe, Category: CategoryNone,
			Intent: IntentNeutral, Actionability: ActionabilityNone, Authorization: AuthorizationUnknown,
			Confidence: 0.75, ReasonCode: "knowledge_no_match", Model: guard.Model,
		}
	}
	candidate.Review = true
	candidate.Confidence = matches[0].Score
	candidate.Signals = []string{"knowledge_match", "guard_" + stableGuardDecision(guard.Decision)}

	var bestRisk *KnowledgeMatch
	var bestSafe *KnowledgeMatch
	for index := range matches {
		match := &matches[index]
		if match.Disposition == KnowledgeSafe && (bestSafe == nil || match.Score > bestSafe.Score) {
			bestSafe = match
		}
		if match.Disposition != KnowledgeSafe && match.Category != CategoryNone && (bestRisk == nil || match.Score > bestRisk.Score) {
			bestRisk = match
		}
	}
	if bestRisk == nil || bestSafe != nil && bestSafe.Score >= bestRisk.Score-0.03 {
		candidate.Signals = append(candidate.Signals, "safe_counterexample")
		return candidate, Adjudication{
			SchemaVersion: SchemaVersion, Verdict: VerdictSafe, Category: CategoryNone,
			Intent: IntentDefensive, Actionability: ActionabilityNone, Authorization: AuthorizationAuthorized,
			Confidence: clampKnowledgeConfidence(matches[0].Score), ReasonCode: "knowledge_safe_counterexample", Model: guard.Model,
		}
	}
	evidence := strings.TrimSpace(bestRisk.Evidence)
	adjudication := Adjudication{
		SchemaVersion: SchemaVersion, Verdict: VerdictReview, Category: bestRisk.Category,
		Intent: bestRisk.Intent, Actionability: bestRisk.Actionability, Authorization: bestRisk.Authorization,
		Confidence: clampKnowledgeConfidence(bestRisk.Score), ReasonCode: "knowledge_domain_candidate", Model: guard.Model,
	}
	if evidence != "" && strings.Contains(input, evidence) {
		adjudication.Evidence = []Evidence{{Quote: evidence, Signal: "requested_action"}}
	}
	return candidate, adjudication
}

func knowledgeEntryScore(original, folded string, inputFeatures map[string]struct{}, entry KnowledgeEntry) (float64, string) {
	best := 0.0
	evidence := ""
	values := append([]string{entry.ExampleText}, entry.Aliases...)
	for index, value := range values {
		value = strings.TrimSpace(value)
		if value == "" {
			continue
		}
		valueFolded := foldKnowledgeText(value)
		if position := strings.Index(folded, valueFolded); position >= 0 {
			score := 0.93
			if index == 0 {
				score = 0.99
			}
			if score > best {
				best = score
				evidence = exactKnowledgeEvidence(original, value)
			}
			continue
		}
		features := knowledgeFeatures(valueFolded)
		score := featureSimilarity(inputFeatures, features)
		if index > 0 {
			score *= 0.92
		}
		if score > best {
			best = score
			evidence = bestSharedEvidence(original, value)
		}
	}
	return math.Round(best*10000) / 10000, evidence
}

func foldKnowledgeText(value string) string {
	value = strings.ToLower(strings.TrimSpace(value))
	var builder strings.Builder
	space := false
	for _, r := range value {
		if unicode.IsLetter(r) || unicode.IsNumber(r) || r >= '\u4e00' && r <= '\u9fff' {
			builder.WriteRune(r)
			space = false
			continue
		}
		if !space {
			builder.WriteByte(' ')
			space = true
		}
	}
	return strings.Join(strings.Fields(builder.String()), " ")
}

func knowledgeFeatures(value string) map[string]struct{} {
	features := map[string]struct{}{}
	for _, field := range strings.Fields(value) {
		if utf8.RuneCountInString(field) >= 2 {
			features["w:"+field] = struct{}{}
		}
	}
	runes := []rune(strings.ReplaceAll(value, " ", ""))
	for index := 0; index+1 < len(runes); index++ {
		features["g:"+string(runes[index:index+2])] = struct{}{}
	}
	return features
}

func featureSimilarity(input, example map[string]struct{}) float64 {
	if len(input) == 0 || len(example) == 0 {
		return 0
	}
	intersection := 0
	for feature := range example {
		if _, ok := input[feature]; ok {
			intersection++
		}
	}
	if intersection == 0 {
		return 0
	}
	overlap := float64(intersection) / float64(min(len(input), len(example)))
	union := len(input) + len(example) - intersection
	jaccard := float64(intersection) / float64(union)
	return overlap*0.72 + jaccard*0.28
}

func exactKnowledgeEvidence(original, candidate string) string {
	lowerOriginal := strings.ToLower(original)
	position := strings.Index(lowerOriginal, strings.ToLower(candidate))
	if position < 0 {
		return ""
	}
	return original[position : position+len(candidate)]
}

func bestSharedEvidence(original, candidate string) string {
	for _, field := range strings.FieldsFunc(candidate, func(r rune) bool { return unicode.IsSpace(r) || unicode.IsPunct(r) }) {
		if utf8.RuneCountInString(field) < 3 {
			continue
		}
		if evidence := exactKnowledgeEvidence(original, field); evidence != "" {
			return evidence
		}
	}
	return ""
}

func clampKnowledgeConfidence(value float64) float64 {
	if value < 0.5 {
		return 0.5
	}
	// Knowledge retrieval is a candidate signal, not a calibrated model. Keep
	// it below the automatic protection threshold until reviewed labels exist.
	if value > 0.94 {
		return 0.94
	}
	return math.Round(value*1000) / 1000
}

func stableGuardDecision(value string) string {
	value = strings.ToLower(strings.TrimSpace(value))
	switch value {
	case "pass", "flag", "critical":
		return value
	default:
		return "unknown"
	}
}
