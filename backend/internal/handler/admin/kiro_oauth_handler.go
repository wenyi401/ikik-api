package admin

import (
	"strconv"
	"strings"

	"github.com/gin-gonic/gin"
	"ikik-api/internal/handler/dto"
	"ikik-api/internal/pkg/response"
	"ikik-api/internal/service"
)

type KiroOAuthHandler struct {
	kiroOAuthService *service.KiroOAuthService
	adminService     service.AdminService
}

func NewKiroOAuthHandler(
	kiroOAuthService *service.KiroOAuthService,
	adminService service.AdminService,
) *KiroOAuthHandler {
	return &KiroOAuthHandler{
		kiroOAuthService: kiroOAuthService,
		adminService:     adminService,
	}
}

type KiroGenerateAuthURLRequest struct {
	ProxyID  *int64 `json:"proxy_id"`
	Provider string `json:"provider"`
}

func (h *KiroOAuthHandler) GenerateAuthURL(c *gin.Context) {
	var req KiroGenerateAuthURLRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, "请求无效: "+err.Error())
		return
	}
	result, err := h.kiroOAuthService.GenerateAuthURL(c.Request.Context(), &service.KiroGenerateAuthURLInput{
		ProxyID:  req.ProxyID,
		Provider: req.Provider,
	})
	if err != nil {
		response.BadRequest(c, "生成授权链接失败: "+err.Error())
		return
	}
	response.Success(c, result)
}

type KiroGenerateIDCAuthURLRequest struct {
	ProxyID  *int64 `json:"proxy_id"`
	StartURL string `json:"start_url"`
	Region   string `json:"region"`
}

func (h *KiroOAuthHandler) GenerateIDCAuthURL(c *gin.Context) {
	var req KiroGenerateIDCAuthURLRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, "请求无效: "+err.Error())
		return
	}
	result, err := h.kiroOAuthService.GenerateIDCAuthURL(c.Request.Context(), &service.KiroGenerateIDCAuthURLInput{
		ProxyID:  req.ProxyID,
		StartURL: req.StartURL,
		Region:   req.Region,
	})
	if err != nil {
		response.BadRequest(c, "生成 IDC 授权链接失败: "+err.Error())
		return
	}
	response.Success(c, result)
}

type KiroExchangeCodeRequest struct {
	SessionID    string `json:"session_id" binding:"required"`
	State        string `json:"state" binding:"required"`
	Code         string `json:"code" binding:"required"`
	CallbackPath string `json:"callback_path"`
	LoginOption  string `json:"login_option"`
	ProxyID      *int64 `json:"proxy_id"`
}

func (h *KiroOAuthHandler) ExchangeCode(c *gin.Context) {
	var req KiroExchangeCodeRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, "请求无效: "+err.Error())
		return
	}
	tokenInfo, err := h.kiroOAuthService.ExchangeCode(c.Request.Context(), &service.KiroExchangeCodeInput{
		SessionID:    req.SessionID,
		State:        req.State,
		Code:         req.Code,
		CallbackPath: req.CallbackPath,
		LoginOption:  req.LoginOption,
		ProxyID:      req.ProxyID,
	})
	if err != nil {
		response.BadRequest(c, "Token 交换失败: "+err.Error())
		return
	}
	response.Success(c, tokenInfo)
}

type KiroRefreshTokenRequest struct {
	RefreshToken string `json:"refresh_token" binding:"required"`
	AuthMethod   string `json:"auth_method"`
	Provider     string `json:"provider"`
	ClientID     string `json:"client_id"`
	ClientSecret string `json:"client_secret"`
	StartURL     string `json:"start_url"`
	Region       string `json:"region"`
	ProfileArn   string `json:"profile_arn"`
	ProxyID      *int64 `json:"proxy_id"`
}

func (h *KiroOAuthHandler) RefreshToken(c *gin.Context) {
	var req KiroRefreshTokenRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, "请求无效: "+err.Error())
		return
	}
	tokenInfo, err := h.kiroOAuthService.RefreshToken(c.Request.Context(), &service.KiroRefreshTokenInput{
		RefreshToken: req.RefreshToken,
		AuthMethod:   req.AuthMethod,
		Provider:     req.Provider,
		ClientID:     req.ClientID,
		ClientSecret: req.ClientSecret,
		StartURL:     req.StartURL,
		Region:       req.Region,
		ProfileArn:   req.ProfileArn,
		ProxyID:      req.ProxyID,
	})
	if err != nil {
		response.BadRequest(c, "刷新 Kiro Token 失败: "+err.Error())
		return
	}
	response.Success(c, tokenInfo)
}

func (h *KiroOAuthHandler) RefreshAccountToken(c *gin.Context) {
	accountID, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil {
		response.BadRequest(c, "Invalid account ID")
		return
	}
	account, err := h.adminService.GetAccount(c.Request.Context(), accountID)
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}
	if account.Platform != service.PlatformKiro {
		response.BadRequest(c, "Account platform does not match Kiro OAuth endpoint")
		return
	}
	if !account.IsOAuth() {
		response.BadRequest(c, "Cannot refresh non-OAuth account credentials")
		return
	}
	tokenInfo, err := h.kiroOAuthService.RefreshAccountToken(c.Request.Context(), account)
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}
	newCredentials := h.kiroOAuthService.BuildAccountCredentials(tokenInfo)
	newCredentials = service.MergeCredentials(account.Credentials, newCredentials)
	updatedAccount, err := h.adminService.UpdateAccount(c.Request.Context(), accountID, &service.UpdateAccountInput{
		Credentials: newCredentials,
	})
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}
	response.Success(c, dto.AccountFromService(updatedAccount))
}

type KiroImportTokenRequest struct {
	TokenJSON              string `json:"token_json" binding:"required"`
	DeviceRegistrationJSON string `json:"device_registration_json"`
}

func (h *KiroOAuthHandler) ImportToken(c *gin.Context) {
	var req KiroImportTokenRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, "请求无效: "+err.Error())
		return
	}
	tokenInfo, err := h.kiroOAuthService.ImportToken(&service.KiroImportTokenInput{
		TokenJSON:              req.TokenJSON,
		DeviceRegistrationJSON: req.DeviceRegistrationJSON,
	})
	if err != nil {
		response.BadRequest(c, "导入 Kiro Token 失败: "+err.Error())
		return
	}
	response.Success(c, tokenInfo)
}

func (h *KiroOAuthHandler) CreateAccountFromOAuth(c *gin.Context) {
	var req struct {
		SessionID    string  `json:"session_id" binding:"required"`
		State        string  `json:"state" binding:"required"`
		Code         string  `json:"code" binding:"required"`
		CallbackPath string  `json:"callback_path"`
		LoginOption  string  `json:"login_option"`
		ProxyID      *int64  `json:"proxy_id"`
		Name         string  `json:"name"`
		Concurrency  int     `json:"concurrency"`
		Priority     int     `json:"priority"`
		GroupIDs     []int64 `json:"group_ids"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, "Invalid request: "+err.Error())
		return
	}
	tokenInfo, err := h.kiroOAuthService.ExchangeCode(c.Request.Context(), &service.KiroExchangeCodeInput{
		SessionID:    req.SessionID,
		State:        req.State,
		Code:         req.Code,
		CallbackPath: req.CallbackPath,
		LoginOption:  req.LoginOption,
		ProxyID:      req.ProxyID,
	})
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}
	credentials := h.kiroOAuthService.BuildAccountCredentials(tokenInfo)
	name := strings.TrimSpace(req.Name)
	if name == "" && tokenInfo.Email != "" {
		name = tokenInfo.Email
	}
	if name == "" {
		name = "Kiro OAuth Account"
	}
	account, err := h.adminService.CreateAccount(c.Request.Context(), &service.CreateAccountInput{
		Name:        name,
		Platform:    service.PlatformKiro,
		Type:        service.AccountTypeOAuth,
		Credentials: credentials,
		Extra: map[string]any{
			"openai_responses_supported": false,
		},
		ProxyID:     req.ProxyID,
		Concurrency: req.Concurrency,
		Priority:    req.Priority,
		GroupIDs:    req.GroupIDs,
	})
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}
	response.Success(c, dto.AccountFromService(account))
}
