package service

import (
	"archive/zip"
	"bytes"
	"context"
	"crypto/sha256"
	"database/sql"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"mime"
	"net/http"
	"path"
	"path/filepath"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"time"

	dbent "ikik-api/ent"
	"ikik-api/internal/config"
	"ikik-api/internal/payment"
	infraerrors "ikik-api/internal/pkg/errors"
)

const (
	settingShopFileCardOSSEnabled         = "shop_file_card_oss_enabled"
	settingShopFileCardOSSEndpoint        = "shop_file_card_oss_endpoint"
	settingShopFileCardOSSRegion          = "shop_file_card_oss_region"
	settingShopFileCardOSSBucket          = "shop_file_card_oss_bucket"
	settingShopFileCardOSSAccessKeyID     = "shop_file_card_oss_access_key_id"
	settingShopFileCardOSSSecretAccessKey = "shop_file_card_oss_secret_access_key"
	settingShopFileCardOSSPrefix          = "shop_file_card_oss_prefix"
	settingShopFileCardOSSForcePathStyle  = "shop_file_card_oss_force_path_style"

	defaultShopFileCardOSSRegion = "oss-cn-hangzhou"
	defaultShopFileCardOSSPrefix = "shop-file-cards/"
)

type ShopFileCardObjectStore interface {
	Upload(ctx context.Context, key string, body io.Reader, contentType string) error
	Download(ctx context.Context, key string) (io.ReadCloser, error)
	Delete(ctx context.Context, key string) error
	HeadBucket(ctx context.Context) error
}

type ShopFileCardObjectStoreFactory func(ctx context.Context, cfg ShopFileCardStorageConfig) (ShopFileCardObjectStore, error)

type ShopFileCardStorageConfig struct {
	Enabled                   bool   `json:"enabled"`
	Endpoint                  string `json:"endpoint"`
	Region                    string `json:"region"`
	Bucket                    string `json:"bucket"`
	AccessKeyID               string `json:"access_key_id"`
	SecretAccessKey           string `json:"secret_access_key,omitempty"`
	SecretAccessKeyConfigured bool   `json:"secret_access_key_configured"`
	Prefix                    string `json:"prefix"`
	ForcePathStyle            bool   `json:"force_path_style"`
	MaxSizeBytes              int64  `json:"max_size_bytes"`
}

type UpdateShopFileCardStorageConfigRequest struct {
	Enabled         *bool   `json:"enabled"`
	Endpoint        *string `json:"endpoint"`
	Region          *string `json:"region"`
	Bucket          *string `json:"bucket"`
	AccessKeyID     *string `json:"access_key_id"`
	SecretAccessKey *string `json:"secret_access_key"`
	Prefix          *string `json:"prefix"`
	ForcePathStyle  *bool   `json:"force_path_style"`
}

type ShopFileCardUpload struct {
	Filename    string
	ContentType string
	Reader      io.Reader
}

type ShopFileCardDownload struct {
	File ShopDeliveredFileDTO
	Body io.ReadCloser
}

type ShopFileCardArchiveItem struct {
	File ShopDeliveredFileDTO
	Body io.ReadCloser
}

type preparedShopFileCardUpload struct {
	Filename    string
	ContentType string
	Data        []byte
	SHA256      string
	StorageKey  string
}

type shopFileCardMeta struct {
	ID               int64
	CardType         string
	StorageProvider  sql.NullString
	StorageKey       sql.NullString
	OriginalFilename sql.NullString
	ContentType      sql.NullString
	ByteSize         sql.NullInt64
	SHA256           sql.NullString
}

type shopSQLQueryer interface {
	QueryContext(ctx context.Context, query string, args ...any) (*sql.Rows, error)
}

type shopSQLExecer interface {
	ExecContext(ctx context.Context, query string, args ...any) (sql.Result, error)
}

func (s *ShopService) GetFileCardStorageConfig(ctx context.Context) (*ShopFileCardStorageConfig, error) {
	cfg, err := s.loadFileCardStorageConfig(ctx)
	if err != nil {
		return nil, err
	}
	cfg.SecretAccessKey = ""
	return cfg, nil
}

func (s *ShopService) UpdateFileCardStorageConfig(ctx context.Context, req UpdateShopFileCardStorageConfigRequest) (*ShopFileCardStorageConfig, error) {
	if s.settingRepo == nil {
		return nil, ErrShopFileCardOSSNotConfigured
	}
	current, err := s.loadFileCardStorageConfig(ctx)
	if err != nil {
		return nil, err
	}
	next := *current
	if req.Enabled != nil {
		next.Enabled = *req.Enabled
	}
	if req.Endpoint != nil {
		next.Endpoint = strings.TrimRight(strings.TrimSpace(*req.Endpoint), "/")
	}
	if req.Region != nil {
		next.Region = strings.TrimSpace(*req.Region)
	}
	if req.Bucket != nil {
		next.Bucket = strings.TrimSpace(*req.Bucket)
	}
	if req.AccessKeyID != nil {
		next.AccessKeyID = strings.TrimSpace(*req.AccessKeyID)
	}
	if req.SecretAccessKey != nil && strings.TrimSpace(*req.SecretAccessKey) != "" {
		next.SecretAccessKey = strings.TrimSpace(*req.SecretAccessKey)
	}
	if req.Prefix != nil {
		next.Prefix = normalizeShopFileCardOSSPrefix(*req.Prefix)
	}
	if req.ForcePathStyle != nil {
		next.ForcePathStyle = *req.ForcePathStyle
	}
	if next.Region == "" {
		next.Region = defaultShopFileCardOSSRegion
	}
	if next.Prefix == "" {
		next.Prefix = defaultShopFileCardOSSPrefix
	}
	next.MaxSizeBytes = ShopFileCardMaxSizeBytes
	if err := validateShopFileCardStorageConfig(next); err != nil {
		return nil, err
	}
	secret, err := s.encryptShopFileCardOSSSecret(next.SecretAccessKey)
	if err != nil {
		return nil, err
	}
	updates := map[string]string{
		settingShopFileCardOSSEnabled:         strconv.FormatBool(next.Enabled),
		settingShopFileCardOSSEndpoint:        strings.TrimRight(strings.TrimSpace(next.Endpoint), "/"),
		settingShopFileCardOSSRegion:          strings.TrimSpace(next.Region),
		settingShopFileCardOSSBucket:          strings.TrimSpace(next.Bucket),
		settingShopFileCardOSSAccessKeyID:     strings.TrimSpace(next.AccessKeyID),
		settingShopFileCardOSSSecretAccessKey: secret,
		settingShopFileCardOSSPrefix:          normalizeShopFileCardOSSPrefix(next.Prefix),
		settingShopFileCardOSSForcePathStyle:  strconv.FormatBool(next.ForcePathStyle),
	}
	if err := s.settingRepo.SetMultiple(ctx, updates); err != nil {
		return nil, fmt.Errorf("update shop file card oss config: %w", err)
	}
	next.SecretAccessKey = ""
	next.SecretAccessKeyConfigured = secret != ""
	return &next, nil
}

func (s *ShopService) TestFileCardStorageConfig(ctx context.Context, req *UpdateShopFileCardStorageConfigRequest) error {
	cfg, err := s.loadFileCardStorageConfig(ctx)
	if err != nil {
		return err
	}
	if req != nil {
		next := *cfg
		if req.Enabled != nil {
			next.Enabled = *req.Enabled
		}
		if req.Endpoint != nil {
			next.Endpoint = strings.TrimRight(strings.TrimSpace(*req.Endpoint), "/")
		}
		if req.Region != nil {
			next.Region = strings.TrimSpace(*req.Region)
		}
		if req.Bucket != nil {
			next.Bucket = strings.TrimSpace(*req.Bucket)
		}
		if req.AccessKeyID != nil {
			next.AccessKeyID = strings.TrimSpace(*req.AccessKeyID)
		}
		if req.SecretAccessKey != nil && strings.TrimSpace(*req.SecretAccessKey) != "" {
			next.SecretAccessKey = strings.TrimSpace(*req.SecretAccessKey)
		}
		if req.Prefix != nil {
			next.Prefix = normalizeShopFileCardOSSPrefix(*req.Prefix)
		}
		if req.ForcePathStyle != nil {
			next.ForcePathStyle = *req.ForcePathStyle
		}
		next.MaxSizeBytes = ShopFileCardMaxSizeBytes
		cfg = &next
	}
	if err := validateShopFileCardStorageConfig(*cfg); err != nil {
		return err
	}
	store, err := s.fileCardStore(ctx, *cfg)
	if err != nil {
		return err
	}
	return store.HeadBucket(ctx)
}

func (s *ShopService) AdminImportFileCardKeys(ctx context.Context, productID int64, uploads []ShopFileCardUpload) ([]ShopCardKeyDTO, error) {
	if len(uploads) == 0 {
		return nil, ErrShopInvalidInput
	}
	if err := s.ensureProductExists(ctx, productID); err != nil {
		return nil, err
	}
	cfg, err := s.loadFileCardStorageConfig(ctx)
	if err != nil {
		return nil, err
	}
	if err := validateShopFileCardStorageConfig(*cfg); err != nil {
		return nil, err
	}
	store, err := s.fileCardStore(ctx, *cfg)
	if err != nil {
		return nil, err
	}

	for _, upload := range uploads {
		if err := validateShopFileCardUpload(upload); err != nil {
			return nil, err
		}
	}

	prepared := make([]preparedShopFileCardUpload, 0, len(uploads))
	for _, upload := range uploads {
		item, err := s.prepareFileCardUpload(*cfg, productID, upload)
		if err != nil {
			return nil, err
		}
		prepared = append(prepared, item)
	}

	tx, err := s.entClient.Tx(ctx)
	if err != nil {
		return nil, fmt.Errorf("begin shop file card import transaction: %w", err)
	}
	committed := false
	defer func() {
		if !committed {
			_ = tx.Rollback()
		}
	}()
	queryer, ok := tx.Driver().(shopSQLQueryer)
	if !ok {
		return nil, fmt.Errorf("shop file card insert requires QueryContext support")
	}

	uploadedKeys := make([]string, 0, len(prepared))
	created := make([]ShopCardKeyDTO, 0, len(prepared))
	for _, item := range prepared {
		if err := store.Upload(ctx, item.StorageKey, bytes.NewReader(item.Data), item.ContentType); err != nil {
			cleanupErr := cleanupUploadedFileCardObjects(ctx, store, uploadedKeys)
			return nil, joinShopFileCardErrors(fmt.Errorf("upload shop file card: %w", err), cleanupErr)
		}
		uploadedKeys = append(uploadedKeys, item.StorageKey)
		dto, err := s.insertFileCardKeyWithQueryer(ctx, queryer, productID, item.StorageKey, item.Filename, item.ContentType, int64(len(item.Data)), item.SHA256)
		if err != nil {
			cleanupErr := cleanupUploadedFileCardObjects(ctx, store, uploadedKeys)
			return nil, joinShopFileCardErrors(err, cleanupErr)
		}
		created = append(created, *dto)
	}
	if err := tx.Commit(); err != nil {
		cleanupErr := cleanupUploadedFileCardObjects(ctx, store, uploadedKeys)
		return nil, joinShopFileCardErrors(fmt.Errorf("commit shop file card import transaction: %w", err), cleanupErr)
	}
	committed = true
	return created, nil
}

func (s *ShopService) GetOrderFileCardDownload(ctx context.Context, userID, orderID, cardID int64) (*ShopFileCardDownload, error) {
	file, err := s.getOrderFileCard(ctx, userID, orderID, cardID)
	if err != nil {
		return nil, err
	}
	store, err := s.fileCardStoreFromSettings(ctx)
	if err != nil {
		return nil, err
	}
	body, err := store.Download(ctx, file.StorageKey)
	if err != nil {
		return nil, fmt.Errorf("download shop file card: %w", err)
	}
	return &ShopFileCardDownload{File: file, Body: body}, nil
}

func (s *ShopService) GetOrderFileCardDownloadForAdmin(ctx context.Context, orderID, cardID int64) (*ShopFileCardDownload, error) {
	file, err := s.getOrderFileCardForAdmin(ctx, orderID, cardID)
	if err != nil {
		return nil, err
	}
	store, err := s.fileCardStoreFromSettings(ctx)
	if err != nil {
		return nil, err
	}
	body, err := store.Download(ctx, file.StorageKey)
	if err != nil {
		return nil, fmt.Errorf("download shop file card: %w", err)
	}
	return &ShopFileCardDownload{File: file, Body: body}, nil
}

func (s *ShopService) WriteOrderFileCardArchive(ctx context.Context, userID, orderID int64, dst io.Writer) (string, error) {
	if dst == nil {
		return "", ErrShopInvalidInput
	}
	files, err := s.listDeliveredFileCardsForUser(ctx, userID, orderID)
	if err != nil {
		return "", err
	}
	return s.writeOrderFileCardArchive(ctx, orderID, files, dst)
}

func (s *ShopService) WriteOrderFileCardArchiveForAdmin(ctx context.Context, orderID int64, dst io.Writer) (string, error) {
	if dst == nil {
		return "", ErrShopInvalidInput
	}
	files, err := s.listDeliveredFileCardsForAdmin(ctx, orderID)
	if err != nil {
		return "", err
	}
	return s.writeOrderFileCardArchive(ctx, orderID, files, dst)
}

func (s *ShopService) writeOrderFileCardArchive(ctx context.Context, orderID int64, files []ShopDeliveredFileDTO, dst io.Writer) (string, error) {
	if len(files) == 0 {
		return "", ErrShopFileCardUnavailable
	}
	store, err := s.fileCardStoreFromSettings(ctx)
	if err != nil {
		return "", err
	}
	zw := zip.NewWriter(dst)
	usedNames := make(map[string]int, len(files))
	for _, file := range files {
		body, err := store.Download(ctx, file.StorageKey)
		if err != nil {
			_ = zw.Close()
			return "", fmt.Errorf("download shop file card for archive: %w", err)
		}
		name := uniqueArchiveFilename(sanitizeShopFilename(file.Filename), usedNames)
		header := &zip.FileHeader{
			Name:   name,
			Method: zip.Deflate,
		}
		header.Modified = time.Now()
		writer, err := zw.CreateHeader(header)
		if err != nil {
			_ = body.Close()
			_ = zw.Close()
			return "", fmt.Errorf("create shop file card archive entry: %w", err)
		}
		if _, err := io.Copy(writer, body); err != nil {
			_ = body.Close()
			_ = zw.Close()
			return "", fmt.Errorf("write shop file card archive entry: %w", err)
		}
		if err := body.Close(); err != nil {
			_ = zw.Close()
			return "", fmt.Errorf("close shop file card object: %w", err)
		}
	}
	if err := zw.Close(); err != nil {
		return "", fmt.Errorf("close shop file card archive: %w", err)
	}
	return fmt.Sprintf("shop-order-%d-files.zip", orderID), nil
}

func (s *ShopService) importOneFileCard(ctx context.Context, store ShopFileCardObjectStore, cfg ShopFileCardStorageConfig, productID int64, upload ShopFileCardUpload) (*ShopCardKeyDTO, string, error) {
	item, err := s.prepareFileCardUpload(cfg, productID, upload)
	if err != nil {
		return nil, "", err
	}
	if err := store.Upload(ctx, item.StorageKey, bytes.NewReader(item.Data), item.ContentType); err != nil {
		return nil, "", fmt.Errorf("upload shop file card: %w", err)
	}
	dto, err := s.insertFileCardKey(ctx, productID, item.StorageKey, item.Filename, item.ContentType, int64(len(item.Data)), item.SHA256)
	if err != nil {
		if deleteErr := store.Delete(ctx, item.StorageKey); deleteErr != nil {
			return nil, item.StorageKey, fmt.Errorf("insert shop file card key: %w; cleanup uploaded object: %v", err, deleteErr)
		}
		return nil, item.StorageKey, fmt.Errorf("insert shop file card key: %w", err)
	}
	return dto, item.StorageKey, nil
}

func (s *ShopService) prepareFileCardUpload(cfg ShopFileCardStorageConfig, productID int64, upload ShopFileCardUpload) (preparedShopFileCardUpload, error) {
	filename := sanitizeShopFilename(upload.Filename)
	data, err := readShopFileCardUpload(upload.Reader, ShopFileCardMaxSizeBytes)
	if err != nil {
		return preparedShopFileCardUpload{}, err
	}
	contentType := normalizeShopFileContentType(upload.ContentType, filename, data)
	sum := sha256.Sum256(data)
	sha := hex.EncodeToString(sum[:])
	storageKey := s.buildFileCardStorageKey(cfg, productID, filename)
	return preparedShopFileCardUpload{
		Filename:    filename,
		ContentType: contentType,
		Data:        data,
		SHA256:      sha,
		StorageKey:  storageKey,
	}, nil
}

func validateShopFileCardUpload(upload ShopFileCardUpload) error {
	filename := sanitizeShopFilename(upload.Filename)
	if filename == "" {
		return ErrShopInvalidInput
	}
	if upload.Reader == nil {
		return ErrShopInvalidInput
	}
	return nil
}

func (s *ShopService) loadFileCardStorageConfig(ctx context.Context) (*ShopFileCardStorageConfig, error) {
	if s.settingRepo == nil {
		return &ShopFileCardStorageConfig{
			Region:       defaultShopFileCardOSSRegion,
			Prefix:       defaultShopFileCardOSSPrefix,
			MaxSizeBytes: ShopFileCardMaxSizeBytes,
		}, nil
	}
	keys := []string{
		settingShopFileCardOSSEnabled,
		settingShopFileCardOSSEndpoint,
		settingShopFileCardOSSRegion,
		settingShopFileCardOSSBucket,
		settingShopFileCardOSSAccessKeyID,
		settingShopFileCardOSSSecretAccessKey,
		settingShopFileCardOSSPrefix,
		settingShopFileCardOSSForcePathStyle,
	}
	vals, err := s.settingRepo.GetMultiple(ctx, keys)
	if err != nil {
		return nil, fmt.Errorf("get shop file card oss config: %w", err)
	}
	secret := s.decryptShopFileCardOSSSecret(vals[settingShopFileCardOSSSecretAccessKey])
	cfg := &ShopFileCardStorageConfig{
		Enabled:                   parseBoolWithDefault(vals[settingShopFileCardOSSEnabled], false),
		Endpoint:                  strings.TrimRight(strings.TrimSpace(vals[settingShopFileCardOSSEndpoint]), "/"),
		Region:                    firstNonEmpty(vals[settingShopFileCardOSSRegion], defaultShopFileCardOSSRegion),
		Bucket:                    strings.TrimSpace(vals[settingShopFileCardOSSBucket]),
		AccessKeyID:               strings.TrimSpace(vals[settingShopFileCardOSSAccessKeyID]),
		SecretAccessKey:           secret,
		SecretAccessKeyConfigured: secret != "",
		Prefix:                    normalizeShopFileCardOSSPrefix(vals[settingShopFileCardOSSPrefix]),
		ForcePathStyle:            parseBoolWithDefault(vals[settingShopFileCardOSSForcePathStyle], false),
		MaxSizeBytes:              ShopFileCardMaxSizeBytes,
	}
	return cfg, nil
}

func (s *ShopService) fileCardStoreFromSettings(ctx context.Context) (ShopFileCardObjectStore, error) {
	cfg, err := s.loadFileCardStorageConfig(ctx)
	if err != nil {
		return nil, err
	}
	if err := validateShopFileCardStorageConfig(*cfg); err != nil {
		return nil, err
	}
	return s.fileCardStore(ctx, *cfg)
}

func (s *ShopService) fileCardStore(ctx context.Context, cfg ShopFileCardStorageConfig) (ShopFileCardObjectStore, error) {
	if s.fileCardStoreFactory == nil {
		return nil, ErrShopFileCardOSSNotConfigured
	}
	return s.fileCardStoreFactory(ctx, cfg)
}

func (s *ShopService) encryptShopFileCardOSSSecret(secret string) (string, error) {
	secret = strings.TrimSpace(secret)
	if secret == "" {
		return "", nil
	}
	payload, err := json.Marshal(map[string]string{"secret": secret})
	if err != nil {
		return "", fmt.Errorf("marshal shop file card oss secret: %w", err)
	}
	if len(s.encryptionKey) != payment.AES256KeySize {
		return "", infraerrors.BadRequest("SHOP_FILE_CARD_OSS_ENCRYPTION_KEY_REQUIRED", "TOTP_ENCRYPTION_KEY must be configured before saving shop file card OSS secret")
	}
	//nolint:staticcheck // SA1019: reused for settings secret storage to match existing payment OSS config handling.
	encrypted, err := payment.Encrypt(string(payload), s.encryptionKey)
	if err != nil {
		return "", fmt.Errorf("encrypt shop file card oss secret: %w", err)
	}
	return encrypted, nil
}

func (s *ShopService) decryptShopFileCardOSSSecret(stored string) string {
	stored = strings.TrimSpace(stored)
	if stored == "" {
		return ""
	}
	if len(s.encryptionKey) == payment.AES256KeySize {
		//nolint:staticcheck // SA1019: see encryptShopFileCardOSSSecret.
		if plaintext, err := payment.Decrypt(stored, s.encryptionKey); err == nil {
			var payload map[string]string
			if err := json.Unmarshal([]byte(plaintext), &payload); err == nil {
				return strings.TrimSpace(payload["secret"])
			}
		}
	}
	if strings.HasPrefix(stored, "{") {
		var payload map[string]string
		if err := json.Unmarshal([]byte(stored), &payload); err == nil {
			return strings.TrimSpace(payload["secret"])
		}
	}
	return ""
}

func validateShopFileCardStorageConfig(cfg ShopFileCardStorageConfig) error {
	if cfg.MaxSizeBytes != ShopFileCardMaxSizeBytes {
		return infraerrors.BadRequest("INVALID_SHOP_FILE_CARD_MAX_SIZE", "shop file card max size is fixed to 204800 bytes")
	}
	if endpoint := strings.TrimSpace(cfg.Endpoint); endpoint != "" {
		if err := config.ValidateAbsoluteHTTPURL(endpoint); err != nil {
			return infraerrors.BadRequest("INVALID_SHOP_FILE_CARD_OSS_ENDPOINT", "shop file card OSS endpoint must be an absolute http(s) URL")
		}
	}
	if !cfg.Enabled {
		return ErrShopFileCardOSSNotConfigured
	}
	if strings.TrimSpace(cfg.Endpoint) == "" {
		return infraerrors.BadRequest("SHOP_FILE_CARD_OSS_ENDPOINT_REQUIRED", "shop file card OSS endpoint is required when enabled")
	}
	if strings.TrimSpace(cfg.Bucket) == "" {
		return infraerrors.BadRequest("SHOP_FILE_CARD_OSS_BUCKET_REQUIRED", "shop file card OSS bucket is required when enabled")
	}
	if strings.TrimSpace(cfg.AccessKeyID) == "" {
		return infraerrors.BadRequest("SHOP_FILE_CARD_OSS_ACCESS_KEY_REQUIRED", "shop file card OSS access key ID is required when enabled")
	}
	if strings.TrimSpace(cfg.SecretAccessKey) == "" {
		return infraerrors.BadRequest("SHOP_FILE_CARD_OSS_SECRET_REQUIRED", "shop file card OSS secret access key is required when enabled")
	}
	return nil
}

func normalizeShopFileCardOSSPrefix(raw string) string {
	prefix := strings.Trim(strings.ReplaceAll(strings.TrimSpace(raw), "\\", "/"), "/")
	if prefix == "" {
		return defaultShopFileCardOSSPrefix
	}
	return prefix + "/"
}

func readShopFileCardUpload(r io.Reader, maxBytes int64) ([]byte, error) {
	if r == nil {
		return nil, ErrShopInvalidInput
	}
	limited := io.LimitReader(r, maxBytes+1)
	data, err := io.ReadAll(limited)
	if err != nil {
		return nil, fmt.Errorf("read shop file card upload: %w", err)
	}
	if len(data) == 0 {
		return nil, ErrShopFileCardEmpty
	}
	if int64(len(data)) > maxBytes {
		return nil, ErrShopFileCardTooLarge
	}
	return data, nil
}

func normalizeShopFileContentType(raw, filename string, data []byte) string {
	if contentType := strings.TrimSpace(raw); contentType != "" {
		if idx := strings.Index(contentType, ";"); idx >= 0 {
			contentType = contentType[:idx]
		}
		return contentType
	}
	if ext := strings.ToLower(filepath.Ext(filename)); ext != "" {
		if contentType := mime.TypeByExtension(ext); strings.TrimSpace(contentType) != "" {
			if idx := strings.Index(contentType, ";"); idx >= 0 {
				contentType = contentType[:idx]
			}
			return contentType
		}
	}
	return http.DetectContentType(data)
}

var shopUnsafeFilenameChars = regexp.MustCompile(`[^A-Za-z0-9._\-\p{Han}]`)

func sanitizeShopFilename(raw string) string {
	name := filepath.Base(strings.ReplaceAll(strings.TrimSpace(raw), "\\", "/"))
	name = strings.TrimSpace(name)
	if name == "." || name == "/" || name == "" {
		return "card-file"
	}
	name = shopUnsafeFilenameChars.ReplaceAllString(name, "_")
	name = strings.Trim(name, " .")
	if name == "" {
		return "card-file"
	}
	runes := []rune(name)
	if len(runes) > 120 {
		ext := filepath.Ext(name)
		base := strings.TrimSuffix(name, ext)
		baseRunes := []rune(base)
		maxBase := 120 - len([]rune(ext))
		if maxBase < 20 {
			maxBase = 20
		}
		if len(baseRunes) > maxBase {
			base = string(baseRunes[:maxBase])
		}
		name = base + ext
	}
	return name
}

func uniqueArchiveFilename(name string, used map[string]int) string {
	name = sanitizeShopFilename(name)
	if used[name] == 0 {
		used[name] = 1
		return name
	}
	used[name]++
	ext := path.Ext(name)
	base := strings.TrimSuffix(name, ext)
	return fmt.Sprintf("%s-%d%s", base, used[name], ext)
}

func (s *ShopService) buildFileCardStorageKey(cfg ShopFileCardStorageConfig, productID int64, filename string) string {
	date := time.Now().UTC().Format("20060102")
	ext := strings.ToLower(filepath.Ext(filename))
	random := strings.ToLower(generateRandomString(24))
	return normalizeShopFileCardOSSPrefix(cfg.Prefix) + fmt.Sprintf("product-%d/%s/%s%s", productID, date, random, ext)
}

func (s *ShopService) insertFileCardKey(ctx context.Context, productID int64, storageKey, filename, contentType string, byteSize int64, sha string) (*ShopCardKeyDTO, error) {
	queryer, ok := s.entClient.Driver().(shopSQLQueryer)
	if !ok {
		return nil, fmt.Errorf("shop file card insert requires QueryContext support")
	}
	return s.insertFileCardKeyWithQueryer(ctx, queryer, productID, storageKey, filename, contentType, byteSize, sha)
}

func (s *ShopService) insertFileCardKeyWithQueryer(ctx context.Context, queryer shopSQLQueryer, productID int64, storageKey, filename, contentType string, byteSize int64, sha string) (*ShopCardKeyDTO, error) {
	if queryer == nil {
		return nil, fmt.Errorf("shop file card insert requires QueryContext support")
	}
	now := time.Now()
	rows, err := queryer.QueryContext(ctx, `
		INSERT INTO shop_card_keys (
			product_id, content, card_type, storage_provider, storage_key,
			original_filename, content_type, byte_size, sha256, status,
			created_at, updated_at
		)
		VALUES ($1, '', $2, $3, $4, $5, $6, $7, $8, $9, $10, $11)
		RETURNING id, product_id, status, created_at, updated_at
	`, productID, ShopCardTypeFile, ShopFileCardStorageProviderOSS, storageKey, filename, contentType, byteSize, sha, ShopCardStatusAvailable, now, now)
	if err != nil {
		return nil, fmt.Errorf("insert shop file card key: %w", err)
	}
	defer func() { _ = rows.Close() }()
	if !rows.Next() {
		if err := rows.Err(); err != nil {
			return nil, fmt.Errorf("scan inserted shop file card key: %w", err)
		}
		return nil, fmt.Errorf("insert shop file card key returned no rows")
	}
	var dto ShopCardKeyDTO
	if err := rows.Scan(&dto.ID, &dto.ProductID, &dto.Status, &dto.CreatedAt, &dto.UpdatedAt); err != nil {
		return nil, fmt.Errorf("scan inserted shop file card key: %w", err)
	}
	dto.CardType = ShopCardTypeFile
	provider := ShopFileCardStorageProviderOSS
	dto.StorageProvider = &provider
	dto.OriginalFilename = &filename
	dto.ContentType = &contentType
	dto.ByteSize = &byteSize
	dto.SHA256 = &sha
	return &dto, rows.Err()
}

func cleanupUploadedFileCardObjects(ctx context.Context, store ShopFileCardObjectStore, keys []string) error {
	if store == nil || len(keys) == 0 {
		return nil
	}
	var cleanupErr error
	for i := len(keys) - 1; i >= 0; i-- {
		key := strings.TrimSpace(keys[i])
		if key == "" {
			continue
		}
		if err := store.Delete(ctx, key); err != nil {
			cleanupErr = errors.Join(cleanupErr, fmt.Errorf("delete uploaded shop file card object %q: %w", key, err))
		}
	}
	return cleanupErr
}

func joinShopFileCardErrors(primary, cleanup error) error {
	if cleanup == nil {
		return primary
	}
	return errors.Join(primary, fmt.Errorf("cleanup uploaded shop file card objects: %w", cleanup))
}

func (s *ShopService) decorateCardKeyDTOsWithFileMeta(ctx context.Context, items []ShopCardKeyDTO) ([]ShopCardKeyDTO, error) {
	ids := make([]int64, 0, len(items))
	for _, item := range items {
		ids = append(ids, item.ID)
	}
	metadata, err := s.fileCardMetaByIDs(ctx, ids)
	if err != nil {
		if isShopUndefinedColumnError(err) {
			return items, nil
		}
		return nil, err
	}
	for i := range items {
		if meta, ok := metadata[items[i].ID]; ok {
			applyFileMetaToCardDTO(&items[i], meta)
		} else if items[i].CardType == "" {
			items[i].CardType = ShopCardTypeText
		}
	}
	return items, nil
}

func (s *ShopService) fileCardMetaByIDs(ctx context.Context, ids []int64) (map[int64]shopFileCardMeta, error) {
	queryer, ok := s.entClient.Driver().(shopSQLQueryer)
	if !ok {
		return make(map[int64]shopFileCardMeta, len(ids)), nil
	}
	return fileCardMetaByIDsWithQueryer(ctx, queryer, ids)
}

func fileCardMetaByIDsWithQueryer(ctx context.Context, queryer shopSQLQueryer, ids []int64) (map[int64]shopFileCardMeta, error) {
	out := make(map[int64]shopFileCardMeta, len(ids))
	if len(ids) == 0 {
		return out, nil
	}
	if queryer == nil {
		return out, nil
	}
	placeholders := make([]string, len(ids))
	args := make([]any, len(ids))
	for i, id := range ids {
		placeholders[i] = "$" + strconv.Itoa(i+1)
		args[i] = id
	}
	rows, err := queryer.QueryContext(ctx, `
		SELECT id, card_type, storage_provider, storage_key, original_filename, content_type, byte_size, sha256
		FROM shop_card_keys
		WHERE id IN (`+strings.Join(placeholders, ",")+`)
	`, args...)
	if err != nil {
		return nil, fmt.Errorf("query shop file card metadata: %w", err)
	}
	defer func() { _ = rows.Close() }()
	for rows.Next() {
		var meta shopFileCardMeta
		if err := rows.Scan(&meta.ID, &meta.CardType, &meta.StorageProvider, &meta.StorageKey, &meta.OriginalFilename, &meta.ContentType, &meta.ByteSize, &meta.SHA256); err != nil {
			return nil, fmt.Errorf("scan shop file card metadata: %w", err)
		}
		out[meta.ID] = meta
	}
	return out, rows.Err()
}

func applyFileMetaToCardDTO(dto *ShopCardKeyDTO, meta shopFileCardMeta) {
	if dto == nil {
		return
	}
	dto.CardType = firstNonEmpty(meta.CardType, ShopCardTypeText)
	if dto.CardType != ShopCardTypeFile {
		return
	}
	dto.Content = ""
	if meta.StorageProvider.Valid {
		v := meta.StorageProvider.String
		dto.StorageProvider = &v
	}
	if meta.OriginalFilename.Valid {
		v := meta.OriginalFilename.String
		dto.OriginalFilename = &v
	}
	if meta.ContentType.Valid {
		v := meta.ContentType.String
		dto.ContentType = &v
	}
	if meta.ByteSize.Valid {
		v := meta.ByteSize.Int64
		dto.ByteSize = &v
	}
	if meta.SHA256.Valid {
		v := meta.SHA256.String
		dto.SHA256 = &v
	}
}

func (s *ShopService) hydrateOrderDeliveredFiles(ctx context.Context, dto *ShopOrderDTO) error {
	if dto == nil || dto.Status != ShopOrderStatusCompleted {
		return nil
	}
	files, err := s.listDeliveredFileCards(ctx, dto.ID)
	if err != nil {
		if isShopUndefinedColumnError(err) {
			dto.DeliveredFiles = []ShopDeliveredFileDTO{}
			return nil
		}
		return err
	}
	dto.DeliveredFiles = files
	return nil
}

func (s *ShopService) listDeliveredFileCards(ctx context.Context, orderID int64) ([]ShopDeliveredFileDTO, error) {
	queryer, ok := s.entClient.Driver().(shopSQLQueryer)
	if !ok {
		return []ShopDeliveredFileDTO{}, nil
	}
	return queryDeliveredFileCards(ctx, queryer, `
		SELECT id, original_filename, content_type, byte_size, sha256, storage_key
		FROM shop_card_keys
		WHERE order_id = $1 AND status = $2 AND card_type = $3
		ORDER BY id ASC
	`, orderID, ShopCardStatusSold, ShopCardTypeFile)
}

func (s *ShopService) listDeliveredFileCardsForUser(ctx context.Context, userID, orderID int64) ([]ShopDeliveredFileDTO, error) {
	queryer, ok := s.entClient.Driver().(shopSQLQueryer)
	if !ok {
		return []ShopDeliveredFileDTO{}, nil
	}
	return queryDeliveredFileCards(ctx, queryer, `
		SELECT ck.id, ck.original_filename, ck.content_type, ck.byte_size, ck.sha256, ck.storage_key
		FROM shop_card_keys ck
		INNER JOIN shop_orders o ON o.id = ck.order_id
		WHERE o.id = $1
			AND o.user_id = $2
			AND o.status = $3
			AND ck.status = $4
			AND ck.card_type = $5
		ORDER BY ck.id ASC
	`, orderID, userID, ShopOrderStatusCompleted, ShopCardStatusSold, ShopCardTypeFile)
}

func (s *ShopService) listDeliveredFileCardsForAdmin(ctx context.Context, orderID int64) ([]ShopDeliveredFileDTO, error) {
	queryer, ok := s.entClient.Driver().(shopSQLQueryer)
	if !ok {
		return []ShopDeliveredFileDTO{}, nil
	}
	return queryDeliveredFileCards(ctx, queryer, `
		SELECT ck.id, ck.original_filename, ck.content_type, ck.byte_size, ck.sha256, ck.storage_key
		FROM shop_card_keys ck
		INNER JOIN shop_orders o ON o.id = ck.order_id
		WHERE o.id = $1
			AND o.status = $2
			AND ck.status = $3
			AND ck.card_type = $4
		ORDER BY ck.id ASC
	`, orderID, ShopOrderStatusCompleted, ShopCardStatusSold, ShopCardTypeFile)
}

func (s *ShopService) getOrderFileCard(ctx context.Context, userID, orderID, cardID int64) (ShopDeliveredFileDTO, error) {
	queryer, ok := s.entClient.Driver().(shopSQLQueryer)
	if !ok {
		return ShopDeliveredFileDTO{}, ErrShopFileCardUnavailable
	}
	files, err := queryDeliveredFileCards(ctx, queryer, `
		SELECT ck.id, ck.original_filename, ck.content_type, ck.byte_size, ck.sha256, ck.storage_key
		FROM shop_card_keys ck
		INNER JOIN shop_orders o ON o.id = ck.order_id
		WHERE o.id = $1
			AND o.user_id = $2
			AND ck.id = $3
			AND o.status = $4
			AND ck.status = $5
			AND ck.card_type = $6
		LIMIT 1
	`, orderID, userID, cardID, ShopOrderStatusCompleted, ShopCardStatusSold, ShopCardTypeFile)
	if err != nil {
		if isShopUndefinedColumnError(err) {
			return ShopDeliveredFileDTO{}, ErrShopFileCardUnavailable
		}
		return ShopDeliveredFileDTO{}, err
	}
	if len(files) == 0 {
		return ShopDeliveredFileDTO{}, ErrShopFileCardUnavailable
	}
	return files[0], nil
}

func (s *ShopService) getOrderFileCardForAdmin(ctx context.Context, orderID, cardID int64) (ShopDeliveredFileDTO, error) {
	queryer, ok := s.entClient.Driver().(shopSQLQueryer)
	if !ok {
		return ShopDeliveredFileDTO{}, ErrShopFileCardUnavailable
	}
	files, err := queryDeliveredFileCards(ctx, queryer, `
		SELECT ck.id, ck.original_filename, ck.content_type, ck.byte_size, ck.sha256, ck.storage_key
		FROM shop_card_keys ck
		INNER JOIN shop_orders o ON o.id = ck.order_id
		WHERE o.id = $1
			AND ck.id = $2
			AND o.status = $3
			AND ck.status = $4
			AND ck.card_type = $5
		LIMIT 1
	`, orderID, cardID, ShopOrderStatusCompleted, ShopCardStatusSold, ShopCardTypeFile)
	if err != nil {
		if isShopUndefinedColumnError(err) {
			return ShopDeliveredFileDTO{}, ErrShopFileCardUnavailable
		}
		return ShopDeliveredFileDTO{}, err
	}
	if len(files) == 0 {
		return ShopDeliveredFileDTO{}, ErrShopFileCardUnavailable
	}
	return files[0], nil
}

func queryDeliveredFileCards(ctx context.Context, queryer shopSQLQueryer, query string, args ...any) ([]ShopDeliveredFileDTO, error) {
	rows, err := queryer.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, fmt.Errorf("query shop delivered file cards: %w", err)
	}
	defer func() { _ = rows.Close() }()
	files := make([]ShopDeliveredFileDTO, 0)
	for rows.Next() {
		var file ShopDeliveredFileDTO
		if err := rows.Scan(&file.ID, &file.Filename, &file.ContentType, &file.ByteSize, &file.SHA256, &file.StorageKey); err != nil {
			return nil, fmt.Errorf("scan shop delivered file card: %w", err)
		}
		files = append(files, file)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	return files, nil
}

func isShopUndefinedColumnError(err error) bool {
	if err == nil {
		return false
	}
	msg := strings.ToLower(err.Error())
	return strings.Contains(msg, "undefined column") || strings.Contains(msg, "no such column")
}

func (s *ShopService) cardTypesInTx(ctx context.Context, tx *dbent.Tx, cardIDs []int64) (map[int64]string, error) {
	out := make(map[int64]string, len(cardIDs))
	if len(cardIDs) == 0 {
		return out, nil
	}
	queryer, ok := tx.Driver().(shopSQLQueryer)
	if !ok {
		return out, nil
	}
	placeholders := make([]string, len(cardIDs))
	args := make([]any, len(cardIDs))
	for i, id := range cardIDs {
		placeholders[i] = "$" + strconv.Itoa(i+1)
		args[i] = id
	}
	rows, err := queryer.QueryContext(ctx, `
		SELECT id, card_type
		FROM shop_card_keys
		WHERE id IN (`+strings.Join(placeholders, ",")+`)
	`, args...)
	if err != nil {
		if isShopUndefinedColumnError(err) {
			return out, nil
		}
		return nil, fmt.Errorf("query shop card types: %w", err)
	}
	defer func() { _ = rows.Close() }()
	for rows.Next() {
		var id int64
		var cardType string
		if err := rows.Scan(&id, &cardType); err != nil {
			return nil, fmt.Errorf("scan shop card type: %w", err)
		}
		out[id] = cardType
	}
	return out, rows.Err()
}

func shopCardIDs(cards []*dbent.ShopCardKey) []int64 {
	ids := make([]int64, 0, len(cards))
	for _, card := range cards {
		ids = append(ids, card.ID)
	}
	sort.Slice(ids, func(i, j int) bool { return ids[i] < ids[j] })
	return ids
}
