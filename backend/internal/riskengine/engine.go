package riskengine

import (
	"context"
	"errors"
	"strings"
	"time"
)

type Input struct {
	RequestID      string
	UserID         int64
	GroupID        int64
	ConversationID string
	Text           string
	ReceivedAt     time.Time
}

type Candidate struct {
	Review           bool             `json:"review"`
	Signals          []string         `json:"signals,omitempty"`
	Confidence       float64          `json:"confidence"`
	KnowledgeVersion int              `json:"knowledge_version,omitempty"`
	KnowledgeMatches []KnowledgeMatch `json:"knowledge_matches,omitempty"`
}

type CandidateDetector interface {
	Detect(context.Context, Input) (Candidate, error)
}

type Adjudicator interface {
	Adjudicate(context.Context, Input, Candidate) (Adjudication, error)
}

type Event struct {
	SchemaVersion       int          `json:"schema_version"`
	RequestID           string       `json:"request_id"`
	UserID              int64        `json:"user_id"`
	GroupID             int64        `json:"group_id"`
	ConversationIDHash  string       `json:"conversation_id_hash,omitempty"`
	InputHash           string       `json:"input_hash"`
	IncidentFingerprint string       `json:"incident_fingerprint"`
	Candidate           Candidate    `json:"candidate"`
	Adjudication        Adjudication `json:"adjudication"`
	Outcome             Outcome      `json:"outcome"`
	ObservedAt          time.Time    `json:"observed_at"`
	Mode                string       `json:"mode"`
}

type Engine struct {
	detector    CandidateDetector
	adjudicator Adjudicator
	policy      Policy
	now         func() time.Time
}

func NewEngine(detector CandidateDetector, adjudicator Adjudicator, policy Policy) (*Engine, error) {
	if detector == nil || adjudicator == nil {
		return nil, errors.New("risk engine detector and adjudicator are required")
	}
	if err := policy.Validate(); err != nil {
		return nil, err
	}
	return &Engine{detector: detector, adjudicator: adjudicator, policy: policy, now: time.Now}, nil
}

// Evaluate only creates a shadow event. Enforcement is intentionally outside
// this package so experiments cannot mutate users, keys, groups, or strikes.
func (e *Engine) Evaluate(ctx context.Context, input Input) (Event, error) {
	input.Text = strings.TrimSpace(input.Text)
	if input.Text == "" {
		return Event{}, errors.New("risk engine input text is required")
	}
	candidate, err := e.detector.Detect(ctx, input)
	if err != nil {
		return Event{}, err
	}
	adjudication := Adjudication{
		SchemaVersion: SchemaVersion, Verdict: VerdictSafe, Category: CategoryNone,
		Intent: IntentNeutral, Actionability: ActionabilityNone, Authorization: AuthorizationUnknown,
		Confidence: 1, ReasonCode: "candidate_filter_clear",
	}
	if candidate.Review {
		adjudication, err = e.adjudicator.Adjudicate(ctx, input, candidate)
		if err != nil {
			return Event{}, err
		}
		if err := adjudication.Validate(input.Text); err != nil {
			return Event{}, err
		}
	}
	outcome := e.policy.Evaluate(adjudication)
	observedAt := input.ReceivedAt
	if observedAt.IsZero() {
		observedAt = e.now()
	}
	return Event{
		SchemaVersion: SchemaVersion, RequestID: input.RequestID, UserID: input.UserID, GroupID: input.GroupID,
		ConversationIDHash: hashOpaque(input.ConversationID), InputHash: hashOpaque(strings.Join(strings.Fields(input.Text), " ")),
		IncidentFingerprint: IncidentFingerprint(input.UserID, input.GroupID, adjudication.Category, input.Text),
		Candidate:           candidate, Adjudication: adjudication, Outcome: outcome, ObservedAt: observedAt.UTC(), Mode: "shadow",
	}, nil
}

// AllTrafficDetector is the safe initial shadow detector. A learned candidate
// detector may replace it only after recall is measured against labeled data.
type AllTrafficDetector struct{}

func (AllTrafficDetector) Detect(_ context.Context, _ Input) (Candidate, error) {
	return Candidate{Review: true, Signals: []string{"shadow_all_traffic"}, Confidence: 1}, nil
}

func hashOpaque(value string) string {
	if strings.TrimSpace(value) == "" {
		return ""
	}
	return IncidentFingerprint(0, 0, CategoryNone, value)
}
