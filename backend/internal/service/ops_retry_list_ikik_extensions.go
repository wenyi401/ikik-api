package service

import (
	"context"
	"database/sql"

	"errors"

	infraerrors "ikik-api/internal/pkg/errors"
)

func (s *OpsService) ListRetryAttemptsByErrorID(ctx context.Context, errorID int64, limit int) ([]*OpsRetryAttempt, error) {
	if err := s.RequireMonitoringEnabled(ctx); err != nil {
		return nil, err
	}
	if s.opsRepo == nil {
		return nil, infraerrors.ServiceUnavailable("OPS_REPO_UNAVAILABLE", "Ops repository not available")
	}
	if errorID <= 0 {
		return nil, infraerrors.BadRequest("OPS_ERROR_INVALID_ID", "invalid error id")
	}
	items, err := s.opsRepo.ListRetryAttemptsByErrorID(ctx, errorID, limit)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return []*OpsRetryAttempt{}, nil
		}
		return nil, infraerrors.InternalServer("OPS_RETRY_LIST_FAILED", "Failed to list retry attempts").WithCause(err)
	}
	return items, nil
}
