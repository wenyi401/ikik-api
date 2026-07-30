//go:build unit

package service

import (
	"context"
	"testing"

	"github.com/stretchr/testify/require"
	"ikik-api/internal/config"
)

type privateGroupCreationRepo struct {
	*mockGroupRepoForGateway
	created *Group
}

func (r *privateGroupCreationRepo) Create(_ context.Context, group *Group) error {
	r.created = group
	return nil
}

type privateGroupSettingRepo struct {
	*settingRepoStub
}

func (r *privateGroupSettingRepo) GetAll(context.Context) (map[string]string, error) {
	return r.values, nil
}

func TestDefaultPrivateGroupAllowMessagesDispatch(t *testing.T) {
	require.True(t, defaultPrivateGroupAllowMessagesDispatch(PlatformOpenAI))
	require.True(t, defaultPrivateGroupAllowMessagesDispatch(" OpenAI "))

	require.False(t, defaultPrivateGroupAllowMessagesDispatch(PlatformAnthropic))
	require.False(t, defaultPrivateGroupAllowMessagesDispatch(PlatformGemini))
	require.False(t, defaultPrivateGroupAllowMessagesDispatch(PlatformAntigravity))
}

func TestDefaultPrivateGroupAllowImageGeneration(t *testing.T) {
	require.True(t, defaultPrivateGroupAllowImageGeneration(PlatformOpenAI))
	require.True(t, defaultPrivateGroupAllowImageGeneration(" OpenAI "))
	require.True(t, defaultPrivateGroupAllowImageGeneration(PlatformGrok))
	require.True(t, defaultPrivateGroupAllowImageGeneration(" GROK "))

	require.False(t, defaultPrivateGroupAllowImageGeneration(PlatformAnthropic))
	require.False(t, defaultPrivateGroupAllowImageGeneration(PlatformGemini))
	require.False(t, defaultPrivateGroupAllowImageGeneration(PlatformAntigravity))
	require.False(t, defaultPrivateGroupAllowImageGeneration(PlatformKiro))
	require.False(t, defaultPrivateGroupAllowImageGeneration(PlatformCustom))
}

func TestFindOrCreateUserPrivateGroupDefaultsImageGenerationByPlatform(t *testing.T) {
	tests := []struct {
		platform string
		want     bool
	}{
		{platform: PlatformOpenAI, want: true},
		{platform: PlatformGrok, want: true},
		{platform: PlatformAnthropic, want: false},
		{platform: PlatformGemini, want: false},
		{platform: PlatformAntigravity, want: false},
		{platform: PlatformKiro, want: false},
		{platform: PlatformCustom, want: false},
	}

	for _, tt := range tests {
		t.Run(tt.platform, func(t *testing.T) {
			repo := &privateGroupCreationRepo{mockGroupRepoForGateway: &mockGroupRepoForGateway{}}
			service := &userPrivateGroupService{
				groupRepo:      repo,
				settingService: NewSettingService(&privateGroupSettingRepo{settingRepoStub: &settingRepoStub{values: map[string]string{}}}, &config.Config{}),
			}

			group, err := service.findOrCreateUserPrivateGroup(context.Background(), 42, tt.platform)

			require.NoError(t, err)
			require.Same(t, group, repo.created)
			require.Equal(t, tt.want, group.AllowImageGeneration)
			require.Equal(t, GroupScopeUserPrivate, group.Scope)
		})
	}
}
