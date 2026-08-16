package handler

import (
	"context"
	"encoding/json"
	"strings"
	"testing"
	"time"

	"ikik-api/internal/pkg/apicompat"
	"ikik-api/internal/pkg/pagination"
	"ikik-api/internal/pkg/usagestats"
	"ikik-api/internal/service"
)

type petToolUserStub struct {
	seenUserID int64
	user       *service.User
}

func (s *petToolUserStub) GetProfile(_ context.Context, userID int64) (*service.User, error) {
	s.seenUserID = userID
	return s.user, nil
}

type petToolAPIStub struct {
	seenUserID int64
	groups     []service.Group
	rates      map[int64]float64
	keys       []service.APIKey
}

func (s *petToolAPIStub) GetAvailableGroups(_ context.Context, userID int64) ([]service.Group, error) {
	s.seenUserID = userID
	return s.groups, nil
}

func (s *petToolAPIStub) GetUserGroupRates(_ context.Context, userID int64) (map[int64]float64, error) {
	s.seenUserID = userID
	return s.rates, nil
}

func (s *petToolAPIStub) List(_ context.Context, userID int64, _ pagination.PaginationParams, _ service.APIKeyListFilters) ([]service.APIKey, *pagination.PaginationResult, error) {
	s.seenUserID = userID
	return s.keys, &pagination.PaginationResult{Total: int64(len(s.keys))}, nil
}

type petToolUsageStub struct {
	seenUserID int64
	stats      *usagestats.BatchUserUsageStats
}

func (s *petToolUsageStub) GetBatchUserUsageStats(_ context.Context, userIDs []int64, _, _ time.Time) (map[int64]*usagestats.BatchUserUsageStats, error) {
	if len(userIDs) > 0 {
		s.seenUserID = userIDs[0]
	}
	return map[int64]*usagestats.BatchUserUsageStats{s.seenUserID: s.stats}, nil
}

func TestPetSupportToolsNeverAcceptUserID(t *testing.T) {
	users := &petToolUserStub{user: &service.User{Status: service.StatusActive, Balance: 12.5}}
	registry := newPetSupportToolRegistry(users, &petToolAPIStub{}, &petToolUsageStub{})
	call := apicompat.ChatToolCall{Function: apicompat.ChatFunctionCall{
		Name: petToolAccountOverview, Arguments: `{"user_id":999}`,
	}}
	result := registry.Execute(context.Background(), 42, call)
	if !strings.Contains(result, `"error":"invalid_arguments"`) {
		t.Fatalf("expected invalid arguments, got %s", result)
	}
	if users.seenUserID != 0 {
		t.Fatalf("tool reached user reader with model-supplied user ID: %d", users.seenUserID)
	}

	call.Function.Arguments = `{}`
	result = registry.Execute(context.Background(), 42, call)
	if users.seenUserID != 42 || !strings.Contains(result, `"balance_usd":12.5`) {
		t.Fatalf("tool did not use authenticated user: id=%d result=%s", users.seenUserID, result)
	}

	call.Function.Arguments = `{ }`
	result = registry.Execute(context.Background(), 42, call)
	if !strings.Contains(result, `"ok":true`) {
		t.Fatalf("valid empty JSON object should be accepted: %s", result)
	}
}

func TestPetSupportToolDefinitionsExposeOnlyReadOnlySchemas(t *testing.T) {
	registry := newPetSupportToolRegistry(&petToolUserStub{}, &petToolAPIStub{}, &petToolUsageStub{})
	definitions := registry.Definitions()
	if len(definitions) != 4 {
		t.Fatalf("expected four tools, got %d", len(definitions))
	}
	encoded, err := json.Marshal(definitions)
	if err != nil {
		t.Fatal(err)
	}
	text := string(encoded)
	for _, name := range []string{petToolAccountOverview, petToolAvailableGroups, petToolAPIKeys, petToolUsageSummary} {
		if !strings.Contains(text, name) {
			t.Fatalf("missing tool %s: %s", name, text)
		}
	}
	if strings.Contains(text, "user_id") || strings.Contains(text, "password") || strings.Contains(text, "token") {
		t.Fatalf("tool schema exposes a forbidden parameter: %s", text)
	}
}

func TestPetSupportGroupToolUsesCurrentUserRate(t *testing.T) {
	api := &petToolAPIStub{
		groups: []service.Group{{ID: 7, Name: "OpenAI", Platform: service.PlatformOpenAI, RateMultiplier: 1.2}},
		rates:  map[int64]float64{7: 0.5},
	}
	registry := newPetSupportToolRegistry(&petToolUserStub{}, api, &petToolUsageStub{})
	result := registry.Execute(context.Background(), 42, apicompat.ChatToolCall{Function: apicompat.ChatFunctionCall{
		Name: petToolAvailableGroups, Arguments: `{"limit":10}`,
	}})
	if api.seenUserID != 42 || !strings.Contains(result, `"effective_rate_multiplier":0.5`) {
		t.Fatalf("unexpected group result: id=%d result=%s", api.seenUserID, result)
	}
}

func TestPetSupportAPIKeyToolNeverReturnsSecret(t *testing.T) {
	now := time.Now()
	group := &service.Group{ID: 7, Name: "OpenAI"}
	groupID := group.ID
	api := &petToolAPIStub{keys: []service.APIKey{
		{ID: 1, Key: "sk-secret", Name: playgroundAPIKeyPrefix + "7", Status: service.StatusAPIKeyActive},
		{ID: 2, Key: "sk-user-secret", Name: "工作密钥", GroupID: &groupID, Group: group, Status: service.StatusAPIKeyActive, Quota: 10, QuotaUsed: 3, LastUsedAt: &now},
	}}
	registry := newPetSupportToolRegistry(&petToolUserStub{}, api, &petToolUsageStub{})
	result := registry.Execute(context.Background(), 42, apicompat.ChatToolCall{Function: apicompat.ChatFunctionCall{
		Name: petToolAPIKeys, Arguments: `{}`,
	}})
	if strings.Contains(result, "sk-secret") || strings.Contains(result, "sk-user-secret") || strings.Contains(result, playgroundAPIKeyPrefix) {
		t.Fatalf("API key tool leaked a secret or internal key: %s", result)
	}
	for _, want := range []string{`"name":"工作密钥"`, `"quota_remaining_usd":7`, `"group_name":"OpenAI"`} {
		if !strings.Contains(result, want) {
			t.Fatalf("API key result missing %s: %s", want, result)
		}
	}
}

func TestPetSupportUsageToolUsesAuthenticatedUser(t *testing.T) {
	usage := &petToolUsageStub{stats: &usagestats.BatchUserUsageStats{
		UserID: 42, TodayActualCost: 1.25, TotalActualCost: 4.5,
	}}
	registry := newPetSupportToolRegistry(&petToolUserStub{}, &petToolAPIStub{}, usage)
	result := registry.Execute(context.Background(), 42, apicompat.ChatToolCall{Function: apicompat.ChatFunctionCall{
		Name: petToolUsageSummary, Arguments: `{"days":7}`,
	}})
	if usage.seenUserID != 42 || !strings.Contains(result, `"today_charged_cost_usd":1.25`) || !strings.Contains(result, `"period_charged_cost_usd":4.5`) {
		t.Fatalf("unexpected usage result: id=%d result=%s", usage.seenUserID, result)
	}
}

func TestPetSupportToolQuestion(t *testing.T) {
	for _, question := range []string{"我今天用了多少钱", "我的 API key 过期了吗", "which group supports image generation"} {
		if !petSupportToolQuestion(question) {
			t.Fatalf("expected tool question: %q", question)
		}
	}
	if petSupportToolQuestion("今天天气怎么样") {
		t.Fatal("weather question should not enter the billed tool flow")
	}
}

func TestPetCompletionMessageParsesToolCalls(t *testing.T) {
	message, err := petCompletionMessage([]byte(`{"choices":[{"message":{"role":"assistant","content":null,"tool_calls":[{"id":"call_1","type":"function","function":{"name":"get_my_account_overview","arguments":"{}"}}]}}]}`))
	if err != nil {
		t.Fatal(err)
	}
	if len(message.ToolCalls) != 1 || message.ToolCalls[0].Function.Name != petToolAccountOverview {
		t.Fatalf("unexpected tool calls: %#v", message.ToolCalls)
	}
}
