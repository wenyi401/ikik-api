package service_test

import (
	"context"
	"database/sql"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	dbent "ikik-api/ent"
	"ikik-api/ent/enttest"
	"ikik-api/ent/usersession"
	"ikik-api/internal/config"
	"ikik-api/internal/repository"
	"ikik-api/internal/service"

	"entgo.io/ent/dialect"
	entsql "entgo.io/ent/dialect/sql"
	_ "modernc.org/sqlite"
)

func newBrowserSessionService(t *testing.T) (*service.AuthService, *service.User, *dbent.Client) {
	t.Helper()
	db, err := sql.Open("sqlite", "file:"+strings.ReplaceAll(t.Name(), "/", "_")+"?mode=memory&cache=shared&_fk=1")
	require.NoError(t, err)
	t.Cleanup(func() { _ = db.Close() })
	_, err = db.Exec("PRAGMA foreign_keys = ON")
	require.NoError(t, err)
	client := enttest.NewClient(t, enttest.WithOptions(dbent.Driver(entsql.OpenDB(dialect.SQLite, db))))
	t.Cleanup(func() { _ = client.Close() })
	ctx := context.Background()
	created, err := client.User.Create().
		SetEmail("session@example.com").
		SetPasswordHash("hash").
		SetRole(service.RoleUser).
		SetStatus(service.StatusActive).
		Save(ctx)
	require.NoError(t, err)
	repo := repository.NewUserRepository(client, db)
	user, err := repo.GetByID(ctx, created.ID)
	require.NoError(t, err)
	cfg := &config.Config{JWT: config.JWTConfig{Secret: "browser-session-test-secret", ExpireHour: 24, RefreshTokenExpireDays: 30}}
	return service.NewAuthService(client, repo, nil, nil, cfg, nil, nil, nil, nil, nil, nil, nil, nil), user, client
}

func TestBrowserSessionRefreshMatchesNewAPIRotationSemantics(t *testing.T) {
	svc, user, client := newBrowserSessionService(t)
	loginCtx := service.WithSessionBinding(context.Background(), &service.SessionBinding{IP: "203.0.113.1", UserAgent: "desktop"})
	pair, err := svc.GenerateTokenPairWithMethod(loginCtx, user, "", "password")
	require.NoError(t, err)
	require.NotEmpty(t, pair.RefreshToken)
	require.Equal(t, 15*60, pair.ExpiresIn)
	require.NotNil(t, pair.Session)
	require.Equal(t, "password", pair.Session.LoginMethod)

	mobileCtx := service.WithSessionBinding(context.Background(), &service.SessionBinding{IP: "198.51.100.8", UserAgent: "mobile"})
	rotated, err := svc.RefreshTokenPairForSession(mobileCtx, pair.RefreshToken, pair.Session.SID)
	require.NoError(t, err)
	require.NotEqual(t, pair.RefreshToken, rotated.RefreshToken)
	require.Equal(t, pair.Session.SID, rotated.Session.SID)
	stored, err := client.UserSession.Query().Where(usersession.SidEQ(pair.Session.SID)).Only(context.Background())
	require.NoError(t, err)
	require.NotNil(t, stored.PreviousValidUntil)
	require.WithinDuration(t, time.Now().Add(30*time.Second), *stored.PreviousValidUntil, 2*time.Second)

	// A concurrent request presenting the immediately previous token receives
	// the deterministic winner instead of losing the login session.
	raceRecovered, err := svc.RefreshTokenPairForSession(mobileCtx, pair.RefreshToken, pair.Session.SID)
	require.NoError(t, err)
	require.Equal(t, rotated.RefreshToken, raceRecovered.RefreshToken)

	// An unknown secret never revokes the victim's valid session.
	_, err = svc.RefreshTokenPairForSession(mobileCtx, pair.Session.SID+".forged-secret", pair.Session.SID)
	require.ErrorIs(t, err, service.ErrRefreshTokenInvalid)
	_, err = svc.RefreshTokenPairForSession(mobileCtx, rotated.RefreshToken, pair.Session.SID)
	require.NoError(t, err)
}

func TestOAuthLoginMethodMatchesNewAPILabels(t *testing.T) {
	require.Equal(t, "wechat", service.OAuthLoginMethod("wechat"))
	require.Equal(t, "telegram", service.OAuthLoginMethod("telegram"))
	require.Equal(t, "oauth:github", service.OAuthLoginMethod(" GitHub "))
	require.Equal(t, "oauth", service.OAuthLoginMethod(""))
}

func TestBrowserSessionKnownPreviousTokenOutsideGraceRevokesSession(t *testing.T) {
	svc, user, client := newBrowserSessionService(t)
	pair, err := svc.GenerateTokenPair(context.Background(), user, "")
	require.NoError(t, err)
	_, err = svc.RefreshTokenPairForSession(context.Background(), pair.RefreshToken, pair.Session.SID)
	require.NoError(t, err)
	_, err = client.UserSession.Update().
		Where(usersession.SidEQ(pair.Session.SID)).
		SetPreviousValidUntil(time.Now().Add(-time.Minute)).
		Save(context.Background())
	require.NoError(t, err)

	_, err = svc.RefreshTokenPairForSession(context.Background(), pair.RefreshToken, pair.Session.SID)
	require.ErrorIs(t, err, service.ErrLoginSessionRevoked)
	stored, err := client.UserSession.Query().Where(usersession.SidEQ(pair.Session.SID)).Only(context.Background())
	require.NoError(t, err)
	require.Equal(t, usersession.StatusRevoked, stored.Status)
	require.Equal(t, "refresh_reuse", stored.RevokedReason)
}

func TestBrowserSessionManagementMatchesNewAPI(t *testing.T) {
	svc, user, _ := newBrowserSessionService(t)
	first, err := svc.GenerateTokenPairWithMethod(context.Background(), user, "", "password")
	require.NoError(t, err)
	second, err := svc.GenerateTokenPairWithMethod(context.Background(), user, "", "passkey")
	require.NoError(t, err)

	sessions, err := svc.ListBrowserSessions(context.Background(), user.ID, first.Session.SID)
	require.NoError(t, err)
	require.Len(t, sessions, 2)
	require.True(t, sessions[0].Current || sessions[1].Current)

	revoked, err := svc.RevokeBrowserSession(context.Background(), user.ID, second.Session.SID, "user_revoked")
	require.NoError(t, err)
	require.True(t, revoked)

	third, err := svc.GenerateTokenPairWithMethod(context.Background(), user, "", "oauth:github")
	require.NoError(t, err)
	revokedCount, err := svc.RevokeOtherBrowserSessions(context.Background(), user.ID, first.Session.SID, "user_revoked_others")
	require.NoError(t, err)
	require.EqualValues(t, 1, revokedCount)

	sessions, err = svc.ListBrowserSessions(context.Background(), user.ID, first.Session.SID)
	require.NoError(t, err)
	require.Len(t, sessions, 1)
	require.Equal(t, first.Session.SID, sessions[0].SID)
	require.True(t, sessions[0].Current)

	_, err = svc.RefreshTokenPairForSession(context.Background(), third.RefreshToken, third.Session.SID)
	require.ErrorIs(t, err, service.ErrLoginSessionRevoked)
}

func TestPasswordSecurityAdvancePreservesCurrentSession(t *testing.T) {
	svc, user, client := newBrowserSessionService(t)
	current, err := svc.GenerateTokenPairWithMethod(context.Background(), user, "", "password")
	require.NoError(t, err)
	other, err := svc.GenerateTokenPairWithMethod(context.Background(), user, "", "password")
	require.NoError(t, err)

	_, err = client.User.UpdateOneID(user.ID).SetPasswordHash("new-password-hash").Save(context.Background())
	require.NoError(t, err)
	rotation, err := svc.AdvanceCurrentBrowserSessionToUserVersion(context.Background(), user.ID, current.Session.SID, "password_changed")
	require.NoError(t, err)
	require.Empty(t, rotation.RefreshToken)
	require.Equal(t, current.Session.SID, rotation.Session.SID)

	claims, err := svc.ValidateToken(rotation.AccessToken)
	require.NoError(t, err)
	require.EqualValues(t, 2, claims.SessionVersion)
	require.NoError(t, svc.ValidateBrowserSessionClaims(context.Background(), claims))

	_, err = svc.RefreshTokenPairForSession(context.Background(), other.RefreshToken, other.Session.SID)
	require.ErrorIs(t, err, service.ErrLoginSessionRevoked)
	_, err = svc.RefreshTokenPairForSession(context.Background(), current.RefreshToken, current.Session.SID)
	require.NoError(t, err)
}
