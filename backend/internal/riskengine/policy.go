package riskengine

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"strconv"
	"strings"
)

type Recommendation string

const (
	RecommendationAllow          Recommendation = "allow"
	RecommendationObserve        Recommendation = "observe"
	RecommendationManualReview   Recommendation = "manual_review"
	RecommendationProtectRequest Recommendation = "protect_request"
	RecommendationStrike         Recommendation = "strike"
)

type CategoryPolicy struct {
	Enabled                 bool
	ProtectConfidence       float64
	StrikeConfidence        float64
	MinimumActionability    Actionability
	RequireUnauthorized     bool
	AllowOperationalUnknown bool
}

type Policy struct {
	Version    int
	Categories map[Category]CategoryPolicy
}

type Outcome struct {
	Recommendation Recommendation `json:"recommendation"`
	WouldProtect   bool           `json:"would_protect"`
	WouldStrike    bool           `json:"would_strike"`
	ReasonCode     string         `json:"reason_code"`
	PolicyVersion  int            `json:"policy_version"`
}

func DefaultShadowPolicy() Policy {
	strict := CategoryPolicy{
		Enabled: true, ProtectConfidence: 0.95, StrikeConfidence: 0.99,
		MinimumActionability: ActionabilityHigh, RequireUnauthorized: true,
	}
	cheat := strict
	cheat.RequireUnauthorized = false
	return Policy{Version: 1, Categories: map[Category]CategoryPolicy{
		CategoryCheatAutomation:           cheat,
		CategoryAuthReverseEngineering:    strict,
		CategoryExploitReverseEngineering: strict,
		CategoryCredentialTheft:           strict,
		CategorySafetyBypass:              strict,
		CategoryAccountAutomation:         strict,
		CategoryCyberAbuse:                strict,
	}}
}

// Evaluate returns a recommendation only. It deliberately has no repository,
// account, API key, or group mutation capability.
func (p Policy) Evaluate(adjudication Adjudication) Outcome {
	outcome := Outcome{Recommendation: RecommendationObserve, PolicyVersion: p.Version}
	if adjudication.Verdict == VerdictSafe {
		outcome.Recommendation = RecommendationAllow
		outcome.ReasonCode = "model_safe"
		return outcome
	}
	if adjudication.Verdict == VerdictAbstain {
		outcome.Recommendation = RecommendationManualReview
		outcome.ReasonCode = "model_abstained"
		return outcome
	}
	categoryPolicy, enabled := p.Categories[adjudication.Category]
	if !enabled || !categoryPolicy.Enabled {
		outcome.ReasonCode = "category_disabled"
		return outcome
	}
	if adjudication.Verdict != VerdictConfirmed {
		outcome.Recommendation = RecommendationManualReview
		outcome.ReasonCode = "model_requested_review"
		return outcome
	}
	if adjudication.Intent != IntentOperational && adjudication.Intent != IntentEvasion &&
		!(categoryPolicy.AllowOperationalUnknown && adjudication.Intent == IntentUnknown) {
		outcome.Recommendation = RecommendationManualReview
		outcome.ReasonCode = "intent_not_operational"
		return outcome
	}
	if actionabilityRank(adjudication.Actionability) < actionabilityRank(categoryPolicy.MinimumActionability) {
		outcome.Recommendation = RecommendationManualReview
		outcome.ReasonCode = "insufficient_actionability"
		return outcome
	}
	if categoryPolicy.RequireUnauthorized && adjudication.Authorization != AuthorizationUnauthorized {
		outcome.Recommendation = RecommendationManualReview
		outcome.ReasonCode = "authorization_not_established"
		return outcome
	}
	if adjudication.Confidence < categoryPolicy.ProtectConfidence {
		outcome.Recommendation = RecommendationManualReview
		outcome.ReasonCode = "below_protection_confidence"
		return outcome
	}
	outcome.WouldProtect = true
	outcome.Recommendation = RecommendationProtectRequest
	outcome.ReasonCode = "request_protection_candidate"
	if adjudication.Confidence >= categoryPolicy.StrikeConfidence {
		outcome.WouldStrike = true
		outcome.Recommendation = RecommendationStrike
		outcome.ReasonCode = "category_strike_candidate"
	}
	return outcome
}

func IncidentFingerprint(userID, groupID int64, category Category, inputText string) string {
	normalized := strings.ToLower(strings.Join(strings.Fields(inputText), " "))
	material := strconv.FormatInt(userID, 10) + "\x00" + strconv.FormatInt(groupID, 10) + "\x00" + string(category) + "\x00" + normalized
	digest := sha256.Sum256([]byte(material))
	return hex.EncodeToString(digest[:])
}

func (p Policy) Validate() error {
	if p.Version <= 0 {
		return fmt.Errorf("risk policy version must be positive")
	}
	for category, rule := range p.Categories {
		if category == CategoryNone {
			return fmt.Errorf("risk policy cannot configure category none")
		}
		if _, ok := domainCategories[category]; !ok {
			return fmt.Errorf("risk policy contains unknown category %q", category)
		}
		if rule.ProtectConfidence < 0 || rule.ProtectConfidence > 1 || rule.StrikeConfidence < 0 || rule.StrikeConfidence > 1 {
			return fmt.Errorf("risk policy confidence for %s must be between 0 and 1", category)
		}
		if rule.StrikeConfidence < rule.ProtectConfidence {
			return fmt.Errorf("risk policy strike confidence for %s must not be lower than protection confidence", category)
		}
		if !validActionability(rule.MinimumActionability) {
			return fmt.Errorf("risk policy actionability for %s is invalid", category)
		}
	}
	return nil
}

func actionabilityRank(value Actionability) int {
	switch value {
	case ActionabilityHigh:
		return 3
	case ActionabilityMedium:
		return 2
	case ActionabilityLow:
		return 1
	default:
		return 0
	}
}
