package admin

import (
	"strconv"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"ikik-api/internal/pkg/pagination"
	"ikik-api/internal/pkg/response"
	"ikik-api/internal/service"
)

type ContentModerationHandler struct {
	service *service.ContentModerationService
}

func NewContentModerationHandler(svc *service.ContentModerationService) *ContentModerationHandler {
	return &ContentModerationHandler{service: svc}
}

type contentModerationConfigRequest struct {
	Enabled              *bool                                        `json:"enabled"`
	Mode                 *string                                      `json:"mode"`
	ModerationProvider   *string                                      `json:"moderation_provider"`
	BaseURL              *string                                      `json:"base_url"`
	Model                *string                                      `json:"model"`
	ClassifierGroupID    *int64                                       `json:"classifier_group_id"`
	ClassifierModels     *[]string                                    `json:"classifier_models"`
	ClassifierPrompt     *string                                      `json:"classifier_prompt"`
	AliyunRegionID       *string                                      `json:"aliyun_region_id"`
	AliyunEndpoint       *string                                      `json:"aliyun_endpoint"`
	AliyunService        *string                                      `json:"aliyun_service"`
	APIKey               *string                                      `json:"api_key"`
	APIKeys              *[]string                                    `json:"api_keys"`
	APIKeysMode          string                                       `json:"api_keys_mode"`
	DeleteAPIKeyHashes   *[]string                                    `json:"delete_api_key_hashes"`
	ClearAPIKey          bool                                         `json:"clear_api_key"`
	TimeoutMS            *int                                         `json:"timeout_ms"`
	SampleRate           *int                                         `json:"sample_rate"`
	AllGroups            *bool                                        `json:"all_groups"`
	GroupIDs             *[]int64                                     `json:"group_ids"`
	RecordNonHits        *bool                                        `json:"record_non_hits"`
	Thresholds           *map[string]float64                          `json:"thresholds"`
	WorkerCount          *int                                         `json:"worker_count"`
	QueueSize            *int                                         `json:"queue_size"`
	BlockStatus          *int                                         `json:"block_status"`
	BlockMessage         *string                                      `json:"block_message"`
	EmailOnHit           *bool                                        `json:"email_on_hit"`
	AutoBanEnabled       *bool                                        `json:"auto_ban_enabled"`
	BanThreshold         *int                                         `json:"ban_threshold"`
	ViolationWindowHours *int                                         `json:"violation_window_hours"`
	RetryCount           *int                                         `json:"retry_count"`
	HitRetentionDays     *int                                         `json:"hit_retention_days"`
	NonHitRetentionDays  *int                                         `json:"non_hit_retention_days"`
	PreHashCheckEnabled  *bool                                        `json:"pre_hash_check_enabled"`
	BlockedKeywords      *[]string                                    `json:"blocked_keywords"`
	KeywordBlockingMode  *string                                      `json:"keyword_blocking_mode"`
	ModelFilter          *service.ContentModerationModelFilter        `json:"model_filter"`
	AdaptivePolicy       *service.ContentModerationAdaptivePolicy     `json:"adaptive_policy"`
	GroupPenalty         *service.ContentModerationGroupPenaltyPolicy `json:"group_penalty"`
}

type contentModerationAPIKeyTestRequest struct {
	APIKeys            []string `json:"api_keys"`
	ModerationProvider string   `json:"moderation_provider"`
	BaseURL            string   `json:"base_url"`
	Model              string   `json:"model"`
	ClassifierGroupID  int64    `json:"classifier_group_id"`
	ClassifierModels   []string `json:"classifier_models"`
	ClassifierPrompt   *string  `json:"classifier_prompt"`
	AliyunRegionID     string   `json:"aliyun_region_id"`
	AliyunEndpoint     string   `json:"aliyun_endpoint"`
	AliyunService      string   `json:"aliyun_service"`
	TimeoutMS          int      `json:"timeout_ms"`
	Prompt             string   `json:"prompt"`
	Images             []string `json:"images"`
}

type contentModerationHashRequest struct {
	InputHash string `json:"input_hash"`
}

type contentModerationRiskProfileRequest struct {
	ManualLevel *string `json:"manual_level"`
	ResetScore  bool    `json:"reset_score"`
}

func (h *ContentModerationHandler) GetConfig(c *gin.Context) {
	cfg, err := h.service.GetConfig(c.Request.Context())
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}
	response.Success(c, cfg)
}

func (h *ContentModerationHandler) UpdateConfig(c *gin.Context) {
	var req contentModerationConfigRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, "Invalid request: "+err.Error())
		return
	}
	cfg, err := h.service.UpdateConfig(c.Request.Context(), service.UpdateContentModerationConfigInput{
		Enabled:              req.Enabled,
		Mode:                 req.Mode,
		ModerationProvider:   req.ModerationProvider,
		BaseURL:              req.BaseURL,
		Model:                req.Model,
		ClassifierGroupID:    req.ClassifierGroupID,
		ClassifierModels:     req.ClassifierModels,
		ClassifierPrompt:     req.ClassifierPrompt,
		AliyunRegionID:       req.AliyunRegionID,
		AliyunEndpoint:       req.AliyunEndpoint,
		AliyunService:        req.AliyunService,
		APIKey:               req.APIKey,
		APIKeys:              req.APIKeys,
		APIKeysMode:          req.APIKeysMode,
		DeleteAPIKeyHashes:   req.DeleteAPIKeyHashes,
		ClearAPIKey:          req.ClearAPIKey,
		TimeoutMS:            req.TimeoutMS,
		SampleRate:           req.SampleRate,
		AllGroups:            req.AllGroups,
		GroupIDs:             req.GroupIDs,
		RecordNonHits:        req.RecordNonHits,
		Thresholds:           req.Thresholds,
		WorkerCount:          req.WorkerCount,
		QueueSize:            req.QueueSize,
		BlockStatus:          req.BlockStatus,
		BlockMessage:         req.BlockMessage,
		EmailOnHit:           req.EmailOnHit,
		AutoBanEnabled:       req.AutoBanEnabled,
		BanThreshold:         req.BanThreshold,
		ViolationWindowHours: req.ViolationWindowHours,
		RetryCount:           req.RetryCount,
		HitRetentionDays:     req.HitRetentionDays,
		NonHitRetentionDays:  req.NonHitRetentionDays,
		PreHashCheckEnabled:  req.PreHashCheckEnabled,
		BlockedKeywords:      req.BlockedKeywords,
		KeywordBlockingMode:  req.KeywordBlockingMode,
		ModelFilter:          req.ModelFilter,
		AdaptivePolicy:       req.AdaptivePolicy,
		GroupPenalty:         req.GroupPenalty,
	})
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}
	response.Success(c, cfg)
}

func (h *ContentModerationHandler) ListRiskProfiles(c *gin.Context) {
	page, pageSize := response.ParsePagination(c)
	result, err := h.service.ListRiskProfiles(c.Request.Context(), service.ContentModerationRiskProfileFilter{
		Pagination: pagination.PaginationParams{
			Page:      page,
			PageSize:  pageSize,
			SortOrder: pagination.SortOrderDesc,
		},
		Level:  c.Query("level"),
		Search: c.Query("search"),
	})
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}
	response.Success(c, result)
}

func (h *ContentModerationHandler) UpdateRiskProfile(c *gin.Context) {
	userID, err := strconv.ParseInt(strings.TrimSpace(c.Param("user_id")), 10, 64)
	if err != nil || userID <= 0 {
		response.BadRequest(c, "Invalid user_id")
		return
	}
	var req contentModerationRiskProfileRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, "Invalid request: "+err.Error())
		return
	}
	result, err := h.service.UpdateRiskProfile(c.Request.Context(), userID, service.UpdateContentModerationRiskProfileInput{
		ManualLevel: req.ManualLevel,
		ResetScore:  req.ResetScore,
	})
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}
	response.Success(c, result)
}

func (h *ContentModerationHandler) ListGroupPenalties(c *gin.Context) {
	page, pageSize := response.ParsePagination(c)
	filter := service.ContentModerationGroupPenaltyFilter{
		Pagination: pagination.PaginationParams{Page: page, PageSize: pageSize, SortOrder: pagination.SortOrderDesc},
		Status:     c.Query("status"),
		Search:     c.Query("search"),
		Category:   c.Query("category"),
	}
	if raw := strings.TrimSpace(c.Query("group_id")); raw != "" {
		groupID, err := strconv.ParseInt(raw, 10, 64)
		if err != nil || groupID <= 0 {
			response.BadRequest(c, "Invalid group_id")
			return
		}
		filter.GroupID = &groupID
	}
	result, err := h.service.ListGroupPenalties(c.Request.Context(), filter)
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}
	response.Success(c, result)
}

func (h *ContentModerationHandler) ListGroupPenaltyEvents(c *gin.Context) {
	userID, groupID, ok := contentModerationPenaltyIDs(c)
	if !ok {
		return
	}
	page, pageSize := response.ParsePagination(c)
	result, err := h.service.ListGroupPenaltyEvents(c.Request.Context(), userID, groupID, pagination.PaginationParams{
		Page: page, PageSize: pageSize, SortOrder: pagination.SortOrderDesc,
	})
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}
	response.Success(c, result)
}

func (h *ContentModerationHandler) ReleaseGroupPenalty(c *gin.Context) {
	userID, groupID, ok := contentModerationPenaltyIDs(c)
	if !ok {
		return
	}
	result, err := h.service.ReleaseGroupPenalty(c.Request.Context(), userID, groupID)
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}
	response.Success(c, result)
}

func (h *ContentModerationHandler) ResetGroupPenalty(c *gin.Context) {
	userID, groupID, ok := contentModerationPenaltyIDs(c)
	if !ok {
		return
	}
	result, err := h.service.ResetGroupPenalty(c.Request.Context(), userID, groupID)
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}
	response.Success(c, result)
}

func (h *ContentModerationHandler) TestAPIKeys(c *gin.Context) {
	var req contentModerationAPIKeyTestRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, "Invalid request: "+err.Error())
		return
	}
	result, err := h.service.TestAPIKeys(c.Request.Context(), service.TestContentModerationAPIKeysInput{
		APIKeys:            req.APIKeys,
		ModerationProvider: req.ModerationProvider,
		BaseURL:            req.BaseURL,
		Model:              req.Model,
		ClassifierGroupID:  req.ClassifierGroupID,
		ClassifierModels:   req.ClassifierModels,
		ClassifierPrompt:   req.ClassifierPrompt,
		AliyunRegionID:     req.AliyunRegionID,
		AliyunEndpoint:     req.AliyunEndpoint,
		AliyunService:      req.AliyunService,
		TimeoutMS:          req.TimeoutMS,
		Prompt:             req.Prompt,
		Images:             req.Images,
	})
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}
	response.Success(c, result)
}

func (h *ContentModerationHandler) GetStatus(c *gin.Context) {
	status, err := h.service.GetStatus(c.Request.Context())
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}
	response.Success(c, status)
}

func (h *ContentModerationHandler) ListLogs(c *gin.Context) {
	page, pageSize := response.ParsePagination(c)
	filter := service.ContentModerationLogFilter{
		Pagination: pagination.PaginationParams{
			Page:      page,
			PageSize:  pageSize,
			SortOrder: pagination.SortOrderDesc,
		},
		Result:   c.Query("result"),
		Endpoint: c.Query("endpoint"),
		Search:   c.Query("search"),
	}
	if raw := strings.TrimSpace(c.Query("group_id")); raw != "" {
		groupID, err := strconv.ParseInt(raw, 10, 64)
		if err != nil || groupID <= 0 {
			response.BadRequest(c, "Invalid group_id")
			return
		}
		filter.GroupID = &groupID
	}
	if raw := strings.TrimSpace(c.Query("from")); raw != "" {
		t, _, err := parseContentModerationDate(raw)
		if err != nil {
			response.BadRequest(c, "Invalid from")
			return
		}
		filter.From = &t
	}
	if raw := strings.TrimSpace(c.Query("to")); raw != "" {
		t, dateOnly, err := parseContentModerationDate(raw)
		if err != nil {
			response.BadRequest(c, "Invalid to")
			return
		}
		if dateOnly {
			t = t.Add(24*time.Hour - time.Nanosecond)
		}
		filter.To = &t
	}
	items, pageResult, err := h.service.ListLogs(c.Request.Context(), filter)
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}
	response.Paginated(c, items, pageResult.Total, pageResult.Page, pageResult.PageSize)
}

func (h *ContentModerationHandler) GetLog(c *gin.Context) {
	logID, err := strconv.ParseInt(strings.TrimSpace(c.Param("log_id")), 10, 64)
	if err != nil || logID <= 0 {
		response.BadRequest(c, "Invalid log_id")
		return
	}
	item, err := h.service.GetLog(c.Request.Context(), logID)
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}
	response.Success(c, item)
}

func (h *ContentModerationHandler) UnbanUser(c *gin.Context) {
	userID, err := strconv.ParseInt(strings.TrimSpace(c.Param("user_id")), 10, 64)
	if err != nil || userID <= 0 {
		response.BadRequest(c, "Invalid user_id")
		return
	}
	result, err := h.service.UnbanUser(c.Request.Context(), userID)
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}
	response.Success(c, result)
}

func (h *ContentModerationHandler) DeleteFlaggedHash(c *gin.Context) {
	var req contentModerationHashRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, "Invalid request: "+err.Error())
		return
	}
	result, err := h.service.DeleteFlaggedInputHash(c.Request.Context(), req.InputHash)
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}
	response.Success(c, result)
}

func (h *ContentModerationHandler) ClearFlaggedHashes(c *gin.Context) {
	result, err := h.service.ClearFlaggedInputHashes(c.Request.Context())
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}
	response.Success(c, result)
}

func parseContentModerationDate(raw string) (time.Time, bool, error) {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return time.Time{}, false, nil
	}
	if t, err := time.Parse(time.RFC3339, raw); err == nil {
		return t, false, nil
	}
	t, err := time.Parse("2006-01-02", raw)
	return t, err == nil, err
}

func contentModerationPenaltyIDs(c *gin.Context) (int64, int64, bool) {
	userID, userErr := strconv.ParseInt(strings.TrimSpace(c.Param("user_id")), 10, 64)
	groupID, groupErr := strconv.ParseInt(strings.TrimSpace(c.Param("group_id")), 10, 64)
	if userErr != nil || groupErr != nil || userID <= 0 || groupID <= 0 {
		response.BadRequest(c, "Invalid user_id or group_id")
		return 0, 0, false
	}
	return userID, groupID, true
}
