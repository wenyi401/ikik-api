package service

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"strings"
	"unicode"

	openapi "github.com/alibabacloud-go/darabonba-openapi/v2/client"
	green20220302 "github.com/alibabacloud-go/green-20220302/v3/client"
	util "github.com/alibabacloud-go/tea-utils/v2/service"
	"github.com/alibabacloud-go/tea/tea"
)

const (
	maxAliyunGuardrailInputRunes = 2000

	aliyunCategoryBlock             = "aliyun.block"
	aliyunCategoryHigh              = "aliyun.high"
	aliyunCategoryContentModeration = "aliyun.contentModeration"
	aliyunCategorySensitive         = "aliyun.sensitive"
	aliyunCategoryAttack            = "aliyun.attack"
)

func (s *ContentModerationService) callAliyunGuardrailOnce(ctx context.Context, cfg *ContentModerationConfig, apiKey string, input any, httpStatus *int) (*moderationAPIResult, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	accessKeyID, accessKeySecret, err := parseAliyunAccessKey(apiKey)
	if err != nil {
		return nil, err
	}
	text := aliyunModerationInputText(input)
	if strings.TrimSpace(text) == "" {
		text = "hello"
	}
	text = trimRunes(normalizeContentModerationText(text), maxAliyunGuardrailInputRunes)
	serviceParameters, err := json.Marshal(map[string]any{
		"content": text,
	})
	if err != nil {
		return nil, fmt.Errorf("marshal aliyun guardrail service parameters: %w", err)
	}

	timeout := cfg.TimeoutMS
	if timeout <= 0 {
		timeout = defaultContentModerationTimeoutMS
	}
	config := &openapi.Config{
		AccessKeyId:     tea.String(accessKeyID),
		AccessKeySecret: tea.String(accessKeySecret),
		RegionId:        tea.String(cfg.AliyunRegionID),
		Endpoint:        tea.String(cfg.AliyunEndpoint),
		ConnectTimeout:  tea.Int(timeout),
		ReadTimeout:     tea.Int(timeout),
	}
	client, err := green20220302.NewClient(config)
	if err != nil {
		return nil, fmt.Errorf("create aliyun guardrail client: %w", err)
	}
	runtime := &util.RuntimeOptions{
		ConnectTimeout: tea.Int(timeout),
		ReadTimeout:    tea.Int(timeout),
	}
	request := &green20220302.TextModerationPlusRequest{
		Service:           tea.String(cfg.AliyunService),
		ServiceParameters: tea.String(string(serviceParameters)),
	}
	result, err := client.TextModerationPlusWithOptions(request, runtime)
	if err != nil {
		return nil, fmt.Errorf("aliyun guardrail request failed: %w", err)
	}
	if result == nil {
		return nil, errors.New("aliyun guardrail returned empty response")
	}
	statusCode := http.StatusOK
	if result.StatusCode != nil {
		statusCode = int(*result.StatusCode)
	}
	if httpStatus != nil {
		*httpStatus = statusCode
	}
	if statusCode < 200 || statusCode >= 300 {
		return nil, fmt.Errorf("aliyun guardrail status %d", statusCode)
	}
	if result.Body == nil {
		return nil, errors.New("aliyun guardrail returned empty body")
	}
	code := int32(0)
	if result.Body.Code != nil {
		code = *result.Body.Code
	}
	if code != http.StatusOK {
		return nil, fmt.Errorf("aliyun guardrail code %d: %s", code, strings.TrimSpace(aliyunStringValue(result.Body.Message)))
	}
	if result.Body.Data == nil {
		return nil, errors.New("aliyun guardrail returned empty data")
	}
	return aliyunGuardrailResultToModerationResult(result.Body.Data), nil
}

func parseAliyunAccessKey(raw string) (string, string, error) {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return "", "", errors.New("aliyun guardrail access key is empty")
	}
	parts := strings.SplitN(raw, ":", 2)
	if len(parts) != 2 {
		parts = strings.Fields(raw)
	}
	if len(parts) != 2 {
		return "", "", errors.New("aliyun guardrail key must be AccessKeyId:AccessKeySecret")
	}
	accessKeyID := strings.TrimSpace(parts[0])
	accessKeySecret := strings.TrimSpace(parts[1])
	if accessKeyID == "" || accessKeySecret == "" {
		return "", "", errors.New("aliyun guardrail key must include AccessKeyId and AccessKeySecret")
	}
	return accessKeyID, accessKeySecret, nil
}

func aliyunModerationInputText(input any) string {
	switch value := input.(type) {
	case string:
		return value
	case []moderationAPIInputPart:
		parts := make([]string, 0, len(value))
		for _, part := range value {
			if strings.TrimSpace(part.Text) != "" {
				parts = append(parts, part.Text)
			}
		}
		return strings.Join(parts, "\n")
	case []any:
		parts := make([]string, 0, len(value))
		for _, item := range value {
			if part, ok := item.(moderationAPIInputPart); ok && strings.TrimSpace(part.Text) != "" {
				parts = append(parts, part.Text)
			}
		}
		return strings.Join(parts, "\n")
	default:
		raw, _ := json.Marshal(value)
		return string(raw)
	}
}

func aliyunGuardrailResultToModerationResult(data *green20220302.TextModerationPlusResponseBodyData) *moderationAPIResult {
	scores := map[string]float64{}
	if data == nil {
		return &moderationAPIResult{CategoryScores: scores}
	}
	riskScore := aliyunRiskLevelScore(aliyunStringValue(data.RiskLevel))
	if riskScore > 0 {
		addModerationScore(scores, aliyunCategoryContentModeration, riskScore)
		if riskScore >= 1 {
			addModerationScore(scores, aliyunCategoryHigh, 1)
			addModerationScore(scores, aliyunCategoryBlock, 1)
		}
	}
	if data.Score != nil {
		addModerationScore(scores, aliyunCategoryContentModeration, normalizeAliyunConfidence(float64(*data.Score)))
	}
	for _, item := range data.Result {
		if item == nil {
			continue
		}
		label := normalizeAliyunGuardrailLabel(aliyunStringValue(item.Label))
		if label == "" || label == "nonlabel" {
			continue
		}
		score := normalizeAliyunConfidencePtr(item.Confidence)
		if score <= 0 {
			score = riskScore
		}
		if score <= 0 {
			score = 1
		}
		addModerationScore(scores, aliyunCategoryContentModeration+"."+label, score)
		if mapped := aliyunContentLabelToModerationCategory(label); mapped != "" {
			addModerationScore(scores, mapped, score)
		}
	}
	sensitiveScore := aliyunSensitiveLevelScore(aliyunStringValue(data.SensitiveLevel))
	if sensitiveScore > 0 {
		addModerationScore(scores, aliyunCategorySensitive, sensitiveScore)
		if sensitiveScore >= 1 {
			addModerationScore(scores, aliyunCategoryHigh, 1)
		}
	}
	for _, item := range data.SensitiveResult {
		if item == nil {
			continue
		}
		label := normalizeAliyunGuardrailLabel(aliyunStringValue(item.Label))
		if label == "" || label == "nonlabel" {
			continue
		}
		score := aliyunSensitiveLevelScore(aliyunStringValue(item.SensitiveLevel))
		if score <= 0 {
			score = sensitiveScore
		}
		if score <= 0 {
			score = 1
		}
		addModerationScore(scores, aliyunCategorySensitive+"."+label, score)
	}
	attackScore := aliyunRiskLevelScore(aliyunStringValue(data.AttackLevel))
	if attackScore > 0 {
		addModerationScore(scores, aliyunCategoryAttack, attackScore)
		if attackScore >= 1 {
			addModerationScore(scores, aliyunCategoryHigh, 1)
		}
	}
	for _, item := range data.AttackResult {
		if item == nil {
			continue
		}
		label := normalizeAliyunGuardrailLabel(aliyunStringValue(item.Label))
		if label == "" || label == "safe" || label == "nonlabel" {
			continue
		}
		score := normalizeAliyunConfidencePtr(item.Confidence)
		if score <= 0 {
			score = aliyunRiskLevelScore(aliyunStringValue(item.AttackLevel))
		}
		if score <= 0 {
			score = attackScore
		}
		if score <= 0 {
			score = 1
		}
		addModerationScore(scores, aliyunCategoryAttack+"."+label, score)
		if mapped := aliyunAttackLabelToModerationCategory(label, aliyunStringValue(item.AttackLevel)); mapped != "" {
			addModerationScore(scores, mapped, score)
		}
	}
	return &moderationAPIResult{CategoryScores: scores}
}

func addModerationScore(scores map[string]float64, category string, score float64) {
	category = strings.TrimSpace(category)
	if category == "" {
		return
	}
	score = clampModerationScore(score)
	if score > scores[category] {
		scores[category] = score
	}
}

func normalizeAliyunConfidencePtr(value *float32) float64 {
	if value == nil {
		return 0
	}
	return normalizeAliyunConfidence(float64(*value))
}

func normalizeAliyunConfidence(value float64) float64 {
	if value < 0 {
		return 0
	}
	if value > 1 {
		value = value / 100
	}
	return clampModerationScore(value)
}

func clampModerationScore(value float64) float64 {
	if value < 0 {
		return 0
	}
	if value > 1 {
		return 1
	}
	return value
}

func aliyunRiskLevelScore(level string) float64 {
	switch strings.ToLower(strings.TrimSpace(level)) {
	case "high":
		return 1
	case "medium":
		return 0.75
	case "low":
		return 0.35
	default:
		return 0
	}
}

func aliyunSensitiveLevelScore(level string) float64 {
	switch strings.ToUpper(strings.TrimSpace(level)) {
	case "S4":
		return 1
	case "S3":
		return 0.85
	case "S2":
		return 0.6
	case "S1":
		return 0.35
	default:
		return 0
	}
}

func aliyunContentLabelToModerationCategory(label string) string {
	label = normalizeAliyunGuardrailLabel(label)
	switch {
	case strings.Contains(label, "minor"):
		return "sexual/minors"
	case strings.Contains(label, "porn") || strings.Contains(label, "sexual") || strings.Contains(label, "sexuality"):
		return "sexual"
	case strings.Contains(label, "self_harm") || strings.Contains(label, "suicide"):
		return "self-harm"
	case strings.Contains(label, "violent") || strings.Contains(label, "violence") || strings.Contains(label, "weapon"):
		return "violence"
	case strings.Contains(label, "drug") || strings.Contains(label, "gambling") || strings.Contains(label, "contraband") || strings.Contains(label, "illegal"):
		return "illicit"
	case strings.Contains(label, "discrimination") || strings.Contains(label, "hate"):
		return "hate"
	case strings.Contains(label, "profanity") || strings.Contains(label, "abuse") || strings.Contains(label, "insult") || strings.Contains(label, "oral"):
		return "harassment"
	case strings.Contains(label, "customized"):
		return "illicit"
	default:
		return ""
	}
}

func aliyunAttackLabelToModerationCategory(label string, level string) string {
	label = normalizeAliyunGuardrailLabel(label)
	if strings.Contains(label, "prompt_injection") || strings.Contains(label, "jailbreak") || strings.Contains(label, "attack") || strings.EqualFold(strings.TrimSpace(level), "high") {
		return "illicit"
	}
	return ""
}

func normalizeAliyunGuardrailLabel(label string) string {
	label = strings.TrimSpace(label)
	if label == "" {
		return ""
	}
	var builder strings.Builder
	builder.Grow(len(label))
	lastUnderscore := false
	for _, r := range strings.ToLower(label) {
		if unicode.IsLetter(r) || unicode.IsDigit(r) {
			_, _ = builder.WriteRune(r)
			lastUnderscore = false
			continue
		}
		if !lastUnderscore {
			_ = builder.WriteByte('_')
			lastUnderscore = true
		}
	}
	return strings.Trim(builder.String(), "_")
}

func aliyunStringValue(value *string) string {
	if value == nil {
		return ""
	}
	return *value
}
