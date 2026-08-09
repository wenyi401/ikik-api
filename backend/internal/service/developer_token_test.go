package service

import (
	"context"
	"errors"
	"testing"
	"time"

	infraerrors "ikik-api/internal/pkg/errors"

	"github.com/stretchr/testify/require"
)

type developerTokenRepoStub struct {
	created *DeveloperToken
	count   int
	token   *DeveloperToken
	getErr  error
	deleted int64
	touched int64
	touchIP string
}

func (s *developerTokenRepoStub) Create(_ context.Context, token *DeveloperToken) error {
	copy := *token
	copy.ID = 41
	s.created = &copy
	*token = copy
	return nil
}

func (s *developerTokenRepoStub) ListByUserID(context.Context, int64) ([]DeveloperToken, error) {
	if s.created == nil {
		return nil, nil
	}
	return []DeveloperToken{*s.created}, nil
}

func (s *developerTokenRepoStub) CountByUserID(context.Context, int64) (int, error) {
	return s.count, nil
}

func (s *developerTokenRepoStub) GetByTokenHash(context.Context, string) (*DeveloperToken, error) {
	if s.getErr != nil {
		return nil, s.getErr
	}
	if s.token == nil {
		return nil, ErrDeveloperTokenNotFound
	}
	copy := *s.token
	return &copy, nil
}

func (s *developerTokenRepoStub) DeleteOwned(_ context.Context, _ int64, tokenID int64) error {
	s.deleted = tokenID
	return nil
}

func (s *developerTokenRepoStub) Touch(_ context.Context, tokenID int64, _ time.Time, ip string) error {
	s.touched = tokenID
	s.touchIP = ip
	return nil
}

type developerTokenUserRepoStub struct {
	UserRepository
	user *User
	err  error
}

func (s *developerTokenUserRepoStub) GetByID(context.Context, int64) (*User, error) {
	return s.user, s.err
}

func TestDeveloperTokenCreateStoresOnlyHashAndNormalizesScopes(t *testing.T) {
	repo := &developerTokenRepoStub{}
	users := &developerTokenUserRepoStub{user: &User{ID: 7, Status: StatusActive, DeveloperAPIEnabled: true}}
	service := NewDeveloperTokenService(repo, users)
	expiresAt := time.Now().Add(24 * time.Hour)

	created, err := service.Create(context.Background(), 7, CreateDeveloperTokenInput{
		Name:      "  CI uploader  ",
		Scopes:    []string{DeveloperScopeAccountsWrite, DeveloperScopeAccountsRead, DeveloperScopeAccountsWrite},
		ExpiresAt: &expiresAt,
	})

	require.NoError(t, err)
	require.NotNil(t, created)
	require.Equal(t, "CI uploader", created.DeveloperToken.Name)
	require.Equal(t, []string{DeveloperScopeAccountsRead, DeveloperScopeAccountsWrite}, created.DeveloperToken.Scopes)
	require.Contains(t, created.Token, DeveloperTokenPrefix)
	require.Equal(t, HashDeveloperToken(created.Token), repo.created.TokenHash)
	require.NotEqual(t, created.Token, repo.created.TokenHash)
	require.Equal(t, created.Token[:12], repo.created.TokenPrefix)
}

func TestDeveloperTokenCreateRequiresEnabledActiveUser(t *testing.T) {
	for name, user := range map[string]*User{
		"disabled feature": {ID: 7, Status: StatusActive, DeveloperAPIEnabled: false},
		"disabled user":    {ID: 7, Status: StatusDisabled, DeveloperAPIEnabled: true},
	} {
		t.Run(name, func(t *testing.T) {
			service := NewDeveloperTokenService(&developerTokenRepoStub{}, &developerTokenUserRepoStub{user: user})
			_, err := service.Create(context.Background(), 7, CreateDeveloperTokenInput{
				Name: "test", Scopes: []string{DeveloperScopeAccountsRead},
			})
			require.ErrorIs(t, err, ErrDeveloperAPIDisabled)
		})
	}
}

func TestDeveloperTokenAuthenticateEnforcesExpiryAndPreservesRepositoryErrors(t *testing.T) {
	activeUser := &User{ID: 7, Status: StatusActive, DeveloperAPIEnabled: true}

	t.Run("expired", func(t *testing.T) {
		expiresAt := time.Now().Add(-time.Minute)
		repo := &developerTokenRepoStub{token: &DeveloperToken{
			ID: 1, Status: StatusActive, ExpiresAt: &expiresAt, User: activeUser,
		}}
		_, err := NewDeveloperTokenService(repo, nil).Authenticate(context.Background(), DeveloperTokenPrefix+"expired")
		require.ErrorIs(t, err, ErrDeveloperTokenExpired)
	})

	t.Run("not found", func(t *testing.T) {
		repo := &developerTokenRepoStub{getErr: ErrDeveloperTokenNotFound}
		_, err := NewDeveloperTokenService(repo, nil).Authenticate(context.Background(), DeveloperTokenPrefix+"missing")
		require.ErrorIs(t, err, ErrDeveloperTokenInvalid)
	})

	t.Run("repository outage", func(t *testing.T) {
		outage := errors.New("database unavailable")
		repo := &developerTokenRepoStub{getErr: outage}
		_, err := NewDeveloperTokenService(repo, nil).Authenticate(context.Background(), DeveloperTokenPrefix+"valid-shape")
		require.ErrorIs(t, err, outage)
		require.NotErrorIs(t, err, ErrDeveloperTokenInvalid)
	})
}

func TestNormalizeDeveloperScopesRejectsUnknownScope(t *testing.T) {
	_, err := NormalizeDeveloperScopes([]string{"accounts:admin"})
	require.Error(t, err)
	require.Equal(t, "DEVELOPER_TOKEN_SCOPE_INVALID", infraerrors.Reason(err))
}

func TestNormalizeDeveloperScopesAllowsBotAccess(t *testing.T) {
	scopes, err := NormalizeDeveloperScopes([]string{DeveloperScopeBotAccess, DeveloperScopeBotAccess})
	require.NoError(t, err)
	require.Equal(t, []string{DeveloperScopeBotAccess}, scopes)
}
