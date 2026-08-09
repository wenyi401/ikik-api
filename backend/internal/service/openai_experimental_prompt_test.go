package service

import (
	"context"
	"database/sql"
	"encoding/json"
	"testing"

	dbent "ikik-api/ent"
	"ikik-api/ent/enttest"

	"entgo.io/ent/dialect"
	entsql "entgo.io/ent/dialect/sql"
	"github.com/stretchr/testify/require"
	_ "modernc.org/sqlite"
)

type openAIExperimentalPromptSettingRepo struct {
	value         string
	getValueCalls int
}

func (r *openAIExperimentalPromptSettingRepo) Get(context.Context, string) (*Setting, error) {
	return nil, ErrSettingNotFound
}
func (r *openAIExperimentalPromptSettingRepo) GetValue(context.Context, string) (string, error) {
	r.getValueCalls++
	if r.value == "" {
		return "", ErrSettingNotFound
	}
	return r.value, nil
}
func (r *openAIExperimentalPromptSettingRepo) Set(_ context.Context, _ string, value string) error {
	r.value = value
	return nil
}
func (r *openAIExperimentalPromptSettingRepo) GetMultiple(context.Context, []string) (map[string]string, error) {
	return map[string]string{}, nil
}
func (r *openAIExperimentalPromptSettingRepo) SetMultiple(context.Context, map[string]string) error {
	return nil
}
func (r *openAIExperimentalPromptSettingRepo) GetAll(context.Context) (map[string]string, error) {
	return map[string]string{}, nil
}
func (r *openAIExperimentalPromptSettingRepo) Delete(context.Context, string) error { return nil }

type openAIExperimentalPromptUserRepo struct {
	UserRepository
	current *User
}

type openAIExperimentalPromptRedeemRepo struct {
	RedeemCodeRepository
	created []RedeemCode
}

func (r *openAIExperimentalPromptRedeemRepo) Create(_ context.Context, code *RedeemCode) error {
	clone := *code
	r.created = append(r.created, clone)
	return nil
}

func (r *openAIExperimentalPromptUserRepo) GetByID(context.Context, int64) (*User, error) {
	clone := *r.current
	return &clone, nil
}

func newOpenAIExperimentalPromptTestClient(t *testing.T) *dbent.Client {
	t.Helper()
	db, err := sql.Open("sqlite", "file:openai_experimental_prompt?mode=memory&cache=shared&_fk=1")
	require.NoError(t, err)
	t.Cleanup(func() { _ = db.Close() })
	_, err = db.Exec("PRAGMA foreign_keys = ON")
	require.NoError(t, err)

	drv := entsql.OpenDB(dialect.SQLite, db)
	client := enttest.NewClient(t, enttest.WithOptions(dbent.Driver(drv)))
	t.Cleanup(func() { _ = client.Close() })
	return client
}

func TestInjectOpenAIExperimentalPromptPreservesRequestShape(t *testing.T) {
	responsesBody := []byte(`{"model":"gpt-5","instructions":"原有指令","input":"你好"}`)
	result := injectOpenAIExperimentalPrompt(responsesBody, "管理员指令")
	var responses map[string]any
	require.NoError(t, json.Unmarshal(result, &responses))
	require.Equal(t, "管理员指令\n\n原有指令", responses["instructions"])
	require.Equal(t, "你好", responses["input"])

	chatBody := []byte(`{"model":"gpt-5","messages":[{"role":"user","content":"你好"}]}`)
	result = injectOpenAIExperimentalPrompt(chatBody, "管理员指令")
	var chat map[string]any
	require.NoError(t, json.Unmarshal(result, &chat))
	messages := chat["messages"].([]any)
	require.Equal(t, "system", messages[0].(map[string]any)["role"])
	require.Equal(t, "管理员指令", messages[0].(map[string]any)["content"])
	require.Equal(t, "user", messages[1].(map[string]any)["role"])
}

func TestOpenAIExperimentalPromptSettingsCacheRefreshesOnSave(t *testing.T) {
	ctx := context.Background()
	repo := &openAIExperimentalPromptSettingRepo{value: `{"prompt":"旧指令","price_cents":660}`}
	settingsService := &SettingService{settingRepo: repo}

	first, err := settingsService.GetOpenAIExperimentalPromptSettings(ctx)
	require.NoError(t, err)
	require.Equal(t, "旧指令", first.Prompt)

	second, err := settingsService.GetOpenAIExperimentalPromptSettings(ctx)
	require.NoError(t, err)
	require.Equal(t, "旧指令", second.Prompt)
	require.Equal(t, 1, repo.getValueCalls)

	_, err = settingsService.SetOpenAIExperimentalPromptSettings(ctx, OpenAIExperimentalPromptSettings{
		Prompt:     "新指令",
		PriceCents: 880,
	})
	require.NoError(t, err)

	updated, err := settingsService.GetOpenAIExperimentalPromptSettings(ctx)
	require.NoError(t, err)
	require.Equal(t, "新指令", updated.Prompt)
	require.Equal(t, int64(880), updated.PriceCents)
	require.Equal(t, 1, repo.getValueCalls)
}

func TestGenerateOpenAIExperimentalPromptRedeemCodes(t *testing.T) {
	repo := &openAIExperimentalPromptRedeemRepo{}
	adminService := &adminServiceImpl{redeemCodeRepo: repo}

	codes, err := adminService.GenerateRedeemCodes(context.Background(), &GenerateRedeemCodesInput{
		Count:      2,
		Type:       RedeemTypeFeature,
		Value:      99,
		FeatureKey: FeatureKeyOpenAIExperimentalPrompt,
	})
	require.NoError(t, err)
	require.Len(t, codes, 2)
	require.Len(t, repo.created, 2)
	for _, code := range codes {
		require.Equal(t, RedeemTypeFeature, code.Type)
		require.Equal(t, FeatureKeyOpenAIExperimentalPrompt, code.FeatureKey)
		require.Zero(t, code.Value)
	}
}

func TestApplyOpenAIExperimentalPromptUsesCurrentGroupAndEntitlement(t *testing.T) {
	repo := &openAIExperimentalPromptSettingRepo{value: `{"prompt":"管理员指令","price_cents":660}`}
	gateway := &OpenAIGatewayService{settingService: &SettingService{settingRepo: repo}}
	body := []byte(`{"model":"gpt-5","input":"你好"}`)
	apiKey := &APIKey{
		OpenAIExperimentalPromptEnabled: true,
		User:                            &User{OpenAIExperimentalPromptUnlocked: true},
		Group: &Group{
			Platform:                        PlatformOpenAI,
			OpenAIExperimentalPromptEnabled: true,
		},
	}

	result := gateway.ApplyOpenAIExperimentalPrompt(context.Background(), apiKey, body)
	require.Contains(t, string(result), "管理员指令")

	apiKey.OpenAIExperimentalPromptEnabled = false
	result = gateway.ApplyOpenAIExperimentalPrompt(context.Background(), apiKey, body)
	require.Equal(t, string(body), string(result))

	apiKey.OpenAIExperimentalPromptEnabled = true
	apiKey.Group.OpenAIExperimentalPromptEnabled = false
	result = gateway.ApplyOpenAIExperimentalPrompt(context.Background(), apiKey, body)
	require.Equal(t, string(body), string(result))

	apiKey.Group.Platform = PlatformAnthropic
	apiKey.Group.OpenAIExperimentalPromptEnabled = true
	result = gateway.ApplyOpenAIExperimentalPrompt(context.Background(), apiKey, body)
	require.Equal(t, string(body), string(result))
}

func TestPurchaseOpenAIExperimentalPromptDeductsBalanceAtomically(t *testing.T) {
	ctx := context.Background()
	client := newOpenAIExperimentalPromptTestClient(t)
	created, err := client.User.Create().
		SetEmail("experimental-prompt@example.com").
		SetPasswordHash("hash").
		SetBalance(10).
		Save(ctx)
	require.NoError(t, err)

	userRepo := &openAIExperimentalPromptUserRepo{current: &User{
		ID:                               created.ID,
		Balance:                          created.Balance,
		OpenAIExperimentalPromptUnlocked: false,
	}}
	settingService := &SettingService{settingRepo: &openAIExperimentalPromptSettingRepo{
		value: `{"prompt":"管理员指令","price_cents":660}`,
	}}
	redeemService := NewRedeemService(nil, userRepo, nil, nil, nil, client, nil, nil)
	redeemService.SetSettingService(settingService)

	status, err := redeemService.PurchaseOpenAIExperimentalPrompt(ctx, created.ID)
	require.NoError(t, err)
	require.True(t, status.Unlocked)

	updated, err := client.User.Get(ctx, created.ID)
	require.NoError(t, err)
	require.True(t, updated.OpenaiExperimentalPromptUnlocked)
	require.InDelta(t, 3.4, updated.Balance, 0.000001)

	_, err = redeemService.PurchaseOpenAIExperimentalPrompt(ctx, created.ID)
	require.ErrorIs(t, err, ErrFeatureAlreadyUnlocked)
}

func TestPurchaseOpenAIExperimentalPromptLeavesBalanceUntouchedWhenInsufficient(t *testing.T) {
	ctx := context.Background()
	client := newOpenAIExperimentalPromptTestClient(t)
	created, err := client.User.Create().
		SetEmail("experimental-prompt-insufficient@example.com").
		SetPasswordHash("hash").
		SetBalance(2.35).
		Save(ctx)
	require.NoError(t, err)

	userRepo := &openAIExperimentalPromptUserRepo{current: &User{
		ID:                               created.ID,
		Balance:                          created.Balance,
		OpenAIExperimentalPromptUnlocked: false,
	}}
	settingService := &SettingService{settingRepo: &openAIExperimentalPromptSettingRepo{
		value: `{"prompt":"管理员指令","price_cents":660}`,
	}}
	redeemService := NewRedeemService(nil, userRepo, nil, nil, nil, client, nil, nil)
	redeemService.SetSettingService(settingService)

	_, err = redeemService.PurchaseOpenAIExperimentalPrompt(ctx, created.ID)
	require.ErrorIs(t, err, ErrInsufficientBalance)

	unchanged, err := client.User.Get(ctx, created.ID)
	require.NoError(t, err)
	require.False(t, unchanged.OpenaiExperimentalPromptUnlocked)
	require.InDelta(t, 2.35, unchanged.Balance, 0.000001)
}
