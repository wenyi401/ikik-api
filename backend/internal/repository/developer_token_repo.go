package repository

import (
	"context"
	"time"

	dbent "ikik-api/ent"
	"ikik-api/ent/developertoken"
	"ikik-api/internal/service"
)

type developerTokenRepository struct {
	client *dbent.Client
}

func NewDeveloperTokenRepository(client *dbent.Client) service.DeveloperTokenRepository {
	return &developerTokenRepository{client: client}
}

func (r *developerTokenRepository) Create(ctx context.Context, token *service.DeveloperToken) error {
	created, err := r.client.DeveloperToken.Create().
		SetUserID(token.UserID).
		SetName(token.Name).
		SetTokenPrefix(token.TokenPrefix).
		SetTokenHash(token.TokenHash).
		SetScopes(token.Scopes).
		SetStatus(token.Status).
		SetNillableExpiresAt(token.ExpiresAt).
		Save(ctx)
	if err != nil {
		return err
	}
	applyDeveloperTokenEntity(token, created)
	return nil
}

func (r *developerTokenRepository) ListByUserID(ctx context.Context, userID int64) ([]service.DeveloperToken, error) {
	rows, err := r.client.DeveloperToken.Query().
		Where(developertoken.UserIDEQ(userID)).
		Order(dbent.Desc(developertoken.FieldCreatedAt)).
		All(ctx)
	if err != nil {
		return nil, err
	}
	out := make([]service.DeveloperToken, 0, len(rows))
	for _, row := range rows {
		out = append(out, *developerTokenEntityToService(row))
	}
	return out, nil
}

func (r *developerTokenRepository) CountByUserID(ctx context.Context, userID int64) (int, error) {
	return r.client.DeveloperToken.Query().Where(developertoken.UserIDEQ(userID)).Count(ctx)
}

func (r *developerTokenRepository) GetByTokenHash(ctx context.Context, tokenHash string) (*service.DeveloperToken, error) {
	row, err := r.client.DeveloperToken.Query().
		Where(developertoken.TokenHashEQ(tokenHash)).
		WithUser().
		Only(ctx)
	if err != nil {
		if dbent.IsNotFound(err) {
			return nil, service.ErrDeveloperTokenNotFound
		}
		return nil, err
	}
	out := developerTokenEntityToService(row)
	out.User = userEntityToService(row.Edges.User)
	return out, nil
}

func (r *developerTokenRepository) DeleteOwned(ctx context.Context, userID, tokenID int64) error {
	row, err := r.client.DeveloperToken.Query().
		Where(developertoken.IDEQ(tokenID), developertoken.UserIDEQ(userID)).
		Only(ctx)
	if err != nil {
		if dbent.IsNotFound(err) {
			return service.ErrDeveloperTokenNotFound
		}
		return err
	}
	return r.client.DeveloperToken.DeleteOne(row).Exec(ctx)
}

func (r *developerTokenRepository) Touch(ctx context.Context, tokenID int64, usedAt time.Time, ip string) error {
	update := r.client.DeveloperToken.Update().
		Where(
			developertoken.IDEQ(tokenID),
			developertoken.Or(
				developertoken.LastUsedAtIsNil(),
				developertoken.LastUsedAtLT(usedAt.Add(-time.Minute)),
			),
		).
		SetLastUsedAt(usedAt)
	if ip != "" {
		update = update.SetLastUsedIP(ip)
	}
	_, err := update.Save(ctx)
	return err
}

func developerTokenEntityToService(row *dbent.DeveloperToken) *service.DeveloperToken {
	if row == nil {
		return nil
	}
	return &service.DeveloperToken{
		ID:          row.ID,
		UserID:      row.UserID,
		Name:        row.Name,
		TokenPrefix: row.TokenPrefix,
		TokenHash:   row.TokenHash,
		Scopes:      append([]string(nil), row.Scopes...),
		Status:      row.Status,
		ExpiresAt:   row.ExpiresAt,
		LastUsedAt:  row.LastUsedAt,
		LastUsedIP:  row.LastUsedIP,
		CreatedAt:   row.CreatedAt,
		UpdatedAt:   row.UpdatedAt,
	}
}

func applyDeveloperTokenEntity(dst *service.DeveloperToken, src *dbent.DeveloperToken) {
	if dst == nil || src == nil {
		return
	}
	converted := developerTokenEntityToService(src)
	*dst = *converted
}

var _ service.DeveloperTokenRepository = (*developerTokenRepository)(nil)
