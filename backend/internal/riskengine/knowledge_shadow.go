package riskengine

import (
	"context"
	"errors"
	"strings"
	"time"
)

type KnowledgeShadowService struct {
	knowledge    *KnowledgeRepository
	observations *ShadowRepository
	policy       Policy
}

func NewKnowledgeShadowService(knowledge *KnowledgeRepository, observations *ShadowRepository) *KnowledgeShadowService {
	return &KnowledgeShadowService{knowledge: knowledge, observations: observations, policy: DefaultShadowPolicy()}
}

func (s *KnowledgeShadowService) Observations() *ShadowRepository {
	if s == nil {
		return nil
	}
	return s.observations
}

// Observe records a knowledge-assisted recommendation in the shadow-only V2
// tables. No-match traffic is skipped; safe counterexamples are retained when
// they compete with a risk example so false positives remain measurable.
func (s *KnowledgeShadowService) Observe(ctx context.Context, input Input, guard GuardSignal) (ShadowPersistResult, bool, error) {
	if s == nil || s.knowledge == nil || s.observations == nil {
		return ShadowPersistResult{}, false, errors.New("risk knowledge shadow service unavailable")
	}
	input.Text = strings.TrimSpace(input.Text)
	if input.Text == "" {
		return ShadowPersistResult{}, false, nil
	}
	snapshot, err := s.knowledge.Snapshot(ctx)
	if err != nil {
		return ShadowPersistResult{}, false, err
	}
	matches := RetrieveKnowledge(input.Text, snapshot, DefaultKnowledgeMatchLimit)
	if len(matches) == 0 {
		return ShadowPersistResult{}, false, nil
	}
	candidate, adjudication := KnowledgeAdjudication(input.Text, snapshot.Version, matches, guard)
	if err := adjudication.Validate(input.Text); err != nil {
		return ShadowPersistResult{}, false, err
	}
	outcome := s.policy.Evaluate(adjudication)
	event := Event{
		SchemaVersion: SchemaVersion, RequestID: input.RequestID, UserID: input.UserID, GroupID: input.GroupID,
		ConversationIDHash: hashOpaque(input.ConversationID), InputHash: hashOpaque(strings.Join(strings.Fields(input.Text), " ")),
		IncidentFingerprint: IncidentFingerprint(input.UserID, input.GroupID, adjudication.Category, input.Text),
		Candidate:           candidate, Adjudication: adjudication, Outcome: outcome, ObservedAt: input.ReceivedAt, Mode: "shadow",
	}
	if event.ObservedAt.IsZero() {
		event.ObservedAt = time.Now().UTC()
	}
	result, err := s.observations.Persist(ctx, event)
	return result, true, err
}
