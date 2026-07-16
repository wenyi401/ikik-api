package service

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
	"github.com/tidwall/gjson"
	"ikik-api/internal/config"
)

func TestNormalizeOpenAIReasoningEffortForGPT56(t *testing.T) {
	tests := []struct {
		name  string
		raw   string
		model string
		want  string
	}{
		{name: "sol", raw: "max", model: "gpt-5.6-sol", want: "max"},
		{name: "terra namespaced", raw: "max", model: "openai/gpt-5.6-terra", want: "max"},
		{name: "luna suffix", raw: "max", model: "gpt-5.6-luna-2026-07-09", want: "max"},
		{name: "other model", raw: "max", model: "gpt-5.5", want: "xhigh"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			require.Equal(t, tt.want, normalizeOpenAIReasoningEffortForModel(tt.raw, tt.model))
		})
	}
}

func TestNormalizeOpenAICodexCompactReasoningEffortScope(t *testing.T) {
	gin.SetMode(gin.TestMode)
	body := []byte(`{"model":"gpt-5.6-sol","reasoning":{"effort":"max","summary":"auto"}}`)

	tests := []struct {
		name    string
		path    string
		account *Account
		changed bool
		want    string
	}{
		{
			name:    "openai oauth compact",
			path:    "/v1/responses/compact",
			account: &Account{Platform: PlatformOpenAI, Type: AccountTypeOAuth},
			changed: true,
			want:    "xhigh",
		},
		{
			name:    "openai oauth responses",
			path:    "/v1/responses",
			account: &Account{Platform: PlatformOpenAI, Type: AccountTypeOAuth},
			want:    "max",
		},
		{
			name:    "openai api key compact",
			path:    "/v1/responses/compact",
			account: &Account{Platform: PlatformOpenAI, Type: AccountTypeAPIKey},
			want:    "max",
		},
		{
			name:    "other platform oauth compact",
			path:    "/v1/responses/compact",
			account: &Account{Platform: PlatformGrok, Type: AccountTypeOAuth},
			want:    "max",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			recorder := httptest.NewRecorder()
			c, _ := gin.CreateTestContext(recorder)
			c.Request = httptest.NewRequest(http.MethodPost, tt.path, nil)

			normalized, changed, err := normalizeOpenAICodexCompactReasoningEffortForAccount(c, tt.account, body)
			require.NoError(t, err)
			require.Equal(t, tt.changed, changed)
			require.Equal(t, tt.want, gjson.GetBytes(normalized, "reasoning.effort").String())
			require.Equal(t, "auto", gjson.GetBytes(normalized, "reasoning.summary").String())
		})
	}
}

func TestExtractOpenAIUsageIncludesCacheWriteTokens(t *testing.T) {
	tests := []struct {
		name string
		body string
		want OpenAIUsage
	}{
		{
			name: "responses official fields",
			body: `{"response":{"usage":{"input_tokens":100,"output_tokens":12,"input_tokens_details":{"cached_tokens":20,"cache_write_tokens":30},"output_tokens_details":{"reasoning_tokens":5,"image_tokens":2}}}}`,
			want: OpenAIUsage{InputTokens: 100, OutputTokens: 12, CacheCreationInputTokens: 30, CacheReadInputTokens: 20, ReasoningTokens: 5, ImageOutputTokens: 2},
		},
		{
			name: "chat official fields",
			body: `{"usage":{"prompt_tokens":80,"completion_tokens":9,"prompt_tokens_details":{"cached_tokens":10,"cache_write_tokens":15},"completion_tokens_details":{"reasoning_tokens":4}}}`,
			want: OpenAIUsage{InputTokens: 80, OutputTokens: 9, CacheCreationInputTokens: 15, CacheReadInputTokens: 10, ReasoningTokens: 4},
		},
		{
			name: "compatible aliases",
			body: `{"usage":{"input_tokens":60,"output_tokens":7,"cache_read_tokens":8,"cache_creation_input_tokens":11}}`,
			want: OpenAIUsage{InputTokens: 60, OutputTokens: 7, CacheCreationInputTokens: 11, CacheReadInputTokens: 8},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, ok := extractOpenAIUsageFromJSONBytes([]byte(tt.body))
			require.True(t, ok)
			require.Equal(t, tt.want, got)
		})
	}
}

func TestGPT56PricingAndChannelCacheWriteOverride(t *testing.T) {
	svc := NewBillingService(&config.Config{}, nil)

	pricing, err := svc.GetModelPricing("gpt-5.6-terra")
	require.NoError(t, err)
	require.InDelta(t, 2.5e-6, pricing.InputPricePerToken, 1e-15)
	require.InDelta(t, 3.125e-6, pricing.CacheCreationPricePerToken, 1e-15)
	require.InDelta(t, 6.25e-6, pricing.CacheCreationPricePerTokenPriority, 1e-15)
	require.Equal(t, openAIGPT54LongContextInputThreshold, pricing.LongContextInputThreshold)

	cost, err := svc.CalculateCostWithServiceTier("gpt-5.6-terra", UsageTokens{
		InputTokens:         100,
		OutputTokens:        10,
		CacheCreationTokens: 40,
		CacheReadTokens:     20,
	}, 1, "priority")
	require.NoError(t, err)
	require.InDelta(t, 100*5e-6, cost.InputCost, 1e-15)
	require.InDelta(t, 10*30e-6, cost.OutputCost, 1e-15)
	require.InDelta(t, 40*6.25e-6, cost.CacheCreationCost, 1e-15)
	require.InDelta(t, 20*0.5e-6, cost.CacheReadCost, 1e-15)

	zero := 0.0
	overridden, err := svc.GetModelPricingWithChannel("gpt-5.6-terra", &ChannelModelPricing{CacheWritePrice: &zero})
	require.NoError(t, err)
	require.True(t, overridden.CacheCreationPriceExplicit)
	require.Zero(t, overridden.CacheCreationPricePerToken)
	require.Zero(t, overridden.CacheCreationPricePerTokenPriority)

	baseAgain, err := svc.GetModelPricing("gpt-5.6-terra")
	require.NoError(t, err)
	require.InDelta(t, 3.125e-6, baseAgain.CacheCreationPricePerToken, 1e-15)
}

func TestOpenAIUsageNestedZeroOverridesTopLevelCacheAliases(t *testing.T) {
	usage, ok := openAIUsageFromGJSON(gjson.Parse(`{
		"input_tokens": 100,
		"cache_creation_input_tokens": 45,
		"cache_read_tokens": 30,
		"input_tokens_details": {"cache_write_tokens": 0, "cached_tokens": 0}
	}`))
	require.True(t, ok)
	require.Zero(t, usage.CacheCreationInputTokens)
	require.Zero(t, usage.CacheReadInputTokens)
}
