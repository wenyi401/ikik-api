package service

import (
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestNormalizeAutoModelSettings(t *testing.T) {
	cfg := NormalizeAutoModelSettings(AutoModelSettings{
		Enabled: true,
		Models: []AutoModelRule{
			{
				Name:              " ikik-auto ",
				Enabled:           true,
				AllowedGroupIDs:   []int64{3, 2, 3, -1, 0},
				RoutingMode:       "openrouter",
				SmallModel:        " gpt-5.4-mini ",
				BalancedModel:     " gpt-5.5 ",
				LargeModel:        " gpt-5.5 ",
				AllowedModels:     []string{" gpt-5.4-mini ", "gpt-5.5", "GPT-5.5"},
				BalancedThreshold: 0,
				LargeThreshold:    20,
				CostQuality:       99,
			},
			{
				Name:       "IKIK-AUTO",
				Enabled:    true,
				SmallModel: "duplicate",
			},
			{
				Name:    "empty-targets",
				Enabled: true,
			},
		},
	})

	if !cfg.Enabled {
		t.Fatal("expected enabled settings")
	}
	if len(cfg.Models) != 1 {
		t.Fatalf("expected one normalized model, got %d", len(cfg.Models))
	}
	rule := cfg.Models[0]
	if rule.Name != "ikik-auto" {
		t.Fatalf("unexpected name: %q", rule.Name)
	}
	if rule.SmallModel != "gpt-5.4-mini" || rule.BalancedModel != "gpt-5.5" || rule.LargeModel != "gpt-5.5" {
		t.Fatalf("unexpected normalized models: %+v", rule)
	}
	if rule.RoutingMode != AutoModelRoutingModeRouter {
		t.Fatalf("unexpected routing mode: %s", rule.RoutingMode)
	}
	if len(rule.AllowedGroupIDs) != 2 || rule.AllowedGroupIDs[0] != 2 || rule.AllowedGroupIDs[1] != 3 {
		t.Fatalf("unexpected allowed groups: %#v", rule.AllowedGroupIDs)
	}
	if len(rule.AllowedModels) != 2 || rule.AllowedModels[0] != "gpt-5.4-mini" || rule.AllowedModels[1] != "gpt-5.5" {
		t.Fatalf("unexpected allowed models: %#v", rule.AllowedModels)
	}
	if rule.CostQuality != 10 {
		t.Fatalf("unexpected cost quality tradeoff: %d", rule.CostQuality)
	}
	if rule.BalancedThreshold != autoModelDefaultBalancedThreshold {
		t.Fatalf("unexpected balanced threshold: %d", rule.BalancedThreshold)
	}
	if rule.LargeThreshold != autoModelDefaultBalancedThreshold {
		t.Fatalf("large threshold should not be below balanced threshold, got %d", rule.LargeThreshold)
	}
}

func TestFindAutoModelRuleRespectsAllowedGroups(t *testing.T) {
	cfg := AutoModelSettings{
		Enabled: true,
		Models: []AutoModelRule{
			{
				Name:            "ikik-auto",
				Enabled:         true,
				SmallModel:      "mini",
				AllowedGroupIDs: []int64{10, 20},
			},
		},
	}
	allowedGroupID := int64(20)
	deniedGroupID := int64(30)

	if _, ok := findAutoModelRule(cfg, "ikik-auto", &allowedGroupID); !ok {
		t.Fatal("expected allowed group to match auto model")
	}
	if _, ok := findAutoModelRule(cfg, "ikik-auto", &deniedGroupID); ok {
		t.Fatal("denied group should not match auto model")
	}
	if _, ok := findAutoModelRule(cfg, "ikik-auto", nil); ok {
		t.Fatal("nil group should not match restricted auto model")
	}
}

func TestEstimateAutoModelScore(t *testing.T) {
	body := []byte(`{"model":"ikik-auto","reasoning":{"effort":"high"},"tools":[{"name":"read"}],"messages":[{"role":"user","content":"debug this large codebase"}]}`)

	score, reason := estimateAutoModelScore(body, AutoModelProtocolOpenAIChat)
	if score < autoModelDefaultLargeThreshold {
		t.Fatalf("expected complex request to reach large threshold, got score=%d reason=%s", score, reason)
	}
	if !strings.Contains(reason, "high_reasoning") || !strings.Contains(reason, "tools") {
		t.Fatalf("unexpected reason: %s", reason)
	}
}

func TestChooseAutoModelTarget(t *testing.T) {
	rule := AutoModelRule{
		SmallModel:        "mini",
		BalancedModel:     "standard",
		LargeModel:        "large",
		BalancedThreshold: 35,
		LargeThreshold:    70,
	}

	if model, tier := chooseAutoModelTarget(rule, 10); model != "mini" || tier != "small" {
		t.Fatalf("expected small target, got %s/%s", model, tier)
	}
	if model, tier := chooseAutoModelTarget(rule, 40); model != "standard" || tier != "balanced" {
		t.Fatalf("expected balanced target, got %s/%s", model, tier)
	}
	if model, tier := chooseAutoModelTarget(rule, 90); model != "large" || tier != "large" {
		t.Fatalf("expected large target, got %s/%s", model, tier)
	}
}

func TestChooseAutoRouterTarget(t *testing.T) {
	rule := AutoModelRule{
		Name:          "ikik-auto",
		RoutingMode:   AutoModelRoutingModeRouter,
		AllowedModels: []string{"gpt-5.4-mini", "gpt-5.5"},
		CostQuality:   8,
		StickySession: true,
	}

	model, tier, reason := chooseAutoRouterTarget(rule, 15, "short_body", []byte(`{"messages":[{"role":"user","content":"hi"}]}`), autoRouterRequestOverride{})
	if model != "gpt-5.4-mini" {
		t.Fatalf("expected simple task to use mini, got %s tier=%s reason=%s", model, tier, reason)
	}

	model, tier, reason = chooseAutoRouterTarget(rule, 90, "tools,high_reasoning", []byte(`{"messages":[{"role":"user","content":"refactor a large system"}]}`), autoRouterRequestOverride{})
	if model != "gpt-5.5" {
		t.Fatalf("expected complex task to use quality model, got %s tier=%s reason=%s", model, tier, reason)
	}
}

func TestExtractAutoRouterRequestOverride(t *testing.T) {
	body := []byte(`{"plugins":[{"id":"auto-router","cost_quality_tradeoff":3,"allowed_models":["gpt-5.5","gpt-5.4-mini"],"session_stickiness":false}]}`)
	override := extractAutoRouterRequestOverride(body)
	if !override.present {
		t.Fatal("expected auto-router override")
	}
	if !override.hasCostQuality || override.costQuality != 3 {
		t.Fatalf("unexpected cost quality override: %+v", override)
	}
	if !override.disableStickiness {
		t.Fatal("expected stickiness override")
	}
	if len(override.allowedModels) != 2 || override.allowedModels[0] != "gpt-5.5" {
		t.Fatalf("unexpected allowed models: %#v", override.allowedModels)
	}
}

func TestStripAutoRouterPluginFromBody(t *testing.T) {
	body := []byte(`{"model":"ikik-auto","plugins":[{"id":"auto-router","allowed_models":["gpt-5.5"]},{"id":"other","foo":true}],"messages":[{"role":"user","content":"hi"}]}`)
	out := StripAutoRouterPluginFromBody(body)
	text := string(out)
	if strings.Contains(text, "auto-router") {
		t.Fatalf("auto-router plugin was not stripped: %s", text)
	}
	if !strings.Contains(text, `"id":"other"`) {
		t.Fatalf("non-router plugin should be preserved: %s", text)
	}

	body = []byte(`{"model":"ikik-auto","plugins":[{"id":"auto-router"}],"messages":[]}`)
	out = StripAutoRouterPluginFromBody(body)
	if strings.Contains(string(out), "plugins") {
		t.Fatalf("empty plugins array should be removed: %s", string(out))
	}
}

func TestParseAutoRouterModelChoice(t *testing.T) {
	result, ok := parseAutoRouterModelChoice(`{"selected_model":"GPT-5.5","confidence":0.82,"reason":"complex coding task"}`, []string{"gpt-5.4-mini", "gpt-5.5"})
	if !ok {
		t.Fatal("expected router choice to parse")
	}
	if result.selectedModel != "gpt-5.5" {
		t.Fatalf("unexpected selected model: %s", result.selectedModel)
	}
	if result.confidence != 0.82 {
		t.Fatalf("unexpected confidence: %f", result.confidence)
	}

	if _, ok := parseAutoRouterModelChoice(`{"selected_model":"not-allowed","confidence":1}`, []string{"gpt-5.4-mini"}); ok {
		t.Fatal("model outside candidates should be rejected")
	}
}

func TestChooseAIAutoRouterTarget(t *testing.T) {
	var sawAuthorization bool
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/v1/chat/completions" {
			t.Fatalf("unexpected path: %s", r.URL.Path)
		}
		if r.Header.Get("Authorization") == "Bearer test-router-key" {
			sawAuthorization = true
		}
		body, _ := io.ReadAll(r.Body)
		if !strings.Contains(string(body), `"model":"gpt-5.4-mini"`) {
			t.Fatalf("router model missing from request body: %s", string(body))
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"choices":[{"message":{"content":"{\"selected_model\":\"gpt-5.5\",\"confidence\":0.77,\"reason\":\"needs stronger model\"}"}}]}`))
	}))
	defer server.Close()

	rule := AutoModelRule{
		Name:               "ikik-auto",
		AIRouterEnabled:    true,
		RouterModel:        "gpt-5.4-mini",
		RouterBaseURL:      server.URL + "/v1",
		RouterAPIKey:       "test-router-key",
		RouterTimeoutMS:    1000,
		RouterMaxTokens:    128,
		RouterReasoning:    "low",
		RouterPrompt:       defaultAutoRouterPrompt(),
		RouterConservative: true,
		AllowedModels:      []string{"gpt-5.4-mini", "gpt-5.5"},
		CostQuality:        7,
	}

	result, ok := chooseAIAutoRouterTarget(context.Background(), rule, 45, "default", []byte(`{"messages":[{"role":"user","content":"help me design this API"}]}`), AutoModelProtocolOpenAIChat, autoRouterRequestOverride{})
	if !ok {
		t.Fatal("expected ai router to choose a model")
	}
	if !sawAuthorization {
		t.Fatal("router api key was not sent")
	}
	if result.selectedModel != "gpt-5.5" {
		t.Fatalf("unexpected selected model: %s", result.selectedModel)
	}
	if result.confidence != 0.77 {
		t.Fatalf("unexpected confidence: %f", result.confidence)
	}
}

func TestBuildAutoModelUsageFields(t *testing.T) {
	fields := BuildAutoModelUsageFields(
		AutoModelDecision{
			Matched:        true,
			RequestedModel: "ikik-auto",
			ResolvedModel:  "gpt-5.5",
		},
		ChannelMappingResult{
			ChannelID:          42,
			Mapped:             true,
			MappedModel:        "upstream-gpt",
			BillingModelSource: BillingModelSourceRequested,
		},
		"upstream-final",
	)

	if fields.ChannelID != 42 {
		t.Fatalf("unexpected channel id: %d", fields.ChannelID)
	}
	if fields.OriginalModel != "ikik-auto" {
		t.Fatalf("unexpected original model: %s", fields.OriginalModel)
	}
	if fields.ChannelMappedModel != "upstream-gpt" {
		t.Fatalf("unexpected channel mapped model: %s", fields.ChannelMappedModel)
	}
	if fields.BillingModelSource != BillingModelSourceChannelMapped {
		t.Fatalf("auto model should bill by real mapped model, got %s", fields.BillingModelSource)
	}
	if fields.ModelMappingChain != "ikik-auto→gpt-5.5→upstream-gpt→upstream-final" {
		t.Fatalf("unexpected mapping chain: %s", fields.ModelMappingChain)
	}
}
