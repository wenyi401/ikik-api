package repository

import (
	"context"
	"errors"
	"fmt"
	"time"

	dbent "ikik-api/ent"
	"ikik-api/ent/apikey"
	"ikik-api/ent/apikeygrouproute"
	"ikik-api/internal/service"
)

func (r *apiKeyRepository) createAPIKeyWithGroupRoutes(ctx context.Context, key *service.APIKey) error {
	err := r.withAPIKeyGroupRoutesTx(ctx, func(txCtx context.Context, client *dbent.Client) error {
		builder := client.APIKey.Create().
			SetUserID(key.UserID).
			SetKey(key.Key).
			SetName(key.Name).
			SetStatus(key.Status).
			SetOpenaiExperimentalPromptEnabled(key.OpenAIExperimentalPromptEnabled).
			SetNillableGroupID(key.GroupID).
			SetNillableLastUsedAt(key.LastUsedAt).
			SetQuota(key.Quota).
			SetQuotaUsed(key.QuotaUsed).
			SetNillableExpiresAt(key.ExpiresAt).
			SetRateLimit5h(key.RateLimit5h).
			SetRateLimit1d(key.RateLimit1d).
			SetRateLimit7d(key.RateLimit7d)

		if len(key.IPWhitelist) > 0 {
			builder.SetIPWhitelist(key.IPWhitelist)
		}
		if len(key.IPBlacklist) > 0 {
			builder.SetIPBlacklist(key.IPBlacklist)
		}

		created, err := builder.Save(txCtx)
		if err != nil {
			return err
		}
		key.ID = created.ID
		key.LastUsedAt = created.LastUsedAt
		key.CreatedAt = created.CreatedAt
		key.UpdatedAt = created.UpdatedAt
		return r.replaceAPIKeyGroupRoutes(txCtx, client, key.ID, key.GroupRoutes)
	})
	return translatePersistenceError(err, nil, service.ErrAPIKeyExists)
}

func (r *apiKeyRepository) updateAPIKeyWithGroupRoutes(ctx context.Context, key *service.APIKey, fields service.APIKeyUpdateFields) error {
	if fields.IsEmpty() {
		return nil
	}
	return r.withAPIKeyGroupRoutesTx(ctx, func(txCtx context.Context, client *dbent.Client) error {
		now := time.Now()
		builder := client.APIKey.Update().
			Where(apikey.IDEQ(key.ID), apikey.DeletedAtIsNil()).
			SetUpdatedAt(now)
		if fields.Name {
			builder.SetName(key.Name)
		}
		if fields.Status {
			builder.SetStatus(key.Status)
		}
		if fields.OpenAIExperimentalPrompt {
			builder.SetOpenaiExperimentalPromptEnabled(key.OpenAIExperimentalPromptEnabled)
		}
		if fields.Quota {
			builder.SetQuota(key.Quota)
		}
		if fields.QuotaUsed {
			builder.SetQuotaUsed(key.QuotaUsed)
		}
		if fields.RateLimits {
			builder.SetRateLimit5h(key.RateLimit5h).SetRateLimit1d(key.RateLimit1d).SetRateLimit7d(key.RateLimit7d)
		}
		if fields.RateLimitUsage {
			builder.SetUsage5h(key.Usage5h).SetUsage1d(key.Usage1d).SetUsage7d(key.Usage7d)
			if key.Window5hStart != nil {
				builder.SetWindow5hStart(*key.Window5hStart)
			} else {
				builder.ClearWindow5hStart()
			}
			if key.Window1dStart != nil {
				builder.SetWindow1dStart(*key.Window1dStart)
			} else {
				builder.ClearWindow1dStart()
			}
			if key.Window7dStart != nil {
				builder.SetWindow7dStart(*key.Window7dStart)
			} else {
				builder.ClearWindow7dStart()
			}
		}
		if fields.GroupID {
			if key.GroupID != nil {
				builder.SetGroupID(*key.GroupID)
			} else {
				builder.ClearGroupID()
			}
		}
		if fields.ExpiresAt {
			if key.ExpiresAt != nil {
				builder.SetExpiresAt(*key.ExpiresAt)
			} else {
				builder.ClearExpiresAt()
			}
		}
		if fields.IPRules {
			if len(key.IPWhitelist) > 0 {
				builder.SetIPWhitelist(key.IPWhitelist)
			} else {
				builder.ClearIPWhitelist()
			}
			if len(key.IPBlacklist) > 0 {
				builder.SetIPBlacklist(key.IPBlacklist)
			} else {
				builder.ClearIPBlacklist()
			}
		}

		affected, err := builder.Save(txCtx)
		if err != nil {
			return err
		}
		if affected == 0 {
			return service.ErrAPIKeyNotFound
		}
		key.UpdatedAt = now
		if fields.GroupRoutes {
			return r.replaceAPIKeyGroupRoutes(txCtx, client, key.ID, key.GroupRoutes)
		}
		return nil
	})
}

func apiKeyGroupRouteQueryOptions(query *dbent.APIKeyGroupRouteQuery) {
	query.
		WithGroup().
		Order(
			dbent.Asc(apikeygrouproute.FieldPriority),
			dbent.Desc(apikeygrouproute.FieldWeight),
			dbent.Asc(apikeygrouproute.FieldGroupID),
		)
}

func attachAPIKeyGroupRoutes(out *service.APIKey, entity *dbent.APIKey) {
	if out == nil || entity == nil || len(entity.Edges.GroupRoutes) == 0 {
		return
	}
	out.GroupRoutes = make([]service.APIKeyGroupRoute, 0, len(entity.Edges.GroupRoutes))
	for _, route := range entity.Edges.GroupRoutes {
		if route == nil {
			continue
		}
		mapped := service.APIKeyGroupRoute{
			ID:              route.ID,
			APIKeyID:        route.APIKeyID,
			GroupID:         route.GroupID,
			Priority:        route.Priority,
			Weight:          route.Weight,
			Enabled:         route.Enabled,
			CooldownSeconds: route.CooldownSeconds,
			CreatedAt:       route.CreatedAt,
			UpdatedAt:       route.UpdatedAt,
		}
		if route.Edges.Group != nil {
			mapped.Group = groupEntityToService(route.Edges.Group)
		}
		out.GroupRoutes = append(out.GroupRoutes, mapped)
	}
}

func (r *apiKeyRepository) replaceAPIKeyGroupRoutes(ctx context.Context, client *dbent.Client, apiKeyID int64, routes []service.APIKeyGroupRoute) error {
	if apiKeyID <= 0 {
		return nil
	}
	if _, err := client.APIKeyGroupRoute.Delete().
		Where(apikeygrouproute.APIKeyIDEQ(apiKeyID)).
		Exec(ctx); err != nil {
		return err
	}
	if len(routes) == 0 {
		return nil
	}

	builders := make([]*dbent.APIKeyGroupRouteCreate, 0, len(routes))
	for _, route := range routes {
		if route.GroupID <= 0 {
			continue
		}
		priority := route.Priority
		if priority <= 0 {
			priority = 100
		}
		weight := route.Weight
		if weight <= 0 {
			weight = 1
		}
		cooldownSeconds := route.CooldownSeconds
		if cooldownSeconds <= 0 {
			cooldownSeconds = 30
		}
		builders = append(builders, client.APIKeyGroupRoute.Create().
			SetAPIKeyID(apiKeyID).
			SetGroupID(route.GroupID).
			SetPriority(priority).
			SetWeight(weight).
			SetEnabled(route.Enabled).
			SetCooldownSeconds(cooldownSeconds))
	}
	if len(builders) == 0 {
		return nil
	}
	return client.APIKeyGroupRoute.CreateBulk(builders...).Exec(ctx)
}

func (r *apiKeyRepository) clearAPIKeyGroupBindings(ctx context.Context, groupID int64) (int64, error) {
	var affected int
	err := r.withAPIKeyGroupRoutesTx(ctx, func(txCtx context.Context, client *dbent.Client) error {
		var err error
		affected, err = client.APIKey.Update().
			Where(apikey.GroupIDEQ(groupID), apikey.DeletedAtIsNil()).
			ClearGroupID().
			Save(txCtx)
		if err != nil {
			return err
		}
		_, err = client.APIKeyGroupRoute.Delete().
			Where(apikeygrouproute.GroupIDEQ(groupID)).
			Exec(txCtx)
		return err
	})
	return int64(affected), err
}

func (r *apiKeyRepository) moveAPIKeyGroupBindings(ctx context.Context, userID, oldGroupID, newGroupID int64) (int64, error) {
	var affected int
	err := r.withAPIKeyGroupRoutesTx(ctx, func(txCtx context.Context, client *dbent.Client) error {
		var err error
		affected, err = client.APIKey.Update().
			Where(apikey.UserIDEQ(userID), apikey.GroupIDEQ(oldGroupID), apikey.DeletedAtIsNil()).
			SetGroupID(newGroupID).
			Save(txCtx)
		if err != nil {
			return err
		}
		_, err = client.APIKeyGroupRoute.Update().
			Where(
				apikeygrouproute.GroupIDEQ(oldGroupID),
				apikeygrouproute.HasAPIKeyWith(apikey.UserIDEQ(userID), apikey.DeletedAtIsNil()),
			).
			SetGroupID(newGroupID).
			Save(txCtx)
		return err
	})
	return int64(affected), err
}

func (r *apiKeyRepository) withAPIKeyGroupRoutesTx(ctx context.Context, fn func(context.Context, *dbent.Client) error) error {
	if tx := dbent.TxFromContext(ctx); tx != nil {
		return fn(ctx, tx.Client())
	}
	tx, err := r.client.Tx(ctx)
	if errors.Is(err, dbent.ErrTxStarted) {
		return fn(ctx, r.client)
	}
	if err != nil {
		return fmt.Errorf("begin api key transaction: %w", err)
	}
	defer func() { _ = tx.Rollback() }()

	txCtx := dbent.NewTxContext(ctx, tx)
	if err := fn(txCtx, tx.Client()); err != nil {
		return err
	}
	if err := tx.Commit(); err != nil {
		return fmt.Errorf("commit api key transaction: %w", err)
	}
	return nil
}
