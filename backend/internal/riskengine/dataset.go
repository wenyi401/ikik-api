package riskengine

import (
	"bufio"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"regexp"
	"sort"
	"strings"
)

type DatasetLabel struct {
	Verdict       Verdict       `json:"verdict"`
	Category      Category      `json:"category"`
	Intent        Intent        `json:"intent"`
	Actionability Actionability `json:"actionability"`
	Authorization Authorization `json:"authorization"`
	Evidence      []Evidence    `json:"evidence,omitempty"`
	ReasonCode    string        `json:"reason_code,omitempty"`
}

type DatasetExample struct {
	ID        string       `json:"id"`
	Text      string       `json:"text"`
	Language  string       `json:"language"`
	Source    string       `json:"source"`
	SourceRef string       `json:"source_ref,omitempty"`
	Split     string       `json:"split"`
	Label     DatasetLabel `json:"label"`
	Notes     string       `json:"notes,omitempty"`
}

type Prediction struct {
	ID             string         `json:"id"`
	Adjudication   Adjudication   `json:"adjudication"`
	Recommendation Recommendation `json:"recommendation,omitempty"`
}

type LabelAssignment struct {
	ID    string       `json:"id"`
	Label DatasetLabel `json:"label"`
	Notes string       `json:"notes,omitempty"`
}

type redactionRule struct {
	pattern     *regexp.Regexp
	replacement string
}

var trainingRedactors = []redactionRule{
	{regexp.MustCompile(`(?i)\b[A-Z0-9._%+-]+@[A-Z0-9.-]+\.[A-Z]{2,}\b`), "[EMAIL]"},
	{regexp.MustCompile(`\b(?:\d{1,3}\.){3}\d{1,3}(?:/\d{1,2})?\b`), "[IP]"},
	{regexp.MustCompile(`(?i)\b[0-9a-f]{1,4}(?::[0-9a-f]{0,4}){2,}(?:/\d{1,3})?\b`), "[IP]"},
	{regexp.MustCompile(`(?i)\bbearer\s+[A-Za-z0-9._~+\-/=]{8,}`), "Bearer [TOKEN]"},
	{regexp.MustCompile(`\bsk-[A-Za-z0-9_-]{8,}\b`), "[API_KEY]"},
	{regexp.MustCompile(`(?i)(password(?:\s+provided\s+in\s+this\s+prompt)?)\s*[:=：]\s*[^\s,;，；]{4,}`), "$1=[SECRET]"},
	{regexp.MustCompile(`(?i)\b(password|passwd|secret|token|api[_ -]?key)\b\s*[:=：]\s*[^\s,;，；]{4,}`), "$1=[SECRET]"},
	{regexp.MustCompile(`(?i)\b(password|passwd)\s+[^\s,;，；]{4,}`), "$1 [SECRET]"},
	{regexp.MustCompile(`(用户名|账号|密码|口令|密钥|令牌)\s*[:=：]\s*[^\s,;，；]{2,}`), "$1：[SECRET]"},
	{regexp.MustCompile(`(?i)https?://[^\s<>"']+`), "[URL]"},
	{regexp.MustCompile(`(?i)\b[A-Z]:\\Users\\[^\\\s]+`), `C:\Users\[USER]`},
	{regexp.MustCompile(`/home/[^/\s]+`), `/home/[USER]`},
	{regexp.MustCompile(`/Users/[^/\s]+`), `/Users/[USER]`},
}

func SanitizeTrainingText(value string) string {
	value = strings.ReplaceAll(value, "\x00", "")
	for _, redactor := range trainingRedactors {
		value = redactor.pattern.ReplaceAllString(value, redactor.replacement)
	}
	return strings.TrimSpace(value)
}

type RawReviewSample struct {
	Source            string `json:"source"`
	SourceID          string `json:"source_id"`
	Text              string `json:"text"`
	SuggestedCategory string `json:"suggested_category,omitempty"`
}

type ReviewItem struct {
	ID                string        `json:"id"`
	Text              string        `json:"text"`
	Language          string        `json:"language"`
	Source            string        `json:"source"`
	SourceRef         string        `json:"source_ref"`
	SuggestedCategory string        `json:"suggested_category,omitempty"`
	Split             string        `json:"split"`
	Status            string        `json:"status"`
	Truncated         bool          `json:"truncated,omitempty"`
	Label             *DatasetLabel `json:"label,omitempty"`
}

func PrepareReviewQueue(reader io.Reader) ([]ReviewItem, error) {
	scanner := bufio.NewScanner(reader)
	scanner.Buffer(make([]byte, 64*1024), 4*1024*1024)
	items := make([]ReviewItem, 0)
	seenText := map[string]struct{}{}
	lineNumber := 0
	for scanner.Scan() {
		lineNumber++
		line := strings.TrimSpace(scanner.Text())
		if line == "" {
			continue
		}
		var raw RawReviewSample
		if err := json.Unmarshal([]byte(line), &raw); err != nil {
			return nil, fmt.Errorf("decode raw review sample line %d: %w", lineNumber, err)
		}
		if !reliableObservedSource(raw.Source, raw.Text) {
			continue
		}
		text, keep := CanonicalizeObservedText(raw.Text)
		if !keep {
			continue
		}
		text = SanitizeTrainingText(text)
		if text == "" {
			continue
		}
		text, truncated := limitRunes(text, 8000)
		textHash := DatasetTextHash(text)
		if _, exists := seenText[textHash]; exists {
			continue
		}
		seenText[textHash] = struct{}{}
		source := strings.TrimSpace(raw.Source)
		sourceRef := DatasetTextHash(source + "\x00" + strings.TrimSpace(raw.SourceID))
		items = append(items, ReviewItem{
			ID: "prod-" + textHash[:16], Text: text, Language: detectLanguage(text),
			Source: source, SourceRef: sourceRef, SuggestedCategory: normalizeSuggestedCategory(raw.SuggestedCategory),
			Split: DeterministicSplit(text), Status: "unreviewed", Truncated: truncated,
		})
	}
	if err := scanner.Err(); err != nil {
		return nil, fmt.Errorf("read raw review samples: %w", err)
	}
	sort.SliceStable(items, func(i, j int) bool { return items[i].ID < items[j].ID })
	return items, nil
}

func CanonicalizeObservedText(value string) (string, bool) {
	value = strings.TrimSpace(value)
	if value == "" {
		return "", false
	}
	if extracted, ok := between(value, "<current_user_request>", "</current_user_request>"); ok {
		value = extracted
	}
	if marker := "## My request for Codex:"; strings.Contains(value, marker) {
		value = value[strings.LastIndex(value, marker)+len(marker):]
	}
	for _, block := range [][2]string{
		{"<environment_context", "</environment_context>"},
		{"<in-app-browser-context", "</in-app-browser-context>"},
		{"<app-context", "</app-context>"},
		{"<INSTRUCTIONS>", "</INSTRUCTIONS>"},
	} {
		value = removeDelimitedBlocks(value, block[0], block[1])
	}
	if index := strings.Index(strings.ToLower(value), "another language model started to solve this problem"); index >= 0 {
		value = value[:index]
	}
	lower := strings.ToLower(strings.Join(strings.Fields(value), " "))
	if strings.HasPrefix(lower, "the following is the codex agent history") ||
		strings.HasPrefix(lower, "openclaw runtime context for the immediately preceding user message") ||
		strings.HasPrefix(lower, "# overview generate 0 to 3 hyperpersonalized suggestions") {
		return "", false
	}
	if question, ok := between(value, "问题：", "解决方案："); ok {
		value = question
	}
	for _, marker := range []string{"系统附加说明：", "\nHandle this request.", "\n历史记录:"} {
		if index := strings.Index(value, marker); index >= 0 {
			value = value[:index]
		}
	}
	value = strings.TrimSpace(strings.TrimPrefix(value, "# AGENTS.md instructions"))
	value = strings.TrimSpace(strings.TrimPrefix(value, "# Global Codex Guidance"))
	value = strings.TrimSpace(value)
	if value == "" || value == "继续" || value == "继续分析" {
		return "", false
	}
	return value, true
}

func WriteReviewQueueJSONL(writer io.Writer, items []ReviewItem) error {
	encoder := json.NewEncoder(writer)
	encoder.SetEscapeHTML(false)
	for _, item := range items {
		if err := encoder.Encode(item); err != nil {
			return fmt.Errorf("write review queue: %w", err)
		}
	}
	return nil
}

func ReadReviewQueueJSONL(reader io.Reader) ([]ReviewItem, error) {
	scanner := bufio.NewScanner(reader)
	scanner.Buffer(make([]byte, 64*1024), 2*1024*1024)
	items := make([]ReviewItem, 0)
	lineNumber := 0
	for scanner.Scan() {
		lineNumber++
		line := strings.TrimSpace(scanner.Text())
		if line == "" {
			continue
		}
		var item ReviewItem
		if err := json.Unmarshal([]byte(line), &item); err != nil {
			return nil, fmt.Errorf("decode review item line %d: %w", lineNumber, err)
		}
		if strings.TrimSpace(item.ID) == "" || strings.TrimSpace(item.Text) == "" {
			return nil, fmt.Errorf("review item line %d requires id and text", lineNumber)
		}
		items = append(items, item)
	}
	if err := scanner.Err(); err != nil {
		return nil, fmt.Errorf("read review queue: %w", err)
	}
	return items, nil
}

func MergeReviewQueues(queues ...[]ReviewItem) []ReviewItem {
	byText := map[string]ReviewItem{}
	for _, queue := range queues {
		for _, item := range queue {
			key := DatasetTextHash(item.Text)
			current, exists := byText[key]
			if !exists || preferReviewItem(item, current) {
				byText[key] = item
			}
		}
	}
	items := make([]ReviewItem, 0, len(byText))
	for _, item := range byText {
		items = append(items, item)
	}
	sort.SliceStable(items, func(i, j int) bool { return items[i].ID < items[j].ID })
	return items
}

func preferReviewItem(candidate, current ReviewItem) bool {
	if current.Source != "content_moderation" && candidate.Source == "content_moderation" {
		return true
	}
	return current.SuggestedCategory == "" && candidate.SuggestedCategory != ""
}

func ReadPredictionsJSONL(reader io.Reader) ([]Prediction, error) {
	scanner := bufio.NewScanner(reader)
	scanner.Buffer(make([]byte, 64*1024), 2*1024*1024)
	items := make([]Prediction, 0)
	lineNumber := 0
	for scanner.Scan() {
		lineNumber++
		line := strings.TrimSpace(scanner.Text())
		if line == "" {
			continue
		}
		var item Prediction
		if err := json.Unmarshal([]byte(line), &item); err != nil {
			return nil, fmt.Errorf("decode prediction line %d: %w", lineNumber, err)
		}
		if strings.TrimSpace(item.ID) == "" {
			return nil, fmt.Errorf("prediction line %d has empty id", lineNumber)
		}
		items = append(items, item)
	}
	if err := scanner.Err(); err != nil {
		return nil, fmt.Errorf("read predictions: %w", err)
	}
	return items, nil
}

func ReadLabelAssignmentsJSONL(reader io.Reader) ([]LabelAssignment, error) {
	scanner := bufio.NewScanner(reader)
	scanner.Buffer(make([]byte, 64*1024), 1024*1024)
	items := make([]LabelAssignment, 0)
	seen := map[string]struct{}{}
	lineNumber := 0
	for scanner.Scan() {
		lineNumber++
		line := strings.TrimSpace(scanner.Text())
		if line == "" {
			continue
		}
		var item LabelAssignment
		if err := json.Unmarshal([]byte(line), &item); err != nil {
			return nil, fmt.Errorf("decode label assignment line %d: %w", lineNumber, err)
		}
		if strings.TrimSpace(item.ID) == "" {
			return nil, fmt.Errorf("label assignment line %d has empty id", lineNumber)
		}
		if _, exists := seen[item.ID]; exists {
			return nil, fmt.Errorf("label assignment line %d duplicates id %q", lineNumber, item.ID)
		}
		seen[item.ID] = struct{}{}
		items = append(items, item)
	}
	if err := scanner.Err(); err != nil {
		return nil, fmt.Errorf("read label assignments: %w", err)
	}
	return items, nil
}

func ApplyReviewLabels(review []ReviewItem, assignments []LabelAssignment) ([]DatasetExample, error) {
	byID := make(map[string]ReviewItem, len(review))
	for _, item := range review {
		byID[item.ID] = item
	}
	result := make([]DatasetExample, 0, len(assignments))
	for _, assignment := range assignments {
		item, exists := byID[assignment.ID]
		if !exists {
			return nil, fmt.Errorf("label assignment references unknown review item %q", assignment.ID)
		}
		example := DatasetExample{
			ID: item.ID, Text: item.Text, Language: item.Language, Source: item.Source,
			SourceRef: item.SourceRef, Split: item.Split, Label: assignment.Label, Notes: assignment.Notes,
		}
		if err := example.Validate(); err != nil {
			return nil, fmt.Errorf("label assignment %q: %w", assignment.ID, err)
		}
		result = append(result, example)
	}
	SortDataset(result)
	return result, nil
}

func SuggestionBaselinePredictions(review []ReviewItem, assignments []LabelAssignment) ([]Prediction, error) {
	byID := make(map[string]ReviewItem, len(review))
	for _, item := range review {
		byID[item.ID] = item
	}
	predictions := make([]Prediction, 0, len(assignments))
	for _, assignment := range assignments {
		item, exists := byID[assignment.ID]
		if !exists {
			return nil, fmt.Errorf("label assignment references unknown review item %q", assignment.ID)
		}
		category := Category(strings.TrimSpace(item.SuggestedCategory))
		adjudication := Adjudication{
			SchemaVersion: SchemaVersion, Verdict: VerdictSafe, Category: CategoryNone,
			Intent: IntentNeutral, Actionability: ActionabilityNone, Authorization: AuthorizationUnknown,
			Confidence: 1, ReasonCode: "legacy_suggestion_safe",
		}
		if category != CategoryNone {
			if _, supported := domainCategories[category]; supported {
				adjudication.Verdict = VerdictConfirmed
				adjudication.Category = category
				adjudication.Intent = IntentOperational
				adjudication.Actionability = ActionabilityHigh
				adjudication.ReasonCode = "legacy_category_hit"
			}
		}
		predictions = append(predictions, Prediction{ID: item.ID, Adjudication: adjudication})
	}
	sort.SliceStable(predictions, func(i, j int) bool { return predictions[i].ID < predictions[j].ID })
	return predictions, nil
}

func WriteDatasetJSONL(writer io.Writer, items []DatasetExample) error {
	encoder := json.NewEncoder(writer)
	encoder.SetEscapeHTML(false)
	for _, item := range items {
		if err := encoder.Encode(item); err != nil {
			return fmt.Errorf("write risk dataset: %w", err)
		}
	}
	return nil
}

func WritePredictionsJSONL(writer io.Writer, items []Prediction) error {
	encoder := json.NewEncoder(writer)
	encoder.SetEscapeHTML(false)
	for _, item := range items {
		if err := encoder.Encode(item); err != nil {
			return fmt.Errorf("write risk predictions: %w", err)
		}
	}
	return nil
}

func between(value, startMarker, endMarker string) (string, bool) {
	start := strings.Index(value, startMarker)
	if start < 0 {
		return "", false
	}
	start += len(startMarker)
	end := strings.Index(value[start:], endMarker)
	if end < 0 {
		return "", false
	}
	return strings.TrimSpace(value[start : start+end]), true
}

func removeDelimitedBlocks(value, startMarker, endMarker string) string {
	for {
		start := strings.Index(value, startMarker)
		if start < 0 {
			return value
		}
		end := strings.Index(value[start:], endMarker)
		if end < 0 {
			return value
		}
		end += start + len(endMarker)
		value = value[:start] + value[end:]
	}
}

func limitRunes(value string, maximum int) (string, bool) {
	runes := []rune(value)
	if maximum <= 0 || len(runes) <= maximum {
		return value, false
	}
	return strings.TrimSpace(string(runes[:maximum])), true
}

func detectLanguage(value string) string {
	for _, r := range value {
		if r >= '\u4e00' && r <= '\u9fff' {
			return "zh"
		}
	}
	return "en"
}

func normalizeSuggestedCategory(value string) string {
	value = strings.TrimSpace(value)
	value = strings.TrimPrefix(value, "gateway_abuse/")
	return strings.TrimPrefix(value, "policy/")
}

func reliableObservedSource(source, text string) bool {
	lower := strings.ToLower(text)
	if strings.Contains(lower, "<current_user_request>") || strings.Contains(lower, "## my request for codex:") {
		return true
	}
	for _, marker := range []string{
		"history to compact:", "existing summary:", "openclaw runtime context",
		"you are codex,", "ou are codex,", "the following is the codex agent history",
		"generate 0 to 3 hyperpersonalized suggestions",
	} {
		if strings.Contains(lower, marker) {
			return false
		}
	}
	if strings.TrimSpace(source) != "prompt_audit" {
		return true
	}
	for _, marker := range []string{
		"# agents.md instructions", "<environment_context", "<in-app-browser-context",
		"another language model started to solve this problem",
	} {
		if strings.Contains(lower, marker) {
			return false
		}
	}
	return true
}

func ReadDatasetJSONL(reader io.Reader) ([]DatasetExample, error) {
	scanner := bufio.NewScanner(reader)
	scanner.Buffer(make([]byte, 64*1024), 2*1024*1024)
	items := make([]DatasetExample, 0)
	seenIDs := map[string]struct{}{}
	seenTexts := map[string]string{}
	lineNumber := 0
	for scanner.Scan() {
		lineNumber++
		line := strings.TrimSpace(scanner.Text())
		if line == "" {
			continue
		}
		var item DatasetExample
		if err := json.Unmarshal([]byte(line), &item); err != nil {
			return nil, fmt.Errorf("decode risk dataset line %d: %w", lineNumber, err)
		}
		if err := item.Validate(); err != nil {
			return nil, fmt.Errorf("validate risk dataset line %d: %w", lineNumber, err)
		}
		if _, exists := seenIDs[item.ID]; exists {
			return nil, fmt.Errorf("risk dataset line %d duplicates id %q", lineNumber, item.ID)
		}
		textHash := DatasetTextHash(item.Text)
		if existingSplit, exists := seenTexts[textHash]; exists && existingSplit != item.Split {
			return nil, fmt.Errorf("risk dataset line %d leaks duplicate text across %s and %s", lineNumber, existingSplit, item.Split)
		}
		seenIDs[item.ID] = struct{}{}
		seenTexts[textHash] = item.Split
		items = append(items, item)
	}
	if err := scanner.Err(); err != nil {
		return nil, fmt.Errorf("read risk dataset: %w", err)
	}
	return items, nil
}

func (e DatasetExample) Validate() error {
	if strings.TrimSpace(e.ID) == "" || strings.TrimSpace(e.Text) == "" {
		return fmt.Errorf("dataset id and text are required")
	}
	if e.Text != SanitizeTrainingText(e.Text) {
		return fmt.Errorf("dataset text contains unredacted secret or identifier")
	}
	if e.Split != "train" && e.Split != "validation" && e.Split != "test" {
		return fmt.Errorf("dataset split must be train, validation, or test")
	}
	if !validVerdict(e.Label.Verdict) || !validIntent(e.Label.Intent) ||
		!validActionability(e.Label.Actionability) || !validAuthorization(e.Label.Authorization) {
		return fmt.Errorf("dataset label contains an invalid enum")
	}
	if _, ok := domainCategories[e.Label.Category]; !ok {
		return fmt.Errorf("dataset label contains invalid category %q", e.Label.Category)
	}
	if (e.Label.Verdict == VerdictSafe || e.Label.Verdict == VerdictAbstain) && e.Label.Category != CategoryNone {
		return fmt.Errorf("safe or abstain dataset label must use category none")
	}
	if (e.Label.Verdict == VerdictReview || e.Label.Verdict == VerdictConfirmed) && e.Label.Category == CategoryNone {
		return fmt.Errorf("review or confirmed dataset label requires a category")
	}
	for _, evidence := range e.Label.Evidence {
		if strings.TrimSpace(evidence.Quote) == "" || strings.TrimSpace(evidence.Signal) == "" {
			return fmt.Errorf("dataset evidence requires quote and signal")
		}
		if !strings.Contains(e.Text, strings.TrimSpace(evidence.Quote)) {
			return fmt.Errorf("dataset evidence quote is not present in text")
		}
	}
	return nil
}

func DatasetTextHash(value string) string {
	normalized := strings.ToLower(strings.Join(strings.Fields(value), " "))
	digest := sha256.Sum256([]byte(normalized))
	return hex.EncodeToString(digest[:])
}

func DeterministicSplit(text string) string {
	digest := sha256.Sum256([]byte(strings.ToLower(strings.Join(strings.Fields(text), " "))))
	value := int(digest[0]) * 100 / 256
	switch {
	case value < 80:
		return "train"
	case value < 90:
		return "validation"
	default:
		return "test"
	}
}

func SortDataset(items []DatasetExample) {
	sort.SliceStable(items, func(i, j int) bool { return items[i].ID < items[j].ID })
}
