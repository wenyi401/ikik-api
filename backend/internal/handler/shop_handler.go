package handler

import (
	"context"
	"fmt"
	"strconv"
	"strings"

	"ikik-api/internal/payment"
	infraerrors "ikik-api/internal/pkg/errors"
	"ikik-api/internal/pkg/response"
	"ikik-api/internal/service"

	"github.com/gin-gonic/gin"
)

type ShopHandler struct {
	shopService *service.ShopService
}

func NewShopHandler(shopService *service.ShopService) *ShopHandler {
	return &ShopHandler{shopService: shopService}
}

func (h *ShopHandler) ListCategories(c *gin.Context) {
	items, err := h.shopService.ListCategories(c.Request.Context(), false)
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}
	response.Success(c, items)
}

func (h *ShopHandler) ListProducts(c *gin.Context) {
	page, pageSize := response.ParsePagination(c)
	categoryID := parseOptionalInt64Query(c, "category_id")
	items, total, err := h.shopService.ListProducts(c.Request.Context(), service.ShopListProductsParams{
		CategoryID: categoryID,
		Keyword:    c.Query("keyword"),
		Page:       page,
		PageSize:   pageSize,
	})
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}
	response.Paginated(c, items, int64(total), page, pageSize)
}

func (h *ShopHandler) GetProduct(c *gin.Context) {
	id, ok := parsePathID(c, "id")
	if !ok {
		return
	}
	item, err := h.shopService.GetProduct(c.Request.Context(), id, false)
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}
	response.Success(c, item)
}

func (h *ShopHandler) ListDrawProgress(c *gin.Context) {
	subject, ok := requireAuth(c)
	if !ok {
		return
	}
	items, err := h.shopService.ListDrawProgress(c.Request.Context(), subject.UserID)
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}
	response.Success(c, items)
}

type createShopOrderRequest struct {
	ProductID         int64  `json:"product_id" binding:"required"`
	Quantity          int    `json:"quantity" binding:"required"`
	PaymentMethod     string `json:"payment_method" binding:"required"`
	OpenID            string `json:"openid"`
	WechatResumeToken string `json:"wechat_resume_token"`
	ReturnURL         string `json:"return_url"`
	PaymentSource     string `json:"payment_source"`
	IsMobile          *bool  `json:"is_mobile,omitempty"`
}

func (h *ShopHandler) CreateOrder(c *gin.Context) {
	subject, ok := requireAuth(c)
	if !ok {
		return
	}

	var req createShopOrderRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, "Invalid request: "+err.Error())
		return
	}
	if strings.TrimSpace(req.WechatResumeToken) != "" {
		claims, err := h.shopService.ParseWeChatPaymentResumeToken(req.WechatResumeToken)
		if err != nil {
			response.ErrorFrom(c, err)
			return
		}
		if err := applyShopWeChatResumeClaims(&req, claims, subject.UserID); err != nil {
			response.ErrorFrom(c, err)
			return
		}
	}

	mobile := isMobile(c)
	if req.IsMobile != nil {
		mobile = *req.IsMobile
	}
	executeUserIdempotentJSON(c, "user.shop.orders.create", req, service.DefaultWriteIdempotencyTTL(), func(ctx context.Context) (any, error) {
		return h.shopService.CreateOrder(ctx, service.ShopCreateOrderRequest{
			UserID:          subject.UserID,
			ProductID:       req.ProductID,
			Quantity:        req.Quantity,
			PaymentMethod:   req.PaymentMethod,
			OpenID:          req.OpenID,
			ClientIP:        c.ClientIP(),
			IsMobile:        mobile,
			IsWeChatBrowser: isWeChatBrowser(c),
			SrcHost:         c.Request.Host,
			SrcURL:          c.Request.Referer(),
			ReturnURL:       req.ReturnURL,
			PaymentSource:   req.PaymentSource,
		})
	})
}

func (h *ShopHandler) GetOrder(c *gin.Context) {
	subject, ok := requireAuth(c)
	if !ok {
		return
	}
	id, ok := parsePathID(c, "id")
	if !ok {
		return
	}
	order, err := h.shopService.GetOrderForUser(c.Request.Context(), subject.UserID, id)
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}
	response.Success(c, order)
}

func (h *ShopHandler) DownloadOrderFile(c *gin.Context) {
	subject, ok := requireAuth(c)
	if !ok {
		return
	}
	orderID, ok := parsePathID(c, "id")
	if !ok {
		return
	}
	cardID, ok := parsePathID(c, "card_id")
	if !ok {
		return
	}
	download, err := h.shopService.GetOrderFileCardDownload(c.Request.Context(), subject.UserID, orderID, cardID)
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}
	defer func() { _ = download.Body.Close() }()
	filename := safeDownloadFilename(download.File.Filename)
	c.Header("Content-Disposition", fmt.Sprintf("attachment; filename=%q", filename))
	c.Header("X-Content-Type-Options", "nosniff")
	c.DataFromReader(200, download.File.ByteSize, download.File.ContentType, download.Body, nil)
}

func (h *ShopHandler) DownloadOrderFilesZip(c *gin.Context) {
	subject, ok := requireAuth(c)
	if !ok {
		return
	}
	orderID, ok := parsePathID(c, "id")
	if !ok {
		return
	}
	filename := fmt.Sprintf("shop-order-%d-files.zip", orderID)
	c.Header("Content-Disposition", fmt.Sprintf("attachment; filename=%q", safeDownloadFilename(filename)))
	c.Header("X-Content-Type-Options", "nosniff")
	c.Header("Content-Type", "application/zip")
	if _, err := h.shopService.WriteOrderFileCardArchive(c.Request.Context(), subject.UserID, orderID, c.Writer); err != nil {
		if !c.Writer.Written() {
			response.ErrorFrom(c, err)
			return
		}
		_ = c.Error(err)
	}
}

func applyShopWeChatResumeClaims(req *createShopOrderRequest, claims *service.WeChatPaymentResumeClaims, userID int64) error {
	if req == nil || claims == nil {
		return nil
	}
	if claims.UserID > 0 && claims.UserID != userID {
		return infraerrors.Forbidden("WECHAT_PAYMENT_USER_MISMATCH", "wechat payment resume token does not belong to the current user")
	}
	if paymentType := service.NormalizeVisibleMethod(claims.PaymentType); paymentType != "" {
		req.PaymentMethod = paymentType
	} else if req.PaymentMethod == "" {
		req.PaymentMethod = payment.TypeWxpay
	}
	if openid := strings.TrimSpace(claims.OpenID); openid != "" {
		req.OpenID = openid
	}
	return nil
}

func parsePathID(c *gin.Context, name string) (int64, bool) {
	id, err := strconv.ParseInt(c.Param(name), 10, 64)
	if err != nil || id <= 0 {
		response.BadRequest(c, "Invalid "+name)
		return 0, false
	}
	return id, true
}

func parseOptionalInt64Query(c *gin.Context, name string) int64 {
	raw := strings.TrimSpace(c.Query(name))
	if raw == "" {
		return 0
	}
	id, err := strconv.ParseInt(raw, 10, 64)
	if err != nil || id < 0 {
		return 0
	}
	return id
}

func safeDownloadFilename(raw string) string {
	name := strings.TrimSpace(strings.ReplaceAll(raw, "\\", "/"))
	if idx := strings.LastIndex(name, "/"); idx >= 0 {
		name = name[idx+1:]
	}
	name = strings.Trim(name, " .")
	if name == "" {
		return "download"
	}
	return name
}
