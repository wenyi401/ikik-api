package admin

import (
	"fmt"
	"io"
	"mime/multipart"
	"strconv"
	"strings"

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
	items, err := h.shopService.AdminListCategories(c.Request.Context())
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}
	response.Success(c, items)
}

func (h *ShopHandler) CreateCategory(c *gin.Context) {
	var req service.ShopCreateCategoryRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, "Invalid request: "+err.Error())
		return
	}
	item, err := h.shopService.AdminCreateCategory(c.Request.Context(), req)
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}
	response.Created(c, item)
}

func (h *ShopHandler) UpdateCategory(c *gin.Context) {
	id, ok := parseAdminShopID(c, "id")
	if !ok {
		return
	}
	var req service.ShopUpdateCategoryRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, "Invalid request: "+err.Error())
		return
	}
	item, err := h.shopService.AdminUpdateCategory(c.Request.Context(), id, req)
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}
	response.Success(c, item)
}

func (h *ShopHandler) DeleteCategory(c *gin.Context) {
	id, ok := parseAdminShopID(c, "id")
	if !ok {
		return
	}
	if err := h.shopService.AdminDeleteCategory(c.Request.Context(), id); err != nil {
		response.ErrorFrom(c, err)
		return
	}
	response.Success(c, gin.H{"message": "disabled"})
}

func (h *ShopHandler) ListProducts(c *gin.Context) {
	page, pageSize := response.ParsePagination(c)
	items, total, err := h.shopService.AdminListProducts(c.Request.Context(), service.ShopListProductsParams{
		CategoryID: parseAdminOptionalInt64Query(c, "category_id"),
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

func (h *ShopHandler) CreateProduct(c *gin.Context) {
	var req service.ShopCreateProductRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, "Invalid request: "+err.Error())
		return
	}
	item, err := h.shopService.AdminCreateProduct(c.Request.Context(), req)
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}
	response.Created(c, item)
}

func (h *ShopHandler) UpdateProduct(c *gin.Context) {
	id, ok := parseAdminShopID(c, "id")
	if !ok {
		return
	}
	var req service.ShopUpdateProductRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, "Invalid request: "+err.Error())
		return
	}
	item, err := h.shopService.AdminUpdateProduct(c.Request.Context(), id, req)
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}
	response.Success(c, item)
}

func (h *ShopHandler) DeleteProduct(c *gin.Context) {
	id, ok := parseAdminShopID(c, "id")
	if !ok {
		return
	}
	if err := h.shopService.AdminDeleteProduct(c.Request.Context(), id); err != nil {
		response.ErrorFrom(c, err)
		return
	}
	response.Success(c, gin.H{"message": "disabled"})
}

func (h *ShopHandler) ListCardKeys(c *gin.Context) {
	page, pageSize := response.ParsePagination(c)
	items, total, err := h.shopService.AdminListCardKeys(c.Request.Context(), service.ShopListCardKeysParams{
		ProductID: parseAdminOptionalInt64Query(c, "product_id"),
		Status:    c.Query("status"),
		Keyword:   c.Query("keyword"),
		Page:      page,
		PageSize:  pageSize,
	})
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}
	response.Paginated(c, items, int64(total), page, pageSize)
}

func (h *ShopHandler) CreateCardKey(c *gin.Context) {
	var req service.ShopCreateCardKeyRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, "Invalid request: "+err.Error())
		return
	}
	item, err := h.shopService.AdminCreateCardKey(c.Request.Context(), req)
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}
	response.Created(c, item)
}

func (h *ShopHandler) ImportCardKeys(c *gin.Context) {
	var req service.ShopImportCardKeysRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, "Invalid request: "+err.Error())
		return
	}
	items, err := h.shopService.AdminImportCardKeys(c.Request.Context(), req)
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}
	response.Created(c, items)
}

func (h *ShopHandler) UpdateCardKey(c *gin.Context) {
	id, ok := parseAdminShopID(c, "id")
	if !ok {
		return
	}
	var req service.ShopUpdateCardKeyRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, "Invalid request: "+err.Error())
		return
	}
	item, err := h.shopService.AdminUpdateCardKey(c.Request.Context(), id, req)
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}
	response.Success(c, item)
}

func (h *ShopHandler) DeleteCardKey(c *gin.Context) {
	id, ok := parseAdminShopID(c, "id")
	if !ok {
		return
	}
	if err := h.shopService.AdminDeleteCardKey(c.Request.Context(), id); err != nil {
		response.ErrorFrom(c, err)
		return
	}
	response.Success(c, gin.H{"message": "deleted"})
}

func (h *ShopHandler) GetOrder(c *gin.Context) {
	id, ok := parseAdminShopID(c, "id")
	if !ok {
		return
	}
	order, err := h.shopService.GetOrderForAdmin(c.Request.Context(), id)
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}
	response.Success(c, order)
}

func (h *ShopHandler) DownloadOrderFile(c *gin.Context) {
	orderID, ok := parseAdminShopID(c, "id")
	if !ok {
		return
	}
	cardID, ok := parseAdminShopID(c, "card_id")
	if !ok {
		return
	}
	download, err := h.shopService.GetOrderFileCardDownloadForAdmin(c.Request.Context(), orderID, cardID)
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}
	defer func() { _ = download.Body.Close() }()
	filename := safeAdminShopDownloadFilename(download.File.Filename)
	c.Header("Content-Disposition", fmt.Sprintf("attachment; filename=%q", filename))
	c.Header("X-Content-Type-Options", "nosniff")
	c.DataFromReader(200, download.File.ByteSize, download.File.ContentType, download.Body, nil)
}

func (h *ShopHandler) DownloadOrderFilesZip(c *gin.Context) {
	orderID, ok := parseAdminShopID(c, "id")
	if !ok {
		return
	}
	filename := fmt.Sprintf("shop-order-%d-files.zip", orderID)
	c.Header("Content-Disposition", fmt.Sprintf("attachment; filename=%q", safeAdminShopDownloadFilename(filename)))
	c.Header("X-Content-Type-Options", "nosniff")
	c.Header("Content-Type", "application/zip")
	if _, err := h.shopService.WriteOrderFileCardArchiveForAdmin(c.Request.Context(), orderID, c.Writer); err != nil {
		if !c.Writer.Written() {
			response.ErrorFrom(c, err)
			return
		}
		_ = c.Error(err)
	}
}

func (h *ShopHandler) GetFileCardStorage(c *gin.Context) {
	cfg, err := h.shopService.GetFileCardStorageConfig(c.Request.Context())
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}
	response.Success(c, cfg)
}

func (h *ShopHandler) UpdateFileCardStorage(c *gin.Context) {
	var req service.UpdateShopFileCardStorageConfigRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, "Invalid request: "+err.Error())
		return
	}
	cfg, err := h.shopService.UpdateFileCardStorageConfig(c.Request.Context(), req)
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}
	response.Success(c, cfg)
}

func (h *ShopHandler) TestFileCardStorage(c *gin.Context) {
	var req service.UpdateShopFileCardStorageConfigRequest
	if c.Request.Body != nil && c.Request.ContentLength != 0 {
		if err := c.ShouldBindJSON(&req); err != nil && err != io.EOF {
			response.BadRequest(c, "Invalid request: "+err.Error())
			return
		}
	}
	if err := h.shopService.TestFileCardStorageConfig(c.Request.Context(), &req); err != nil {
		response.ErrorFrom(c, err)
		return
	}
	response.Success(c, gin.H{"message": "ok"})
}

func (h *ShopHandler) ImportFileCardKeys(c *gin.Context) {
	productID, err := strconv.ParseInt(strings.TrimSpace(c.PostForm("product_id")), 10, 64)
	if err != nil || productID <= 0 {
		response.BadRequest(c, "Invalid product_id")
		return
	}
	form, err := c.MultipartForm()
	if err != nil {
		response.BadRequest(c, "Invalid multipart form: "+err.Error())
		return
	}
	files := form.File["files"]
	if len(files) == 0 {
		files = form.File["file"]
	}
	if len(files) == 0 {
		response.BadRequest(c, "No files uploaded")
		return
	}
	uploads := make([]service.ShopFileCardUpload, 0, len(files))
	opened := make([]multipart.File, 0, len(files))
	defer func() {
		for _, f := range opened {
			_ = f.Close()
		}
	}()
	for _, header := range files {
		if header == nil {
			continue
		}
		if header.Size <= 0 {
			response.ErrorFrom(c, service.ErrShopFileCardEmpty)
			return
		}
		if header.Size > service.ShopFileCardMaxSizeBytes {
			response.ErrorFrom(c, service.ErrShopFileCardTooLarge)
			return
		}
		file, err := header.Open()
		if err != nil {
			response.BadRequest(c, "Open uploaded file failed: "+err.Error())
			return
		}
		opened = append(opened, file)
		uploads = append(uploads, service.ShopFileCardUpload{
			Filename:    header.Filename,
			ContentType: header.Header.Get("Content-Type"),
			Reader:      file,
		})
	}
	items, err := h.shopService.AdminImportFileCardKeys(c.Request.Context(), productID, uploads)
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}
	response.Created(c, items)
}

func parseAdminShopID(c *gin.Context, name string) (int64, bool) {
	id, err := strconv.ParseInt(c.Param(name), 10, 64)
	if err != nil || id <= 0 {
		response.BadRequest(c, "Invalid "+name)
		return 0, false
	}
	return id, true
}

func parseAdminOptionalInt64Query(c *gin.Context, name string) int64 {
	raw := c.Query(name)
	if raw == "" {
		return 0
	}
	id, err := strconv.ParseInt(raw, 10, 64)
	if err != nil || id < 0 {
		return 0
	}
	return id
}

func safeAdminShopDownloadFilename(raw string) string {
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
