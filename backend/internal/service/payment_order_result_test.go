package service

import (
	"context"
	"net/url"
	"strings"
	"testing"
	"time"

	dbent "ikik-api/ent"
	"ikik-api/internal/payment"
	infraerrors "ikik-api/internal/pkg/errors"
)

func TestBuildCreateOrderResponseDefaultsToOrderCreated(t *testing.T) {
	t.Parallel()

	expiresAt := time.Date(2026, 4, 16, 12, 0, 0, 0, time.UTC)
	resp := buildCreateOrderResponse(
		&dbent.PaymentOrder{
			ID:         42,
			Amount:     12.34,
			FeeRate:    0.03,
			ExpiresAt:  expiresAt,
			OutTradeNo: "sub2_42",
		},
		CreateOrderRequest{PaymentType: payment.TypeWxpay},
		12.71,
		&payment.InstanceSelection{PaymentMode: "qrcode"},
		&payment.CreatePaymentResponse{
			TradeNo: "sub2_42",
			QRCode:  "weixin://wxpay/bizpayurl?pr=test",
		},
		payment.CreatePaymentResultOrderCreated,
	)

	if resp.ResultType != payment.CreatePaymentResultOrderCreated {
		t.Fatalf("result type = %q, want %q", resp.ResultType, payment.CreatePaymentResultOrderCreated)
	}
	if resp.OutTradeNo != "sub2_42" {
		t.Fatalf("out_trade_no = %q, want %q", resp.OutTradeNo, "sub2_42")
	}
	if resp.QRCode != "weixin://wxpay/bizpayurl?pr=test" {
		t.Fatalf("qr_code = %q, want %q", resp.QRCode, "weixin://wxpay/bizpayurl?pr=test")
	}
	if resp.JSAPI != nil || resp.JSAPIPayload != nil {
		t.Fatal("order_created response should not include jsapi payload")
	}
	if !resp.ExpiresAt.Equal(expiresAt) {
		t.Fatalf("expires_at = %v, want %v", resp.ExpiresAt, expiresAt)
	}
}

func TestBuildCreateOrderResponseCopiesJSAPIPayload(t *testing.T) {
	t.Parallel()

	jsapiPayload := &payment.WechatJSAPIPayload{
		AppID:     "wx123",
		TimeStamp: "1712345678",
		NonceStr:  "nonce-123",
		Package:   "prepay_id=wx123",
		SignType:  "RSA",
		PaySign:   "signed-payload",
	}
	resp := buildCreateOrderResponse(
		&dbent.PaymentOrder{
			ID:         88,
			Amount:     66.88,
			FeeRate:    0.01,
			ExpiresAt:  time.Date(2026, 4, 16, 13, 0, 0, 0, time.UTC),
			OutTradeNo: "sub2_88",
		},
		CreateOrderRequest{PaymentType: payment.TypeWxpay},
		67.55,
		&payment.InstanceSelection{PaymentMode: "popup"},
		&payment.CreatePaymentResponse{
			TradeNo:    "sub2_88",
			ResultType: payment.CreatePaymentResultJSAPIReady,
			JSAPI:      jsapiPayload,
		},
		payment.CreatePaymentResultJSAPIReady,
	)

	if resp.ResultType != payment.CreatePaymentResultJSAPIReady {
		t.Fatalf("result type = %q, want %q", resp.ResultType, payment.CreatePaymentResultJSAPIReady)
	}
	if resp.JSAPI == nil || resp.JSAPIPayload == nil {
		t.Fatal("expected jsapi payload aliases to be populated")
	}
	if resp.JSAPI != jsapiPayload || resp.JSAPIPayload != jsapiPayload {
		t.Fatal("expected jsapi aliases to preserve the original pointer")
	}
}

func TestMaybeBuildWeChatOAuthRequiredResponse(t *testing.T) {
	t.Setenv("PAYMENT_RESUME_SIGNING_KEY", "0123456789abcdef0123456789abcdef")

	svc := newWeChatPaymentOAuthTestService(map[string]string{
		SettingKeyWeChatConnectEnabled:             "true",
		SettingKeyWeChatConnectAppID:               "wx123456",
		SettingKeyWeChatConnectAppSecret:           "wechat-secret",
		SettingKeyWeChatConnectMode:                "mp",
		SettingKeyWeChatConnectScopes:              "snsapi_base",
		SettingKeyWeChatConnectRedirectURL:         "https://api.example.com/api/v1/auth/oauth/wechat/callback",
		SettingKeyWeChatConnectFrontendRedirectURL: "/auth/wechat/callback",
	})

	resp, err := svc.maybeBuildWeChatOAuthRequiredResponse(context.Background(), CreateOrderRequest{
		UserID:          123,
		Amount:          12.5,
		PaymentType:     payment.TypeWxpay,
		IsWeChatBrowser: true,
		SrcURL:          "https://merchant.example/payment?from=wechat",
		OrderType:       payment.OrderTypeBalance,
	}, 12.5, 12.88, 0.03)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if resp == nil {
		t.Fatal("expected oauth_required response, got nil")
	}
	if resp.ResultType != payment.CreatePaymentResultOAuthRequired {
		t.Fatalf("result type = %q, want %q", resp.ResultType, payment.CreatePaymentResultOAuthRequired)
	}
	if resp.OAuth == nil {
		t.Fatal("expected oauth payload, got nil")
	}
	if resp.OAuth.AppID != "wx123456" {
		t.Fatalf("appid = %q, want %q", resp.OAuth.AppID, "wx123456")
	}
	if resp.OAuth.Scope != "snsapi_base" {
		t.Fatalf("scope = %q, want %q", resp.OAuth.Scope, "snsapi_base")
	}
	if resp.OAuth.RedirectURL != "/auth/wechat/payment/callback" {
		t.Fatalf("redirect_url = %q, want %q", resp.OAuth.RedirectURL, "/auth/wechat/payment/callback")
	}
	parsedAuthorizeURL, err := url.Parse(resp.OAuth.AuthorizeURL)
	if err != nil {
		t.Fatalf("parse authorize_url: %v", err)
	}
	if parsedAuthorizeURL.Path != "/api/v1/auth/oauth/wechat/payment/start" {
		t.Fatalf("authorize_url path = %q", parsedAuthorizeURL.Path)
	}
	contextToken := parsedAuthorizeURL.Query().Get("context_token")
	if contextToken == "" {
		t.Fatalf("authorize_url missing context_token: %q", resp.OAuth.AuthorizeURL)
	}
	claims, err := svc.paymentResume().ParseWeChatPaymentOAuthContextToken(contextToken)
	if err != nil {
		t.Fatalf("parse context token: %v", err)
	}
	if claims.UserID != 123 {
		t.Fatalf("context user id = %d, want 123", claims.UserID)
	}
	if claims.Amount != "12.5" || claims.OrderType != payment.OrderTypeBalance || claims.PaymentType != payment.TypeWxpay || claims.RedirectTo != "/purchase?from=wechat" {
		t.Fatalf("unexpected context claims: %+v", claims)
	}
}

func TestMaybeBuildWeChatOAuthRequiredResponseRequiresMPConfigInWeChat(t *testing.T) {
	t.Parallel()

	svc := newWeChatPaymentOAuthTestService(nil)

	resp, err := svc.maybeBuildWeChatOAuthRequiredResponse(context.Background(), CreateOrderRequest{
		UserID:          123,
		Amount:          12.5,
		PaymentType:     payment.TypeWxpay,
		IsWeChatBrowser: true,
		SrcURL:          "https://merchant.example/payment?from=wechat",
		OrderType:       payment.OrderTypeBalance,
	}, 12.5, 12.88, 0.03)
	if resp != nil {
		t.Fatalf("expected nil response, got %+v", resp)
	}
	if err == nil {
		t.Fatal("expected error, got nil")
	}

	appErr := infraerrors.FromError(err)
	if appErr.Reason != "WECHAT_PAYMENT_MP_NOT_CONFIGURED" {
		t.Fatalf("reason = %q, want %q", appErr.Reason, "WECHAT_PAYMENT_MP_NOT_CONFIGURED")
	}
}

func TestMaybeBuildWeChatOAuthRequiredResponseRequiresResumeSigningKey(t *testing.T) {
	t.Parallel()

	svc := &PaymentService{
		configService: &PaymentConfigService{
			settingRepo: &paymentConfigSettingRepoStub{values: map[string]string{
				SettingKeyWeChatConnectEnabled:             "true",
				SettingKeyWeChatConnectAppID:               "wx123456",
				SettingKeyWeChatConnectAppSecret:           "wechat-secret",
				SettingKeyWeChatConnectMode:                "mp",
				SettingKeyWeChatConnectScopes:              "snsapi_base",
				SettingKeyWeChatConnectRedirectURL:         "https://api.example.com/api/v1/auth/oauth/wechat/callback",
				SettingKeyWeChatConnectFrontendRedirectURL: "/auth/wechat/callback",
			}},
			// Intentionally missing payment resume signing key.
			encryptionKey: nil,
		},
	}

	resp, err := svc.maybeBuildWeChatOAuthRequiredResponse(context.Background(), CreateOrderRequest{
		UserID:          123,
		Amount:          12.5,
		PaymentType:     payment.TypeWxpay,
		IsWeChatBrowser: true,
		SrcURL:          "https://merchant.example/payment?from=wechat",
		OrderType:       payment.OrderTypeBalance,
	}, 12.5, 12.88, 0.03)
	if resp != nil {
		t.Fatalf("expected nil response, got %+v", resp)
	}
	if err == nil {
		t.Fatal("expected error, got nil")
	}

	appErr := infraerrors.FromError(err)
	if appErr.Reason != "PAYMENT_RESUME_NOT_CONFIGURED" {
		t.Fatalf("reason = %q, want %q", appErr.Reason, "PAYMENT_RESUME_NOT_CONFIGURED")
	}
}

func TestMaybeBuildWeChatOAuthRequiredResponseFallsBackToConfiguredLegacySigningKey(t *testing.T) {
	svc := &PaymentService{
		configService: &PaymentConfigService{
			settingRepo: &paymentConfigSettingRepoStub{values: map[string]string{
				SettingKeyWeChatConnectEnabled:             "true",
				SettingKeyWeChatConnectAppID:               "wx123456",
				SettingKeyWeChatConnectAppSecret:           "wechat-secret",
				SettingKeyWeChatConnectMode:                "mp",
				SettingKeyWeChatConnectScopes:              "snsapi_base",
				SettingKeyWeChatConnectRedirectURL:         "https://api.example.com/api/v1/auth/oauth/wechat/callback",
				SettingKeyWeChatConnectFrontendRedirectURL: "/auth/wechat/callback",
			}},
			// Legacy stable signing key remains available for no-config upgrade compatibility.
			encryptionKey: []byte("0123456789abcdef0123456789abcdef"),
		},
	}

	resp, err := svc.maybeBuildWeChatOAuthRequiredResponse(context.Background(), CreateOrderRequest{
		UserID:          123,
		Amount:          12.5,
		PaymentType:     payment.TypeWxpay,
		IsWeChatBrowser: true,
		SrcURL:          "https://merchant.example/payment?from=wechat",
		OrderType:       payment.OrderTypeBalance,
	}, 12.5, 12.88, 0.03)
	if err != nil {
		t.Fatalf("expected nil error, got %v", err)
	}
	if resp == nil {
		t.Fatal("expected oauth-required response, got nil")
	}
	if resp.ResultType != payment.CreatePaymentResultOAuthRequired {
		t.Fatalf("result type = %q, want %q", resp.ResultType, payment.CreatePaymentResultOAuthRequired)
	}
	if resp.OAuth == nil || strings.TrimSpace(resp.OAuth.AuthorizeURL) == "" {
		t.Fatalf("expected oauth redirect payload, got %+v", resp.OAuth)
	}
}

func TestMaybeBuildWeChatOAuthRequiredResponseForSelectionSkipsEasyPayProvider(t *testing.T) {
	svc := newWeChatPaymentOAuthTestService(map[string]string{
		SettingKeyWeChatConnectEnabled:             "true",
		SettingKeyWeChatConnectAppID:               "wx123456",
		SettingKeyWeChatConnectAppSecret:           "wechat-secret",
		SettingKeyWeChatConnectMode:                "mp",
		SettingKeyWeChatConnectScopes:              "snsapi_base",
		SettingKeyWeChatConnectRedirectURL:         "https://api.example.com/api/v1/auth/oauth/wechat/callback",
		SettingKeyWeChatConnectFrontendRedirectURL: "/auth/wechat/callback",
	})

	resp, err := svc.maybeBuildWeChatOAuthRequiredResponseForSelection(context.Background(), CreateOrderRequest{
		Amount:          12.5,
		PaymentType:     payment.TypeWxpay,
		IsWeChatBrowser: true,
		OrderType:       payment.OrderTypeBalance,
	}, 12.5, 12.88, 0.03, &payment.InstanceSelection{
		ProviderKey: payment.TypeEasyPay,
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if resp != nil {
		t.Fatalf("expected nil response, got %+v", resp)
	}
}

func TestComputeValidityDaysSupportsSingularAndPluralUnits(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		days int
		unit string
		want int
	}{
		{name: "days", days: 1, unit: "days", want: 1},
		{name: "week", days: 1, unit: "week", want: 7},
		{name: "weeks", days: 2, unit: "weeks", want: 14},
		{name: "month", days: 1, unit: "month", want: 30},
		{name: "months", days: 1, unit: "months", want: 30},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			if got := psComputeValidityDays(tt.days, tt.unit); got != tt.want {
				t.Fatalf("psComputeValidityDays(%d, %q) = %d, want %d", tt.days, tt.unit, got, tt.want)
			}
		})
	}
}

func newWeChatPaymentOAuthTestService(values map[string]string) *PaymentService {
	return &PaymentService{
		configService: &PaymentConfigService{
			settingRepo:   &paymentConfigSettingRepoStub{values: values},
			encryptionKey: []byte("0123456789abcdef0123456789abcdef"),
		},
	}
}
