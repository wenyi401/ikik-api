package riskengine

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"strings"
)

const SchemaVersion = 1

type Category string

const (
	CategoryNone                      Category = "none"
	CategoryCheatAutomation           Category = "cheat_automation"
	CategoryAuthReverseEngineering    Category = "auth_reverse_engineering"
	CategoryExploitReverseEngineering Category = "exploit_reverse_engineering"
	CategoryCredentialTheft           Category = "credential_theft"
	CategorySafetyBypass              Category = "safety_bypass"
	CategoryAccountAutomation         Category = "account_automation"
	CategoryCyberAbuse                Category = "cyber_abuse"
)

var domainCategories = map[Category]struct{}{
	CategoryNone: {}, CategoryCheatAutomation: {}, CategoryAuthReverseEngineering: {},
	CategoryExploitReverseEngineering: {}, CategoryCredentialTheft: {}, CategorySafetyBypass: {},
	CategoryAccountAutomation: {}, CategoryCyberAbuse: {},
}

type Verdict string

const (
	VerdictSafe      Verdict = "safe"
	VerdictReview    Verdict = "review"
	VerdictConfirmed Verdict = "confirmed"
	VerdictAbstain   Verdict = "abstain"
)

type Intent string

const (
	IntentNeutral     Intent = "neutral"
	IntentEducational Intent = "educational"
	IntentDefensive   Intent = "defensive"
	IntentOperational Intent = "operational"
	IntentEvasion     Intent = "evasion"
	IntentUnknown     Intent = "unknown"
)

type Actionability string

const (
	ActionabilityNone   Actionability = "none"
	ActionabilityLow    Actionability = "low"
	ActionabilityMedium Actionability = "medium"
	ActionabilityHigh   Actionability = "high"
)

type Authorization string

const (
	AuthorizationAuthorized   Authorization = "authorized"
	AuthorizationUnauthorized Authorization = "unauthorized"
	AuthorizationUnknown      Authorization = "unknown"
)

type Evidence struct {
	Quote  string `json:"quote"`
	Signal string `json:"signal"`
}

type Adjudication struct {
	SchemaVersion int           `json:"schema_version"`
	Verdict       Verdict       `json:"verdict"`
	Category      Category      `json:"category"`
	Intent        Intent        `json:"intent"`
	Actionability Actionability `json:"actionability"`
	Authorization Authorization `json:"authorization"`
	Confidence    float64       `json:"confidence"`
	Evidence      []Evidence    `json:"evidence"`
	ReasonCode    string        `json:"reason_code"`
	Model         string        `json:"model,omitempty"`
}

func ParseAdjudication(inputText string, raw []byte) (Adjudication, error) {
	var result Adjudication
	decoder := json.NewDecoder(strings.NewReader(stripJSONFence(string(raw))))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&result); err != nil {
		return result, fmt.Errorf("decode risk adjudication: %w", err)
	}
	var trailing any
	if err := decoder.Decode(&trailing); !errors.Is(err, io.EOF) {
		if err == nil {
			return result, errors.New("decode risk adjudication: multiple JSON values")
		}
		return result, fmt.Errorf("decode risk adjudication trailing data: %w", err)
	}
	if err := result.Validate(inputText); err != nil {
		return result, err
	}
	return result, nil
}

func (a Adjudication) Validate(inputText string) error {
	if a.SchemaVersion != SchemaVersion {
		return fmt.Errorf("risk adjudication schema_version must be %d", SchemaVersion)
	}
	if !validVerdict(a.Verdict) {
		return fmt.Errorf("invalid risk adjudication verdict %q", a.Verdict)
	}
	if _, ok := domainCategories[a.Category]; !ok {
		return fmt.Errorf("invalid risk adjudication category %q", a.Category)
	}
	if !validIntent(a.Intent) {
		return fmt.Errorf("invalid risk adjudication intent %q", a.Intent)
	}
	if !validActionability(a.Actionability) {
		return fmt.Errorf("invalid risk adjudication actionability %q", a.Actionability)
	}
	if !validAuthorization(a.Authorization) {
		return fmt.Errorf("invalid risk adjudication authorization %q", a.Authorization)
	}
	if a.Confidence < 0 || a.Confidence > 1 {
		return errors.New("risk adjudication confidence must be between 0 and 1")
	}
	if !validStableCode(a.ReasonCode, 64) {
		return errors.New("risk adjudication reason_code must be a short stable code")
	}
	if a.Verdict == VerdictSafe || a.Verdict == VerdictAbstain {
		if a.Category != CategoryNone {
			return errors.New("safe or abstain adjudication must use category none")
		}
	} else if a.Category == CategoryNone {
		return errors.New("review or confirmed adjudication requires a domain category")
	}
	if a.Verdict == VerdictConfirmed && len(a.Evidence) == 0 {
		return errors.New("confirmed adjudication requires quoted evidence")
	}
	for _, evidence := range a.Evidence {
		quote := strings.TrimSpace(evidence.Quote)
		if quote == "" || !validEvidenceSignal(evidence.Signal) {
			return errors.New("risk adjudication evidence requires quote and signal")
		}
		if !strings.Contains(inputText, quote) {
			return errors.New("risk adjudication evidence quote is not present in input")
		}
	}
	return nil
}

func validStableCode(value string, maximum int) bool {
	value = strings.TrimSpace(value)
	if value == "" || len(value) > maximum {
		return false
	}
	for _, r := range value {
		if (r < 'a' || r > 'z') && (r < '0' || r > '9') && r != '_' {
			return false
		}
	}
	return true
}

func validEvidenceSignal(value string) bool {
	switch strings.TrimSpace(value) {
	case "requested_action", "target", "evasion", "authorization", "operational_detail":
		return true
	default:
		return false
	}
}

func stripJSONFence(value string) string {
	value = strings.TrimSpace(value)
	value = strings.TrimPrefix(value, "```json")
	value = strings.TrimPrefix(value, "```")
	value = strings.TrimSuffix(value, "```")
	return strings.TrimSpace(value)
}

func validVerdict(value Verdict) bool {
	switch value {
	case VerdictSafe, VerdictReview, VerdictConfirmed, VerdictAbstain:
		return true
	default:
		return false
	}
}

func validIntent(value Intent) bool {
	switch value {
	case IntentNeutral, IntentEducational, IntentDefensive, IntentOperational, IntentEvasion, IntentUnknown:
		return true
	default:
		return false
	}
}

func validActionability(value Actionability) bool {
	switch value {
	case ActionabilityNone, ActionabilityLow, ActionabilityMedium, ActionabilityHigh:
		return true
	default:
		return false
	}
}

func validAuthorization(value Authorization) bool {
	switch value {
	case AuthorizationAuthorized, AuthorizationUnauthorized, AuthorizationUnknown:
		return true
	default:
		return false
	}
}
