package service

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"errors"
	"fmt"
	"image"
	_ "image/gif"
	_ "image/jpeg"
	_ "image/png"
	"io"
	"mime"
	"net/http"
	"path/filepath"
	"strings"
	"time"

	"github.com/google/uuid"
	"golang.org/x/image/webp"
	"ikik-api/internal/config"
	infraerrors "ikik-api/internal/pkg/errors"
)

const (
	ReceiptCodePaymentMethodAlipay = "alipay"
	ReceiptCodePaymentMethodWeChat = "wechat"

	receiptCodeStorageProviderOSS = "oss"
	receiptCodeStorageProviderDB  = "db_inline"
	defaultReceiptCodeMaxBytes    = int64(1024 * 1024)
)

var (
	ErrReceiptCodeStorageNotConfigured = infraerrors.BadRequest("RECEIPT_CODE_STORAGE_NOT_CONFIGURED", "receipt code storage is not configured")
	ErrReceiptCodeNotFound             = infraerrors.NotFound("RECEIPT_CODE_NOT_FOUND", "receipt code not found")
	ErrReceiptCodePaymentMethodInvalid = infraerrors.BadRequest("RECEIPT_CODE_PAYMENT_METHOD_INVALID", "payment method is invalid")
	ErrReceiptCodeFileRequired         = infraerrors.BadRequest("RECEIPT_CODE_FILE_REQUIRED", "receipt code image is required")
	ErrReceiptCodeFileTooLarge         = infraerrors.BadRequest("RECEIPT_CODE_FILE_TOO_LARGE", "receipt code image is too large")
	ErrReceiptCodeInvalidImage         = infraerrors.BadRequest("RECEIPT_CODE_INVALID_IMAGE", "receipt code must be a valid PNG, JPEG, GIF, or WebP image")
)

type ReceiptCode struct {
	ID              int64     `json:"id"`
	UserID          int64     `json:"user_id"`
	PaymentMethod   string    `json:"payment_method"`
	StorageProvider string    `json:"storage_provider"`
	StorageKey      string    `json:"-"`
	URL             string    `json:"url,omitempty"`
	ContentType     string    `json:"content_type"`
	ByteSize        int       `json:"byte_size"`
	SHA256          string    `json:"sha256"`
	CreatedAt       time.Time `json:"created_at"`
	UpdatedAt       time.Time `json:"updated_at"`
}

type ReceiptCodeUploadInput struct {
	UserID        int64
	PaymentMethod string
	FileName      string
	ContentType   string
	Body          io.Reader
	Size          int64
}

type ReceiptCodeUpsertInput struct {
	UserID          int64
	PaymentMethod   string
	StorageProvider string
	StorageKey      string
	URL             string
	ContentType     string
	ByteSize        int
	SHA256          string
}

type ReceiptCodeRepository interface {
	GetReceiptCode(ctx context.Context, userID int64, paymentMethod string) (*ReceiptCode, error)
	UpsertReceiptCode(ctx context.Context, input ReceiptCodeUpsertInput) (*ReceiptCode, error)
	DeleteReceiptCode(ctx context.Context, userID int64, paymentMethod string) (*ReceiptCode, error)
	ReceiptCodeInUse(ctx context.Context, storageKey string) (bool, error)
}

type ReceiptCodeObjectStore interface {
	Upload(ctx context.Context, key string, body io.Reader, contentType string) error
	Delete(ctx context.Context, key string) error
	PresignURL(ctx context.Context, key string, expiry time.Duration) (string, error)
	PublicURL(key string) string
}

type ReceiptCodeObjectStoreFactory func(ctx context.Context, cfg config.ReceiptCodeStorageConfig) (ReceiptCodeObjectStore, error)

type ReceiptCodeStorageConfigProvider interface {
	GetReceiptCodeStorageConfig(ctx context.Context) (config.ReceiptCodeStorageConfig, error)
}

type ReceiptCodeService struct {
	repo         ReceiptCodeRepository
	cfgProvider  ReceiptCodeStorageConfigProvider
	storeFactory ReceiptCodeObjectStoreFactory
}

func NewReceiptCodeService(repo ReceiptCodeRepository, cfgProvider ReceiptCodeStorageConfigProvider, storeFactory ReceiptCodeObjectStoreFactory) *ReceiptCodeService {
	return &ReceiptCodeService{
		repo:         repo,
		cfgProvider:  cfgProvider,
		storeFactory: storeFactory,
	}
}

func (s *ReceiptCodeService) Get(ctx context.Context, userID int64, paymentMethod string) (*ReceiptCode, error) {
	method := normalizeReceiptCodePaymentMethod(paymentMethod)
	if method == "" {
		return nil, ErrReceiptCodePaymentMethodInvalid
	}

	code, err := s.repo.GetReceiptCode(ctx, userID, method)
	if err != nil {
		return nil, err
	}
	if code == nil {
		return nil, nil
	}
	if err := s.attachAccessURL(ctx, code); err != nil {
		return nil, err
	}
	return code, nil
}

func (s *ReceiptCodeService) Upload(ctx context.Context, input ReceiptCodeUploadInput) (*ReceiptCode, error) {
	method := normalizeReceiptCodePaymentMethod(input.PaymentMethod)
	if method == "" {
		return nil, ErrReceiptCodePaymentMethodInvalid
	}
	if input.Body == nil {
		return nil, ErrReceiptCodeFileRequired
	}

	cfg, storageConfigured, err := s.optionalStorageConfig(ctx)
	if err != nil {
		return nil, err
	}

	maxSize := maxReceiptCodeSizeBytes(cfg)
	if input.Size > maxSize {
		return nil, ErrReceiptCodeFileTooLarge.WithMetadata(map[string]string{
			"max_size_bytes": fmt.Sprintf("%d", maxSize),
		})
	}

	data, err := readReceiptCodeUpload(input.Body, maxSize)
	if err != nil {
		return nil, err
	}
	contentType, ext, err := detectReceiptCodeImage(data, input.ContentType, input.FileName)
	if err != nil {
		return nil, err
	}

	sum := sha256.Sum256(data)
	sha := hex.EncodeToString(sum[:])

	old, err := s.repo.GetReceiptCode(ctx, input.UserID, method)
	if err != nil {
		return nil, fmt.Errorf("get old receipt code: %w", err)
	}

	key := buildReceiptCodeInlineKey(input.UserID, method, ext)
	url := buildReceiptCodeDataURL(contentType, data)
	provider := receiptCodeStorageProviderDB
	var store ReceiptCodeObjectStore

	if storageConfigured {
		key = buildReceiptCodeObjectKey(cfg.Prefix, input.UserID, method, ext)
		provider = receiptCodeStorageProviderOSS

		store, err = s.store(ctx, cfg)
		if err != nil {
			return nil, err
		}
		if err := store.Upload(ctx, key, bytes.NewReader(data), contentType); err != nil {
			return nil, fmt.Errorf("upload receipt code object: %w", err)
		}
		url = store.PublicURL(key)
	}

	code, err := s.repo.UpsertReceiptCode(ctx, ReceiptCodeUpsertInput{
		UserID:          input.UserID,
		PaymentMethod:   method,
		StorageProvider: provider,
		StorageKey:      key,
		URL:             url,
		ContentType:     contentType,
		ByteSize:        len(data),
		SHA256:          sha,
	})
	if err != nil {
		if store != nil {
			_ = store.Delete(ctx, key)
		}
		return nil, fmt.Errorf("save receipt code metadata: %w", err)
	}

	if store != nil && receiptCodeUsesObjectStore(old) && old.StorageKey != "" && old.StorageKey != key {
		inUse, inUseErr := s.repo.ReceiptCodeInUse(ctx, old.StorageKey)
		if inUseErr != nil {
			return nil, fmt.Errorf("check old receipt code usage: %w", inUseErr)
		}
		if !inUse {
			_ = store.Delete(ctx, old.StorageKey)
		}
	}
	if err := s.attachAccessURL(ctx, code); err != nil {
		return nil, err
	}
	return code, nil
}

func (s *ReceiptCodeService) Delete(ctx context.Context, userID int64, paymentMethod string) error {
	method := normalizeReceiptCodePaymentMethod(paymentMethod)
	if method == "" {
		return ErrReceiptCodePaymentMethodInvalid
	}

	deleted, err := s.repo.DeleteReceiptCode(ctx, userID, method)
	if err != nil {
		return err
	}
	if deleted == nil || deleted.StorageKey == "" {
		return nil
	}
	if !receiptCodeUsesObjectStore(deleted) {
		return nil
	}
	inUse, err := s.repo.ReceiptCodeInUse(ctx, deleted.StorageKey)
	if err != nil {
		return fmt.Errorf("check deleted receipt code usage: %w", err)
	}
	if inUse {
		return nil
	}
	cfg, err := s.storageConfig(ctx)
	if err != nil {
		if errors.Is(err, ErrReceiptCodeStorageNotConfigured) {
			return nil
		}
		return err
	}
	store, err := s.store(ctx, cfg)
	if err != nil {
		return err
	}
	return store.Delete(ctx, deleted.StorageKey)
}

func (s *ReceiptCodeService) optionalStorageConfig(ctx context.Context) (config.ReceiptCodeStorageConfig, bool, error) {
	if s == nil || s.cfgProvider == nil {
		return config.ReceiptCodeStorageConfig{}, false, nil
	}
	cfg, err := s.cfgProvider.GetReceiptCodeStorageConfig(ctx)
	if err != nil {
		return config.ReceiptCodeStorageConfig{}, false, fmt.Errorf("get receipt code storage config: %w", err)
	}
	return cfg, receiptCodeStorageConfigured(cfg), nil
}

func (s *ReceiptCodeService) storageConfig(ctx context.Context) (config.ReceiptCodeStorageConfig, error) {
	if s == nil || s.cfgProvider == nil {
		return config.ReceiptCodeStorageConfig{}, ErrReceiptCodeStorageNotConfigured
	}
	cfg, err := s.cfgProvider.GetReceiptCodeStorageConfig(ctx)
	if err != nil {
		return config.ReceiptCodeStorageConfig{}, fmt.Errorf("get receipt code storage config: %w", err)
	}
	if !receiptCodeStorageConfigured(cfg) {
		return config.ReceiptCodeStorageConfig{}, ErrReceiptCodeStorageNotConfigured
	}
	return cfg, nil
}

func receiptCodeStorageConfigured(cfg config.ReceiptCodeStorageConfig) bool {
	return cfg.Enabled &&
		strings.TrimSpace(cfg.Endpoint) != "" &&
		strings.TrimSpace(cfg.Bucket) != "" &&
		strings.TrimSpace(cfg.AccessKeyID) != "" &&
		strings.TrimSpace(cfg.SecretAccessKey) != ""
}

func (s *ReceiptCodeService) ensureConfigured(ctx context.Context) error {
	if s == nil || s.storeFactory == nil {
		return ErrReceiptCodeStorageNotConfigured
	}
	_, err := s.storageConfig(ctx)
	return err
}

func (s *ReceiptCodeService) store(ctx context.Context, cfg config.ReceiptCodeStorageConfig) (ReceiptCodeObjectStore, error) {
	if s == nil || s.storeFactory == nil || !receiptCodeStorageConfigured(cfg) {
		return nil, ErrReceiptCodeStorageNotConfigured
	}
	store, err := s.storeFactory(ctx, cfg)
	if err != nil {
		return nil, fmt.Errorf("create receipt code object store: %w", err)
	}
	return store, nil
}

func (s *ReceiptCodeService) attachAccessURL(ctx context.Context, code *ReceiptCode) error {
	if code == nil || strings.TrimSpace(code.StorageKey) == "" {
		return nil
	}
	if !receiptCodeUsesObjectStore(code) {
		return nil
	}
	cfg, err := s.storageConfig(ctx)
	if err != nil {
		if errors.Is(err, ErrReceiptCodeStorageNotConfigured) {
			code.URL = ""
			return nil
		}
		return err
	}
	store, err := s.store(ctx, cfg)
	if err != nil {
		if errors.Is(err, ErrReceiptCodeStorageNotConfigured) {
			code.URL = ""
			return nil
		}
		return err
	}
	if url := store.PublicURL(code.StorageKey); url != "" {
		code.URL = url
		return nil
	}
	expiry := time.Duration(cfg.PresignExpireSeconds) * time.Second
	if expiry <= 0 {
		expiry = 5 * time.Minute
	}
	url, err := store.PresignURL(ctx, code.StorageKey, expiry)
	if err != nil {
		return fmt.Errorf("presign receipt code object: %w", err)
	}
	code.URL = url
	return nil
}

func receiptCodeUsesObjectStore(code *ReceiptCode) bool {
	if code == nil {
		return false
	}
	provider := strings.ToLower(strings.TrimSpace(code.StorageProvider))
	return provider == "" || provider == receiptCodeStorageProviderOSS
}

func maxReceiptCodeSizeBytes(cfg config.ReceiptCodeStorageConfig) int64 {
	if cfg.MaxSizeBytes <= 0 {
		return defaultReceiptCodeMaxBytes
	}
	return cfg.MaxSizeBytes
}

func normalizeReceiptCodePaymentMethod(raw string) string {
	switch strings.ToLower(strings.TrimSpace(raw)) {
	case ReceiptCodePaymentMethodAlipay:
		return ReceiptCodePaymentMethodAlipay
	case ReceiptCodePaymentMethodWeChat, "weixin", "wxpay":
		return ReceiptCodePaymentMethodWeChat
	default:
		return ""
	}
}

func readReceiptCodeUpload(r io.Reader, maxSize int64) ([]byte, error) {
	if maxSize <= 0 {
		maxSize = defaultReceiptCodeMaxBytes
	}
	limited := io.LimitReader(r, maxSize+1)
	data, err := io.ReadAll(limited)
	if err != nil {
		return nil, fmt.Errorf("read receipt code image: %w", err)
	}
	if len(data) == 0 {
		return nil, ErrReceiptCodeFileRequired
	}
	if int64(len(data)) > maxSize {
		return nil, ErrReceiptCodeFileTooLarge.WithMetadata(map[string]string{
			"max_size_bytes": fmt.Sprintf("%d", maxSize),
		})
	}
	return data, nil
}

func detectReceiptCodeImage(data []byte, declaredContentType, fileName string) (string, string, error) {
	contentType := http.DetectContentType(data)
	switch contentType {
	case "image/png":
		if _, _, err := image.DecodeConfig(bytes.NewReader(data)); err != nil {
			return "", "", ErrReceiptCodeInvalidImage
		}
		return contentType, ".png", nil
	case "image/jpeg":
		if _, _, err := image.DecodeConfig(bytes.NewReader(data)); err != nil {
			return "", "", ErrReceiptCodeInvalidImage
		}
		return contentType, ".jpg", nil
	case "image/gif":
		if _, _, err := image.DecodeConfig(bytes.NewReader(data)); err != nil {
			return "", "", ErrReceiptCodeInvalidImage
		}
		return contentType, ".gif", nil
	case "application/octet-stream":
		if strings.EqualFold(normalizeContentType(declaredContentType), "image/webp") ||
			strings.EqualFold(filepath.Ext(fileName), ".webp") {
			if _, err := webp.DecodeConfig(bytes.NewReader(data)); err != nil {
				return "", "", ErrReceiptCodeInvalidImage
			}
			return "image/webp", ".webp", nil
		}
	}
	return "", "", ErrReceiptCodeInvalidImage
}

func normalizeContentType(raw string) string {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return ""
	}
	mediaType, _, err := mime.ParseMediaType(raw)
	if err != nil {
		return strings.ToLower(raw)
	}
	return strings.ToLower(strings.TrimSpace(mediaType))
}

func buildReceiptCodeDataURL(contentType string, data []byte) string {
	contentType = normalizeContentType(contentType)
	if contentType == "" {
		contentType = "application/octet-stream"
	}
	return "data:" + contentType + ";base64," + base64.StdEncoding.EncodeToString(data)
}

func buildReceiptCodeInlineKey(userID int64, paymentMethod, ext string) string {
	if !strings.HasPrefix(ext, ".") {
		ext = "." + ext
	}
	return fmt.Sprintf("receipt-codes-inline/%d/%s-%s%s", userID, paymentMethod, uuid.NewString(), ext)
}

func buildReceiptCodeObjectKey(prefix string, userID int64, paymentMethod, ext string) string {
	normalizedPrefix := strings.Trim(strings.ReplaceAll(prefix, "\\", "/"), "/")
	if normalizedPrefix == "" {
		normalizedPrefix = "receipt-codes"
	}
	if !strings.HasPrefix(ext, ".") {
		ext = "." + ext
	}
	return fmt.Sprintf("%s/%d/%s-%s%s", normalizedPrefix, userID, paymentMethod, uuid.NewString(), ext)
}
