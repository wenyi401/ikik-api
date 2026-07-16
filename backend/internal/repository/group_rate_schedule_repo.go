package repository

import (
	"context"
	"database/sql"
	"errors"
	"time"

	"ikik-api/internal/service"
)

const groupRateScheduleMultiplierEpsilon = 0.0000001

type groupRateScheduleRepository struct {
	db  *sql.DB
	sql sqlExecutor
}

func NewGroupRateScheduleRepository(sqlDB *sql.DB) service.GroupRateScheduleRepository {
	return &groupRateScheduleRepository{db: sqlDB, sql: sqlDB}
}

func (r *groupRateScheduleRepository) ListByGroupID(ctx context.Context, groupID int64) ([]service.GroupRateSchedule, error) {
	rows, err := r.sql.QueryContext(ctx, `
		SELECT
			s.id, s.group_id, s.target_user_id,
			COALESCE(u.username, ''), COALESCE(u.email, ''),
			s.start_minute, s.end_minute, s.rate_multiplier, s.enabled, s.created_at, s.updated_at
		FROM group_rate_schedules s
		LEFT JOIN users u ON u.id = s.target_user_id AND u.deleted_at IS NULL
		WHERE s.group_id = $1
		ORDER BY s.target_user_id NULLS FIRST, s.start_minute, s.end_minute, s.id
	`, groupID)
	if err != nil {
		return nil, err
	}
	return scanGroupRateSchedules(rows)
}

func (r *groupRateScheduleRepository) ReplaceForGroup(ctx context.Context, groupID int64, schedules []service.GroupRateScheduleInput) ([]service.GroupRateSchedule, error) {
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return nil, err
	}
	defer func() { _ = tx.Rollback() }()

	var existingGroupID int64
	if err := scanSingleRow(ctx, tx, `
		SELECT id
		FROM groups
		WHERE id = $1 AND deleted_at IS NULL
		FOR UPDATE
	`, []any{groupID}, &existingGroupID); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, service.ErrGroupNotFound
		}
		return nil, err
	}

	if _, err := tx.ExecContext(ctx, `DELETE FROM group_rate_schedules WHERE group_id = $1`, groupID); err != nil {
		return nil, err
	}

	if len(schedules) > 0 {
		now := time.Now()
		for _, schedule := range schedules {
			var targetUserID any
			if schedule.TargetUserID != nil {
				targetUserID = *schedule.TargetUserID
			}
			if _, err := tx.ExecContext(ctx, `
				INSERT INTO group_rate_schedules (
					group_id, target_user_id, start_minute, end_minute, rate_multiplier, enabled, created_at, updated_at
				)
				VALUES ($1, $2, $3, $4, $5, $6, $7, $7)
			`, groupID, targetUserID, schedule.StartMinute, schedule.EndMinute, schedule.RateMultiplier, schedule.Enabled, now); err != nil {
				return nil, err
			}
		}
	}

	rows, err := tx.QueryContext(ctx, `
		SELECT
			s.id, s.group_id, s.target_user_id,
			COALESCE(u.username, ''), COALESCE(u.email, ''),
			s.start_minute, s.end_minute, s.rate_multiplier, s.enabled, s.created_at, s.updated_at
		FROM group_rate_schedules s
		LEFT JOIN users u ON u.id = s.target_user_id AND u.deleted_at IS NULL
		WHERE s.group_id = $1
		ORDER BY s.target_user_id NULLS FIRST, s.start_minute, s.end_minute, s.id
	`, groupID)
	if err != nil {
		return nil, err
	}
	out, err := scanGroupRateSchedules(rows)
	if err != nil {
		return nil, err
	}

	if err := tx.Commit(); err != nil {
		return nil, err
	}
	return out, nil
}

func (r *groupRateScheduleRepository) ListEnabled(ctx context.Context) ([]service.GroupRateSchedule, error) {
	rows, err := r.sql.QueryContext(ctx, `
		SELECT
			s.id, s.group_id, s.target_user_id,
			COALESCE(u.username, ''), COALESCE(u.email, ''),
			s.start_minute, s.end_minute, s.rate_multiplier, s.enabled, s.created_at, s.updated_at
		FROM group_rate_schedules s
		JOIN groups g ON g.id = s.group_id AND g.deleted_at IS NULL
		LEFT JOIN users u ON u.id = s.target_user_id AND u.deleted_at IS NULL
		WHERE s.enabled = TRUE
		ORDER BY s.group_id, s.target_user_id NULLS FIRST, s.start_minute, s.end_minute, s.id
	`)
	if err != nil {
		return nil, err
	}
	return scanGroupRateSchedules(rows)
}

func (r *groupRateScheduleRepository) ListManagedGroupIDs(ctx context.Context) ([]int64, error) {
	rows, err := r.sql.QueryContext(ctx, `
		SELECT group_id
		FROM (
			SELECT DISTINCT s.group_id
			FROM group_rate_schedules s
			JOIN groups g ON g.id = s.group_id AND g.deleted_at IS NULL
			WHERE s.enabled = TRUE
			UNION
			SELECT st.group_id
			FROM group_rate_schedule_states st
			JOIN groups g ON g.id = st.group_id AND g.deleted_at IS NULL
			UNION
			SELECT ust.group_id
			FROM group_rate_schedule_user_states ust
			JOIN groups g ON g.id = ust.group_id AND g.deleted_at IS NULL
		) AS managed
		ORDER BY group_id
	`)
	if err != nil {
		return nil, err
	}
	defer func() { _ = rows.Close() }()

	var groupIDs []int64
	for rows.Next() {
		var groupID int64
		if err := rows.Scan(&groupID); err != nil {
			return nil, err
		}
		groupIDs = append(groupIDs, groupID)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	return groupIDs, nil
}

func (r *groupRateScheduleRepository) ListManagedTargetUserIDs(ctx context.Context, groupID int64) ([]int64, error) {
	rows, err := r.sql.QueryContext(ctx, `
		SELECT user_id
		FROM (
			SELECT DISTINCT s.target_user_id AS user_id
			FROM group_rate_schedules s
			JOIN users u ON u.id = s.target_user_id AND u.deleted_at IS NULL
			WHERE s.group_id = $1 AND s.enabled = TRUE AND s.target_user_id IS NOT NULL
			UNION
			SELECT ust.user_id
			FROM group_rate_schedule_user_states ust
			JOIN users u ON u.id = ust.user_id AND u.deleted_at IS NULL
			WHERE ust.group_id = $1
		) AS managed
		ORDER BY user_id
	`, groupID)
	if err != nil {
		return nil, err
	}
	defer func() { _ = rows.Close() }()

	var userIDs []int64
	for rows.Next() {
		var userID int64
		if err := rows.Scan(&userID); err != nil {
			return nil, err
		}
		userIDs = append(userIDs, userID)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	return userIDs, nil
}

func (r *groupRateScheduleRepository) ApplyScheduledMultiplier(ctx context.Context, groupID int64, scheduleID int64, rateMultiplier float64) (bool, error) {
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return false, err
	}
	defer func() { _ = tx.Rollback() }()

	var currentMultiplier float64
	if err := scanSingleRow(ctx, tx, `
		SELECT rate_multiplier
		FROM groups
		WHERE id = $1 AND deleted_at IS NULL
		FOR UPDATE
	`, []any{groupID}, &currentMultiplier); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return false, service.ErrGroupNotFound
		}
		return false, err
	}

	now := time.Now()
	if _, err := tx.ExecContext(ctx, `
		INSERT INTO group_rate_schedule_states (
			group_id, base_rate_multiplier, applied_schedule_id, created_at, updated_at
		)
		VALUES ($1, $2, $3, $4, $4)
		ON CONFLICT (group_id)
		DO UPDATE SET
			applied_schedule_id = EXCLUDED.applied_schedule_id,
			updated_at = EXCLUDED.updated_at
	`, groupID, currentMultiplier, scheduleID, now); err != nil {
		return false, err
	}

	changed := mathAbs(currentMultiplier-rateMultiplier) > groupRateScheduleMultiplierEpsilon
	if changed {
		if _, err := tx.ExecContext(ctx, `
			UPDATE groups
			SET rate_multiplier = $2, updated_at = $3
			WHERE id = $1 AND deleted_at IS NULL
		`, groupID, rateMultiplier, now); err != nil {
			return false, err
		}
		if err := enqueueSchedulerOutbox(ctx, tx, service.SchedulerOutboxEventGroupChanged, nil, &groupID, nil); err != nil {
			return false, err
		}
	}

	if err := tx.Commit(); err != nil {
		return false, err
	}
	return changed, nil
}

func (r *groupRateScheduleRepository) RestoreBaseMultiplier(ctx context.Context, groupID int64) (bool, error) {
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return false, err
	}
	defer func() { _ = tx.Rollback() }()

	var baseMultiplier float64
	if err := scanSingleRow(ctx, tx, `
		SELECT base_rate_multiplier
		FROM group_rate_schedule_states
		WHERE group_id = $1
		FOR UPDATE
	`, []any{groupID}, &baseMultiplier); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return false, nil
		}
		return false, err
	}

	var currentMultiplier float64
	if err := scanSingleRow(ctx, tx, `
		SELECT rate_multiplier
		FROM groups
		WHERE id = $1 AND deleted_at IS NULL
		FOR UPDATE
	`, []any{groupID}, &currentMultiplier); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return false, service.ErrGroupNotFound
		}
		return false, err
	}

	now := time.Now()
	changed := mathAbs(currentMultiplier-baseMultiplier) > groupRateScheduleMultiplierEpsilon
	if changed {
		if _, err := tx.ExecContext(ctx, `
			UPDATE groups
			SET rate_multiplier = $2, updated_at = $3
			WHERE id = $1 AND deleted_at IS NULL
		`, groupID, baseMultiplier, now); err != nil {
			return false, err
		}
		if err := enqueueSchedulerOutbox(ctx, tx, service.SchedulerOutboxEventGroupChanged, nil, &groupID, nil); err != nil {
			return false, err
		}
	}

	if _, err := tx.ExecContext(ctx, `DELETE FROM group_rate_schedule_states WHERE group_id = $1`, groupID); err != nil {
		return false, err
	}
	if err := tx.Commit(); err != nil {
		return false, err
	}
	return changed, nil
}

func (r *groupRateScheduleRepository) ApplyScheduledUserMultiplier(ctx context.Context, groupID int64, userID int64, scheduleID int64, rateMultiplier float64) (bool, error) {
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return false, err
	}
	defer func() { _ = tx.Rollback() }()

	var currentMultiplier sql.NullFloat64
	err = scanSingleRow(ctx, tx, `
		SELECT rate_multiplier
		FROM user_group_rate_multipliers
		WHERE group_id = $1 AND user_id = $2
		FOR UPDATE
	`, []any{groupID, userID}, &currentMultiplier)
	if err != nil && !errors.Is(err, sql.ErrNoRows) {
		return false, err
	}

	now := time.Now()
	var baseMultiplier any
	if currentMultiplier.Valid {
		baseMultiplier = currentMultiplier.Float64
	}
	if _, err := tx.ExecContext(ctx, `
		INSERT INTO group_rate_schedule_user_states (
			group_id, user_id, base_rate_multiplier, applied_schedule_id, created_at, updated_at
		)
		VALUES ($1, $2, $3, $4, $5, $5)
		ON CONFLICT (group_id, user_id)
		DO UPDATE SET
			applied_schedule_id = EXCLUDED.applied_schedule_id,
			updated_at = EXCLUDED.updated_at
	`, groupID, userID, baseMultiplier, scheduleID, now); err != nil {
		return false, err
	}

	changed := !currentMultiplier.Valid || mathAbs(currentMultiplier.Float64-rateMultiplier) > groupRateScheduleMultiplierEpsilon
	if changed {
		if _, err := tx.ExecContext(ctx, `
			INSERT INTO user_group_rate_multipliers (user_id, group_id, rate_multiplier, created_at, updated_at)
			VALUES ($1, $2, $3, $4, $4)
			ON CONFLICT (user_id, group_id)
			DO UPDATE SET
				rate_multiplier = EXCLUDED.rate_multiplier,
				updated_at = EXCLUDED.updated_at
		`, userID, groupID, rateMultiplier, now); err != nil {
			return false, err
		}
		if err := enqueueSchedulerOutbox(ctx, tx, service.SchedulerOutboxEventGroupChanged, nil, &groupID, nil); err != nil {
			return false, err
		}
	}

	if err := tx.Commit(); err != nil {
		return false, err
	}
	return changed, nil
}

func (r *groupRateScheduleRepository) RestoreBaseUserMultiplier(ctx context.Context, groupID int64, userID int64) (bool, error) {
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return false, err
	}
	defer func() { _ = tx.Rollback() }()

	var baseMultiplier sql.NullFloat64
	if err := scanSingleRow(ctx, tx, `
		SELECT base_rate_multiplier
		FROM group_rate_schedule_user_states
		WHERE group_id = $1 AND user_id = $2
		FOR UPDATE
	`, []any{groupID, userID}, &baseMultiplier); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return false, nil
		}
		return false, err
	}

	var currentMultiplier sql.NullFloat64
	err = scanSingleRow(ctx, tx, `
		SELECT rate_multiplier
		FROM user_group_rate_multipliers
		WHERE group_id = $1 AND user_id = $2
		FOR UPDATE
	`, []any{groupID, userID}, &currentMultiplier)
	if err != nil && !errors.Is(err, sql.ErrNoRows) {
		return false, err
	}

	now := time.Now()
	changed := nullFloatChanged(currentMultiplier, baseMultiplier)
	if changed {
		if baseMultiplier.Valid {
			if _, err := tx.ExecContext(ctx, `
				INSERT INTO user_group_rate_multipliers (user_id, group_id, rate_multiplier, created_at, updated_at)
				VALUES ($1, $2, $3, $4, $4)
				ON CONFLICT (user_id, group_id)
				DO UPDATE SET
					rate_multiplier = EXCLUDED.rate_multiplier,
					updated_at = EXCLUDED.updated_at
			`, userID, groupID, baseMultiplier.Float64, now); err != nil {
				return false, err
			}
		} else {
			if _, err := tx.ExecContext(ctx, `
				UPDATE user_group_rate_multipliers
				SET rate_multiplier = NULL, updated_at = $3
				WHERE group_id = $1 AND user_id = $2
			`, groupID, userID, now); err != nil {
				return false, err
			}
			if _, err := tx.ExecContext(ctx, `
				DELETE FROM user_group_rate_multipliers
				WHERE group_id = $1 AND user_id = $2 AND rate_multiplier IS NULL AND rpm_override IS NULL
			`, groupID, userID); err != nil {
				return false, err
			}
		}
		if err := enqueueSchedulerOutbox(ctx, tx, service.SchedulerOutboxEventGroupChanged, nil, &groupID, nil); err != nil {
			return false, err
		}
	}

	if _, err := tx.ExecContext(ctx, `
		DELETE FROM group_rate_schedule_user_states
		WHERE group_id = $1 AND user_id = $2
	`, groupID, userID); err != nil {
		return false, err
	}

	if err := tx.Commit(); err != nil {
		return false, err
	}
	return changed, nil
}

func scanGroupRateSchedules(rows *sql.Rows) ([]service.GroupRateSchedule, error) {
	defer func() { _ = rows.Close() }()
	var out []service.GroupRateSchedule
	for rows.Next() {
		var schedule service.GroupRateSchedule
		var targetUserID sql.NullInt64
		if err := rows.Scan(
			&schedule.ID,
			&schedule.GroupID,
			&targetUserID,
			&schedule.TargetUserName,
			&schedule.TargetUserEmail,
			&schedule.StartMinute,
			&schedule.EndMinute,
			&schedule.RateMultiplier,
			&schedule.Enabled,
			&schedule.CreatedAt,
			&schedule.UpdatedAt,
		); err != nil {
			return nil, err
		}
		if targetUserID.Valid {
			v := targetUserID.Int64
			schedule.TargetUserID = &v
		}
		out = append(out, schedule)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	return out, nil
}

func mathAbs(v float64) float64 {
	if v < 0 {
		return -v
	}
	return v
}

func nullFloatChanged(a, b sql.NullFloat64) bool {
	if a.Valid != b.Valid {
		return true
	}
	if !a.Valid {
		return false
	}
	return mathAbs(a.Float64-b.Float64) > groupRateScheduleMultiplierEpsilon
}
