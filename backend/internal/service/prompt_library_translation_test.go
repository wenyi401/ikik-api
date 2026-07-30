package service

import (
	"context"
	"encoding/json"
	"errors"
	"testing"

	"github.com/stretchr/testify/require"
)

type promptLibraryTranslationRepoStub struct {
	items       map[string]PromptLibraryTranslation
	upsertCalls int
}

func (r *promptLibraryTranslationRepoStub) GetByPromptIDs(
	_ context.Context,
	locale string,
	promptIDs []string,
) (map[string]PromptLibraryTranslation, error) {
	result := make(map[string]PromptLibraryTranslation, len(promptIDs))
	for _, id := range promptIDs {
		item, ok := r.items[id]
		if ok && item.Locale == locale {
			result[id] = item
		}
	}
	return result, nil
}

func (r *promptLibraryTranslationRepoStub) Upsert(_ context.Context, items []PromptLibraryTranslation) error {
	r.upsertCalls++
	if r.items == nil {
		r.items = make(map[string]PromptLibraryTranslation)
	}
	for _, item := range items {
		r.items[item.PromptID] = item
	}
	return nil
}

func (r *promptLibraryTranslationRepoStub) Count(_ context.Context, locale string) (int64, error) {
	var count int64
	for _, item := range r.items {
		if item.Locale == locale {
			count++
		}
	}
	return count, nil
}

type promptLibraryTranslationSettingRepoStub struct {
	SettingRepository
	value string
}

func (r *promptLibraryTranslationSettingRepoStub) GetValue(_ context.Context, key string) (string, error) {
	if key != SettingKeyPromptLibraryTranslationConfig || r.value == "" {
		return "", ErrSettingNotFound
	}
	return r.value, nil
}

func (r *promptLibraryTranslationSettingRepoStub) Set(_ context.Context, key, value string) error {
	if key == SettingKeyPromptLibraryTranslationConfig {
		r.value = value
	}
	return nil
}

type promptLibraryTranslationGroupRepoStub struct {
	GroupRepository
	group *Group
}

func (r *promptLibraryTranslationGroupRepoStub) GetByID(_ context.Context, id int64) (*Group, error) {
	if r.group == nil || r.group.ID != id {
		return nil, ErrGroupNotFound
	}
	return r.group, nil
}

type promptLibraryTranslationGatewayStub struct {
	response []byte
	err      error
	calls    int
}

func (g *promptLibraryTranslationGatewayStub) ForwardContentModerationClassifier(
	_ context.Context,
	_ ContentModerationClassifierGatewayInput,
) (*ContentModerationClassifierGatewayResponse, error) {
	g.calls++
	if g.err != nil {
		return nil, g.err
	}
	return &ContentModerationClassifierGatewayResponse{StatusCode: 200, Body: g.response}, nil
}

func TestPromptLibraryTranslationService_LocalizeTranslatesAndClassifiesOnce(t *testing.T) {
	repo := &promptLibraryTranslationRepoStub{items: make(map[string]PromptLibraryTranslation)}
	settings := &promptLibraryTranslationSettingRepoStub{value: `{"enabled":true,"group_id":9,"model":"gpt-test","target_locale":"zh-CN"}`}
	groups := &promptLibraryTranslationGroupRepoStub{group: &Group{
		ID:       9,
		Platform: PlatformOpenAI,
		Status:   StatusActive,
		Scope:    GroupScopePublic,
	}}
	gateway := &promptLibraryTranslationGatewayStub{response: []byte(`{
		"choices":[{"message":{"content":"{\"translations\":[{\"id\":\"text-1\",\"title\":\"仓库修复工作流\",\"description\":\"检查并修复代码仓库\",\"category\":\"workflow\"},{\"id\":\"image-1\",\"title\":\"电影感肖像\",\"description\":\"生成肖像图像\",\"category\":\"coding\"}]}"}}]
	}`)}
	service := &PromptLibraryTranslationService{
		repo:        repo,
		settingRepo: settings,
		groupRepo:   groups,
		gateway:     gateway,
	}
	raw := json.RawMessage(`{
		"prompts":[
			{"id":"text-1","title":"Repository repair","description":"Fix a codebase","content":"Inspect files and repair tests","type":"TEXT","category":{"slug":"programming"}},
			{"id":"image-1","title":"Cinematic portrait","description":"Generate a portrait","content":"A cinematic portrait","type":"IMAGE","category":{"slug":"image-generation"}}
		],
		"total":2
	}`)

	localized, err := service.Localize(context.Background(), raw, "zh-CN")
	require.NoError(t, err)
	require.Equal(t, 1, gateway.calls)
	require.Equal(t, 1, repo.upsertCalls)

	var zh struct {
		Prompts []struct {
			ID           string `json:"id"`
			Title        string `json:"title"`
			IKIKCategory string `json:"ikikCategory"`
		} `json:"prompts"`
	}
	require.NoError(t, json.Unmarshal(localized, &zh))
	require.Equal(t, "仓库修复工作流", zh.Prompts[0].Title)
	require.Equal(t, "workflow", zh.Prompts[0].IKIKCategory)
	require.Equal(t, "image", zh.Prompts[1].IKIKCategory)

	english, err := service.Localize(context.Background(), raw, "en")
	require.NoError(t, err)
	require.Equal(t, 1, gateway.calls, "cached classification must not call the model again")
	var en struct {
		Prompts []struct {
			Title        string `json:"title"`
			IKIKCategory string `json:"ikikCategory"`
		} `json:"prompts"`
	}
	require.NoError(t, json.Unmarshal(english, &en))
	require.Equal(t, "Repository repair", en.Prompts[0].Title)
	require.Equal(t, "workflow", en.Prompts[0].IKIKCategory)
}

func TestPromptLibraryTranslationService_LocalizeFallsBackToSourceMetadata(t *testing.T) {
	service := &PromptLibraryTranslationService{
		repo: &promptLibraryTranslationRepoStub{items: make(map[string]PromptLibraryTranslation)},
		settingRepo: &promptLibraryTranslationSettingRepoStub{
			value: `{"enabled":true,"group_id":9,"model":"gpt-test","target_locale":"zh-CN"}`,
		},
		groupRepo: &promptLibraryTranslationGroupRepoStub{group: &Group{
			ID:       9,
			Platform: PlatformOpenAI,
			Status:   StatusActive,
			Scope:    GroupScopePublic,
		}},
		gateway: &promptLibraryTranslationGatewayStub{err: errors.New("model unavailable")},
	}
	raw := json.RawMessage(`{"prompts":[
		{"id":"text-1","title":"Debug tests","description":"Repair tests","content":"Fix unit tests","type":"TEXT","category":{"slug":"software-development"}},
		{"id":"video-1","title":"Camera move","description":"Video prompt","content":"Move the camera","type":"VIDEO","category":{"slug":"creative"}}
	]}`)

	localized, err := service.Localize(context.Background(), raw, "zh-CN")
	require.NoError(t, err)
	var page struct {
		Prompts []struct {
			IKIKCategory string `json:"ikikCategory"`
		} `json:"prompts"`
	}
	require.NoError(t, json.Unmarshal(localized, &page))
	require.Equal(t, "coding", page.Prompts[0].IKIKCategory)
	require.Equal(t, "video", page.Prompts[1].IKIKCategory)
}
