package securityaudit

import (
	"context"
	"errors"
	"strconv"
	"strings"

	infraerrors "ikik-api/internal/pkg/errors"
	"ikik-api/internal/pkg/response"
	"ikik-api/internal/server/middleware"

	"github.com/gin-gonic/gin"
	"ikik-api/internal/riskengine"
)

type PromptAdminService interface {
	GetConfig() (PublicConfig, error)
	SaveConfig(context.Context, UpdateConfigRequest, int64) (PublicConfig, error)
	Probe(context.Context, ProbeRequest) ProbeResult
	Runtime(context.Context) RuntimeSnapshot
	ListEvents(context.Context, EventFilter, int, int) (*EventPage, error)
	GetEvent(context.Context, int64) (*Event, error)
	DeleteEvent(context.Context, int64) (*DeleteResult, error)
	DeleteEventsByIDs(context.Context, []int64) (*DeleteResult, error)
	PreviewDelete(context.Context, EventFilter, int64) (*DeletePreview, error)
	DeleteByFilter(context.Context, DeleteByFilterRequest, int64) (*DeleteResult, error)
	ListPromptAuditUserProfiles(context.Context, int, int, bool, string) (*PromptAuditUserProfilePage, error)
	UnblockPromptAuditUser(context.Context, int64) (*PromptAuditUserProfile, error)
}

type PromptAdminHandler struct{ service PromptAdminService }

type PromptKnowledgeAdminService interface {
	KnowledgeSummary(context.Context) (*riskengine.KnowledgeSummary, error)
	ListKnowledge(context.Context, riskengine.KnowledgeFilter, int, int) (*riskengine.KnowledgeEntryPage, error)
	CreateKnowledge(context.Context, riskengine.KnowledgeWriteInput, int64) (*riskengine.KnowledgeEntry, int, error)
	UpdateKnowledge(context.Context, int64, riskengine.KnowledgeWriteInput, int64) (*riskengine.KnowledgeEntry, int, error)
	ListKnowledgeObservations(context.Context, riskengine.ObservationFilter, int, int) (*riskengine.ObservationPage, error)
	ReviewKnowledgeObservation(context.Context, int64, riskengine.ObservationReviewInput, int64) (*riskengine.Observation, error)
}

type PromptAuditTestRequest struct {
	Prompt string `json:"prompt"`
}

func NewPromptAdminHandler(service PromptAdminService) *PromptAdminHandler {
	return &PromptAdminHandler{service: service}
}

func (h *PromptAdminHandler) GetConfig(c *gin.Context) {
	config, err := h.service.GetConfig()
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}
	response.Success(c, config)
}

func (h *PromptAdminHandler) UpdateConfig(c *gin.Context) {
	var request UpdateConfigRequest
	if err := c.ShouldBindJSON(&request); err != nil {
		setPromptAdminAudit(c, "failed", "prompt_audit_invalid_config_request", nil)
		response.ErrorFrom(c, infraerrors.BadRequest("prompt_audit_invalid_config_request", "提示词审计配置请求无效"))
		return
	}
	config, err := h.service.SaveConfig(c.Request.Context(), request, adminID(c))
	if err != nil {
		setPromptAdminAudit(c, "failed", infraerrors.Reason(err), configAuditFields(request, nil))
		response.ErrorFrom(c, err)
		return
	}
	setPromptAdminAudit(c, "success", "", configAuditFields(request, &config))
	response.Success(c, config)
}

func (h *PromptAdminHandler) ProbeEndpoint(c *gin.Context) {
	var request ProbeRequest
	if err := c.ShouldBindJSON(&request); err != nil {
		setPromptAdminAudit(c, "failed", "prompt_audit_invalid_probe_request", nil)
		response.ErrorFrom(c, infraerrors.BadRequest("prompt_audit_invalid_probe_request", "审计节点探测请求无效"))
		return
	}
	result := h.service.Probe(c.Request.Context(), request)
	status := "failed"
	if result.OK {
		status = "success"
	}
	setPromptAdminAudit(c, status, result.ErrorCode, map[string]any{
		"guard_endpoint_id": request.Endpoint.ID, "http_status": result.HTTPStatus,
		"latency_ms": result.LatencyMS, "token_applied": result.TokenApplied, "retryable": result.Retryable,
	})
	response.Success(c, result)
}

func (h *PromptAdminHandler) TestPrompt(c *gin.Context) {
	var request PromptAuditTestRequest
	if err := c.ShouldBindJSON(&request); err != nil || strings.TrimSpace(request.Prompt) == "" {
		response.ErrorFrom(c, infraerrors.BadRequest("prompt_audit_test_prompt_required", "请输入需要测试的提示词"))
		return
	}
	tester, ok := h.service.(interface {
		TestPrompt(context.Context, string) (*PromptAuditTestResult, error)
	})
	if !ok {
		response.ErrorFrom(c, errors.New("prompt audit test service unavailable"))
		return
	}
	result, err := tester.TestPrompt(c.Request.Context(), request.Prompt)
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}
	setPromptAdminAudit(c, "success", "", map[string]any{
		"decision": result.Result.Decision, "chunk_total": result.ChunkTotal, "latency_ms": result.LatencyMS,
	})
	response.Success(c, result)
}

func (h *PromptAdminHandler) GetRuntime(c *gin.Context) {
	response.Success(c, h.service.Runtime(c.Request.Context()))
}

func (h *PromptAdminHandler) GetKnowledgeSummary(c *gin.Context) {
	service, ok := h.service.(PromptKnowledgeAdminService)
	if !ok {
		response.ErrorFrom(c, errors.New("risk knowledge service unavailable"))
		return
	}
	result, err := service.KnowledgeSummary(c.Request.Context())
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}
	response.Success(c, result)
}

func (h *PromptAdminHandler) ListKnowledge(c *gin.Context) {
	service, ok := h.service.(PromptKnowledgeAdminService)
	if !ok {
		response.ErrorFrom(c, errors.New("risk knowledge service unavailable"))
		return
	}
	page, pageSize, ok := promptAdminPagination(c)
	if !ok {
		return
	}
	filter, err := knowledgeFilterFromQuery(c)
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}
	if err := riskengine.ValidateKnowledgeFilter(filter); err != nil {
		response.ErrorFrom(c, infraerrors.BadRequest("risk_knowledge_invalid_filter", "知识样本筛选无效"))
		return
	}
	result, err := service.ListKnowledge(c.Request.Context(), filter, page, pageSize)
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}
	response.Success(c, result)
}

func (h *PromptAdminHandler) CreateKnowledge(c *gin.Context) {
	service, ok := h.service.(PromptKnowledgeAdminService)
	if !ok {
		response.ErrorFrom(c, errors.New("risk knowledge service unavailable"))
		return
	}
	var input riskengine.KnowledgeWriteInput
	if err := c.ShouldBindJSON(&input); err != nil {
		response.ErrorFrom(c, infraerrors.BadRequest("risk_knowledge_invalid_request", "知识样本请求无效"))
		return
	}
	input, err := riskengine.NormalizeKnowledgeInput(input)
	if err != nil {
		response.ErrorFrom(c, infraerrors.BadRequest("risk_knowledge_invalid_entry", "知识样本内容无效"))
		return
	}
	entry, version, err := service.CreateKnowledge(c.Request.Context(), input, adminID(c))
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}
	setPromptAdminAudit(c, "success", "", map[string]any{"knowledge_entry_id": entry.ID, "knowledge_version": version})
	response.Success(c, gin.H{"entry": entry, "version": version})
}

func (h *PromptAdminHandler) UpdateKnowledge(c *gin.Context) {
	service, ok := h.service.(PromptKnowledgeAdminService)
	if !ok {
		response.ErrorFrom(c, errors.New("risk knowledge service unavailable"))
		return
	}
	id, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil || id <= 0 {
		response.ErrorFrom(c, infraerrors.BadRequest("risk_knowledge_invalid_id", "知识样本 ID 无效"))
		return
	}
	var input riskengine.KnowledgeWriteInput
	if err := c.ShouldBindJSON(&input); err != nil {
		response.ErrorFrom(c, infraerrors.BadRequest("risk_knowledge_invalid_request", "知识样本请求无效"))
		return
	}
	input, err = riskengine.NormalizeKnowledgeInput(input)
	if err != nil {
		response.ErrorFrom(c, infraerrors.BadRequest("risk_knowledge_invalid_entry", "知识样本内容无效"))
		return
	}
	entry, version, err := service.UpdateKnowledge(c.Request.Context(), id, input, adminID(c))
	if errors.Is(err, riskengine.ErrKnowledgeEntryNotFound) {
		response.ErrorFrom(c, infraerrors.NotFound("risk_knowledge_not_found", "知识样本不存在"))
		return
	}
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}
	setPromptAdminAudit(c, "success", "", map[string]any{"knowledge_entry_id": entry.ID, "knowledge_version": version})
	response.Success(c, gin.H{"entry": entry, "version": version})
}

func (h *PromptAdminHandler) ListKnowledgeObservations(c *gin.Context) {
	service, ok := h.service.(PromptKnowledgeAdminService)
	if !ok {
		response.ErrorFrom(c, errors.New("risk knowledge service unavailable"))
		return
	}
	page, pageSize, ok := promptAdminPagination(c)
	if !ok {
		return
	}
	filter, err := observationFilterFromQuery(c)
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}
	if err := riskengine.ValidateObservationFilter(filter); err != nil {
		response.ErrorFrom(c, infraerrors.BadRequest("risk_observation_invalid_filter", "影子观察筛选无效"))
		return
	}
	result, err := service.ListKnowledgeObservations(c.Request.Context(), filter, page, pageSize)
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}
	response.Success(c, result)
}

func (h *PromptAdminHandler) ReviewKnowledgeObservation(c *gin.Context) {
	service, ok := h.service.(PromptKnowledgeAdminService)
	if !ok {
		response.ErrorFrom(c, errors.New("risk knowledge service unavailable"))
		return
	}
	id, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil || id <= 0 {
		response.ErrorFrom(c, infraerrors.BadRequest("risk_observation_invalid_id", "影子观察 ID 无效"))
		return
	}
	var input riskengine.ObservationReviewInput
	if err := c.ShouldBindJSON(&input); err != nil {
		response.ErrorFrom(c, infraerrors.BadRequest("risk_observation_invalid_review", "影子观察复核请求无效"))
		return
	}
	if err := riskengine.ValidateObservationReviewInput(input); err != nil {
		response.ErrorFrom(c, infraerrors.BadRequest("risk_observation_invalid_review", "影子观察复核内容无效"))
		return
	}
	item, err := service.ReviewKnowledgeObservation(c.Request.Context(), id, input, adminID(c))
	if errors.Is(err, riskengine.ErrObservationNotFound) {
		response.ErrorFrom(c, infraerrors.NotFound("risk_observation_not_found", "影子观察不存在"))
		return
	}
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}
	setPromptAdminAudit(c, "success", "", map[string]any{"observation_id": id, "review_status": input.Status})
	response.Success(c, item)
}

func (h *PromptAdminHandler) ListEvents(c *gin.Context) {
	page, err := positiveIntQuery(c, "page", 1, 0)
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}
	pageSize, err := positiveIntQuery(c, "page_size", 20, 100)
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}
	filter, err := eventFilterFromQuery(c)
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}
	result, err := h.service.ListEvents(c.Request.Context(), filter, page, pageSize)
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}
	response.Success(c, result)
}

func (h *PromptAdminHandler) ListProfiles(c *gin.Context) {
	page, err := positiveIntQuery(c, "page", 1, 0)
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}
	pageSize, err := positiveIntQuery(c, "page_size", 20, 100)
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}
	result, err := h.service.ListPromptAuditUserProfiles(c.Request.Context(), page, pageSize, c.Query("blocked_only") == "true", c.Query("keyword"))
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}
	response.Success(c, result)
}

func (h *PromptAdminHandler) UnblockProfile(c *gin.Context) {
	userID, err := strconv.ParseInt(c.Param("user_id"), 10, 64)
	if err != nil || userID <= 0 {
		response.ErrorFrom(c, infraerrors.BadRequest("prompt_audit_invalid_user_id", "用户 ID 无效"))
		return
	}
	profile, err := h.service.UnblockPromptAuditUser(c.Request.Context(), userID)
	if errors.Is(err, ErrPromptAuditProfileNotFound) {
		response.ErrorFrom(c, infraerrors.NotFound("prompt_audit_profile_not_found", "提示词审计用户标记不存在"))
		return
	}
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}
	setPromptAdminAudit(c, "success", "", map[string]any{"target_user_id": userID, "action": "unblock_prompt_audit_user"})
	response.Success(c, profile)
}

func (h *PromptAdminHandler) GetEvent(c *gin.Context) {
	id, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil || id <= 0 {
		response.ErrorFrom(c, infraerrors.BadRequest("prompt_audit_invalid_event_id", "事件 ID 无效"))
		return
	}
	event, err := h.service.GetEvent(c.Request.Context(), id)
	if errors.Is(err, ErrEventNotFound) {
		response.ErrorFrom(c, infraerrors.NotFound("prompt_audit_event_not_found", "提示词审计事件不存在"))
		return
	}
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}
	response.Success(c, event)
}

func (h *PromptAdminHandler) DeleteEvent(c *gin.Context) {
	id, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil || id <= 0 {
		setPromptAdminAudit(c, "failed", "prompt_audit_invalid_event_id", nil)
		response.ErrorFrom(c, infraerrors.BadRequest("prompt_audit_invalid_event_id", "事件 ID 无效"))
		return
	}
	result, err := h.service.DeleteEvent(c.Request.Context(), id)
	if err != nil {
		setPromptAdminAudit(c, "failed", infraerrors.Reason(err), map[string]any{"event_id": id})
		response.ErrorFrom(c, err)
		return
	}
	setPromptAdminAudit(c, "success", "", deleteAuditFields(result, map[string]any{"event_id": id}))
	LogWarn(EventEventDeleted, map[string]any{"user_id": adminID(c), "event_id": id, "status": "deleted"})
	response.Success(c, result)
}

type batchDeleteRequest struct {
	IDs []int64 `json:"ids" binding:"required"`
}

func (h *PromptAdminHandler) BatchDelete(c *gin.Context) {
	var request batchDeleteRequest
	if err := c.ShouldBindJSON(&request); err != nil || len(request.IDs) == 0 || len(request.IDs) > 500 {
		setPromptAdminAudit(c, "failed", "prompt_audit_invalid_delete_batch", nil)
		response.ErrorFrom(c, infraerrors.BadRequest("prompt_audit_invalid_delete_batch", "批量删除必须包含 1-500 个事件 ID"))
		return
	}
	for _, id := range request.IDs {
		if id <= 0 {
			setPromptAdminAudit(c, "failed", "prompt_audit_invalid_event_id", map[string]any{"requested_count": len(request.IDs)})
			response.ErrorFrom(c, infraerrors.BadRequest("prompt_audit_invalid_event_id", "事件 ID 无效"))
			return
		}
	}
	result, err := h.service.DeleteEventsByIDs(c.Request.Context(), request.IDs)
	if err != nil {
		setPromptAdminAudit(c, "failed", infraerrors.Reason(err), map[string]any{"requested_count": len(request.IDs)})
		response.ErrorFrom(c, err)
		return
	}
	setPromptAdminAudit(c, "success", "", deleteAuditFields(result, map[string]any{"requested_count": len(request.IDs)}))
	LogWarn(EventEventsDeleted, map[string]any{"user_id": adminID(c), "status": "deleted"})
	response.Success(c, result)
}

func (h *PromptAdminHandler) DeletePreview(c *gin.Context) {
	var filter EventFilter
	if err := c.ShouldBindJSON(&filter); err != nil {
		setPromptAdminAudit(c, "failed", "prompt_audit_delete_preview_invalid", nil)
		response.ErrorFrom(c, infraerrors.BadRequest("prompt_audit_delete_preview_invalid", "删除预览筛选无效"))
		return
	}
	preview, err := h.service.PreviewDelete(c.Request.Context(), filter, adminID(c))
	if err != nil {
		setPromptAdminAudit(c, "failed", "prompt_audit_delete_preview_invalid", nil)
		response.ErrorFrom(c, infraerrors.BadRequest("prompt_audit_delete_preview_invalid", "删除预览筛选无效"))
		return
	}
	setPromptAdminAudit(c, "success", "", map[string]any{
		"matched_count": preview.MatchedCount, "snapshot_max_id": preview.SnapshotMaxID, "filter_hash": preview.FilterHash,
	})
	response.Success(c, preview)
}

func (h *PromptAdminHandler) DeleteByFilter(c *gin.Context) {
	var request DeleteByFilterRequest
	if err := c.ShouldBindJSON(&request); err != nil {
		setPromptAdminAudit(c, "failed", "prompt_audit_delete_confirmation_invalid", nil)
		response.ErrorFrom(c, infraerrors.BadRequest("prompt_audit_delete_confirmation_invalid", "删除确认无效或已过期"))
		return
	}
	result, err := h.service.DeleteByFilter(c.Request.Context(), request, adminID(c))
	if err != nil {
		setPromptAdminAudit(c, "failed", "prompt_audit_delete_confirmation_invalid", map[string]any{
			"snapshot_max_id": request.SnapshotMaxID, "filter_hash": request.FilterHash, "confirm": request.Confirm,
		})
		response.ErrorFrom(c, infraerrors.BadRequest("prompt_audit_delete_confirmation_invalid", "删除确认无效或已过期"))
		return
	}
	setPromptAdminAudit(c, "success", "", deleteAuditFields(result, map[string]any{
		"snapshot_max_id": request.SnapshotMaxID, "filter_hash": request.FilterHash, "confirm": request.Confirm,
	}))
	response.Success(c, result)
}

func setPromptAdminAudit(c *gin.Context, result, errorCode string, fields map[string]any) {
	details := make(map[string]any, len(fields)+2)
	details["result"] = result
	if strings.TrimSpace(errorCode) != "" {
		details["error_code"] = errorCode
	}
	for key, value := range fields {
		details[key] = value
	}
	middleware.SetAuditExtra(c, details)
}

func configAuditFields(request UpdateConfigRequest, saved *PublicConfig) map[string]any {
	version := request.ExpectedConfigVersion
	if saved != nil {
		version = saved.ConfigVersion
	}
	return map[string]any{
		"enabled": request.Enabled, "blocking_enabled": request.BlockingEnabled,
		"blocking_latest_turn_only": request.BlockingLatestTurnOnly,
		"async_latest_user_only":    request.AsyncLatestUserOnly,
		"enforcement_mode":          request.EnforcementMode,
		"config_version":            version, "endpoint_count": len(request.Endpoints),
		"scanner_count": len(request.Scanners), "all_groups": request.AllGroups,
		"group_count": len(request.GroupIDs),
	}
}

func deleteAuditFields(result *DeleteResult, base map[string]any) map[string]any {
	fields := make(map[string]any, len(base)+2)
	for key, value := range base {
		fields[key] = value
	}
	if result != nil {
		fields["deleted_events"] = result.DeletedEvents
		fields["deleted_jobs"] = result.DeletedJobs
	}
	return fields
}

func adminID(c *gin.Context) int64 {
	subject, ok := middleware.GetAuthSubjectFromContext(c)
	if !ok {
		return 0
	}
	return subject.UserID
}

func eventFilterFromQuery(c *gin.Context) (EventFilter, error) {
	groupID, err := optionalPositiveInt64Query(c, "group_id")
	if err != nil {
		return EventFilter{}, err
	}
	userID, err := optionalPositiveInt64Query(c, "user_id")
	if err != nil {
		return EventFilter{}, err
	}
	apiKeyID, err := optionalPositiveInt64Query(c, "api_key_id")
	if err != nil {
		return EventFilter{}, err
	}
	filter := EventFilter{
		Decision: c.Query("decision"), RiskLevel: c.Query("risk_level"), Endpoint: c.Query("endpoint"),
		GroupID: groupID, UserID: userID, APIKeyID: apiKeyID, RequestID: c.Query("request_id"),
		PromptHash: c.Query("prompt_hash"), Keyword: c.Query("keyword"),
	}
	if value := strings.TrimSpace(c.Query("start_at")); value != "" {
		filter.StartAt = parseTimeQuery(value)
		if filter.StartAt == nil {
			return EventFilter{}, infraerrors.BadRequest("prompt_audit_invalid_time", "开始时间无效")
		}
	}
	if value := strings.TrimSpace(c.Query("end_at")); value != "" {
		filter.EndAt = parseTimeQuery(value)
		if filter.EndAt == nil {
			return EventFilter{}, infraerrors.BadRequest("prompt_audit_invalid_time", "结束时间无效")
		}
	}
	return filter, nil
}

func optionalPositiveInt64Query(c *gin.Context, key string) (*int64, error) {
	value := strings.TrimSpace(c.Query(key))
	if value == "" {
		return nil, nil
	}
	parsed, err := strconv.ParseInt(value, 10, 64)
	if err != nil || parsed <= 0 {
		return nil, infraerrors.BadRequest("prompt_audit_invalid_filter_id", "事件筛选 ID 无效")
	}
	return &parsed, nil
}

func promptAdminPagination(c *gin.Context) (int, int, bool) {
	page, err := positiveIntQuery(c, "page", 1, 0)
	if err != nil {
		response.ErrorFrom(c, err)
		return 0, 0, false
	}
	pageSize, err := positiveIntQuery(c, "page_size", 20, 100)
	if err != nil {
		response.ErrorFrom(c, err)
		return 0, 0, false
	}
	return page, pageSize, true
}

func knowledgeFilterFromQuery(c *gin.Context) (riskengine.KnowledgeFilter, error) {
	filter := riskengine.KnowledgeFilter{
		Topic: riskengine.KnowledgeTopic(c.Query("topic")), Category: riskengine.Category(c.Query("category")),
		Disposition: riskengine.KnowledgeDisposition(c.Query("disposition")), Keyword: c.Query("keyword"),
	}
	if value := strings.TrimSpace(c.Query("enabled")); value != "" {
		enabled, err := strconv.ParseBool(value)
		if err != nil {
			return filter, infraerrors.BadRequest("risk_knowledge_invalid_filter", "知识样本启用状态无效")
		}
		filter.Enabled = &enabled
	}
	return filter, nil
}

func observationFilterFromQuery(c *gin.Context) (riskengine.ObservationFilter, error) {
	userID, err := optionalPositiveInt64Query(c, "user_id")
	if err != nil {
		return riskengine.ObservationFilter{}, err
	}
	groupID, err := optionalPositiveInt64Query(c, "group_id")
	if err != nil {
		return riskengine.ObservationFilter{}, err
	}
	return riskengine.ObservationFilter{
		ReviewStatus: riskengine.ObservationReviewStatus(c.Query("review_status")),
		Category:     riskengine.Category(c.Query("category")), UserID: userID, GroupID: groupID,
		Keyword: c.Query("keyword"),
	}, nil
}

func positiveIntQuery(c *gin.Context, key string, defaultValue, maxValue int) (int, error) {
	value := strings.TrimSpace(c.Query(key))
	if value == "" {
		return defaultValue, nil
	}
	parsed, err := strconv.Atoi(value)
	if err != nil || parsed <= 0 || (maxValue > 0 && parsed > maxValue) {
		return 0, infraerrors.BadRequest("prompt_audit_invalid_pagination", "分页参数无效")
	}
	return parsed, nil
}
