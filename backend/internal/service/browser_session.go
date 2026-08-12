package service

import (
	"context"
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha256"
	"database/sql"
	"encoding/hex"
	"errors"
	"fmt"
	"strings"
	"time"

	dbent "ikik-api/ent"
	infraerrors "ikik-api/internal/pkg/errors"
)

const (
	browserAccessTokenTTL       = 15 * time.Minute
	browserSessionTTL           = 30 * 24 * time.Hour
	browserRefreshReplayWindow  = 30 * time.Second
	browserSessionActiveLimit   = 50
	browserSessionIssuanceLimit = 100
	browserSessionIssueWindow   = 24 * time.Hour
)

var (
	ErrLoginSessionRevoked  = infraerrors.Unauthorized("AUTH_SESSION_REVOKED", "login session has been revoked")
	ErrLoginSessionMismatch = infraerrors.Conflict("AUTH_SESSION_MISMATCH", "login session does not match the expected session")
	ErrRefreshRace          = infraerrors.Conflict("AUTH_REFRESH_RACE", "refresh token was already rotated")
	ErrLoginSessionLimit    = infraerrors.Conflict("AUTH_SESSION_LIMIT", "active login session limit reached")
	ErrSessionIssuanceLimit = infraerrors.TooManyRequests("AUTH_SESSION_ISSUANCE_LIMIT", "login session issuance limit reached")
)

type LoginSessionView struct {
	SID          string `json:"sid"`
	Current      bool   `json:"current"`
	LoginMethod  string `json:"login_method"`
	IP           string `json:"ip"`
	UserAgent    string `json:"user_agent"`
	CreatedAt    int64  `json:"created_at"`
	LastActiveAt int64  `json:"last_active_at"`
	ExpiresAt    int64  `json:"expires_at"`
}

type browserSession struct {
	SID                 string
	UserID              int64
	Version             int64
	UserAuthVersion     int64
	Status              string
	RefreshHash         string
	PreviousRefreshHash string
	PreviousValidUntil  sql.NullTime
	LoginMethod         string
	IP                  string
	UserAgent           string
	CreatedAt           time.Time
	LastActiveAt        time.Time
	ExpiresAt           time.Time
	RevokedAt           sql.NullTime
	RevokedReason       string
}

func (s *AuthService) createBrowserSession(ctx context.Context, user *User, loginMethod string) (*TokenPair, error) {
	if s == nil || s.entClient == nil || user == nil || user.ID <= 0 || !user.IsActive() {
		return nil, ErrServiceUnavailable
	}
	now := time.Now().UTC()
	client := s.browserSessionClient(ctx)
	active, err := countBrowserSessions(ctx, client, user.ID, true, now)
	if err != nil {
		return nil, fmt.Errorf("count active browser sessions: %w", err)
	}
	if active >= browserSessionActiveLimit {
		return nil, ErrLoginSessionLimit
	}
	issued, err := countBrowserSessions(ctx, client, user.ID, false, now.Add(-browserSessionIssueWindow))
	if err != nil {
		return nil, fmt.Errorf("count issued browser sessions: %w", err)
	}
	if issued >= browserSessionIssuanceLimit {
		return nil, ErrSessionIssuanceLimit
	}

	sid, err := randomUUIDString()
	if err != nil {
		return nil, fmt.Errorf("generate browser session id: %w", err)
	}
	secret, err := randomSecret(64)
	if err != nil {
		return nil, fmt.Errorf("generate browser refresh secret: %w", err)
	}
	loginMethod = strings.TrimSpace(loginMethod)
	if loginMethod == "" {
		loginMethod = "unknown"
	}
	binding := SessionBindingFromContext(ctx)
	ip, userAgent := "", ""
	if binding != nil {
		ip = truncateAuthMetadata(binding.IP, 64)
		userAgent = truncateAuthMetadata(binding.UserAgent, 512)
	}
	expiresAt := now.Add(browserSessionTTL)
	_, err = client.ExecContext(ctx, `
		INSERT INTO user_sessions (
			sid, user_id, version, user_auth_version, status, refresh_hash,
			login_method, ip, user_agent, created_at, updated_at, last_active_at, expires_at
		) VALUES ($1, $2, 1, $3, 'active', $4, $5, $6, $7, $8, $8, $8, $9)`,
		sid, user.ID, resolvedTokenVersion(user), s.hashBrowserRefreshSecret(secret),
		loginMethod, ip, userAgent, now, expiresAt,
	)
	if err != nil {
		return nil, fmt.Errorf("create browser session: %w", err)
	}

	session := &browserSession{
		SID: sid, UserID: user.ID, Version: 1, UserAuthVersion: resolvedTokenVersion(user),
		Status: "active", RefreshHash: s.hashBrowserRefreshSecret(secret), LoginMethod: loginMethod,
		IP: ip, UserAgent: userAgent, CreatedAt: now, LastActiveAt: now, ExpiresAt: expiresAt,
	}
	pair, err := s.issueBrowserTokenPair(user, session, sid+"."+secret)
	if err != nil {
		_ = s.revokeBrowserSessionBySID(ctx, user.ID, sid, "token_issue_failed")
		return nil, err
	}
	return pair, nil
}

func (s *AuthService) browserSessionClient(ctx context.Context) *dbent.Client {
	if tx := dbent.TxFromContext(ctx); tx != nil {
		return tx.Client()
	}
	return s.entClient
}

func countBrowserSessions(ctx context.Context, client browserSessionQueryer, userID int64, active bool, cutoff time.Time) (int, error) {
	query := `SELECT COUNT(*) FROM user_sessions WHERE user_id = $1 AND created_at > $2`
	args := []any{userID, cutoff}
	if active {
		query = `SELECT COUNT(*) FROM user_sessions WHERE user_id = $1 AND status = 'active' AND expires_at > $2`
	}
	rows, err := client.QueryContext(ctx, query, args...)
	if err != nil {
		return 0, err
	}
	defer rows.Close()
	if !rows.Next() {
		return 0, rows.Err()
	}
	var count int
	if err := rows.Scan(&count); err != nil {
		return 0, err
	}
	return count, nil
}

func (s *AuthService) refreshBrowserSession(ctx context.Context, rawToken, expectedSID string) (*TokenPairWithUser, error) {
	sid, secret, ok := splitBrowserRefreshToken(rawToken)
	if !ok {
		return nil, ErrRefreshTokenInvalid
	}
	if expectedSID = strings.TrimSpace(expectedSID); expectedSID != "" && expectedSID != sid {
		return nil, ErrLoginSessionMismatch
	}

	tx, err := s.entClient.Tx(ctx)
	if err != nil {
		return nil, ErrServiceUnavailable
	}
	defer func() { _ = tx.Rollback() }()
	client := tx.Client()
	session, err := queryBrowserSession(ctx, client, sid, true)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, ErrRefreshTokenInvalid
		}
		return nil, ErrServiceUnavailable
	}
	now := time.Now().UTC()
	if session.Status != "active" || session.RevokedAt.Valid || !session.ExpiresAt.After(now) {
		return nil, ErrLoginSessionRevoked
	}
	user, err := s.userRepo.GetByID(ctx, session.UserID)
	if err != nil || user == nil || !user.IsActive() || resolvedTokenVersion(user) != session.UserAuthVersion {
		_, _ = client.ExecContext(ctx, `UPDATE user_sessions SET status = 'revoked', revoked_at = $2, revoked_reason = 'user_security_changed', updated_at = $2 WHERE sid = $1 AND status = 'active'`, sid, now)
		_ = tx.Commit()
		return nil, ErrLoginSessionRevoked
	}

	presentedHash := s.hashBrowserRefreshSecret(secret)
	nextSecret := s.deriveNextBrowserRefreshSecret(sid, secret)
	nextHash := s.hashBrowserRefreshSecret(nextSecret)
	binding := SessionBindingFromContext(ctx)
	ip, userAgent := session.IP, session.UserAgent
	if binding != nil {
		ip = truncateAuthMetadata(binding.IP, 64)
		userAgent = truncateAuthMetadata(binding.UserAgent, 512)
	}

	switch {
	case hmac.Equal([]byte(session.RefreshHash), []byte(presentedHash)):
		result, updateErr := client.ExecContext(ctx, `
			UPDATE user_sessions
			SET previous_refresh_hash = refresh_hash, previous_valid_until = $4,
				refresh_hash = $3, last_active_at = $2, updated_at = $2, ip = $5, user_agent = $6
			WHERE sid = $1 AND status = 'active' AND refresh_hash = $7 AND expires_at > $2`,
			sid, now, nextHash, now.Add(browserRefreshReplayWindow), ip, userAgent, presentedHash,
		)
		if updateErr != nil {
			return nil, ErrServiceUnavailable
		}
		affected, _ := result.RowsAffected()
		if affected != 1 {
			return nil, ErrRefreshRace
		}
		session.PreviousRefreshHash = session.RefreshHash
		session.PreviousValidUntil = sql.NullTime{Time: now.Add(browserRefreshReplayWindow), Valid: true}
		session.RefreshHash = nextHash
		session.LastActiveAt = now
		session.IP, session.UserAgent = ip, userAgent
	case session.PreviousRefreshHash != "" && hmac.Equal([]byte(session.PreviousRefreshHash), []byte(presentedHash)):
		if !session.PreviousValidUntil.Valid || now.After(session.PreviousValidUntil.Time) {
			_, _ = client.ExecContext(ctx, `UPDATE user_sessions SET status = 'revoked', revoked_at = $2, revoked_reason = 'refresh_reuse', updated_at = $2 WHERE sid = $1 AND status = 'active'`, sid, now)
			_ = tx.Commit()
			return nil, ErrLoginSessionRevoked
		}
		if !hmac.Equal([]byte(session.RefreshHash), []byte(nextHash)) {
			return nil, ErrRefreshRace
		}
	default:
		// An unknown secret must not revoke the victim's real session.
		return nil, ErrRefreshTokenInvalid
	}

	if err := tx.Commit(); err != nil {
		return nil, ErrServiceUnavailable
	}
	pair, err := s.issueBrowserTokenPair(user, session, sid+"."+nextSecret)
	if err != nil {
		return nil, err
	}
	return &TokenPairWithUser{TokenPair: *pair, UserRole: user.Role, User: user}, nil
}

func (s *AuthService) RefreshTokenPairForSession(ctx context.Context, rawToken, expectedSID string) (*TokenPairWithUser, error) {
	if _, _, ok := splitBrowserRefreshToken(rawToken); ok && s.entClient != nil {
		return s.refreshBrowserSession(ctx, rawToken, expectedSID)
	}
	return s.RefreshTokenPair(ctx, rawToken)
}

func (s *AuthService) issueBrowserTokenPair(user *User, session *browserSession, rawRefreshToken string) (*TokenPair, error) {
	accessToken, expiresAt, err := s.generateBrowserAccessToken(user, session.SID, session.Version)
	if err != nil {
		return nil, fmt.Errorf("generate browser access token: %w", err)
	}
	return &TokenPair{
		AccessToken: accessToken, RefreshToken: rawRefreshToken,
		ExpiresIn: int(browserAccessTokenTTL / time.Second), AccessExpiresAt: expiresAt.Unix(),
		Session: browserSessionView(session),
	}, nil
}

func (s *AuthService) ValidateBrowserSessionClaims(ctx context.Context, claims *JWTClaims) error {
	if claims == nil || claims.SessionVersion <= 0 {
		return nil
	}
	session, err := queryBrowserSession(ctx, s.entClient, claims.SessionID, false)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return ErrLoginSessionRevoked
		}
		return ErrServiceUnavailable
	}
	if session.UserID != claims.UserID || session.Version != claims.SessionVersion ||
		session.UserAuthVersion != claims.TokenVersion || session.Status != "active" ||
		session.RevokedAt.Valid || !session.ExpiresAt.After(time.Now().UTC()) {
		return ErrLoginSessionRevoked
	}
	return nil
}

func (s *AuthService) revokeBrowserSessionByToken(ctx context.Context, rawToken, expectedSID, reason string) error {
	sid, secret, ok := splitBrowserRefreshToken(rawToken)
	if !ok {
		return nil
	}
	if expectedSID = strings.TrimSpace(expectedSID); expectedSID != "" && sid != expectedSID {
		return ErrLoginSessionMismatch
	}
	tx, err := s.entClient.Tx(ctx)
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback() }()
	session, err := queryBrowserSession(ctx, tx.Client(), sid, true)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil
		}
		return err
	}
	now := time.Now().UTC()
	presented := s.hashBrowserRefreshSecret(secret)
	validCurrent := hmac.Equal([]byte(session.RefreshHash), []byte(presented))
	validPrevious := session.PreviousRefreshHash != "" && session.PreviousValidUntil.Valid && !now.After(session.PreviousValidUntil.Time) && hmac.Equal([]byte(session.PreviousRefreshHash), []byte(presented))
	if session.Status == "active" && (validCurrent || validPrevious) {
		_, err = tx.Client().ExecContext(ctx, `UPDATE user_sessions SET status = 'revoked', revoked_at = $2, revoked_reason = $3, updated_at = $2 WHERE sid = $1 AND status = 'active'`, sid, now, reason)
		if err != nil {
			return err
		}
	}
	return tx.Commit()
}

func (s *AuthService) revokeBrowserSessionBySID(ctx context.Context, userID int64, sid, reason string) error {
	if s == nil || s.entClient == nil || sid == "" {
		return nil
	}
	now := time.Now().UTC()
	_, err := s.entClient.ExecContext(ctx, `UPDATE user_sessions SET status = 'revoked', revoked_at = $4, revoked_reason = $3, updated_at = $4 WHERE sid = $1 AND user_id = $2 AND status = 'active'`, sid, userID, reason, now)
	return err
}

func (s *AuthService) revokeAllBrowserSessions(ctx context.Context, userID int64, reason string) error {
	if s == nil || s.entClient == nil {
		return nil
	}
	now := time.Now().UTC()
	_, err := s.entClient.ExecContext(ctx, `UPDATE user_sessions SET status = 'revoked', revoked_at = $3, revoked_reason = $2, updated_at = $3 WHERE user_id = $1 AND status = 'active'`, userID, reason, now)
	return err
}

// ListBrowserSessions returns the active, unexpired sessions for a user.
func (s *AuthService) ListBrowserSessions(ctx context.Context, userID int64, currentSID string) ([]LoginSessionView, error) {
	if s == nil || s.entClient == nil || userID <= 0 {
		return nil, ErrServiceUnavailable
	}
	rows, err := s.entClient.QueryContext(ctx, `SELECT sid, login_method, ip, user_agent, created_at, last_active_at, expires_at
		FROM user_sessions WHERE user_id = $1 AND status = 'active' AND revoked_at IS NULL AND expires_at > $2
		ORDER BY last_active_at DESC`, userID, time.Now().UTC())
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	views := make([]LoginSessionView, 0)
	for rows.Next() {
		var sid, loginMethod, ip, userAgent string
		var createdAt, lastActiveAt, expiresAt time.Time
		if err := rows.Scan(&sid, &loginMethod, &ip, &userAgent, &createdAt, &lastActiveAt, &expiresAt); err != nil {
			return nil, err
		}
		views = append(views, LoginSessionView{
			SID: sid, Current: sid == currentSID, LoginMethod: loginMethod,
			IP: ip, UserAgent: userAgent, CreatedAt: createdAt.Unix(),
			LastActiveAt: lastActiveAt.Unix(), ExpiresAt: expiresAt.Unix(),
		})
	}
	return views, rows.Err()
}

// RevokeBrowserSession revokes one session owned by the authenticated user.
func (s *AuthService) RevokeBrowserSession(ctx context.Context, userID int64, sid, reason string) (bool, error) {
	if s == nil || s.entClient == nil || userID <= 0 || strings.TrimSpace(sid) == "" {
		return false, nil
	}
	now := time.Now().UTC()
	result, err := s.entClient.ExecContext(ctx, `UPDATE user_sessions SET status = 'revoked', revoked_at = $4, revoked_reason = $3, updated_at = $4
		WHERE sid = $1 AND user_id = $2 AND status = 'active'`, strings.TrimSpace(sid), userID, reason, now)
	if err != nil {
		return false, err
	}
	affected, err := result.RowsAffected()
	return affected > 0, err
}

// RevokeOtherBrowserSessions preserves the current browser session.
func (s *AuthService) RevokeOtherBrowserSessions(ctx context.Context, userID int64, currentSID, reason string) (int64, error) {
	if s == nil || s.entClient == nil || userID <= 0 || strings.TrimSpace(currentSID) == "" {
		return 0, ErrLoginSessionMismatch
	}
	now := time.Now().UTC()
	result, err := s.entClient.ExecContext(ctx, `UPDATE user_sessions SET status = 'revoked', revoked_at = $4, revoked_reason = $3, updated_at = $4
		WHERE user_id = $1 AND sid <> $2 AND status = 'active'`, userID, strings.TrimSpace(currentSID), reason, now)
	if err != nil {
		return 0, err
	}
	return result.RowsAffected()
}

// AdvanceCurrentBrowserSessionToUserVersion preserves the current session
// after a password or security-factor change and revokes every other session.
func (s *AuthService) AdvanceCurrentBrowserSessionToUserVersion(ctx context.Context, userID int64, currentSID, reason string) (*TokenPair, error) {
	if s == nil || s.entClient == nil || s.userRepo == nil || userID <= 0 || strings.TrimSpace(currentSID) == "" {
		return nil, ErrLoginSessionMismatch
	}
	user, err := s.userRepo.GetByID(ctx, userID)
	if err != nil || user == nil || !user.IsActive() {
		return nil, ErrLoginSessionRevoked
	}
	tx, err := s.entClient.Tx(ctx)
	if err != nil {
		return nil, ErrServiceUnavailable
	}
	defer func() { _ = tx.Rollback() }()
	session, err := queryBrowserSession(ctx, tx.Client(), strings.TrimSpace(currentSID), true)
	if err != nil || session.UserID != userID || session.Status != "active" || session.RevokedAt.Valid || !session.ExpiresAt.After(time.Now().UTC()) {
		return nil, ErrLoginSessionRevoked
	}
	now := time.Now().UTC()
	nextVersion := resolvedTokenVersion(user)
	result, err := tx.Client().ExecContext(ctx, `UPDATE user_sessions SET version = $5, user_auth_version = $6, last_active_at = $7, updated_at = $7
		WHERE sid = $1 AND user_id = $2 AND status = 'active' AND version = $3 AND user_auth_version = $4`,
		session.SID, userID, session.Version, session.UserAuthVersion, session.Version+1, nextVersion, now)
	if err != nil {
		return nil, err
	}
	affected, err := result.RowsAffected()
	if err != nil || affected != 1 {
		return nil, ErrLoginSessionRevoked
	}
	if _, err := tx.Client().ExecContext(ctx, `UPDATE user_sessions SET status = 'revoked', revoked_at = $4, revoked_reason = $3, updated_at = $4
		WHERE user_id = $1 AND sid <> $2 AND status = 'active'`, userID, session.SID, reason, now); err != nil {
		return nil, err
	}
	if err := tx.Commit(); err != nil {
		return nil, ErrServiceUnavailable
	}
	session.Version++
	session.UserAuthVersion = nextVersion
	session.LastActiveAt = now
	return s.issueBrowserTokenPair(user, session, "")
}

type browserSessionQueryer interface {
	QueryContext(context.Context, string, ...any) (*sql.Rows, error)
}

func queryBrowserSession(ctx context.Context, queryer browserSessionQueryer, sid string, forUpdate bool) (*browserSession, error) {
	query := `SELECT sid, user_id, version, user_auth_version, status, refresh_hash,
		previous_refresh_hash, previous_valid_until, login_method, ip, user_agent,
		created_at, last_active_at, expires_at, revoked_at, revoked_reason
		FROM user_sessions WHERE sid = $1`
	// Rotation uses a compare-and-swap UPDATE, so it remains single-winner on
	// PostgreSQL and SQLite without relying on dialect-specific row locks.
	_ = forUpdate
	rows, err := queryer.QueryContext(ctx, query, sid)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	if !rows.Next() {
		if err := rows.Err(); err != nil {
			return nil, err
		}
		return nil, sql.ErrNoRows
	}
	var session browserSession
	err = rows.Scan(
		&session.SID, &session.UserID, &session.Version, &session.UserAuthVersion,
		&session.Status, &session.RefreshHash, &session.PreviousRefreshHash,
		&session.PreviousValidUntil, &session.LoginMethod, &session.IP, &session.UserAgent,
		&session.CreatedAt, &session.LastActiveAt, &session.ExpiresAt,
		&session.RevokedAt, &session.RevokedReason,
	)
	return &session, err
}

func browserSessionView(session *browserSession) *LoginSessionView {
	return &LoginSessionView{
		SID: session.SID, Current: true, LoginMethod: session.LoginMethod,
		IP: session.IP, UserAgent: session.UserAgent, CreatedAt: session.CreatedAt.Unix(),
		LastActiveAt: session.LastActiveAt.Unix(), ExpiresAt: session.ExpiresAt.Unix(),
	}
}

func splitBrowserRefreshToken(raw string) (string, string, bool) {
	sid, secret, ok := strings.Cut(strings.TrimSpace(raw), ".")
	if !ok || len(sid) != 36 || secret == "" || strings.Contains(secret, ".") {
		return "", "", false
	}
	return sid, secret, true
}

func BrowserRefreshTokenSID(raw string) (string, bool) {
	sid, _, ok := splitBrowserRefreshToken(raw)
	return sid, ok
}

func (s *AuthService) RevokeBrowserSessionByToken(ctx context.Context, rawToken, expectedSID, reason string) error {
	return s.revokeBrowserSessionByToken(ctx, rawToken, expectedSID, reason)
}

func randomUUIDString() (string, error) {
	b := make([]byte, 16)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	b[6] = (b[6] & 0x0f) | 0x40
	b[8] = (b[8] & 0x3f) | 0x80
	return fmt.Sprintf("%08x-%04x-%04x-%04x-%012x", b[0:4], b[4:6], b[6:8], b[8:10], b[10:16]), nil
}

func randomSecret(chars int) (string, error) {
	buf := make([]byte, (chars+1)/2)
	if _, err := rand.Read(buf); err != nil {
		return "", err
	}
	return hex.EncodeToString(buf)[:chars], nil
}

func (s *AuthService) hashBrowserRefreshSecret(secret string) string {
	mac := hmac.New(sha256.New, []byte(s.cfg.JWT.Secret+":refresh"))
	_, _ = mac.Write([]byte(secret))
	return hex.EncodeToString(mac.Sum(nil))
}

func (s *AuthService) deriveNextBrowserRefreshSecret(sid, secret string) string {
	mac := hmac.New(sha256.New, []byte(s.cfg.JWT.Secret+":refresh-rotate"))
	_, _ = mac.Write([]byte(sid + "." + secret))
	return hex.EncodeToString(mac.Sum(nil))
}

func truncateAuthMetadata(value string, max int) string {
	value = strings.TrimSpace(value)
	if len(value) <= max {
		return value
	}
	return value[:max]
}
