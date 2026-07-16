package service

import (
	"context"
	"encoding/json"
	"fmt"
	"math"
	"strconv"
	"strings"

	dbent "ikik-api/ent"
	"ikik-api/ent/paymentproviderinstance"
	"ikik-api/internal/config"
	"ikik-api/internal/payment"
	infraerrors "ikik-api/internal/pkg/errors"
)

const (
	SettingPaymentEnabled      = "payment_enabled"
	SettingMinRechargeAmount   = "MIN_RECHARGE_AMOUNT"
	SettingMaxRechargeAmount   = "MAX_RECHARGE_AMOUNT"
	SettingDailyRechargeLimit  = "DAILY_RECHARGE_LIMIT"
	SettingOrderTimeoutMinutes = "ORDER_TIMEOUT_MINUTES"
	SettingMaxPendingOrders    = "MAX_PENDING_ORDERS"
	SettingEnabledPaymentTypes = "ENABLED_PAYMENT_TYPES"
	SettingLoadBalanceStrategy = "LOAD_BALANCE_STRATEGY"
	SettingBalancePayDisabled  = "BALANCE_PAYMENT_DISABLED"
	SettingBalanceRechargeMult = "BALANCE_RECHARGE_MULTIPLIER"
	SettingRechargeFeeRate     = "RECHARGE_FEE_RATE"
	SettingProductNamePrefix   = "PRODUCT_NAME_PREFIX"
	SettingProductNameSuffix   = "PRODUCT_NAME_SUFFIX"
	SettingHelpImageURL        = "PAYMENT_HELP_IMAGE_URL"
	SettingHelpText            = "PAYMENT_HELP_TEXT"
	SettingCancelRateLimitOn   = "CANCEL_RATE_LIMIT_ENABLED"
	SettingCancelRateLimitMax  = "CANCEL_RATE_LIMIT_MAX"
	SettingCancelWindowSize    = "CANCEL_RATE_LIMIT_WINDOW"
	SettingCancelWindowUnit    = "CANCEL_RATE_LIMIT_UNIT"
	SettingCancelWindowMode    = "CANCEL_RATE_LIMIT_WINDOW_MODE"

	SettingPaymentReceiptCodeOSSEnabled              = "payment_receipt_code_oss_enabled"
	SettingPaymentReceiptCodeOSSEndpoint             = "payment_receipt_code_oss_endpoint"
	SettingPaymentReceiptCodeOSSRegion               = "payment_receipt_code_oss_region"
	SettingPaymentReceiptCodeOSSBucket               = "payment_receipt_code_oss_bucket"
	SettingPaymentReceiptCodeOSSAccessKeyID          = "payment_receipt_code_oss_access_key_id"
	SettingPaymentReceiptCodeOSSSecretAccessKey      = "payment_receipt_code_oss_secret_access_key"
	SettingPaymentReceiptCodeOSSPrefix               = "payment_receipt_code_oss_prefix"
	SettingPaymentReceiptCodeOSSPublicBaseURL        = "payment_receipt_code_oss_public_base_url"
	SettingPaymentReceiptCodeOSSForcePathStyle       = "payment_receipt_code_oss_force_path_style"
	SettingPaymentReceiptCodeOSSMaxSizeBytes         = "payment_receipt_code_oss_max_size_bytes"
	SettingPaymentReceiptCodeOSSPresignExpireSeconds = "payment_receipt_code_oss_presign_expire_seconds"
)

// Default values for payment configuration settings.
const (
	defaultOrderTimeoutMin  = 30
	defaultMaxPendingOrders = 3
)

const (
	defaultReceiptCodeOSSRegion               = "oss-cn-hangzhou"
	defaultReceiptCodeOSSPrefix               = "receipt-codes/"
	defaultReceiptCodeOSSMaxSizeBytes         = int64(1024 * 1024)
	defaultReceiptCodeOSSPresignExpireSeconds = 300
)

// PaymentConfig holds the payment system configuration.
type PaymentConfig struct {
	Enabled                   bool     `json:"enabled"`
	MinAmount                 float64  `json:"min_amount"`
	MaxAmount                 float64  `json:"max_amount"`
	DailyLimit                float64  `json:"daily_limit"`
	OrderTimeoutMin           int      `json:"order_timeout_minutes"`
	MaxPendingOrders          int      `json:"max_pending_orders"`
	EnabledTypes              []string `json:"enabled_payment_types"`
	BalanceDisabled           bool     `json:"balance_disabled"`
	BalanceRechargeMultiplier float64  `json:"balance_recharge_multiplier"`
	RechargeFeeRate           float64  `json:"recharge_fee_rate"`
	LoadBalanceStrategy       string   `json:"load_balance_strategy"`
	ProductNamePrefix         string   `json:"product_name_prefix"`
	ProductNameSuffix         string   `json:"product_name_suffix"`
	HelpImageURL              string   `json:"help_image_url"`
	HelpText                  string   `json:"help_text"`
	StripePublishableKey      string   `json:"stripe_publishable_key,omitempty"`

	// Cancel rate limit settings
	CancelRateLimitEnabled bool   `json:"cancel_rate_limit_enabled"`
	CancelRateLimitMax     int    `json:"cancel_rate_limit_max"`
	CancelRateLimitWindow  int    `json:"cancel_rate_limit_window"`
	CancelRateLimitUnit    string `json:"cancel_rate_limit_unit"`
	CancelRateLimitMode    string `json:"cancel_rate_limit_window_mode"`

	ReceiptCodeOSS ReceiptCodeOSSConfig `json:"receipt_code_oss"`
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

// UpdatePaymentConfigRequest contains fields to update payment configuration.
type UpdatePaymentConfigRequest struct {
	Enabled                   *bool    `json:"enabled"`
	MinAmount                 *float64 `json:"min_amount"`
	MaxAmount                 *float64 `json:"max_amount"`
	DailyLimit                *float64 `json:"daily_limit"`
	OrderTimeoutMin           *int     `json:"order_timeout_minutes"`
	MaxPendingOrders          *int     `json:"max_pending_orders"`
	EnabledTypes              []string `json:"enabled_payment_types"`
	BalanceDisabled           *bool    `json:"balance_disabled"`
	BalanceRechargeMultiplier *float64 `json:"balance_recharge_multiplier"`
	RechargeFeeRate           *float64 `json:"recharge_fee_rate"`
	LoadBalanceStrategy       *string  `json:"load_balance_strategy"`
	ProductNamePrefix         *string  `json:"product_name_prefix"`
	ProductNameSuffix         *string  `json:"product_name_suffix"`
	HelpImageURL              *string  `json:"help_image_url"`
	HelpText                  *string  `json:"help_text"`

	// Cancel rate limit settings
	CancelRateLimitEnabled *bool   `json:"cancel_rate_limit_enabled"`
	CancelRateLimitMax     *int    `json:"cancel_rate_limit_max"`
	CancelRateLimitWindow  *int    `json:"cancel_rate_limit_window"`
	CancelRateLimitUnit    *string `json:"cancel_rate_limit_unit"`
	CancelRateLimitMode    *string `json:"cancel_rate_limit_window_mode"`

	VisibleMethodAlipaySource  *string `json:"payment_visible_method_alipay_source"`
	VisibleMethodWxpaySource   *string `json:"payment_visible_method_wxpay_source"`
	VisibleMethodAlipayEnabled *bool   `json:"payment_visible_method_alipay_enabled"`
	VisibleMethodWxpayEnabled  *bool   `json:"payment_visible_method_wxpay_enabled"`

	ReceiptCodeOSSEnabled              *bool   `json:"payment_receipt_code_oss_enabled"`
	ReceiptCodeOSSEndpoint             *string `json:"payment_receipt_code_oss_endpoint"`
	ReceiptCodeOSSRegion               *string `json:"payment_receipt_code_oss_region"`
	ReceiptCodeOSSBucket               *string `json:"payment_receipt_code_oss_bucket"`
	ReceiptCodeOSSAccessKeyID          *string `json:"payment_receipt_code_oss_access_key_id"`
	ReceiptCodeOSSSecretAccessKey      *string `json:"payment_receipt_code_oss_secret_access_key"`
	ReceiptCodeOSSPrefix               *string `json:"payment_receipt_code_oss_prefix"`
	ReceiptCodeOSSPublicBaseURL        *string `json:"payment_receipt_code_oss_public_base_url"`
	ReceiptCodeOSSForcePathStyle       *bool   `json:"payment_receipt_code_oss_force_path_style"`
	ReceiptCodeOSSMaxSizeBytes         *int64  `json:"payment_receipt_code_oss_max_size_bytes"`
	ReceiptCodeOSSPresignExpireSeconds *int    `json:"payment_receipt_code_oss_presign_expire_seconds"`
}

// MethodLimits holds per-payment-type limits.
type MethodLimits struct {
	PaymentType string  `json:"payment_type"`
	Currency    string  `json:"currency,omitempty"`
	FeeRate     float64 `json:"fee_rate"`
	DailyLimit  float64 `json:"daily_limit"`
	SingleMin   float64 `json:"single_min"`
	SingleMax   float64 `json:"single_max"`
}

// MethodLimitsResponse is the full response for the user-facing /limits API.
// It includes per-method limits and the global widest range (union of all methods).
type MethodLimitsResponse struct {
	Methods   map[string]MethodLimits `json:"methods"`
	GlobalMin float64                 `json:"global_min"` // 0 = no minimum
	GlobalMax float64                 `json:"global_max"` // 0 = no maximum
}

type CreateProviderInstanceRequest struct {
	ProviderKey     string            `json:"provider_key"`
	Name            string            `json:"name"`
	Config          map[string]string `json:"config"`
	SupportedTypes  []string          `json:"supported_types"`
	Enabled         bool              `json:"enabled"`
	PaymentMode     string            `json:"payment_mode"`
	SortOrder       int               `json:"sort_order"`
	Limits          string            `json:"limits"`
	RefundEnabled   bool              `json:"refund_enabled"`
	AllowUserRefund bool              `json:"allow_user_refund"`
}

type UpdateProviderInstanceRequest struct {
	Name            *string           `json:"name"`
	Config          map[string]string `json:"config"`
	SupportedTypes  []string          `json:"supported_types"`
	Enabled         *bool             `json:"enabled"`
	PaymentMode     *string           `json:"payment_mode"`
	SortOrder       *int              `json:"sort_order"`
	Limits          *string           `json:"limits"`
	RefundEnabled   *bool             `json:"refund_enabled"`
	AllowUserRefund *bool             `json:"allow_user_refund"`
}
type CreatePlanRequest struct {
	GroupID       int64    `json:"group_id"`
	Name          string   `json:"name"`
	Description   string   `json:"description"`
	Price         float64  `json:"price"`
	OriginalPrice *float64 `json:"original_price"`
	ValidityDays  int      `json:"validity_days"`
	ValidityUnit  string   `json:"validity_unit"`
	Features      string   `json:"features"`
	ProductName   string   `json:"product_name"`
	ForSale       bool     `json:"for_sale"`
	SortOrder     int      `json:"sort_order"`
}

type UpdatePlanRequest struct {
	GroupID       *int64   `json:"group_id"`
	Name          *string  `json:"name"`
	Description   *string  `json:"description"`
	Price         *float64 `json:"price"`
	OriginalPrice *float64 `json:"original_price"`
	ValidityDays  *int     `json:"validity_days"`
	ValidityUnit  *string  `json:"validity_unit"`
	Features      *string  `json:"features"`
	ProductName   *string  `json:"product_name"`
	ForSale       *bool    `json:"for_sale"`
	SortOrder     *int     `json:"sort_order"`
}

// PaymentConfigService manages payment configuration and CRUD for
// provider instances, channels, and subscription plans.
type PaymentConfigService struct {
	entClient     *dbent.Client
	settingRepo   SettingRepository
	encryptionKey []byte
	envConfig     *config.Config
}

// NewPaymentConfigService creates a new PaymentConfigService.
func NewPaymentConfigService(entClient *dbent.Client, settingRepo SettingRepository, encryptionKey []byte, envConfig *config.Config) *PaymentConfigService {
	return &PaymentConfigService{entClient: entClient, settingRepo: settingRepo, encryptionKey: encryptionKey, envConfig: envConfig}
}

// IsPaymentEnabled returns whether the payment system is enabled.
func (s *PaymentConfigService) IsPaymentEnabled(ctx context.Context) bool {
	val, err := s.settingRepo.GetValue(ctx, SettingPaymentEnabled)
	if err != nil {
		return false
	}
	return val == "true"
}

// GetPaymentConfig returns the full payment configuration.
func (s *PaymentConfigService) GetPaymentConfig(ctx context.Context) (*PaymentConfig, error) {
	keys := []string{
		SettingPaymentEnabled, SettingMinRechargeAmount, SettingMaxRechargeAmount,
		SettingDailyRechargeLimit, SettingOrderTimeoutMinutes, SettingMaxPendingOrders,
		SettingEnabledPaymentTypes, SettingBalancePayDisabled, SettingBalanceRechargeMult, SettingRechargeFeeRate, SettingLoadBalanceStrategy,
		SettingProductNamePrefix, SettingProductNameSuffix,
		SettingHelpImageURL, SettingHelpText,
		SettingCancelRateLimitOn, SettingCancelRateLimitMax,
		SettingCancelWindowSize, SettingCancelWindowUnit, SettingCancelWindowMode,
		SettingPaymentVisibleMethodAlipayEnabled, SettingPaymentVisibleMethodAlipaySource,
		SettingPaymentVisibleMethodWxpayEnabled, SettingPaymentVisibleMethodWxpaySource,
		SettingPaymentReceiptCodeOSSEnabled, SettingPaymentReceiptCodeOSSEndpoint,
		SettingPaymentReceiptCodeOSSRegion, SettingPaymentReceiptCodeOSSBucket,
		SettingPaymentReceiptCodeOSSAccessKeyID, SettingPaymentReceiptCodeOSSSecretAccessKey,
		SettingPaymentReceiptCodeOSSPrefix, SettingPaymentReceiptCodeOSSPublicBaseURL,
		SettingPaymentReceiptCodeOSSForcePathStyle, SettingPaymentReceiptCodeOSSMaxSizeBytes,
		SettingPaymentReceiptCodeOSSPresignExpireSeconds,
	}
	vals, err := s.settingRepo.GetMultiple(ctx, keys)
	if err != nil {
		return nil, fmt.Errorf("get payment config settings: %w", err)
	}
	cfg := s.parsePaymentConfig(vals)
	// Load Stripe publishable key from the first enabled Stripe provider instance
	cfg.StripePublishableKey = s.getStripePublishableKey(ctx)
	return cfg, nil
}

func (s *PaymentConfigService) parsePaymentConfig(vals map[string]string) *PaymentConfig {
	cfg := &PaymentConfig{
		Enabled:                   vals[SettingPaymentEnabled] == "true",
		MinAmount:                 pcParseFloat(vals[SettingMinRechargeAmount], 1),
		MaxAmount:                 pcParseFloat(vals[SettingMaxRechargeAmount], 0),
		DailyLimit:                pcParseFloat(vals[SettingDailyRechargeLimit], 0),
		OrderTimeoutMin:           pcParseInt(vals[SettingOrderTimeoutMinutes], defaultOrderTimeoutMin),
		MaxPendingOrders:          pcParseInt(vals[SettingMaxPendingOrders], defaultMaxPendingOrders),
		BalanceDisabled:           vals[SettingBalancePayDisabled] == "true",
		BalanceRechargeMultiplier: normalizeBalanceRechargeMultiplier(pcParseFloat(vals[SettingBalanceRechargeMult], defaultBalanceRechargeMultiplier)),
		RechargeFeeRate:           pcParseFloat(vals[SettingRechargeFeeRate], 0),
		LoadBalanceStrategy:       vals[SettingLoadBalanceStrategy],
		ProductNamePrefix:         vals[SettingProductNamePrefix],
		ProductNameSuffix:         vals[SettingProductNameSuffix],
		HelpImageURL:              vals[SettingHelpImageURL],
		HelpText:                  vals[SettingHelpText],

		CancelRateLimitEnabled: vals[SettingCancelRateLimitOn] == "true",
		CancelRateLimitMax:     pcParseInt(vals[SettingCancelRateLimitMax], 10),
		CancelRateLimitWindow:  pcParseInt(vals[SettingCancelWindowSize], 1),
		CancelRateLimitUnit:    vals[SettingCancelWindowUnit],
		CancelRateLimitMode:    vals[SettingCancelWindowMode],
	}
	cfg.ReceiptCodeOSS = s.parseReceiptCodeOSSConfig(vals)
	if cfg.LoadBalanceStrategy == "" {
		cfg.LoadBalanceStrategy = payment.DefaultLoadBalanceStrategy
	}
	if raw := vals[SettingEnabledPaymentTypes]; raw != "" {
		types := make([]string, 0, len(strings.Split(raw, ",")))
		for _, t := range strings.Split(raw, ",") {
			t = strings.TrimSpace(t)
			if t != "" {
				types = append(types, t)
			}
		}
		cfg.EnabledTypes = NormalizeVisibleMethods(types)
	}
	return cfg
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

// getStripePublishableKey finds the publishable key from the first enabled Stripe provider instance.
func (s *PaymentConfigService) getStripePublishableKey(ctx context.Context) string {
	if s.entClient == nil {
		return ""
	}
	instances, err := s.entClient.PaymentProviderInstance.Query().
		Where(
			paymentproviderinstance.EnabledEQ(true),
			paymentproviderinstance.ProviderKeyEQ(payment.TypeStripe),
		).Limit(1).All(ctx)
	if err != nil || len(instances) == 0 {
		return ""
	}
	cfg, err := s.decryptConfig(instances[0].Config)
	if err != nil || cfg == nil {
		return ""
	}
	return cfg[payment.ConfigKeyPublishableKey]
}

// UpdatePaymentConfig updates the payment configuration settings.
// NOTE: This function exceeds 30 lines because each field requires an independent
// nil-check before serialisation — this is inherent to patch-style update patterns
// and cannot be meaningfully decomposed without introducing unnecessary abstraction.
func (s *PaymentConfigService) UpdatePaymentConfig(ctx context.Context, req UpdatePaymentConfigRequest) error {
	if req.BalanceRechargeMultiplier != nil {
		if math.IsNaN(*req.BalanceRechargeMultiplier) || math.IsInf(*req.BalanceRechargeMultiplier, 0) || *req.BalanceRechargeMultiplier <= 0 {
			return infraerrors.BadRequest("INVALID_BALANCE_RECHARGE_MULTIPLIER", "balance recharge multiplier must be greater than 0")
		}
	}
	if req.RechargeFeeRate != nil {
		v := *req.RechargeFeeRate
		if math.IsNaN(v) || math.IsInf(v, 0) || v < 0 || v > 100 {
			return infraerrors.BadRequest("INVALID_RECHARGE_FEE_RATE", "recharge fee rate must be between 0 and 100")
		}
		// Enforce max 2 decimal places
		if math.Round(v*100) != v*100 {
			return infraerrors.BadRequest("INVALID_RECHARGE_FEE_RATE", "recharge fee rate allows at most 2 decimal places")
		}
	}
	m := map[string]string{
		SettingPaymentEnabled:                    formatBoolOrEmpty(req.Enabled),
		SettingMinRechargeAmount:                 formatPositiveFloat(req.MinAmount),
		SettingMaxRechargeAmount:                 formatPositiveFloat(req.MaxAmount),
		SettingDailyRechargeLimit:                formatPositiveFloat(req.DailyLimit),
		SettingOrderTimeoutMinutes:               formatPositiveInt(req.OrderTimeoutMin),
		SettingMaxPendingOrders:                  formatPositiveInt(req.MaxPendingOrders),
		SettingBalancePayDisabled:                formatBoolOrEmpty(req.BalanceDisabled),
		SettingBalanceRechargeMult:               formatPositiveFloat(req.BalanceRechargeMultiplier),
		SettingRechargeFeeRate:                   formatNonNegativeFloat(req.RechargeFeeRate),
		SettingLoadBalanceStrategy:               derefStr(req.LoadBalanceStrategy),
		SettingProductNamePrefix:                 derefStr(req.ProductNamePrefix),
		SettingProductNameSuffix:                 derefStr(req.ProductNameSuffix),
		SettingHelpImageURL:                      derefStr(req.HelpImageURL),
		SettingHelpText:                          derefStr(req.HelpText),
		SettingCancelRateLimitOn:                 formatBoolOrEmpty(req.CancelRateLimitEnabled),
		SettingCancelRateLimitMax:                formatPositiveInt(req.CancelRateLimitMax),
		SettingCancelWindowSize:                  formatPositiveInt(req.CancelRateLimitWindow),
		SettingCancelWindowUnit:                  derefStr(req.CancelRateLimitUnit),
		SettingCancelWindowMode:                  derefStr(req.CancelRateLimitMode),
		SettingPaymentVisibleMethodAlipaySource:  derefStr(req.VisibleMethodAlipaySource),
		SettingPaymentVisibleMethodWxpaySource:   derefStr(req.VisibleMethodWxpaySource),
		SettingPaymentVisibleMethodAlipayEnabled: formatBoolOrEmpty(req.VisibleMethodAlipayEnabled),
		SettingPaymentVisibleMethodWxpayEnabled:  formatBoolOrEmpty(req.VisibleMethodWxpayEnabled),
	}
	if receiptCodeOSSFieldsProvided(req) {
		receiptCodeUpdates, err := s.buildReceiptCodeOSSUpdates(ctx, req)
		if err != nil {
			return err
		}
		for key, value := range receiptCodeUpdates {
			m[key] = value
		}
	}
	if req.EnabledTypes != nil {
		m[SettingEnabledPaymentTypes] = strings.Join(req.EnabledTypes, ",")
	} else {
		m[SettingEnabledPaymentTypes] = ""
	}
	return s.settingRepo.SetMultiple(ctx, m)
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

func formatBoolOrEmpty(v *bool) string {
	if v == nil {
		return ""
	}
	return strconv.FormatBool(*v)
}

func formatPositiveFloat(v *float64) string {
	if v == nil || *v <= 0 {
		return "" // empty → parsePaymentConfig uses default
	}
	return strconv.FormatFloat(*v, 'f', 2, 64)
}

func formatNonNegativeFloat(v *float64) string {
	if v == nil || *v < 0 {
		return ""
	}
	return strconv.FormatFloat(*v, 'f', 2, 64)
}

func formatPositiveInt(v *int) string {
	if v == nil || *v <= 0 {
		return ""
	}
	return strconv.Itoa(*v)
}

func derefStr(v *string) string {
	if v == nil {
		return ""
	}
	return *v
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
	//nolint:staticcheck // SA1019: reused for settings secret storage until a shared secret-store abstraction exists.
	encrypted, err := payment.Encrypt(string(payload), s.encryptionKey)
	if err != nil {
		return "", fmt.Errorf("encrypt receipt code oss secret: %w", err)
	}
	return encrypted, nil
}

func (s *PaymentConfigService) decryptReceiptCodeOSSSecret(stored string) string {
	stored = strings.TrimSpace(stored)
	if stored == "" {
		return ""
	}
	if len(s.encryptionKey) == payment.AES256KeySize {
		//nolint:staticcheck // SA1019: see encryptReceiptCodeOSSSecret.
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

func splitTypes(s string) []string {
	if s == "" {
		return nil
	}
	parts := strings.Split(s, ",")
	result := make([]string, 0, len(parts))
	for _, p := range parts {
		p = strings.TrimSpace(p)
		if p != "" {
			result = append(result, p)
		}
	}
	return result
}

func joinTypes(types []string) string {
	return strings.Join(types, ",")
}

func pcParseFloat(s string, defaultVal float64) float64 {
	if s == "" {
		return defaultVal
	}
	v, err := strconv.ParseFloat(s, 64)
	if err != nil {
		return defaultVal
	}
	return v
}

func pcParseInt(s string, defaultVal int) int {
	if s == "" {
		return defaultVal
	}
	v, err := strconv.Atoi(s)
	if err != nil {
		return defaultVal
	}
	return v
}

func buildVisibleMethodSourceAvailability(instances []*dbent.PaymentProviderInstance) map[string]bool {
	available := make(map[string]bool, 4)
	for _, inst := range instances {
		switch inst.ProviderKey {
		case payment.TypeAlipay:
			if inst.SupportedTypes == "" || payment.InstanceSupportsType(inst.SupportedTypes, payment.TypeAlipay) || payment.InstanceSupportsType(inst.SupportedTypes, payment.TypeAlipayDirect) {
				available[VisibleMethodSourceOfficialAlipay] = true
			}
		case payment.TypeWxpay:
			if inst.SupportedTypes == "" || payment.InstanceSupportsType(inst.SupportedTypes, payment.TypeWxpay) || payment.InstanceSupportsType(inst.SupportedTypes, payment.TypeWxpayDirect) {
				available[VisibleMethodSourceOfficialWechat] = true
			}
		case payment.TypeEasyPay:
			for _, supportedType := range splitTypes(inst.SupportedTypes) {
				switch NormalizeVisibleMethod(supportedType) {
				case payment.TypeAlipay:
					available[VisibleMethodSourceEasyPayAlipay] = true
				case payment.TypeWxpay:
					available[VisibleMethodSourceEasyPayWechat] = true
				}
			}
		}
	}
	return available
}

func applyVisibleMethodRoutingToEnabledTypes(base []string, vals map[string]string, available map[string]bool) []string {
	shouldExpose := map[string]bool{
		payment.TypeAlipay: visibleMethodShouldBeExposed(payment.TypeAlipay, vals, available),
		payment.TypeWxpay:  visibleMethodShouldBeExposed(payment.TypeWxpay, vals, available),
	}

	seen := make(map[string]struct{}, len(base)+2)
	out := make([]string, 0, len(base)+2)
	appendType := func(paymentType string) {
		paymentType = NormalizeVisibleMethod(paymentType)
		if paymentType == "" {
			return
		}
		if _, ok := seen[paymentType]; ok {
			return
		}
		seen[paymentType] = struct{}{}
		out = append(out, paymentType)
	}

	for _, paymentType := range base {
		visibleMethod := NormalizeVisibleMethod(paymentType)
		switch visibleMethod {
		case payment.TypeAlipay, payment.TypeWxpay:
			if shouldExpose[visibleMethod] {
				appendType(visibleMethod)
			}
		default:
			appendType(visibleMethod)
		}
	}

	for _, visibleMethod := range []string{payment.TypeAlipay, payment.TypeWxpay} {
		if shouldExpose[visibleMethod] {
			appendType(visibleMethod)
		}
	}
	return out
}

func visibleMethodShouldBeExposed(method string, vals map[string]string, available map[string]bool) bool {
	enabledKey := visibleMethodEnabledSettingKey(method)
	sourceKey := visibleMethodSourceSettingKey(method)
	if enabledKey == "" || sourceKey == "" || vals[enabledKey] != "true" {
		return false
	}
	source := NormalizeVisibleMethodSource(method, vals[sourceKey])
	return source != "" && available[source]
}
