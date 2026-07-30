package securityaudit

import (
	"crypto/sha256"
	"encoding/hex"
	"regexp"
	"strings"
	"unicode"
)

// LocalPromptSignal is deliberately structural. A single word such as
// "jailbreak" or "research" is not enough to block a request.
type LocalPromptSignal struct {
	Name   string
	Weight int
}

type LocalPromptAssessment struct {
	NormalizedHash       string
	Score                int
	Signals              []LocalPromptSignal
	RequiresRemoteReview bool
}

var localPromptPatterns = []struct {
	name    string
	weight  int
	pattern *regexp.Regexp
}{
	{name: "instruction_override", weight: 35, pattern: regexp.MustCompile(`(?i)(ignore|override|disregard|replace|bypass).{0,80}(previous|system|developer|safety|policy|instruction|rule)`)},
	{name: "instruction_override_zh", weight: 35, pattern: regexp.MustCompile(`(忽略|无视|覆盖|绕过|替换).{0,40}(系统|开发者|安全|策略|政策|规则|指令|限制)`)},
	{name: "refusal_suppression", weight: 30, pattern: regexp.MustCompile(`(?i)(do not|don't|never|stop|suppress|remove).{0,60}(refuse|拒绝|safety|policy|fallback)`)},
	{name: "refusal_suppression_zh", weight: 30, pattern: regexp.MustCompile(`(不得|不要|禁止|永不).{0,40}(拒绝|提及|遵守).{0,40}(安全|政策|规则|限制|指令)`)},
	{name: "unrestricted_role", weight: 30, pattern: regexp.MustCompile(`(?i)(unrestricted|uncensored|developer mode|debug mode|sandbox mode|no safety|no restrictions)`)},
	{name: "unrestricted_role_zh", weight: 30, pattern: regexp.MustCompile(`(无限制|无审查|开发者模式|调试模式|不受限制|解除限制|无安全限制)`)},
	{name: "persistent_instruction", weight: 25, pattern: regexp.MustCompile(`(?i)(remember|persist|always follow|from now on|以后始终|持续遵守|remember this rule)`)},
	{name: "prompt_extraction", weight: 20, pattern: regexp.MustCompile(`(?i)(reveal|print|show|泄露|输出).{0,60}(system prompt|developer message|hidden instruction|系统提示词|开发者指令)`)},
	{name: "jailbreak_template", weight: 35, pattern: regexp.MustCompile(`(?i)(jailbreak|do anything now|\bdan\b).{0,80}(prompt|instruction|mode|template|persona|rule)|(prompt|instruction|mode|template|persona|rule).{0,80}(jailbreak|do anything now|\bdan\b)`)},
	{name: "policy_concealment", weight: 25, pattern: regexp.MustCompile(`(?i)(do not|don't|never).{0,60}(mention|reveal|admit|acknowledge).{0,60}(policy|safety|restriction|instruction)`)},
	{name: "unconditional_compliance", weight: 25, pattern: regexp.MustCompile(`(?i)(answer|respond|comply|obey).{0,60}(anything|every request|without refusal|without restriction|regardless of policy)`)},
}

// AssessLocalPrompt is a zero-token fast path. Uncertain requests continue to
// the configured model-backed audit pipeline.
func AssessLocalPrompt(value string) LocalPromptAssessment {
	fingerprintText := normalizePromptFingerprint(value)
	digest := sha256.Sum256([]byte(fingerprintText))
	assessment := LocalPromptAssessment{NormalizedHash: hex.EncodeToString(digest[:])}
	signalText := normalizePromptSignalText(fingerprintText)
	for _, candidate := range localPromptPatterns {
		if candidate.pattern.MatchString(signalText) {
			assessment.Signals = append(assessment.Signals, LocalPromptSignal{Name: candidate.name, Weight: candidate.weight})
			assessment.Score += candidate.weight
		}
	}
	assessment.RequiresRemoteReview = len(assessment.Signals) > 0
	return assessment
}

// normalizePromptFingerprint only removes presentation-level variation. It
// preserves punctuation and symbols so semantically different prompts cannot
// collapse onto the same auto-blocking fingerprint.
func normalizePromptFingerprint(value string) string {
	var builder strings.Builder
	for _, r := range strings.TrimSpace(value) {
		if unicode.Is(unicode.Cf, r) {
			continue
		}
		if r == '\u3000' {
			r = ' '
		} else if r >= '\uff01' && r <= '\uff5e' {
			r -= 0xfee0
		}
		if unicode.IsSpace(r) {
			builder.WriteByte(' ')
			continue
		}
		builder.WriteRune(unicode.ToLower(r))
	}
	return strings.Join(strings.Fields(builder.String()), " ")
}

func normalizePromptSignalText(value string) string {
	var builder strings.Builder
	for _, r := range value {
		if unicode.IsSpace(r) || unicode.IsPunct(r) || unicode.IsSymbol(r) {
			builder.WriteByte(' ')
			continue
		}
		builder.WriteRune(r)
	}
	return strings.Join(strings.Fields(builder.String()), " ")
}
