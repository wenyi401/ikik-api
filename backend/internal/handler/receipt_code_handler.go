package handler

import (
	"github.com/gin-gonic/gin"
	"ikik-api/internal/pkg/response"
	middleware2 "ikik-api/internal/server/middleware"
	"ikik-api/internal/service"
)

const receiptCodeMultipartField = "file"

type ReceiptCodeHandler struct {
	service *service.ReceiptCodeService
}

func NewReceiptCodeHandler(service *service.ReceiptCodeService) *ReceiptCodeHandler {
	return &ReceiptCodeHandler{service: service}
}

func (h *ReceiptCodeHandler) Get(c *gin.Context) {
	subject, ok := middleware2.GetAuthSubjectFromContext(c)
	if !ok {
		response.Unauthorized(c, "User not authenticated")
		return
	}

	code, err := h.service.Get(c.Request.Context(), subject.UserID, c.Query("payment_method"))
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}
	response.Success(c, code)
}

func (h *ReceiptCodeHandler) Upload(c *gin.Context) {
	subject, ok := middleware2.GetAuthSubjectFromContext(c)
	if !ok {
		response.Unauthorized(c, "User not authenticated")
		return
	}

	fileHeader, err := c.FormFile(receiptCodeMultipartField)
	if err != nil {
		response.ErrorFrom(c, service.ErrReceiptCodeFileRequired)
		return
	}
	file, err := fileHeader.Open()
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}
	defer func() { _ = file.Close() }()

	contentType := fileHeader.Header.Get("Content-Type")
	if contentType == "" {
		contentType = "application/octet-stream"
	}

	code, err := h.service.Upload(c.Request.Context(), service.ReceiptCodeUploadInput{
		UserID:        subject.UserID,
		PaymentMethod: c.PostForm("payment_method"),
		FileName:      fileHeader.Filename,
		ContentType:   contentType,
		Body:          file,
		Size:          fileHeader.Size,
	})
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}
	response.Success(c, code)
}

func (h *ReceiptCodeHandler) Delete(c *gin.Context) {
	subject, ok := middleware2.GetAuthSubjectFromContext(c)
	if !ok {
		response.Unauthorized(c, "User not authenticated")
		return
	}

	if err := h.service.Delete(c.Request.Context(), subject.UserID, c.Query("payment_method")); err != nil {
		response.ErrorFrom(c, err)
		return
	}
	response.Success(c, gin.H{"deleted": true})
}
