package service

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"ikik-api/internal/config"
	infraerrors "ikik-api/internal/pkg/errors"
	"ikik-api/internal/pkg/logger"
	"ikik-api/internal/pkg/pagination"
)

const (
	usageCleanupWorkerName        = "usage_cleanup_worker"
	usageCleanupAutoRetentionName = "usage_cleanup_auto_retention"
)

type usageCleanupBackupCreator interface {
	CreateUsageLogsArchive(ctx context.Context, input UsageLogsArchiveInput) (*BackupRecord, error)
}

// UsageCleanupService 负责创建与执行使用记录清理任务
type UsageCleanupService struct {
	repo        UsageCleanupRepository
	timingWheel *TimingWheelService
	dashboard   *DashboardAggregationService
	backup      usageCleanupBackupCreator
	settingRepo SettingRepository
	cfg         *config.Config

	running     int32
	autoRunning int32
	startOnce   sync.Once
	stopOnce    sync.Once
	lastAutoRun atomic.Value // time.Time

	workerCtx    context.Context
	workerCancel context.CancelFunc
}

func NewUsageCleanupService(repo UsageCleanupRepository, timingWheel *TimingWheelService, dashboard *DashboardAggregationService, cfg *config.Config) *UsageCleanupService {
	return NewUsageCleanupServiceWithBackup(repo, timingWheel, dashboard, nil, nil, cfg)
}

func NewUsageCleanupServiceWithBackup(repo UsageCleanupRepository, timingWheel *TimingWheelService, dashboard *DashboardAggregationService, backup usageCleanupBackupCreator, settingRepo SettingRepository, cfg *config.Config) *UsageCleanupService {
	workerCtx, workerCancel := context.WithCancel(context.Background())
	return &UsageCleanupService{
		repo:         repo,
		timingWheel:  timingWheel,
		dashboard:    dashboard,
		backup:       backup,
		settingRepo:  settingRepo,
		cfg:          cfg,
		workerCtx:    workerCtx,
		workerCancel: workerCancel,
	}
}

func describeUsageCleanupFilters(filters UsageCleanupFilters) string {
	var parts []string
	parts = append(parts, "start="+filters.StartTime.UTC().Format(time.RFC3339))
	parts = append(parts, "end="+filters.EndTime.UTC().Format(time.RFC3339))
	if filters.UserID != nil {
		parts = append(parts, fmt.Sprintf("user_id=%d", *filters.UserID))
	}
	if filters.APIKeyID != nil {
		parts = append(parts, fmt.Sprintf("api_key_id=%d", *filters.APIKeyID))
	}
	if filters.AccountID != nil {
		parts = append(parts, fmt.Sprintf("account_id=%d", *filters.AccountID))
	}
	if filters.GroupID != nil {
		parts = append(parts, fmt.Sprintf("group_id=%d", *filters.GroupID))
	}
	if filters.Model != nil {
		parts = append(parts, "model="+strings.TrimSpace(*filters.Model))
	}
	if filters.RequestType != nil {
		parts = append(parts, "request_type="+RequestTypeFromInt16(*filters.RequestType).String())
	}
	if filters.Stream != nil {
		parts = append(parts, fmt.Sprintf("stream=%t", *filters.Stream))
	}
	if filters.BillingType != nil {
		parts = append(parts, fmt.Sprintf("billing_type=%d", *filters.BillingType))
	}
	return strings.Join(parts, " ")
}

func (s *UsageCleanupService) Start() {
	if s == nil {
		return
	}
	if s.cfg != nil && !s.cfg.UsageCleanup.Enabled {
		logger.LegacyPrintf("service.usage_cleanup", "[UsageCleanup] not started (disabled)")
		return
	}
	if s.repo == nil || s.timingWheel == nil {
		logger.LegacyPrintf("service.usage_cleanup", "[UsageCleanup] not started (missing deps)")
		return
	}

	interval := s.workerInterval()
	s.startOnce.Do(func() {
		s.timingWheel.ScheduleRecurring(usageCleanupWorkerName, interval, s.runOnce)
		logger.LegacyPrintf("service.usage_cleanup", "[UsageCleanup] started (interval=%s max_range_days=%d batch_size=%d task_timeout=%s)", interval, s.maxRangeDays(), s.batchSize(), s.taskTimeout())
		autoCfg := s.effectiveAutoRetentionConfig(context.Background())
		autoInterval := autoRetentionCheckInterval(autoCfg)
		s.timingWheel.ScheduleRecurring(usageCleanupAutoRetentionName, autoInterval, s.runAutoRetentionIfDue)
		s.timingWheel.Schedule(usageCleanupAutoRetentionName+":startup", time.Minute, s.runAutoRetentionOnce)
		logger.LegacyPrintf("service.usage_cleanup", "[UsageCleanup] auto retention watcher started (check_interval=%s run_interval=%s enabled=%t retain_days=%d window_days=%d backup_expire_days=%d)", autoInterval, autoRetentionInterval(autoCfg), autoCfg.Enabled, autoCfg.RetainDays, autoCfg.WindowDays, autoCfg.BackupExpireDays)
	})
}

func (s *UsageCleanupService) Stop() {
	if s == nil {
		return
	}
	s.stopOnce.Do(func() {
		if s.workerCancel != nil {
			s.workerCancel()
		}
		if s.timingWheel != nil {
			s.timingWheel.Cancel(usageCleanupWorkerName)
			s.timingWheel.Cancel(usageCleanupAutoRetentionName)
			s.timingWheel.Cancel(usageCleanupAutoRetentionName + ":startup")
		}
		logger.LegacyPrintf("service.usage_cleanup", "[UsageCleanup] stopped")
	})
}

func (s *UsageCleanupService) ListTasks(ctx context.Context, params pagination.PaginationParams) ([]UsageCleanupTask, *pagination.PaginationResult, error) {
	if s == nil || s.repo == nil {
		return nil, nil, fmt.Errorf("cleanup service not ready")
	}
	return s.repo.ListTasks(ctx, params)
}

func (s *UsageCleanupService) CreateTask(ctx context.Context, filters UsageCleanupFilters, createdBy int64) (*UsageCleanupTask, error) {
	if s == nil || s.repo == nil {
		return nil, fmt.Errorf("cleanup service not ready")
	}
	if s.cfg != nil && !s.cfg.UsageCleanup.Enabled {
		return nil, infraerrors.New(http.StatusServiceUnavailable, "USAGE_CLEANUP_DISABLED", "usage cleanup is disabled")
	}
	if createdBy <= 0 {
		return nil, infraerrors.BadRequest("USAGE_CLEANUP_INVALID_CREATOR", "invalid creator")
	}
	return s.createTask(ctx, filters, createdBy, true)
}

func (s *UsageCleanupService) createSystemTask(ctx context.Context, filters UsageCleanupFilters) (*UsageCleanupTask, error) {
	return s.createTask(ctx, filters, UsageCleanupSystemActor, false)
}

func (s *UsageCleanupService) createTask(ctx context.Context, filters UsageCleanupFilters, createdBy int64, runImmediately bool) (*UsageCleanupTask, error) {
	logger.LegacyPrintf("service.usage_cleanup", "[UsageCleanup] create_task requested: operator=%d %s", createdBy, describeUsageCleanupFilters(filters))
	sanitizeUsageCleanupFilters(&filters)
	if err := s.validateFilters(filters); err != nil {
		logger.LegacyPrintf("service.usage_cleanup", "[UsageCleanup] create_task rejected: operator=%d err=%v %s", createdBy, err, describeUsageCleanupFilters(filters))
		return nil, err
	}

	task := &UsageCleanupTask{
		Status:        UsageCleanupStatusPending,
		Filters:       filters,
		CreatedBy:     createdBy,
		CreatedSource: usageCleanupCreatedSource(createdBy),
	}
	if err := s.repo.CreateTask(ctx, task); err != nil {
		logger.LegacyPrintf("service.usage_cleanup", "[UsageCleanup] create_task persist failed: operator=%d err=%v %s", createdBy, err, describeUsageCleanupFilters(filters))
		return nil, fmt.Errorf("create cleanup task: %w", err)
	}
	logger.LegacyPrintf("service.usage_cleanup", "[UsageCleanup] create_task persisted: task=%d operator=%d status=%s deleted_rows=%d %s", task.ID, createdBy, task.Status, task.DeletedRows, describeUsageCleanupFilters(filters))
	if runImmediately {
		go s.runOnce()
	}
	return task, nil
}

func (s *UsageCleanupService) runAutoRetentionOnce() {
	if s == nil {
		return
	}
	autoCfg := s.effectiveAutoRetentionConfig(context.Background())
	if !s.autoRetentionEnabled(autoCfg) {
		return
	}
	if !atomic.CompareAndSwapInt32(&s.autoRunning, 0, 1) {
		logger.LegacyPrintf("service.usage_cleanup", "[UsageCleanup] auto retention skipped: already_running=true")
		return
	}
	defer atomic.StoreInt32(&s.autoRunning, 0)

	if s.repo == nil || s.dashboard == nil || s.backup == nil {
		logger.LegacyPrintf("service.usage_cleanup", "[UsageCleanup] auto retention skipped: missing dependency repo=%t dashboard=%t backup=%t", s.repo != nil, s.dashboard != nil, s.backup != nil)
		return
	}

	ctx, cancel := context.WithTimeout(context.Background(), s.autoRetentionTimeout())
	defer cancel()

	now := time.Now().UTC()
	cutoff := truncateToDayUTC(now.AddDate(0, 0, -autoCfg.RetainDays))
	oldest, err := s.repo.FindOldestUsageLogBefore(ctx, cutoff)
	if err != nil {
		logger.LegacyPrintf("service.usage_cleanup", "[UsageCleanup] auto retention oldest usage lookup failed: err=%v", err)
		return
	}
	if oldest == nil {
		logger.LegacyPrintf("service.usage_cleanup", "[UsageCleanup] auto retention skipped: no usage_logs before cutoff=%s", cutoff.Format(time.RFC3339))
		return
	}
	start := truncateToDayUTC(oldest.UTC())
	end := start.AddDate(0, 0, autoCfg.WindowDays)
	if end.After(cutoff) {
		end = cutoff
	}
	if !end.After(start) {
		logger.LegacyPrintf("service.usage_cleanup", "[UsageCleanup] auto retention skipped: invalid window start=%s end=%s", start.Format(time.RFC3339), end.Format(time.RFC3339))
		return
	}

	filters := UsageCleanupFilters{StartTime: start, EndTime: usageCleanupInclusiveEnd(end)}

	logger.LegacyPrintf("service.usage_cleanup", "[UsageCleanup] auto retention preparing: retain_days=%d start=%s end=%s", autoCfg.RetainDays, start.Format(time.RFC3339), end.Format(time.RFC3339))
	if err := s.dashboard.RecomputeRangeSync(ctx, start, end); err != nil {
		logger.LegacyPrintf("service.usage_cleanup", "[UsageCleanup] auto retention aggregation failed: err=%v", err)
		return
	}
	if err := s.repo.SnapshotUsageLogs(ctx, filters); err != nil {
		logger.LegacyPrintf("service.usage_cleanup", "[UsageCleanup] auto retention daily snapshot failed: err=%v", err)
		return
	}

	archiveStream, err := s.repo.ExportUsageLogs(ctx, filters)
	if err != nil {
		logger.LegacyPrintf("service.usage_cleanup", "[UsageCleanup] auto retention usage_logs archive export failed: err=%v", err)
		return
	}
	record, err := s.backup.CreateUsageLogsArchive(ctx, UsageLogsArchiveInput{
		Stream:     archiveStream,
		StartTime:  start,
		EndTime:    end,
		ExpireDays: autoCfg.BackupExpireDays,
	})
	if err != nil {
		logger.LegacyPrintf("service.usage_cleanup", "[UsageCleanup] auto retention usage_logs archive failed: err=%v", err)
		return
	}
	if record == nil || record.Status != "completed" {
		logger.LegacyPrintf("service.usage_cleanup", "[UsageCleanup] auto retention usage_logs archive incomplete: record=%v", record)
		return
	}

	task, err := s.createSystemTask(ctx, filters)
	if err != nil {
		logger.LegacyPrintf("service.usage_cleanup", "[UsageCleanup] auto retention create cleanup task failed: err=%v", err)
		return
	}
	s.lastAutoRun.Store(time.Now().UTC())
	logger.LegacyPrintf("service.usage_cleanup", "[UsageCleanup] auto retention cleanup task created: task=%d backup=%s start=%s end=%s", task.ID, record.ID, start.Format(time.RFC3339), end.Format(time.RFC3339))
	s.runOnce()
}

func (s *UsageCleanupService) runAutoRetentionIfDue() {
	if s == nil {
		return
	}
	autoCfg := s.effectiveAutoRetentionConfig(context.Background())
	if !s.autoRetentionEnabled(autoCfg) {
		return
	}
	lastAny := s.lastAutoRun.Load()
	if lastAny != nil {
		if last, ok := lastAny.(time.Time); ok && time.Since(last) < autoRetentionInterval(autoCfg) {
			return
		}
	}
	s.runAutoRetentionOnce()
}

func (s *UsageCleanupService) runOnce() {
	svc := s
	if svc == nil {
		return
	}
	if !atomic.CompareAndSwapInt32(&svc.running, 0, 1) {
		logger.LegacyPrintf("service.usage_cleanup", "[UsageCleanup] run_once skipped: already_running=true")
		return
	}
	defer atomic.StoreInt32(&svc.running, 0)

	parent := context.Background()
	if svc.workerCtx != nil {
		parent = svc.workerCtx
	}
	ctx, cancel := context.WithTimeout(parent, svc.taskTimeout())
	defer cancel()

	task, err := svc.repo.ClaimNextPendingTask(ctx, int64(svc.taskTimeout().Seconds()))
	if err != nil {
		logger.LegacyPrintf("service.usage_cleanup", "[UsageCleanup] claim pending task failed: %v", err)
		return
	}
	if task == nil {
		slog.Debug("[UsageCleanup] run_once done: no_task=true")
		return
	}

	logger.LegacyPrintf("service.usage_cleanup", "[UsageCleanup] task claimed: task=%d status=%s created_by=%d deleted_rows=%d %s", task.ID, task.Status, task.CreatedBy, task.DeletedRows, describeUsageCleanupFilters(task.Filters))
	svc.executeTask(ctx, task)
}

func (s *UsageCleanupService) executeTask(ctx context.Context, task *UsageCleanupTask) {
	if task == nil {
		return
	}

	batchSize := s.batchSize()
	deletedTotal := task.DeletedRows
	start := time.Now()
	logger.LegacyPrintf("service.usage_cleanup", "[UsageCleanup] task started: task=%d batch_size=%d deleted_rows=%d %s", task.ID, batchSize, deletedTotal, describeUsageCleanupFilters(task.Filters))
	var batchNum int

	for {
		if ctx != nil && ctx.Err() != nil {
			logger.LegacyPrintf("service.usage_cleanup", "[UsageCleanup] task interrupted: task=%d err=%v", task.ID, ctx.Err())
			return
		}
		canceled, err := s.isTaskCanceled(ctx, task.ID)
		if err != nil {
			s.markTaskFailed(task.ID, deletedTotal, err)
			return
		}
		if canceled {
			logger.LegacyPrintf("service.usage_cleanup", "[UsageCleanup] task canceled: task=%d deleted_rows=%d duration=%s", task.ID, deletedTotal, time.Since(start))
			return
		}

		batchNum++
		deleted, err := s.repo.DeleteUsageLogsBatch(ctx, task.Filters, batchSize)
		if err != nil {
			if errors.Is(err, context.Canceled) || errors.Is(err, context.DeadlineExceeded) {
				// 任务被中断（例如服务停止/超时），保持 running 状态，后续通过 stale reclaim 续跑。
				logger.LegacyPrintf("service.usage_cleanup", "[UsageCleanup] task interrupted: task=%d err=%v", task.ID, err)
				return
			}
			s.markTaskFailed(task.ID, deletedTotal, err)
			return
		}
		deletedTotal += deleted
		if deleted > 0 {
			updateCtx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
			if err := s.repo.UpdateTaskProgress(updateCtx, task.ID, deletedTotal); err != nil {
				logger.LegacyPrintf("service.usage_cleanup", "[UsageCleanup] task progress update failed: task=%d deleted_rows=%d err=%v", task.ID, deletedTotal, err)
			}
			cancel()
		}
		if batchNum <= 3 || batchNum%20 == 0 || deleted < int64(batchSize) {
			logger.LegacyPrintf("service.usage_cleanup", "[UsageCleanup] task batch done: task=%d batch=%d deleted=%d deleted_total=%d", task.ID, batchNum, deleted, deletedTotal)
		}
		if deleted == 0 || deleted < int64(batchSize) {
			break
		}
	}

	updateCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	if err := s.repo.MarkTaskSucceeded(updateCtx, task.ID, deletedTotal); err != nil {
		logger.LegacyPrintf("service.usage_cleanup", "[UsageCleanup] update task succeeded failed: task=%d err=%v", task.ID, err)
	} else {
		logger.LegacyPrintf("service.usage_cleanup", "[UsageCleanup] task succeeded: task=%d deleted_rows=%d duration=%s", task.ID, deletedTotal, time.Since(start))
	}

	if s.dashboard != nil && task.CreatedSource != UsageCleanupCreatedSourceAutoRetention {
		if err := s.dashboard.TriggerRecomputeRange(task.Filters.StartTime, task.Filters.EndTime); err != nil {
			logger.LegacyPrintf("service.usage_cleanup", "[UsageCleanup] trigger dashboard recompute failed: task=%d err=%v", task.ID, err)
		} else {
			logger.LegacyPrintf("service.usage_cleanup", "[UsageCleanup] trigger dashboard recompute: task=%d start=%s end=%s", task.ID, task.Filters.StartTime.UTC().Format(time.RFC3339), task.Filters.EndTime.UTC().Format(time.RFC3339))
		}
	}
}

func (s *UsageCleanupService) markTaskFailed(taskID int64, deletedRows int64, err error) {
	msg := strings.TrimSpace(err.Error())
	if len(msg) > 500 {
		msg = msg[:500]
	}
	logger.LegacyPrintf("service.usage_cleanup", "[UsageCleanup] task failed: task=%d deleted_rows=%d err=%s", taskID, deletedRows, msg)
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	if updateErr := s.repo.MarkTaskFailed(ctx, taskID, deletedRows, msg); updateErr != nil {
		logger.LegacyPrintf("service.usage_cleanup", "[UsageCleanup] update task failed failed: task=%d err=%v", taskID, updateErr)
	}
}

func (s *UsageCleanupService) isTaskCanceled(ctx context.Context, taskID int64) (bool, error) {
	if s == nil || s.repo == nil {
		return false, fmt.Errorf("cleanup service not ready")
	}
	checkCtx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	status, err := s.repo.GetTaskStatus(checkCtx, taskID)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return false, nil
		}
		return false, err
	}
	if status == UsageCleanupStatusCanceled {
		logger.LegacyPrintf("service.usage_cleanup", "[UsageCleanup] task cancel detected: task=%d", taskID)
	}
	return status == UsageCleanupStatusCanceled, nil
}

func (s *UsageCleanupService) validateFilters(filters UsageCleanupFilters) error {
	if filters.StartTime.IsZero() || filters.EndTime.IsZero() {
		return infraerrors.BadRequest("USAGE_CLEANUP_MISSING_RANGE", "start_date and end_date are required")
	}
	if filters.EndTime.Before(filters.StartTime) {
		return infraerrors.BadRequest("USAGE_CLEANUP_INVALID_RANGE", "end_date must be after start_date")
	}
	maxDays := s.maxRangeDays()
	if maxDays > 0 {
		delta := filters.EndTime.Sub(filters.StartTime)
		if delta > time.Duration(maxDays)*24*time.Hour {
			return infraerrors.BadRequest("USAGE_CLEANUP_RANGE_TOO_LARGE", fmt.Sprintf("date range exceeds %d days", maxDays))
		}
	}
	return nil
}

func (s *UsageCleanupService) CancelTask(ctx context.Context, taskID int64, canceledBy int64) error {
	if s == nil || s.repo == nil {
		return fmt.Errorf("cleanup service not ready")
	}
	if s.cfg != nil && !s.cfg.UsageCleanup.Enabled {
		return infraerrors.New(http.StatusServiceUnavailable, "USAGE_CLEANUP_DISABLED", "usage cleanup is disabled")
	}
	if canceledBy <= 0 {
		return infraerrors.BadRequest("USAGE_CLEANUP_INVALID_CANCELLER", "invalid canceller")
	}
	status, err := s.repo.GetTaskStatus(ctx, taskID)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return infraerrors.New(http.StatusNotFound, "USAGE_CLEANUP_TASK_NOT_FOUND", "cleanup task not found")
		}
		return err
	}
	logger.LegacyPrintf("service.usage_cleanup", "[UsageCleanup] cancel_task requested: task=%d operator=%d status=%s", taskID, canceledBy, status)
	if status == UsageCleanupStatusCanceled {
		logger.LegacyPrintf("service.usage_cleanup", "[UsageCleanup] cancel_task idempotent hit: task=%d operator=%d", taskID, canceledBy)
		return nil
	}
	if status != UsageCleanupStatusPending && status != UsageCleanupStatusRunning {
		return infraerrors.New(http.StatusConflict, "USAGE_CLEANUP_CANCEL_CONFLICT", "cleanup task cannot be canceled in current status")
	}
	ok, err := s.repo.CancelTask(ctx, taskID, canceledBy)
	if err != nil {
		return err
	}
	if !ok {
		// 状态可能并发改变
		currentStatus, getErr := s.repo.GetTaskStatus(ctx, taskID)
		if getErr == nil && currentStatus == UsageCleanupStatusCanceled {
			logger.LegacyPrintf("service.usage_cleanup", "[UsageCleanup] cancel_task idempotent race hit: task=%d operator=%d", taskID, canceledBy)
			return nil
		}
		return infraerrors.New(http.StatusConflict, "USAGE_CLEANUP_CANCEL_CONFLICT", "cleanup task cannot be canceled in current status")
	}
	logger.LegacyPrintf("service.usage_cleanup", "[UsageCleanup] cancel_task done: task=%d operator=%d", taskID, canceledBy)
	return nil
}

func sanitizeUsageCleanupFilters(filters *UsageCleanupFilters) {
	if filters == nil {
		return
	}
	if filters.UserID != nil && *filters.UserID <= 0 {
		filters.UserID = nil
	}
	if filters.APIKeyID != nil && *filters.APIKeyID <= 0 {
		filters.APIKeyID = nil
	}
	if filters.AccountID != nil && *filters.AccountID <= 0 {
		filters.AccountID = nil
	}
	if filters.GroupID != nil && *filters.GroupID <= 0 {
		filters.GroupID = nil
	}
	if filters.Model != nil {
		model := strings.TrimSpace(*filters.Model)
		if model == "" {
			filters.Model = nil
		} else {
			filters.Model = &model
		}
	}
	if filters.RequestType != nil {
		requestType := RequestType(*filters.RequestType)
		if !requestType.IsValid() {
			filters.RequestType = nil
		} else {
			value := int16(requestType.Normalize())
			filters.RequestType = &value
			filters.Stream = nil
		}
	}
	if filters.BillingType != nil && *filters.BillingType < 0 {
		filters.BillingType = nil
	}
}

func usageCleanupCreatedSource(createdBy int64) string {
	if createdBy == UsageCleanupSystemActor {
		return UsageCleanupCreatedSourceAutoRetention
	}
	return UsageCleanupCreatedSourceAdmin
}

func usageCleanupInclusiveEnd(exclusiveEnd time.Time) time.Time {
	return exclusiveEnd.Add(-time.Microsecond)
}

func (s *UsageCleanupService) maxRangeDays() int {
	if s == nil || s.cfg == nil {
		return 31
	}
	if s.cfg.UsageCleanup.MaxRangeDays > 0 {
		return s.cfg.UsageCleanup.MaxRangeDays
	}
	return 31
}

func (s *UsageCleanupService) batchSize() int {
	if s == nil || s.cfg == nil {
		return 5000
	}
	if s.cfg.UsageCleanup.BatchSize > 0 {
		return s.cfg.UsageCleanup.BatchSize
	}
	return 5000
}

func (s *UsageCleanupService) workerInterval() time.Duration {
	if s == nil || s.cfg == nil {
		return 10 * time.Second
	}
	if s.cfg.UsageCleanup.WorkerIntervalSeconds > 0 {
		return time.Duration(s.cfg.UsageCleanup.WorkerIntervalSeconds) * time.Second
	}
	return 10 * time.Second
}

func (s *UsageCleanupService) taskTimeout() time.Duration {
	if s == nil || s.cfg == nil {
		return 30 * time.Minute
	}
	if s.cfg.UsageCleanup.TaskTimeoutSeconds > 0 {
		return time.Duration(s.cfg.UsageCleanup.TaskTimeoutSeconds) * time.Second
	}
	return 30 * time.Minute
}

func (s *UsageCleanupService) autoRetentionEnabled(autoCfg config.UsageCleanupAutoRetentionConfig) bool {
	return s != nil && s.cfg != nil && s.cfg.UsageCleanup.Enabled && autoCfg.Enabled
}

func (s *UsageCleanupService) autoRetentionDays() int {
	if s == nil || s.cfg == nil || s.cfg.UsageCleanup.AutoRetention.RetainDays <= 0 {
		return 3
	}
	return s.cfg.UsageCleanup.AutoRetention.RetainDays
}

func (s *UsageCleanupService) autoRetentionWindowDays() int {
	if s == nil || s.cfg == nil || s.cfg.UsageCleanup.AutoRetention.WindowDays <= 0 {
		return 1
	}
	return s.cfg.UsageCleanup.AutoRetention.WindowDays
}

func (s *UsageCleanupService) autoRetentionInterval() time.Duration {
	if s == nil || s.cfg == nil || s.cfg.UsageCleanup.AutoRetention.RunIntervalHours <= 0 {
		return 24 * time.Hour
	}
	return time.Duration(s.cfg.UsageCleanup.AutoRetention.RunIntervalHours) * time.Hour
}

func autoRetentionInterval(cfg config.UsageCleanupAutoRetentionConfig) time.Duration {
	if cfg.RunIntervalHours <= 0 {
		return 24 * time.Hour
	}
	return time.Duration(cfg.RunIntervalHours) * time.Hour
}

func autoRetentionCheckInterval(cfg config.UsageCleanupAutoRetentionConfig) time.Duration {
	interval := autoRetentionInterval(cfg)
	if interval <= time.Hour {
		return interval
	}
	return time.Hour
}

func (s *UsageCleanupService) autoRetentionBackupExpireDays() int {
	if s == nil || s.cfg == nil {
		return 14
	}
	return s.cfg.UsageCleanup.AutoRetention.BackupExpireDays
}

func (s *UsageCleanupService) autoRetentionTimeout() time.Duration {
	timeout := 30*time.Minute + s.taskTimeout()
	if timeout < time.Hour {
		return time.Hour
	}
	return timeout
}

func (s *UsageCleanupService) effectiveAutoRetentionConfig(ctx context.Context) config.UsageCleanupAutoRetentionConfig {
	cfg := s.defaultAutoRetentionConfig()
	if s == nil || s.settingRepo == nil {
		return cfg
	}
	dbCtx, cancel := context.WithTimeout(ctx, 3*time.Second)
	defer cancel()
	raw, err := s.settingRepo.GetValue(dbCtx, settingKeyUsageRetention)
	if err != nil || strings.TrimSpace(raw) == "" {
		if err != nil {
			logger.LegacyPrintf("service.usage_cleanup", "[UsageCleanup] load dynamic auto retention config failed, using config file defaults: %v", err)
		}
		return cfg
	}
	var stored UsageRetentionConfig
	if err := json.Unmarshal([]byte(raw), &stored); err != nil {
		logger.LegacyPrintf("service.usage_cleanup", "[UsageCleanup] dynamic auto retention config is corrupt, using config file defaults: %v", err)
		return cfg
	}
	if err := validateUsageRetentionConfig(stored, s.maxRangeDays()); err != nil {
		logger.LegacyPrintf("service.usage_cleanup", "[UsageCleanup] dynamic auto retention config is invalid, using config file defaults: %v", err)
		return cfg
	}
	return config.UsageCleanupAutoRetentionConfig{
		Enabled:          stored.Enabled,
		RetainDays:       stored.RetainDays,
		RunIntervalHours: stored.RunIntervalHours,
		WindowDays:       stored.WindowDays,
		BackupExpireDays: stored.BackupExpireDays,
	}
}

func (s *UsageCleanupService) defaultAutoRetentionConfig() config.UsageCleanupAutoRetentionConfig {
	cfg := config.UsageCleanupAutoRetentionConfig{
		Enabled:          false,
		RetainDays:       3,
		RunIntervalHours: 24,
		WindowDays:       1,
		BackupExpireDays: 14,
	}
	if s == nil || s.cfg == nil {
		return cfg
	}
	auto := s.cfg.UsageCleanup.AutoRetention
	cfg.Enabled = auto.Enabled
	if auto.RetainDays > 0 {
		cfg.RetainDays = auto.RetainDays
	}
	if auto.RunIntervalHours > 0 {
		cfg.RunIntervalHours = auto.RunIntervalHours
	}
	if auto.WindowDays > 0 {
		cfg.WindowDays = auto.WindowDays
	}
	if auto.BackupExpireDays >= 0 {
		cfg.BackupExpireDays = auto.BackupExpireDays
	}
	return cfg
}
