package service

import (
	"context"
	"fmt"
	"math"
	"os"
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
	// SettingSubscriptionUSDToCNYRate 是订阅 CNY 换算汇率（1 USD = X CNY）。
	// 0/未配置 = 关闭换算（订阅按 price 数值直付），显式配置后 CNY 通道订阅按 price × rate 收款。
	SettingSubscriptionUSDToCNYRate      = "SUBSCRIPTION_USD_TO_CNY_RATE"
	SettingRechargeFeeRate               = "RECHARGE_FEE_RATE"
	SettingProductNamePrefix             = "PRODUCT_NAME_PREFIX"
	SettingProductNameSuffix             = "PRODUCT_NAME_SUFFIX"
	SettingHelpImageURL                  = "PAYMENT_HELP_IMAGE_URL"
	SettingHelpText                      = "PAYMENT_HELP_TEXT"
	SettingCancelRateLimitOn             = "CANCEL_RATE_LIMIT_ENABLED"
	SettingCancelRateLimitMax            = "CANCEL_RATE_LIMIT_MAX"
	SettingCancelWindowSize              = "CANCEL_RATE_LIMIT_WINDOW"
	SettingCancelWindowUnit              = "CANCEL_RATE_LIMIT_UNIT"
	SettingCancelWindowMode              = "CANCEL_RATE_LIMIT_WINDOW_MODE"
	SettingAlipayForceQRCode             = "ALIPAY_FORCE_QRCODE"
	SettingAlipayMobilePrecreateDeepLink = "ALIPAY_MOBILE_PRECREATE_DEEP_LINK"
)

// Default values for payment configuration settings.
const (
	defaultOrderTimeoutMin  = 30
	defaultMaxPendingOrders = 3
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
	// SubscriptionUSDToCNYRate 为 0 时订阅换算关闭（兼容存量行为）。
	SubscriptionUSDToCNYRate float64 `json:"subscription_usd_to_cny_rate"`
	RechargeFeeRate          float64 `json:"recharge_fee_rate"`
	LoadBalanceStrategy      string  `json:"load_balance_strategy"`
	ProductNamePrefix        string  `json:"product_name_prefix"`
	ProductNameSuffix        string  `json:"product_name_suffix"`
	HelpImageURL             string  `json:"help_image_url"`
	HelpText                 string  `json:"help_text"`
	StripePublishableKey     string  `json:"stripe_publishable_key,omitempty"`

	// Cancel rate limit settings
	CancelRateLimitEnabled bool   `json:"cancel_rate_limit_enabled"`
	CancelRateLimitMax     int    `json:"cancel_rate_limit_max"`
	CancelRateLimitWindow  int    `json:"cancel_rate_limit_window"`
	CancelRateLimitUnit    string `json:"cancel_rate_limit_unit"`
	CancelRateLimitMode    string `json:"cancel_rate_limit_window_mode"`

	// Force Alipay mobile users to use QR code instead of mobile redirect
	AlipayForceQRCode             bool                 `json:"alipay_force_qrcode"`
	AlipayMobilePrecreateDeepLink bool                 `json:"alipay_mobile_precreate_deep_link"`
	ReceiptCodeOSS                ReceiptCodeOSSConfig `json:"receipt_code_oss"`
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
	SubscriptionUSDToCNYRate  *float64 `json:"subscription_usd_to_cny_rate"`
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

	// Force Alipay mobile users to use QR code instead of mobile redirect
	AlipayForceQRCode             *bool `json:"alipay_force_qrcode"`
	AlipayMobilePrecreateDeepLink *bool `json:"alipay_mobile_precreate_deep_link"`

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
	DisplayName string  `json:"display_name,omitempty"`
	Currency    string  `json:"currency"`
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
	Currency      string   `json:"currency"`
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
	Currency      *string  `json:"currency"`
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
func NewPaymentConfigService(entClient *dbent.Client, settingRepo SettingRepository, encryptionKey []byte, envConfigs ...*config.Config) *PaymentConfigService {
	service := &PaymentConfigService{entClient: entClient, settingRepo: settingRepo, encryptionKey: encryptionKey}
	if len(envConfigs) > 0 {
		service.envConfig = envConfigs[0]
	}
	return service
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
		SettingEnabledPaymentTypes, SettingBalancePayDisabled, SettingBalanceRechargeMult, SettingSubscriptionUSDToCNYRate, SettingRechargeFeeRate, SettingLoadBalanceStrategy,
		SettingProductNamePrefix, SettingProductNameSuffix,
		SettingHelpImageURL, SettingHelpText,
		SettingCancelRateLimitOn, SettingCancelRateLimitMax,
		SettingCancelWindowSize, SettingCancelWindowUnit, SettingCancelWindowMode,
		SettingAlipayForceQRCode, SettingAlipayMobilePrecreateDeepLink,
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
		SubscriptionUSDToCNYRate:  normalizeSubscriptionUSDToCNYRate(pcParseFloat(vals[SettingSubscriptionUSDToCNYRate], 0)),
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

		AlipayForceQRCode:             vals[SettingAlipayForceQRCode] == "true",
		AlipayMobilePrecreateDeepLink: vals[SettingAlipayMobilePrecreateDeepLink] == "true",
		ReceiptCodeOSS:                s.parseReceiptCodeOSSConfig(vals),
	}
	cfg.AlipayMobilePrecreateDeepLink = pcEnvBoolOverride(SettingAlipayMobilePrecreateDeepLink, cfg.AlipayMobilePrecreateDeepLink)
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

func pcEnvBoolOverride(key string, fallback bool) bool {
	raw, ok := os.LookupEnv(key)
	if !ok || strings.TrimSpace(raw) == "" {
		return fallback
	}
	value, err := strconv.ParseBool(strings.TrimSpace(raw))
	if err != nil {
		return fallback
	}
	return value
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
	if req.SubscriptionUSDToCNYRate != nil {
		v := *req.SubscriptionUSDToCNYRate
		if math.IsNaN(v) || math.IsInf(v, 0) || v < 0 {
			return infraerrors.BadRequest("INVALID_SUBSCRIPTION_USD_TO_CNY_RATE", "subscription USD to CNY rate must be 0 (disabled) or a positive number")
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
	m := make(map[string]string)
	set := func(key string, provided bool, value string) {
		if provided {
			m[key] = value
		}
	}
	set(SettingPaymentEnabled, req.Enabled != nil, formatBoolOrEmpty(req.Enabled))
	set(SettingMinRechargeAmount, req.MinAmount != nil, formatPositiveFloat(req.MinAmount))
	set(SettingMaxRechargeAmount, req.MaxAmount != nil, formatPositiveFloat(req.MaxAmount))
	set(SettingDailyRechargeLimit, req.DailyLimit != nil, formatPositiveFloat(req.DailyLimit))
	set(SettingOrderTimeoutMinutes, req.OrderTimeoutMin != nil, formatPositiveInt(req.OrderTimeoutMin))
	set(SettingMaxPendingOrders, req.MaxPendingOrders != nil, formatPositiveInt(req.MaxPendingOrders))
	if req.EnabledTypes != nil {
		m[SettingEnabledPaymentTypes] = strings.Join(req.EnabledTypes, ",")
	}
	set(SettingBalancePayDisabled, req.BalanceDisabled != nil, formatBoolOrEmpty(req.BalanceDisabled))
	set(SettingBalanceRechargeMult, req.BalanceRechargeMultiplier != nil, formatPositiveFloat(req.BalanceRechargeMultiplier))
	set(SettingSubscriptionUSDToCNYRate, req.SubscriptionUSDToCNYRate != nil, formatPositiveFloatExact(req.SubscriptionUSDToCNYRate))
	set(SettingRechargeFeeRate, req.RechargeFeeRate != nil, formatNonNegativeFloat(req.RechargeFeeRate))
	set(SettingLoadBalanceStrategy, req.LoadBalanceStrategy != nil, derefStr(req.LoadBalanceStrategy))
	set(SettingProductNamePrefix, req.ProductNamePrefix != nil, derefStr(req.ProductNamePrefix))
	set(SettingProductNameSuffix, req.ProductNameSuffix != nil, derefStr(req.ProductNameSuffix))
	set(SettingHelpImageURL, req.HelpImageURL != nil, derefStr(req.HelpImageURL))
	set(SettingHelpText, req.HelpText != nil, derefStr(req.HelpText))
	set(SettingCancelRateLimitOn, req.CancelRateLimitEnabled != nil, formatBoolOrEmpty(req.CancelRateLimitEnabled))
	set(SettingCancelRateLimitMax, req.CancelRateLimitMax != nil, formatPositiveInt(req.CancelRateLimitMax))
	set(SettingCancelWindowSize, req.CancelRateLimitWindow != nil, formatPositiveInt(req.CancelRateLimitWindow))
	set(SettingCancelWindowUnit, req.CancelRateLimitUnit != nil, derefStr(req.CancelRateLimitUnit))
	set(SettingCancelWindowMode, req.CancelRateLimitMode != nil, derefStr(req.CancelRateLimitMode))
	set(SettingAlipayForceQRCode, req.AlipayForceQRCode != nil, formatBoolOrEmpty(req.AlipayForceQRCode))
	set(SettingAlipayMobilePrecreateDeepLink, req.AlipayMobilePrecreateDeepLink != nil, formatBoolOrEmpty(req.AlipayMobilePrecreateDeepLink))
	set(SettingPaymentVisibleMethodAlipaySource, req.VisibleMethodAlipaySource != nil, derefStr(req.VisibleMethodAlipaySource))
	set(SettingPaymentVisibleMethodWxpaySource, req.VisibleMethodWxpaySource != nil, derefStr(req.VisibleMethodWxpaySource))
	set(SettingPaymentVisibleMethodAlipayEnabled, req.VisibleMethodAlipayEnabled != nil, formatBoolOrEmpty(req.VisibleMethodAlipayEnabled))
	set(SettingPaymentVisibleMethodWxpayEnabled, req.VisibleMethodWxpayEnabled != nil, formatBoolOrEmpty(req.VisibleMethodWxpayEnabled))
	if receiptCodeOSSFieldsProvided(req) {
		updates, err := s.buildReceiptCodeOSSUpdates(ctx, req)
		if err != nil {
			return err
		}
		for key, value := range updates {
			m[key] = value
		}
	}
	return s.settingRepo.SetMultiple(ctx, m)
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

// formatPositiveFloatExact 保留完整精度，用于汇率等对小数位敏感的配置。
func formatPositiveFloatExact(v *float64) string {
	if v == nil || *v <= 0 {
		return "" // empty → parsePaymentConfig 视为未配置（换算关闭）
	}
	return strconv.FormatFloat(*v, 'f', -1, 64)
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
