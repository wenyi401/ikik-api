package admin

import (
	"strconv"

	"ikik-api/internal/handler/dto"
	"ikik-api/internal/pkg/response"
	"ikik-api/internal/service"

	"github.com/gin-gonic/gin"
)

type CarpoolHandler struct {
	carpoolService *service.CarpoolService
	settingService *service.SettingService
}

func NewCarpoolHandler(carpoolService *service.CarpoolService, settingService *service.SettingService) *CarpoolHandler {
	return &CarpoolHandler{carpoolService: carpoolService, settingService: settingService}
}

func (h *CarpoolHandler) ensureFeatureEnabled(c *gin.Context) bool {
	if h.settingService != nil && !h.settingService.IsCarpoolEnabled(c.Request.Context()) {
		response.NotFound(c, "Carpool feature is disabled")
		return false
	}
	return true
}

func (h *CarpoolHandler) List(c *gin.Context) {
	if !h.ensureFeatureEnabled(c) {
		return
	}
	page, pageSize := response.ParsePagination(c)
	ownerUserID := parsePositiveInt64Query(c, "owner_user_id")
	result, err := h.carpoolService.AdminListPools(c.Request.Context(), service.AdminCarpoolPoolFilters{
		Page:        page,
		PageSize:    pageSize,
		Search:      c.Query("search"),
		Platform:    c.Query("platform"),
		Status:      c.Query("status"),
		OwnerUserID: ownerUserID,
	})
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}

	items := make([]dto.AdminCarpoolPoolSummary, 0, len(result.Items))
	for i := range result.Items {
		items = append(items, *dto.AdminCarpoolPoolSummaryFromService(&result.Items[i]))
	}
	response.Paginated(c, items, result.Total, page, pageSize)
}

func (h *CarpoolHandler) Get(c *gin.Context) {
	if !h.ensureFeatureEnabled(c) {
		return
	}
	poolID, ok := parseCarpoolIDParam(c)
	if !ok {
		return
	}
	detail, err := h.carpoolService.AdminGetDetail(c.Request.Context(), poolID)
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}
	response.Success(c, dto.CarpoolPoolDetailFromService(detail))
}

func (h *CarpoolHandler) Close(c *gin.Context) {
	if !h.ensureFeatureEnabled(c) {
		return
	}
	poolID, ok := parseCarpoolIDParam(c)
	if !ok {
		return
	}
	detail, err := h.carpoolService.AdminClosePool(c.Request.Context(), poolID)
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}
	response.Success(c, dto.CarpoolPoolDetailFromService(detail))
}

func (h *CarpoolHandler) Repair(c *gin.Context) {
	if !h.ensureFeatureEnabled(c) {
		return
	}
	poolID, ok := parseCarpoolIDParam(c)
	if !ok {
		return
	}
	detail, err := h.carpoolService.AdminRepairPool(c.Request.Context(), poolID)
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}
	response.Success(c, dto.CarpoolPoolDetailFromService(detail))
}

func (h *CarpoolHandler) Delete(c *gin.Context) {
	if !h.ensureFeatureEnabled(c) {
		return
	}
	poolID, ok := parseCarpoolIDParam(c)
	if !ok {
		return
	}
	if err := h.carpoolService.AdminDeletePool(c.Request.Context(), poolID); err != nil {
		response.ErrorFrom(c, err)
		return
	}
	response.Success(c, gin.H{"message": "deleted"})
}

func parseCarpoolIDParam(c *gin.Context) (int64, bool) {
	id, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil || id <= 0 {
		response.BadRequest(c, "Invalid id")
		return 0, false
	}
	return id, true
}

func parsePositiveInt64Query(c *gin.Context, name string) int64 {
	raw := c.Query(name)
	if raw == "" {
		return 0
	}
	value, err := strconv.ParseInt(raw, 10, 64)
	if err != nil || value <= 0 {
		return 0
	}
	return value
}
