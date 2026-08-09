package service

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"errors"
	"sort"
	"strings"
	"time"

	infraerrors "ikik-api/internal/pkg/errors"
)

const (
	DeveloperTokenPrefix        = "ikd_"
	DeveloperScopeAccountsRead  = "accounts:read"
	DeveloperScopeAccountsWrite = "accounts:write"
	DeveloperScopeAccountsShare = "accounts:share"
	DeveloperScopeBotAccess     = "bot:access"
	MaxDeveloperTokensPerUser   = 10
)

var (
	ErrDeveloperAPIDisabled   = infraerrors.Forbidden("DEVELOPER_API_DISABLED", "developer API access is not enabled for this user")
	ErrDeveloperTokenInvalid  = infraerrors.Unauthorized("INVALID_DEVELOPER_TOKEN", "invalid developer token")
	ErrDeveloperTokenExpired  = infraerrors.Unauthorized("DEVELOPER_TOKEN_EXPIRED", "developer token has expired")
	ErrDeveloperTokenScope    = infraerrors.Forbidden("DEVELOPER_TOKEN_SCOPE_REQUIRED", "developer token does not have the required scope")
	ErrDeveloperTokenLimit    = infraerrors.Conflict("DEVELOPER_TOKEN_LIMIT_REACHED", "developer token limit reached")
	ErrDeveloperTokenNotFound = infraerrors.NotFound("DEVELOPER_TOKEN_NOT_FOUND", "developer token not found")
)

type DeveloperToken struct {
	ID          int64      `json:"id"`
	UserID      int64      `json:"user_id"`
	Name        string     `json:"name"`
	TokenPrefix string     `json:"token_prefix"`
	TokenHash   string     `json:"-"`
	Scopes      []string   `json:"scopes"`
	Status      string     `json:"status"`
	ExpiresAt   *time.Time `json:"expires_at,omitempty"`
	LastUsedAt  *time.Time `json:"last_used_at,omitempty"`
	LastUsedIP  *string    `json:"last_used_ip,omitempty"`
	CreatedAt   time.Time  `json:"created_at"`
	UpdatedAt   time.Time  `json:"updated_at"`
	User        *User      `json:"-"`
}

func (t *DeveloperToken) HasScope(scope string) bool {
	if t == nil {
		return false
	}
	for _, candidate := range t.Scopes {
		if candidate == scope {
			return true
		}
	}
	return false
}

type DeveloperTokenRepository interface {
	Create(ctx context.Context, token *DeveloperToken) error
	ListByUserID(ctx context.Context, userID int64) ([]DeveloperToken, error)
	CountByUserID(ctx context.Context, userID int64) (int, error)
	GetByTokenHash(ctx context.Context, tokenHash string) (*DeveloperToken, error)
	DeleteOwned(ctx context.Context, userID, tokenID int64) error
	Touch(ctx context.Context, tokenID int64, usedAt time.Time, ip string) error
}

type DeveloperTokenService struct {
	repo     DeveloperTokenRepository
	userRepo UserRepository
}

type CreateDeveloperTokenInput struct {
	Name      string
	Scopes    []string
	ExpiresAt *time.Time
}

type CreatedDeveloperToken struct {
	DeveloperToken *DeveloperToken `json:"developer_token"`
	Token          string          `json:"token"`
}

func NewDeveloperTokenService(repo DeveloperTokenRepository, userRepo UserRepository) *DeveloperTokenService {
	return &DeveloperTokenService{repo: repo, userRepo: userRepo}
}

func (s *DeveloperTokenService) Create(ctx context.Context, userID int64, input CreateDeveloperTokenInput) (*CreatedDeveloperToken, error) {
	user, err := s.enabledUser(ctx, userID)
	if err != nil {
		return nil, err
	}
	_ = user

	name := strings.TrimSpace(input.Name)
	if name == "" || len(name) > 100 {
		return nil, infraerrors.BadRequest("DEVELOPER_TOKEN_NAME_INVALID", "developer token name is required and must not exceed 100 characters")
	}
	scopes, err := NormalizeDeveloperScopes(input.Scopes)
	if err != nil {
		return nil, err
	}
	if input.ExpiresAt != nil && !input.ExpiresAt.After(time.Now()) {
		return nil, infraerrors.BadRequest("DEVELOPER_TOKEN_EXPIRY_INVALID", "developer token expiry must be in the future")
	}
	count, err := s.repo.CountByUserID(ctx, userID)
	if err != nil {
		return nil, err
	}
	if count >= MaxDeveloperTokensPerUser {
		return nil, ErrDeveloperTokenLimit
	}

	raw, err := generateDeveloperToken()
	if err != nil {
		return nil, err
	}
	token := &DeveloperToken{
		UserID:      userID,
		Name:        name,
		TokenPrefix: raw[:min(len(raw), 12)],
		TokenHash:   HashDeveloperToken(raw),
		Scopes:      scopes,
		Status:      StatusActive,
		ExpiresAt:   input.ExpiresAt,
	}
	if err := s.repo.Create(ctx, token); err != nil {
		return nil, err
	}
	return &CreatedDeveloperToken{DeveloperToken: token, Token: raw}, nil
}

func (s *DeveloperTokenService) List(ctx context.Context, userID int64) ([]DeveloperToken, error) {
	if _, err := s.enabledUser(ctx, userID); err != nil {
		return nil, err
	}
	return s.repo.ListByUserID(ctx, userID)
}

func (s *DeveloperTokenService) Delete(ctx context.Context, userID, tokenID int64) error {
	if _, err := s.enabledUser(ctx, userID); err != nil {
		return err
	}
	return s.repo.DeleteOwned(ctx, userID, tokenID)
}

func (s *DeveloperTokenService) Authenticate(ctx context.Context, raw string) (*DeveloperToken, error) {
	raw = strings.TrimSpace(raw)
	if !strings.HasPrefix(raw, DeveloperTokenPrefix) || len(raw) > 128 {
		return nil, ErrDeveloperTokenInvalid
	}
	token, err := s.repo.GetByTokenHash(ctx, HashDeveloperToken(raw))
	if errors.Is(err, ErrDeveloperTokenNotFound) || token == nil && err == nil {
		return nil, ErrDeveloperTokenInvalid
	}
	if err != nil {
		return nil, err
	}
	if token.Status != StatusActive || token.User == nil || !token.User.IsActive() || !token.User.DeveloperAPIEnabled {
		return nil, ErrDeveloperTokenInvalid
	}
	if token.ExpiresAt != nil && !time.Now().Before(*token.ExpiresAt) {
		return nil, ErrDeveloperTokenExpired
	}
	return token, nil
}

func (s *DeveloperTokenService) Touch(ctx context.Context, tokenID int64, ip string) {
	if s == nil || s.repo == nil || tokenID <= 0 {
		return
	}
	_ = s.repo.Touch(ctx, tokenID, time.Now().UTC(), strings.TrimSpace(ip))
}

func (s *DeveloperTokenService) enabledUser(ctx context.Context, userID int64) (*User, error) {
	if s == nil || s.repo == nil || s.userRepo == nil || userID <= 0 {
		return nil, ErrDeveloperAPIDisabled
	}
	user, err := s.userRepo.GetByID(ctx, userID)
	if err != nil {
		return nil, err
	}
	if !user.IsActive() || !user.DeveloperAPIEnabled {
		return nil, ErrDeveloperAPIDisabled
	}
	return user, nil
}

func NormalizeDeveloperScopes(values []string) ([]string, error) {
	allowed := map[string]struct{}{
		DeveloperScopeAccountsRead:  {},
		DeveloperScopeAccountsWrite: {},
		DeveloperScopeAccountsShare: {},
		DeveloperScopeBotAccess:     {},
	}
	seen := make(map[string]struct{}, len(values))
	for _, value := range values {
		value = strings.ToLower(strings.TrimSpace(value))
		if _, ok := allowed[value]; !ok {
			return nil, infraerrors.BadRequest("DEVELOPER_TOKEN_SCOPE_INVALID", "developer token contains an unsupported scope").WithMetadata(map[string]string{"scope": value})
		}
		seen[value] = struct{}{}
	}
	if len(seen) == 0 {
		return nil, infraerrors.BadRequest("DEVELOPER_TOKEN_SCOPE_INVALID", "at least one developer token scope is required")
	}
	out := make([]string, 0, len(seen))
	for scope := range seen {
		out = append(out, scope)
	}
	sort.Strings(out)
	return out, nil
}

func HashDeveloperToken(raw string) string {
	sum := sha256.Sum256([]byte(raw))
	return hex.EncodeToString(sum[:])
}

func generateDeveloperToken() (string, error) {
	bytes := make([]byte, 32)
	if _, err := rand.Read(bytes); err != nil {
		return "", err
	}
	return DeveloperTokenPrefix + base64.RawURLEncoding.EncodeToString(bytes), nil
}
