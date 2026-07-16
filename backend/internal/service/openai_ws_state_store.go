package service

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"strings"
	"sync"
	"sync/atomic"
	"time"
)

const (
	openAIWSResponseAccountCachePrefix = "openai:response:"
	openAIWSResponseSessionCachePrefix = "openai:response_session:"
	openAIWSSessionLatestCachePrefix   = "openai:session_latest_response:"
	openAIWSStateStoreCleanupInterval  = time.Minute
	openAIWSStateStoreCleanupMaxPerMap = 512
	openAIWSStateStoreMaxEntriesPerMap = 65536
	openAIWSStateStoreRedisTimeout     = 3 * time.Second
)

type openAIWSAccountBinding struct {
	accountID int64
	expiresAt time.Time
}

type openAIWSConnBinding struct {
	connID    string
	expiresAt time.Time
}

type openAIWSTurnStateBinding struct {
	turnState string
	expiresAt time.Time
}

type openAIWSSessionConnBinding struct {
	connID    string
	expiresAt time.Time
}

type openAIWSStringBinding struct {
	value     string
	expiresAt time.Time
}

type openAIWSStringCache interface {
	GetSessionString(ctx context.Context, groupID int64, sessionHash string) (string, error)
	SetSessionString(ctx context.Context, groupID int64, sessionHash string, value string, ttl time.Duration) error
	DeleteSessionString(ctx context.Context, groupID int64, sessionHash string) error
}

// OpenAIWSStateStore 管理 WSv2 的粘连状态。
// - response_id -> account_id 用于续链路由
// - response_id -> conn_id 用于连接内上下文复用
// - session_hash -> latest_response_id 用于多实例续链纠偏
// - response_id -> session_hash 用于判断旧 response 是否属于当前会话
//
// response_id -> account_id 优先走 GatewayCache（Redis），同时维护本地热缓存。
// response_id -> conn_id 仅在本进程内有效。
// session_hash -> latest_response_id / response_id -> session_hash 优先走可选 Redis 字符串缓存，
// 不可用时退回本地缓存，仅提供本实例内纠偏。
type OpenAIWSStateStore interface {
	BindResponseAccount(ctx context.Context, groupID int64, responseID string, accountID int64, ttl time.Duration) error
	GetResponseAccount(ctx context.Context, groupID int64, responseID string) (int64, error)
	DeleteResponseAccount(ctx context.Context, groupID int64, responseID string) error

	BindSessionResponse(ctx context.Context, groupID int64, sessionHash, responseID string, ttl time.Duration) error
	GetSessionLatestResponse(ctx context.Context, groupID int64, sessionHash string) (string, error)
	GetResponseSession(ctx context.Context, groupID int64, responseID string) (string, error)

	BindResponseConn(responseID, connID string, ttl time.Duration)
	GetResponseConn(responseID string) (string, bool)
	DeleteResponseConn(responseID string)

	BindSessionTurnState(groupID int64, sessionHash, turnState string, ttl time.Duration)
	GetSessionTurnState(groupID int64, sessionHash string) (string, bool)
	DeleteSessionTurnState(groupID int64, sessionHash string)

	BindSessionConn(groupID int64, sessionHash, connID string, ttl time.Duration)
	GetSessionConn(groupID int64, sessionHash string) (string, bool)
	DeleteSessionConn(groupID int64, sessionHash string)
}

type defaultOpenAIWSStateStore struct {
	cache GatewayCache

	responseToAccountMu       sync.RWMutex
	responseToAccount         map[string]openAIWSAccountBinding
	responseToConnMu          sync.RWMutex
	responseToConn            map[string]openAIWSConnBinding
	sessionToLatestResponseMu sync.RWMutex
	sessionToLatestResponse   map[string]openAIWSStringBinding
	responseToSessionMu       sync.RWMutex
	responseToSession         map[string]openAIWSStringBinding
	sessionToTurnStateMu      sync.RWMutex
	sessionToTurnState        map[string]openAIWSTurnStateBinding
	sessionToConnMu           sync.RWMutex
	sessionToConn             map[string]openAIWSSessionConnBinding

	lastCleanupUnixNano atomic.Int64
}

// NewOpenAIWSStateStore 创建默认 WS 状态存储。
func NewOpenAIWSStateStore(cache GatewayCache) OpenAIWSStateStore {
	store := &defaultOpenAIWSStateStore{
		cache:                   cache,
		responseToAccount:       make(map[string]openAIWSAccountBinding, 256),
		responseToConn:          make(map[string]openAIWSConnBinding, 256),
		sessionToLatestResponse: make(map[string]openAIWSStringBinding, 256),
		responseToSession:       make(map[string]openAIWSStringBinding, 256),
		sessionToTurnState:      make(map[string]openAIWSTurnStateBinding, 256),
		sessionToConn:           make(map[string]openAIWSSessionConnBinding, 256),
	}
	store.lastCleanupUnixNano.Store(time.Now().UnixNano())
	return store
}

func (s *defaultOpenAIWSStateStore) BindResponseAccount(ctx context.Context, groupID int64, responseID string, accountID int64, ttl time.Duration) error {
	id := normalizeOpenAIWSResponseID(responseID)
	if id == "" || accountID <= 0 {
		return nil
	}
	ttl = normalizeOpenAIWSTTL(ttl)
	s.maybeCleanup()

	expiresAt := time.Now().Add(ttl)
	s.responseToAccountMu.Lock()
	ensureBindingCapacity(s.responseToAccount, id, openAIWSStateStoreMaxEntriesPerMap)
	s.responseToAccount[id] = openAIWSAccountBinding{accountID: accountID, expiresAt: expiresAt}
	s.responseToAccountMu.Unlock()

	if s.cache == nil {
		return nil
	}
	cacheKey := openAIWSResponseAccountCacheKey(id)
	cacheCtx, cancel := withOpenAIWSStateStoreRedisTimeout(ctx)
	defer cancel()
	return s.cache.SetSessionAccountID(cacheCtx, groupID, cacheKey, accountID, ttl)
}

func (s *defaultOpenAIWSStateStore) GetResponseAccount(ctx context.Context, groupID int64, responseID string) (int64, error) {
	id := normalizeOpenAIWSResponseID(responseID)
	if id == "" {
		return 0, nil
	}
	s.maybeCleanup()

	now := time.Now()
	s.responseToAccountMu.RLock()
	if binding, ok := s.responseToAccount[id]; ok {
		if now.Before(binding.expiresAt) {
			accountID := binding.accountID
			s.responseToAccountMu.RUnlock()
			return accountID, nil
		}
	}
	s.responseToAccountMu.RUnlock()

	if s.cache == nil {
		return 0, nil
	}

	cacheKey := openAIWSResponseAccountCacheKey(id)
	cacheCtx, cancel := withOpenAIWSStateStoreRedisTimeout(ctx)
	defer cancel()
	accountID, err := s.cache.GetSessionAccountID(cacheCtx, groupID, cacheKey)
	if err != nil || accountID <= 0 {
		// 缓存读取失败不阻断主流程，按未命中降级。
		return 0, nil
	}
	return accountID, nil
}

func (s *defaultOpenAIWSStateStore) DeleteResponseAccount(ctx context.Context, groupID int64, responseID string) error {
	id := normalizeOpenAIWSResponseID(responseID)
	if id == "" {
		return nil
	}
	s.responseToAccountMu.Lock()
	delete(s.responseToAccount, id)
	s.responseToAccountMu.Unlock()

	if s.cache == nil {
		return nil
	}
	cacheCtx, cancel := withOpenAIWSStateStoreRedisTimeout(ctx)
	defer cancel()
	return s.cache.DeleteSessionAccountID(cacheCtx, groupID, openAIWSResponseAccountCacheKey(id))
}

func (s *defaultOpenAIWSStateStore) BindSessionResponse(ctx context.Context, groupID int64, sessionHash, responseID string, ttl time.Duration) error {
	sessionKey := openAIWSSessionTurnStateKey(groupID, sessionHash)
	responseKey := openAIWSResponseGroupKey(groupID, responseID)
	normalizedSessionHash := strings.TrimSpace(sessionHash)
	normalizedResponseID := normalizeOpenAIWSResponseID(responseID)
	if sessionKey == "" || responseKey == "" || normalizedSessionHash == "" || normalizedResponseID == "" {
		return nil
	}
	ttl = normalizeOpenAIWSTTL(ttl)
	s.maybeCleanup()

	expiresAt := time.Now().Add(ttl)
	s.sessionToLatestResponseMu.Lock()
	ensureBindingCapacity(s.sessionToLatestResponse, sessionKey, openAIWSStateStoreMaxEntriesPerMap)
	s.sessionToLatestResponse[sessionKey] = openAIWSStringBinding{value: normalizedResponseID, expiresAt: expiresAt}
	s.sessionToLatestResponseMu.Unlock()

	s.responseToSessionMu.Lock()
	ensureBindingCapacity(s.responseToSession, responseKey, openAIWSStateStoreMaxEntriesPerMap)
	s.responseToSession[responseKey] = openAIWSStringBinding{value: normalizedSessionHash, expiresAt: expiresAt}
	s.responseToSessionMu.Unlock()

	stringCache, ok := s.cache.(openAIWSStringCache)
	if !ok || stringCache == nil {
		return nil
	}

	cacheCtx, cancel := withOpenAIWSStateStoreRedisTimeout(ctx)
	defer cancel()
	if err := stringCache.SetSessionString(cacheCtx, groupID, openAIWSSessionLatestResponseCacheKey(sessionHash), normalizedResponseID, ttl); err != nil {
		return err
	}
	return stringCache.SetSessionString(cacheCtx, groupID, openAIWSResponseSessionCacheKey(responseID), normalizedSessionHash, ttl)
}

func (s *defaultOpenAIWSStateStore) GetSessionLatestResponse(ctx context.Context, groupID int64, sessionHash string) (string, error) {
	key := openAIWSSessionTurnStateKey(groupID, sessionHash)
	if key == "" {
		return "", nil
	}
	s.maybeCleanup()

	stringCache, hasStringCache := s.cache.(openAIWSStringCache)
	if hasStringCache && stringCache != nil {
		cacheCtx, cancel := withOpenAIWSStateStoreRedisTimeout(ctx)
		defer cancel()
		value, err := stringCache.GetSessionString(cacheCtx, groupID, openAIWSSessionLatestResponseCacheKey(sessionHash))
		if err != nil {
			return "", nil
		}
		return strings.TrimSpace(value), nil
	}

	now := time.Now()
	s.sessionToLatestResponseMu.RLock()
	if binding, ok := s.sessionToLatestResponse[key]; ok {
		if now.Before(binding.expiresAt) {
			value := strings.TrimSpace(binding.value)
			s.sessionToLatestResponseMu.RUnlock()
			return value, nil
		}
	}
	s.sessionToLatestResponseMu.RUnlock()
	return "", nil
}

func (s *defaultOpenAIWSStateStore) GetResponseSession(ctx context.Context, groupID int64, responseID string) (string, error) {
	key := openAIWSResponseGroupKey(groupID, responseID)
	if key == "" {
		return "", nil
	}
	s.maybeCleanup()

	now := time.Now()
	s.responseToSessionMu.RLock()
	if binding, ok := s.responseToSession[key]; ok {
		if now.Before(binding.expiresAt) {
			value := strings.TrimSpace(binding.value)
			s.responseToSessionMu.RUnlock()
			return value, nil
		}
	}
	s.responseToSessionMu.RUnlock()

	stringCache, ok := s.cache.(openAIWSStringCache)
	if !ok || stringCache == nil {
		return "", nil
	}
	cacheCtx, cancel := withOpenAIWSStateStoreRedisTimeout(ctx)
	defer cancel()
	value, err := stringCache.GetSessionString(cacheCtx, groupID, openAIWSResponseSessionCacheKey(responseID))
	if err != nil {
		return "", nil
	}
	return strings.TrimSpace(value), nil
}

func (s *defaultOpenAIWSStateStore) BindResponseConn(responseID, connID string, ttl time.Duration) {
	id := normalizeOpenAIWSResponseID(responseID)
	conn := strings.TrimSpace(connID)
	if id == "" || conn == "" {
		return
	}
	ttl = normalizeOpenAIWSTTL(ttl)
	s.maybeCleanup()

	s.responseToConnMu.Lock()
	ensureBindingCapacity(s.responseToConn, id, openAIWSStateStoreMaxEntriesPerMap)
	s.responseToConn[id] = openAIWSConnBinding{
		connID:    conn,
		expiresAt: time.Now().Add(ttl),
	}
	s.responseToConnMu.Unlock()
}

func (s *defaultOpenAIWSStateStore) GetResponseConn(responseID string) (string, bool) {
	id := normalizeOpenAIWSResponseID(responseID)
	if id == "" {
		return "", false
	}
	s.maybeCleanup()

	now := time.Now()
	s.responseToConnMu.RLock()
	binding, ok := s.responseToConn[id]
	s.responseToConnMu.RUnlock()
	if !ok || now.After(binding.expiresAt) || strings.TrimSpace(binding.connID) == "" {
		return "", false
	}
	return binding.connID, true
}

func (s *defaultOpenAIWSStateStore) DeleteResponseConn(responseID string) {
	id := normalizeOpenAIWSResponseID(responseID)
	if id == "" {
		return
	}
	s.responseToConnMu.Lock()
	delete(s.responseToConn, id)
	s.responseToConnMu.Unlock()
}

func (s *defaultOpenAIWSStateStore) BindSessionTurnState(groupID int64, sessionHash, turnState string, ttl time.Duration) {
	key := openAIWSSessionTurnStateKey(groupID, sessionHash)
	state := strings.TrimSpace(turnState)
	if key == "" || state == "" {
		return
	}
	ttl = normalizeOpenAIWSTTL(ttl)
	s.maybeCleanup()

	s.sessionToTurnStateMu.Lock()
	ensureBindingCapacity(s.sessionToTurnState, key, openAIWSStateStoreMaxEntriesPerMap)
	s.sessionToTurnState[key] = openAIWSTurnStateBinding{
		turnState: state,
		expiresAt: time.Now().Add(ttl),
	}
	s.sessionToTurnStateMu.Unlock()
}

func (s *defaultOpenAIWSStateStore) GetSessionTurnState(groupID int64, sessionHash string) (string, bool) {
	key := openAIWSSessionTurnStateKey(groupID, sessionHash)
	if key == "" {
		return "", false
	}
	s.maybeCleanup()

	now := time.Now()
	s.sessionToTurnStateMu.RLock()
	binding, ok := s.sessionToTurnState[key]
	s.sessionToTurnStateMu.RUnlock()
	if !ok || now.After(binding.expiresAt) || strings.TrimSpace(binding.turnState) == "" {
		return "", false
	}
	return binding.turnState, true
}

func (s *defaultOpenAIWSStateStore) DeleteSessionTurnState(groupID int64, sessionHash string) {
	key := openAIWSSessionTurnStateKey(groupID, sessionHash)
	if key == "" {
		return
	}
	s.sessionToTurnStateMu.Lock()
	delete(s.sessionToTurnState, key)
	s.sessionToTurnStateMu.Unlock()
}

func (s *defaultOpenAIWSStateStore) BindSessionConn(groupID int64, sessionHash, connID string, ttl time.Duration) {
	key := openAIWSSessionTurnStateKey(groupID, sessionHash)
	conn := strings.TrimSpace(connID)
	if key == "" || conn == "" {
		return
	}
	ttl = normalizeOpenAIWSTTL(ttl)
	s.maybeCleanup()

	s.sessionToConnMu.Lock()
	ensureBindingCapacity(s.sessionToConn, key, openAIWSStateStoreMaxEntriesPerMap)
	s.sessionToConn[key] = openAIWSSessionConnBinding{
		connID:    conn,
		expiresAt: time.Now().Add(ttl),
	}
	s.sessionToConnMu.Unlock()
}

func (s *defaultOpenAIWSStateStore) GetSessionConn(groupID int64, sessionHash string) (string, bool) {
	key := openAIWSSessionTurnStateKey(groupID, sessionHash)
	if key == "" {
		return "", false
	}
	s.maybeCleanup()

	now := time.Now()
	s.sessionToConnMu.RLock()
	binding, ok := s.sessionToConn[key]
	s.sessionToConnMu.RUnlock()
	if !ok || now.After(binding.expiresAt) || strings.TrimSpace(binding.connID) == "" {
		return "", false
	}
	return binding.connID, true
}

func (s *defaultOpenAIWSStateStore) DeleteSessionConn(groupID int64, sessionHash string) {
	key := openAIWSSessionTurnStateKey(groupID, sessionHash)
	if key == "" {
		return
	}
	s.sessionToConnMu.Lock()
	delete(s.sessionToConn, key)
	s.sessionToConnMu.Unlock()
}

func (s *defaultOpenAIWSStateStore) maybeCleanup() {
	if s == nil {
		return
	}
	now := time.Now()
	last := time.Unix(0, s.lastCleanupUnixNano.Load())
	if now.Sub(last) < openAIWSStateStoreCleanupInterval {
		return
	}
	if !s.lastCleanupUnixNano.CompareAndSwap(last.UnixNano(), now.UnixNano()) {
		return
	}

	// 增量限额清理，避免高规模下一次性全量扫描导致长时间阻塞。
	s.responseToAccountMu.Lock()
	cleanupExpiredAccountBindings(s.responseToAccount, now, openAIWSStateStoreCleanupMaxPerMap)
	s.responseToAccountMu.Unlock()

	s.responseToConnMu.Lock()
	cleanupExpiredConnBindings(s.responseToConn, now, openAIWSStateStoreCleanupMaxPerMap)
	s.responseToConnMu.Unlock()

	s.sessionToLatestResponseMu.Lock()
	cleanupExpiredStringBindings(s.sessionToLatestResponse, now, openAIWSStateStoreCleanupMaxPerMap)
	s.sessionToLatestResponseMu.Unlock()

	s.responseToSessionMu.Lock()
	cleanupExpiredStringBindings(s.responseToSession, now, openAIWSStateStoreCleanupMaxPerMap)
	s.responseToSessionMu.Unlock()

	s.sessionToTurnStateMu.Lock()
	cleanupExpiredTurnStateBindings(s.sessionToTurnState, now, openAIWSStateStoreCleanupMaxPerMap)
	s.sessionToTurnStateMu.Unlock()

	s.sessionToConnMu.Lock()
	cleanupExpiredSessionConnBindings(s.sessionToConn, now, openAIWSStateStoreCleanupMaxPerMap)
	s.sessionToConnMu.Unlock()
}

func cleanupExpiredAccountBindings(bindings map[string]openAIWSAccountBinding, now time.Time, maxScan int) {
	if len(bindings) == 0 || maxScan <= 0 {
		return
	}
	scanned := 0
	for key, binding := range bindings {
		if now.After(binding.expiresAt) {
			delete(bindings, key)
		}
		scanned++
		if scanned >= maxScan {
			break
		}
	}
}

func cleanupExpiredConnBindings(bindings map[string]openAIWSConnBinding, now time.Time, maxScan int) {
	if len(bindings) == 0 || maxScan <= 0 {
		return
	}
	scanned := 0
	for key, binding := range bindings {
		if now.After(binding.expiresAt) {
			delete(bindings, key)
		}
		scanned++
		if scanned >= maxScan {
			break
		}
	}
}

func cleanupExpiredTurnStateBindings(bindings map[string]openAIWSTurnStateBinding, now time.Time, maxScan int) {
	if len(bindings) == 0 || maxScan <= 0 {
		return
	}
	scanned := 0
	for key, binding := range bindings {
		if now.After(binding.expiresAt) {
			delete(bindings, key)
		}
		scanned++
		if scanned >= maxScan {
			break
		}
	}
}

func cleanupExpiredSessionConnBindings(bindings map[string]openAIWSSessionConnBinding, now time.Time, maxScan int) {
	if len(bindings) == 0 || maxScan <= 0 {
		return
	}
	scanned := 0
	for key, binding := range bindings {
		if now.After(binding.expiresAt) {
			delete(bindings, key)
		}
		scanned++
		if scanned >= maxScan {
			break
		}
	}
}

func cleanupExpiredStringBindings(bindings map[string]openAIWSStringBinding, now time.Time, maxScan int) {
	if len(bindings) == 0 || maxScan <= 0 {
		return
	}
	scanned := 0
	for key, binding := range bindings {
		if now.After(binding.expiresAt) {
			delete(bindings, key)
		}
		scanned++
		if scanned >= maxScan {
			break
		}
	}
}

func ensureBindingCapacity[T any](bindings map[string]T, incomingKey string, maxEntries int) {
	if len(bindings) < maxEntries || maxEntries <= 0 {
		return
	}
	if _, exists := bindings[incomingKey]; exists {
		return
	}
	// 固定上限保护：淘汰任意一项，优先保证内存有界。
	for key := range bindings {
		delete(bindings, key)
		return
	}
}

func normalizeOpenAIWSResponseID(responseID string) string {
	return strings.TrimSpace(responseID)
}

func openAIWSResponseAccountCacheKey(responseID string) string {
	sum := sha256.Sum256([]byte(responseID))
	return openAIWSResponseAccountCachePrefix + hex.EncodeToString(sum[:])
}

func openAIWSResponseSessionCacheKey(responseID string) string {
	id := normalizeOpenAIWSResponseID(responseID)
	if id == "" {
		return ""
	}
	sum := sha256.Sum256([]byte(id))
	return openAIWSResponseSessionCachePrefix + hex.EncodeToString(sum[:])
}

func openAIWSSessionLatestResponseCacheKey(sessionHash string) string {
	hash := strings.TrimSpace(sessionHash)
	if hash == "" {
		return ""
	}
	sum := sha256.Sum256([]byte(hash))
	return openAIWSSessionLatestCachePrefix + hex.EncodeToString(sum[:])
}

func normalizeOpenAIWSTTL(ttl time.Duration) time.Duration {
	if ttl <= 0 {
		return time.Hour
	}
	return ttl
}

func openAIWSSessionTurnStateKey(groupID int64, sessionHash string) string {
	hash := strings.TrimSpace(sessionHash)
	if hash == "" {
		return ""
	}
	return fmt.Sprintf("%d:%s", groupID, hash)
}

func openAIWSResponseGroupKey(groupID int64, responseID string) string {
	id := normalizeOpenAIWSResponseID(responseID)
	if id == "" {
		return ""
	}
	return fmt.Sprintf("%d:%s", groupID, id)
}

func withOpenAIWSStateStoreRedisTimeout(ctx context.Context) (context.Context, context.CancelFunc) {
	if ctx == nil {
		ctx = context.Background()
	}
	return context.WithTimeout(ctx, openAIWSStateStoreRedisTimeout)
}
