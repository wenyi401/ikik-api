package service

import (
	"context"
	"encoding/json"
	"fmt"

	"strconv"
	"strings"

	"ikik-api/internal/config"
	"ikik-api/internal/payment"
	infraerrors "ikik-api/internal/pkg/errors"
)

func (s *PaymentConfigService) GetReceiptCodeStorageConfig(ctx context.Context) (config.ReceiptCodeStorageConfig, error) {
	cfg, err := s.GetPaymentConfig(ctx)
	if err != nil {
		return config.ReceiptCodeStorageConfig{}, err
	}
	oss := cfg.ReceiptCodeOSS
	return config.ReceiptCodeStorageConfig{
		Enabled:              oss.Enabled,
		Endpoint:             oss.Endpoint,
		Region:               oss.Region,
		Bucket:               oss.Bucket,
		AccessKeyID:          oss.AccessKeyID,
		SecretAccessKey:      oss.SecretAccessKey,
		Prefix:               oss.Prefix,
		PublicBaseURL:        oss.PublicBaseURL,
		ForcePathStyle:       oss.ForcePathStyle,
		MaxSizeBytes:         oss.MaxSizeBytes,
		PresignExpireSeconds: oss.PresignExpireSeconds,
	}, nil
}

type ReceiptCodeOSSConfig struct {
	Enabled                   bool   `json:"enabled"`
	Endpoint                  string `json:"endpoint"`
	Region                    string `json:"region"`
	Bucket                    string `json:"bucket"`
	AccessKeyID               string `json:"access_key_id"`
	SecretAccessKey           string `json:"-"`
	SecretAccessKeyConfigured bool   `json:"secret_access_key_configured"`
	Prefix                    string `json:"prefix"`
	PublicBaseURL             string `json:"public_base_url"`
	ForcePathStyle            bool   `json:"force_path_style"`
	MaxSizeBytes              int64  `json:"max_size_bytes"`
	PresignExpireSeconds      int    `json:"presign_expire_seconds"`
}

const (
	SettingPaymentReceiptCodeOSSAccessKeyID = "payment_receipt_code_oss_access_key_id"
)
const (
	SettingPaymentReceiptCodeOSSBucket = "payment_receipt_code_oss_bucket"
)
const (
	SettingPaymentReceiptCodeOSSEnabled = "payment_receipt_code_oss_enabled"
)
const (
	SettingPaymentReceiptCodeOSSEndpoint = "payment_receipt_code_oss_endpoint"
)
const (
	SettingPaymentReceiptCodeOSSForcePathStyle = "payment_receipt_code_oss_force_path_style"
)
const (
	SettingPaymentReceiptCodeOSSMaxSizeBytes = "payment_receipt_code_oss_max_size_bytes"
)
const (
	SettingPaymentReceiptCodeOSSPrefix = "payment_receipt_code_oss_prefix"
)
const (
	SettingPaymentReceiptCodeOSSPresignExpireSeconds = "payment_receipt_code_oss_presign_expire_seconds"
)
const (
	SettingPaymentReceiptCodeOSSPublicBaseURL = "payment_receipt_code_oss_public_base_url"
)
const (
	SettingPaymentReceiptCodeOSSRegion = "payment_receipt_code_oss_region"
)
const (
	SettingPaymentReceiptCodeOSSSecretAccessKey = "payment_receipt_code_oss_secret_access_key"
)

func (s *PaymentConfigService) buildReceiptCodeOSSUpdates(ctx context.Context, req UpdatePaymentConfigRequest) (map[string]string, error) {
	current, err := s.GetReceiptCodeStorageConfig(ctx)
	if err != nil {
		return nil, fmt.Errorf("get receipt code oss config: %w", err)
	}
	next := current
	if req.ReceiptCodeOSSEnabled != nil {
		next.Enabled = *req.ReceiptCodeOSSEnabled
	}
	if req.ReceiptCodeOSSEndpoint != nil {
		next.Endpoint = strings.TrimSpace(*req.ReceiptCodeOSSEndpoint)
	}
	if req.ReceiptCodeOSSRegion != nil {
		next.Region = strings.TrimSpace(*req.ReceiptCodeOSSRegion)
	}
	if req.ReceiptCodeOSSBucket != nil {
		next.Bucket = strings.TrimSpace(*req.ReceiptCodeOSSBucket)
	}
	secretProvided := req.ReceiptCodeOSSSecretAccessKey != nil && strings.TrimSpace(*req.ReceiptCodeOSSSecretAccessKey) != ""
	if req.ReceiptCodeOSSAccessKeyID != nil &&
		strings.TrimSpace(*req.ReceiptCodeOSSAccessKeyID) != strings.TrimSpace(current.AccessKeyID) &&
		!secretProvided {
		return nil, infraerrors.BadRequest(
			"RECEIPT_CODE_OSS_SECRET_REQUIRED_FOR_NEW_ACCESS_KEY",
			"enter the Secret Access Key that belongs to the new Access Key ID",
		)
	}
	if req.ReceiptCodeOSSAccessKeyID != nil {
		next.AccessKeyID = strings.TrimSpace(*req.ReceiptCodeOSSAccessKeyID)
	}
	if secretProvided {
		next.SecretAccessKey = strings.TrimSpace(*req.ReceiptCodeOSSSecretAccessKey)
	}
	if req.ReceiptCodeOSSPrefix != nil {
		next.Prefix = normalizeReceiptCodeOSSPrefix(*req.ReceiptCodeOSSPrefix)
	}
	if req.ReceiptCodeOSSPublicBaseURL != nil {
		next.PublicBaseURL = strings.TrimRight(strings.TrimSpace(*req.ReceiptCodeOSSPublicBaseURL), "/")
	}
	if req.ReceiptCodeOSSForcePathStyle != nil {
		next.ForcePathStyle = *req.ReceiptCodeOSSForcePathStyle
	}
	if req.ReceiptCodeOSSMaxSizeBytes != nil {
		next.MaxSizeBytes = *req.ReceiptCodeOSSMaxSizeBytes
	}
	if req.ReceiptCodeOSSPresignExpireSeconds != nil {
		next.PresignExpireSeconds = *req.ReceiptCodeOSSPresignExpireSeconds
	}
	if next.Region == "" {
		next.Region = defaultReceiptCodeOSSRegion
	}
	if next.Prefix == "" {
		next.Prefix = defaultReceiptCodeOSSPrefix
	}
	if next.MaxSizeBytes <= 0 {
		next.MaxSizeBytes = defaultReceiptCodeOSSMaxSizeBytes
	}
	if next.PresignExpireSeconds <= 0 {
		next.PresignExpireSeconds = defaultReceiptCodeOSSPresignExpireSeconds
	}
	if err := validateReceiptCodeOSSConfig(next); err != nil {
		return nil, err
	}
	secret, err := s.encryptReceiptCodeOSSSecret(next.SecretAccessKey)
	if err != nil {
		return nil, err
	}
	return map[string]string{
		SettingPaymentReceiptCodeOSSEnabled:              strconv.FormatBool(next.Enabled),
		SettingPaymentReceiptCodeOSSEndpoint:             strings.TrimSpace(next.Endpoint),
		SettingPaymentReceiptCodeOSSRegion:               strings.TrimSpace(next.Region),
		SettingPaymentReceiptCodeOSSBucket:               strings.TrimSpace(next.Bucket),
		SettingPaymentReceiptCodeOSSAccessKeyID:          strings.TrimSpace(next.AccessKeyID),
		SettingPaymentReceiptCodeOSSSecretAccessKey:      secret,
		SettingPaymentReceiptCodeOSSPrefix:               normalizeReceiptCodeOSSPrefix(next.Prefix),
		SettingPaymentReceiptCodeOSSPublicBaseURL:        strings.TrimRight(strings.TrimSpace(next.PublicBaseURL), "/"),
		SettingPaymentReceiptCodeOSSForcePathStyle:       strconv.FormatBool(next.ForcePathStyle),
		SettingPaymentReceiptCodeOSSMaxSizeBytes:         strconv.FormatInt(next.MaxSizeBytes, 10),
		SettingPaymentReceiptCodeOSSPresignExpireSeconds: strconv.Itoa(next.PresignExpireSeconds),
	}, nil
}

func (s *PaymentConfigService) decryptReceiptCodeOSSSecret(stored string) string {
	stored = strings.TrimSpace(stored)
	if stored == "" {
		return ""
	}
	if len(s.encryptionKey) == payment.AES256KeySize {

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

const (
	defaultReceiptCodeOSSMaxSizeBytes = int64(1024 * 1024)
)
const (
	defaultReceiptCodeOSSPrefix = "receipt-codes/"
)
const (
	defaultReceiptCodeOSSPresignExpireSeconds = 300
)
const (
	defaultReceiptCodeOSSRegion = "oss-cn-hangzhou"
)

func (s *PaymentConfigService) encryptReceiptCodeOSSSecret(secret string) (string, error) {
	secret = strings.TrimSpace(secret)
	if secret == "" {
		return "", nil
	}
	payload, err := json.Marshal(map[string]string{"secret": secret})
	if err != nil {
		return "", fmt.Errorf("marshal receipt code oss secret: %w", err)
	}
	if len(s.encryptionKey) != payment.AES256KeySize {
		return "", infraerrors.BadRequest("PAYMENT_ENCRYPTION_KEY_REQUIRED", "TOTP_ENCRYPTION_KEY must be configured before saving receipt code OSS secret")
	}

	encrypted, err := payment.Encrypt(string(payload), s.encryptionKey)
	if err != nil {
		return "", fmt.Errorf("encrypt receipt code oss secret: %w", err)
	}
	return encrypted, nil
}
func firstPositiveInt(v int, fallback int) int {
	if v > 0 {
		return v
	}
	return fallback
}

func firstPositiveInt64(v int64, fallback int64) int64 {
	if v > 0 {
		return v
	}
	return fallback
}

func normalizeReceiptCodeOSSPrefix(raw string) string {
	prefix := strings.Trim(strings.ReplaceAll(strings.TrimSpace(raw), "\\", "/"), "/")
	if prefix == "" {
		return defaultReceiptCodeOSSPrefix
	}
	return prefix + "/"
}
func parseBoolWithDefault(raw string, fallback bool) bool {
	switch strings.ToLower(strings.TrimSpace(raw)) {
	case "true", "1", "yes", "on":
		return true
	case "false", "0", "no", "off":
		return false
	default:
		return fallback
	}
}

func parseInt64WithDefault(raw string, fallback int64) int64 {
	if strings.TrimSpace(raw) == "" {
		return fallback
	}
	v, err := strconv.ParseInt(strings.TrimSpace(raw), 10, 64)
	if err != nil {
		return fallback
	}
	return v
}
func (s *PaymentConfigService) parseReceiptCodeOSSConfig(vals map[string]string) ReceiptCodeOSSConfig {
	env := config.ReceiptCodeStorageConfig{}
	if s != nil && s.envConfig != nil {
		env = s.envConfig.ReceiptCodeStorage
	}

	rawSecret := strings.TrimSpace(vals[SettingPaymentReceiptCodeOSSSecretAccessKey])
	secret := s.decryptReceiptCodeOSSSecret(rawSecret)
	if secret == "" && env.Enabled {
		secret = strings.TrimSpace(env.SecretAccessKey)
	}

	cfg := ReceiptCodeOSSConfig{
		Enabled:                   parseBoolWithDefault(vals[SettingPaymentReceiptCodeOSSEnabled], env.Enabled),
		Endpoint:                  firstNonEmpty(vals[SettingPaymentReceiptCodeOSSEndpoint], env.Endpoint),
		Region:                    firstNonEmpty(vals[SettingPaymentReceiptCodeOSSRegion], env.Region, defaultReceiptCodeOSSRegion),
		Bucket:                    firstNonEmpty(vals[SettingPaymentReceiptCodeOSSBucket], env.Bucket),
		AccessKeyID:               firstNonEmpty(vals[SettingPaymentReceiptCodeOSSAccessKeyID], env.AccessKeyID),
		SecretAccessKey:           secret,
		SecretAccessKeyConfigured: secret != "",
		Prefix:                    firstNonEmpty(vals[SettingPaymentReceiptCodeOSSPrefix], env.Prefix, defaultReceiptCodeOSSPrefix),
		PublicBaseURL:             firstNonEmpty(vals[SettingPaymentReceiptCodeOSSPublicBaseURL], env.PublicBaseURL),
		ForcePathStyle:            parseBoolWithDefault(vals[SettingPaymentReceiptCodeOSSForcePathStyle], env.ForcePathStyle),
		MaxSizeBytes:              parseInt64WithDefault(vals[SettingPaymentReceiptCodeOSSMaxSizeBytes], firstPositiveInt64(env.MaxSizeBytes, defaultReceiptCodeOSSMaxSizeBytes)),
		PresignExpireSeconds:      pcParseInt(vals[SettingPaymentReceiptCodeOSSPresignExpireSeconds], firstPositiveInt(env.PresignExpireSeconds, defaultReceiptCodeOSSPresignExpireSeconds)),
	}
	if !cfg.Enabled && strings.TrimSpace(vals[SettingPaymentReceiptCodeOSSEnabled]) == "" {
		cfg.Enabled = env.Enabled
	}
	cfg.Prefix = normalizeReceiptCodeOSSPrefix(cfg.Prefix)
	return cfg
}

func receiptCodeOSSFieldsProvided(req UpdatePaymentConfigRequest) bool {
	return req.ReceiptCodeOSSEnabled != nil ||
		req.ReceiptCodeOSSEndpoint != nil ||
		req.ReceiptCodeOSSRegion != nil ||
		req.ReceiptCodeOSSBucket != nil ||
		req.ReceiptCodeOSSAccessKeyID != nil ||
		req.ReceiptCodeOSSSecretAccessKey != nil ||
		req.ReceiptCodeOSSPrefix != nil ||
		req.ReceiptCodeOSSPublicBaseURL != nil ||
		req.ReceiptCodeOSSForcePathStyle != nil ||
		req.ReceiptCodeOSSMaxSizeBytes != nil ||
		req.ReceiptCodeOSSPresignExpireSeconds != nil
}

func validateReceiptCodeOSSConfig(cfg config.ReceiptCodeStorageConfig) error {
	if cfg.MaxSizeBytes <= 0 {
		return infraerrors.BadRequest("INVALID_RECEIPT_CODE_OSS_MAX_SIZE", "receipt code OSS max size must be greater than 0")
	}
	if cfg.MaxSizeBytes > 5*1024*1024 {
		return infraerrors.BadRequest("INVALID_RECEIPT_CODE_OSS_MAX_SIZE", "receipt code OSS max size must be <= 5242880")
	}
	if cfg.PresignExpireSeconds <= 0 || cfg.PresignExpireSeconds > 3600 {
		return infraerrors.BadRequest("INVALID_RECEIPT_CODE_OSS_PRESIGN_EXPIRE", "receipt code OSS presign expire seconds must be between 1 and 3600")
	}
	if endpoint := strings.TrimSpace(cfg.Endpoint); endpoint != "" {
		if err := config.ValidateAbsoluteHTTPURL(endpoint); err != nil {
			return infraerrors.BadRequest("INVALID_RECEIPT_CODE_OSS_ENDPOINT", "receipt code OSS endpoint must be an absolute http(s) URL")
		}
	}
	if publicBaseURL := strings.TrimSpace(cfg.PublicBaseURL); publicBaseURL != "" {
		if err := config.ValidateAbsoluteHTTPURL(publicBaseURL); err != nil {
			return infraerrors.BadRequest("INVALID_RECEIPT_CODE_OSS_PUBLIC_BASE_URL", "receipt code OSS public base URL must be an absolute http(s) URL")
		}
	}
	if !cfg.Enabled {
		return nil
	}
	if strings.TrimSpace(cfg.Endpoint) == "" {
		return infraerrors.BadRequest("RECEIPT_CODE_OSS_ENDPOINT_REQUIRED", "receipt code OSS endpoint is required when enabled")
	}
	if strings.TrimSpace(cfg.Bucket) == "" {
		return infraerrors.BadRequest("RECEIPT_CODE_OSS_BUCKET_REQUIRED", "receipt code OSS bucket is required when enabled")
	}
	if strings.TrimSpace(cfg.AccessKeyID) == "" {
		return infraerrors.BadRequest("RECEIPT_CODE_OSS_ACCESS_KEY_REQUIRED", "receipt code OSS access key ID is required when enabled")
	}
	if strings.TrimSpace(cfg.SecretAccessKey) == "" {
		return infraerrors.BadRequest("RECEIPT_CODE_OSS_SECRET_REQUIRED", "receipt code OSS secret access key is required when enabled")
	}
	return nil
}
