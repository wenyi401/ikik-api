package service

import (
	"sync"
	"time"
)

const (
	claudeWebConversationTTL        = 30 * time.Minute
	claudeWebConversationMaxEntries = 10_000
)

type claudeWebConversationState struct {
	mu                sync.Mutex
	conversationID    string
	lastHumanUUID     string
	lastAssistantUUID string
	updatedAt         time.Time
}

type claudeWebConversationStore struct {
	mu      sync.Mutex
	entries map[string]*claudeWebConversationState
	ttl     time.Duration
	max     int
}

func newClaudeWebConversationStore(ttl time.Duration, maxEntries int) *claudeWebConversationStore {
	if ttl <= 0 {
		ttl = claudeWebConversationTTL
	}
	if maxEntries <= 0 {
		maxEntries = claudeWebConversationMaxEntries
	}
	return &claudeWebConversationStore{
		entries: make(map[string]*claudeWebConversationState),
		ttl:     ttl,
		max:     maxEntries,
	}
}

func (s *claudeWebConversationStore) acquire(key string) *claudeWebConversationState {
	if s == nil || key == "" {
		return nil
	}
	now := time.Now()
	s.mu.Lock()
	s.cleanupLocked(now)
	state := s.entries[key]
	if state == nil {
		if len(s.entries) >= s.max {
			s.evictOldestLocked()
		}
		state = &claudeWebConversationState{updatedAt: now}
		s.entries[key] = state
	}
	s.mu.Unlock()
	state.mu.Lock()
	state.updatedAt = now
	return state
}

func (s *claudeWebConversationStore) release(state *claudeWebConversationState) {
	if state == nil {
		return
	}
	state.updatedAt = time.Now()
	state.mu.Unlock()
}

func (s *claudeWebConversationStore) invalidate(key string, state *claudeWebConversationState) {
	if s == nil || key == "" || state == nil {
		return
	}
	s.mu.Lock()
	if current := s.entries[key]; current == state {
		delete(s.entries, key)
	}
	s.mu.Unlock()
}

func (s *claudeWebConversationStore) cleanupLocked(now time.Time) {
	cutoff := now.Add(-s.ttl)
	for key, state := range s.entries {
		if !state.mu.TryLock() {
			continue
		}
		if state.updatedAt.Before(cutoff) {
			delete(s.entries, key)
		}
		state.mu.Unlock()
	}
}

func (s *claudeWebConversationStore) evictOldestLocked() {
	var oldestKey string
	var oldestState *claudeWebConversationState
	var oldestTime time.Time
	for key, state := range s.entries {
		if !state.mu.TryLock() {
			continue
		}
		updatedAt := state.updatedAt
		state.mu.Unlock()
		if oldestState == nil || updatedAt.Before(oldestTime) {
			oldestKey = key
			oldestState = state
			oldestTime = updatedAt
		}
	}
	if oldestState == nil || !oldestState.mu.TryLock() {
		return
	}
	delete(s.entries, oldestKey)
	oldestState.mu.Unlock()
}

var defaultClaudeWebConversationStore = newClaudeWebConversationStore(
	claudeWebConversationTTL,
	claudeWebConversationMaxEntries,
)
