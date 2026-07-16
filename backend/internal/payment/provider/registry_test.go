package provider

import (
	"errors"
	"fmt"
	"strings"
	"testing"

	"ikik-api/internal/payment"
	infraerrors "ikik-api/internal/pkg/errors"
)

// TestCreateProviderUnknownKeyMessage 锁定 unknown-key 错误文案（等价性硬约束 B2）：
// 必须与改造前 factory.go switch default 分支的文案逐字节一致。
func TestCreateProviderUnknownKeyMessage(t *testing.T) {
	prov, err := CreateProvider("nope", "inst-1", map[string]string{})
	if prov != nil {
		t.Fatalf("expected nil provider for unknown key, got %T", prov)
	}
	if err == nil {
		t.Fatal("expected error for unknown provider key")
	}
	const want = "unknown provider key: nope"
	if got := err.Error(); got != want {
		t.Fatalf("unknown-key error message changed:\n got: %q\nwant: %q", got, want)
	}
}

// TestRegistryContainsExactlyAllProviderKeys 验证 5 个 provider 的 init 自注册全部生效，
// 且没有意外多注册的 key。
func TestRegistryContainsExactlyAllProviderKeys(t *testing.T) {
	want := []string{
		payment.TypeEasyPay,
		payment.TypeAlipay,
		payment.TypeWxpay,
		payment.TypeStripe,
		payment.TypeAirwallex,
	}
	if len(constructors) != len(want) {
		t.Fatalf("expected %d registered provider keys, got %d: %v", len(want), len(constructors), registeredKeys())
	}
	for _, key := range want {
		if _, ok := constructors[key]; !ok {
			t.Errorf("provider key %q not registered", key)
		}
	}
}

func registeredKeys() []string {
	keys := make([]string, 0, len(constructors))
	for k := range constructors {
		keys = append(keys, k)
	}
	return keys
}

// TestCreateProviderDispatchesToConstructors 验证每个已注册 key 都能分发到对应构造器：
// 空 config 下返回的错误必须是各 provider 自身的校验错误（原样透传，等价性硬约束 B1），
// 而不是 unknown-key 错误或任何包裹后的错误。
func TestCreateProviderDispatchesToConstructors(t *testing.T) {
	cases := []struct {
		key     string
		wantErr string
	}{
		{payment.TypeEasyPay, "easypay config missing required key: pid"},
		{payment.TypeAlipay, "alipay config missing required key: appId"},
		{payment.TypeStripe, "stripe config missing required key: secretKey"},
		{payment.TypeAirwallex, "airwallex config missing required key: clientId"},
	}
	for _, tc := range cases {
		_, err := CreateProvider(tc.key, "_validate_", map[string]string{})
		if err == nil {
			t.Errorf("key %q: expected constructor validation error, got nil", tc.key)
			continue
		}
		if got := err.Error(); got != tc.wantErr {
			t.Errorf("key %q: constructor error changed:\n got: %q\nwant: %q", tc.key, got, tc.wantErr)
		}
	}
}

// TestCreateProviderPassesThroughApplicationError 验证 wxpay 构造器返回的结构化
// *infraerrors.ApplicationError 被原样透传不包裹（等价性硬约束 B1）：
// "_validate_" 路径依赖该结构化错误做前端 i18n（service/payment_config_providers.go）。
func TestCreateProviderPassesThroughApplicationError(t *testing.T) {
	_, err := CreateProvider(payment.TypeWxpay, "_validate_", map[string]string{})
	if err == nil {
		t.Fatal("expected wxpay constructor validation error, got nil")
	}
	appErr, ok := err.(*infraerrors.ApplicationError)
	if !ok {
		var as *infraerrors.ApplicationError
		if errors.As(err, &as) {
			t.Fatalf("ApplicationError was wrapped instead of passed through as-is: %T", err)
		}
		t.Fatalf("expected *infraerrors.ApplicationError, got %T: %v", err, err)
	}
	if appErr.Reason != "WXPAY_CONFIG_MISSING_KEY" {
		t.Errorf("unexpected reason: got %q, want %q", appErr.Reason, "WXPAY_CONFIG_MISSING_KEY")
	}
	if appErr.Metadata["key"] != "appId" {
		t.Errorf("unexpected metadata key: got %q, want %q", appErr.Metadata["key"], "appId")
	}
}

// TestCreateProviderSuccessPath 验证命中 key 且 config 合法时返回可用的 Provider 实例。
func TestCreateProviderSuccessPath(t *testing.T) {
	prov, err := CreateProvider(payment.TypeStripe, "inst-42", map[string]string{"secretKey": "sk_test_123"})
	if err != nil {
		t.Fatalf("expected success, got error: %v", err)
	}
	if prov == nil {
		t.Fatal("expected non-nil provider")
	}
	if got := prov.ProviderKey(); got != payment.TypeStripe {
		t.Fatalf("unexpected provider key: got %q, want %q", got, payment.TypeStripe)
	}
}

// TestRegisterDuplicateKeyPanics 验证重复注册同一 key 在 init 期立即 panic，
// 使插装错误尽早暴露。复用已注册的 stripe key，panic 发生在写入之前，
// 不会污染包级注册表状态（等价性硬约束 B3）。
func TestRegisterDuplicateKeyPanics(t *testing.T) {
	before := len(constructors)
	defer func() {
		r := recover()
		if r == nil {
			t.Fatal("expected panic on duplicate registration")
		}
		msg := fmt.Sprint(r)
		if !strings.Contains(msg, fmt.Sprintf("%q", payment.TypeStripe)) {
			t.Fatalf("panic message should contain the duplicate key %q, got: %s", payment.TypeStripe, msg)
		}
		if len(constructors) != before {
			t.Fatalf("duplicate registration mutated registry state: %d -> %d", before, len(constructors))
		}
	}()
	register(payment.TypeStripe, func(string, map[string]string) (payment.Provider, error) {
		return nil, nil
	})
}
