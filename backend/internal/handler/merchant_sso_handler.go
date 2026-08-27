package handler

import (
	"strconv"
	"strings"

	"ikik-api/internal/pkg/response"
	middleware2 "ikik-api/internal/server/middleware"
	"ikik-api/internal/service"

	"github.com/gin-gonic/gin"
)

// MerchantSSOHandler exposes the user launch flow and admin configuration
// endpoints for the dynamic_api merchant integration.
type MerchantSSOHandler struct {
	service *service.MerchantSSOService
}

func NewMerchantSSOHandler(svc *service.MerchantSSOService) *MerchantSSOHandler {
	return &MerchantSSOHandler{service: svc}
}

func (h *MerchantSSOHandler) ListUserIntegrations(c *gin.Context) {
	items, err := h.service.ListIntegrations(c.Request.Context(), true)
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}
	response.Success(c, gin.H{"integrations": items})
}

func (h *MerchantSSOHandler) Login(c *gin.Context) {
	subject, ok := middleware2.GetAuthSubjectFromContext(c)
	if !ok {
		response.Unauthorized(c, "User not authenticated")
		return
	}
	result, err := h.service.StartLogin(c.Request.Context(), c.Param("merchant_code"), subject.UserID)
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}
	response.Success(c, result)
}

type merchantSSOIntegrationRequest struct {
	MerchantCode         string   `json:"merchant_code"`
	MerchantName         string   `json:"merchant_name"`
	Enabled              *bool    `json:"enabled"`
	RegisterLoginURL     string   `json:"register_login_url"`
	LoginURL             string   `json:"login_url"`
	UserSyncURL          string   `json:"user_sync_url"`
	UserSyncAuthType     string   `json:"user_sync_auth_type"`
	AllowedRedirectHosts []string `json:"allowed_redirect_hosts"`
}

func (h *MerchantSSOHandler) AdminList(c *gin.Context) {
	items, err := h.service.ListIntegrations(c.Request.Context(), false)
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}
	response.Success(c, gin.H{"integrations": items})
}

func (h *MerchantSSOHandler) AdminCreate(c *gin.Context) {
	var req merchantSSOIntegrationRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, "Invalid merchant SSO integration request")
		return
	}
	item := merchantSSOIntegrationFromRequest(req)
	if err := h.service.SaveIntegration(c.Request.Context(), item, nil); err != nil {
		response.ErrorFrom(c, err)
		return
	}
	item.HMACConfigured = item.HMACSecretEncrypted != ""
	item.HMACSecretEncrypted = ""
	response.Created(c, item)
}

func (h *MerchantSSOHandler) AdminUpdate(c *gin.Context) {
	id, err := strconv.ParseInt(strings.TrimSpace(c.Param("id")), 10, 64)
	if err != nil || id <= 0 {
		response.BadRequest(c, "Invalid merchant SSO integration id")
		return
	}
	var req merchantSSOIntegrationRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, "Invalid merchant SSO integration request")
		return
	}
	item, err := h.service.GetIntegrationConfig(c.Request.Context(), id)
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}
	applyMerchantSSOIntegrationRequest(item, req)
	if err := h.service.SaveIntegration(c.Request.Context(), item, nil); err != nil {
		response.ErrorFrom(c, err)
		return
	}
	item.HMACConfigured = item.HMACSecretEncrypted != ""
	item.HMACSecretEncrypted = ""
	response.Success(c, item)
}

func (h *MerchantSSOHandler) AdminSyncUsers(c *gin.Context) {
	id, err := strconv.ParseInt(strings.TrimSpace(c.Param("id")), 10, 64)
	if err != nil || id <= 0 {
		response.BadRequest(c, "Invalid merchant SSO integration id")
		return
	}
	result, err := h.service.SyncUsers(c.Request.Context(), id)
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}
	response.Success(c, result)
}

func (h *MerchantSSOHandler) AdminGenerateHMACSecret(c *gin.Context) {
	id, err := strconv.ParseInt(strings.TrimSpace(c.Param("id")), 10, 64)
	if err != nil || id <= 0 {
		response.BadRequest(c, "Invalid merchant SSO integration id")
		return
	}
	secret, err := h.service.GenerateHMACSecret(c.Request.Context(), id)
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}
	response.Success(c, gin.H{"hmac_secret": secret, "one_time": true})
}

func (h *MerchantSSOHandler) AdminListBindings(c *gin.Context) {
	id, err := strconv.ParseInt(strings.TrimSpace(c.Param("id")), 10, 64)
	if err != nil || id <= 0 {
		response.BadRequest(c, "Invalid merchant SSO integration id")
		return
	}
	items, err := h.service.ListBindings(c.Request.Context(), id)
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}
	response.Success(c, gin.H{"bindings": items})
}

func merchantSSOIntegrationFromRequest(req merchantSSOIntegrationRequest) *service.MerchantSSOIntegration {
	item := &service.MerchantSSOIntegration{
		MerchantCode:         strings.TrimSpace(req.MerchantCode),
		MerchantName:         strings.TrimSpace(req.MerchantName),
		Enabled:              req.Enabled != nil && *req.Enabled,
		RegisterLoginURL:     strings.TrimSpace(req.RegisterLoginURL),
		LoginURL:             strings.TrimSpace(req.LoginURL),
		UserSyncURL:          strings.TrimSpace(req.UserSyncURL),
		UserSyncAuthType:     strings.TrimSpace(req.UserSyncAuthType),
		AllowedRedirectHosts: req.AllowedRedirectHosts,
	}
	return item
}

func applyMerchantSSOIntegrationRequest(item *service.MerchantSSOIntegration, req merchantSSOIntegrationRequest) {
	if strings.TrimSpace(req.MerchantCode) != "" {
		item.MerchantCode = strings.TrimSpace(req.MerchantCode)
	}
	if req.MerchantName != "" {
		item.MerchantName = strings.TrimSpace(req.MerchantName)
	}
	if req.RegisterLoginURL != "" {
		item.RegisterLoginURL = strings.TrimSpace(req.RegisterLoginURL)
	}
	if req.LoginURL != "" {
		item.LoginURL = strings.TrimSpace(req.LoginURL)
	}
	if req.UserSyncURL != "" {
		item.UserSyncURL = strings.TrimSpace(req.UserSyncURL)
	}
	if req.UserSyncAuthType != "" {
		item.UserSyncAuthType = strings.TrimSpace(req.UserSyncAuthType)
	}
	if req.AllowedRedirectHosts != nil {
		item.AllowedRedirectHosts = req.AllowedRedirectHosts
	}
	// Enabled is intentionally always applied: false is a useful way to turn an integration off.
	if req.Enabled != nil {
		item.Enabled = *req.Enabled
	}
}
