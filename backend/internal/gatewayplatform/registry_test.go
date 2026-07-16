package gatewayplatform

import (
	"context"
	"sync"
	"testing"

	"ikik-api/internal/service"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
)

// stubProvider 是注册表测试用的最小 Provider 实现。
type stubProvider struct {
	platform string
}

func (s *stubProvider) Platform() string { return s.platform }

func (s *stubProvider) Forward(_ context.Context, _ *gin.Context, _ *service.Account, _ *ForwardRequest) (*service.ForwardResult, error) {
	return nil, nil
}

func TestRegistry_GetReturnsRegisteredProvider(t *testing.T) {
	anthropic := &stubProvider{platform: service.PlatformAnthropic}
	antigravity := &stubProvider{platform: service.PlatformAntigravity}
	gemini := &stubProvider{platform: service.PlatformGemini}

	registry := NewRegistry(anthropic, antigravity, gemini)

	require.Same(t, Provider(anthropic), registry.Get(service.PlatformAnthropic))
	require.Same(t, Provider(antigravity), registry.Get(service.PlatformAntigravity))
	require.Same(t, Provider(gemini), registry.Get(service.PlatformGemini))
}

func TestRegistry_GetUnregisteredPlatformReturnsNil(t *testing.T) {
	registry := NewRegistry(&stubProvider{platform: service.PlatformAnthropic})

	require.Nil(t, registry.Get(service.PlatformOpenAI), "未注册平台必须返回 nil")
	require.Nil(t, registry.Get(""), "空平台必须返回 nil")

	var nilRegistry *Registry
	require.Nil(t, nilRegistry.Get(service.PlatformAnthropic), "nil Registry 必须返回 nil 而非 panic")
}

func TestNewRegistry_DuplicatePlatformPanics(t *testing.T) {
	require.PanicsWithValue(t,
		`gatewayplatform: duplicate provider for platform "anthropic"`,
		func() {
			NewRegistry(
				&stubProvider{platform: service.PlatformAnthropic},
				&stubProvider{platform: service.PlatformAnthropic},
			)
		})
}

func TestNewRegistry_NilProviderPanics(t *testing.T) {
	require.PanicsWithValue(t, "gatewayplatform: nil provider", func() {
		NewRegistry(&stubProvider{platform: service.PlatformAnthropic}, nil)
	})
}

func TestNewRegistry_EmptyPlatformPanics(t *testing.T) {
	require.PanicsWithValue(t, "gatewayplatform: provider with empty platform", func() {
		NewRegistry(&stubProvider{platform: ""})
	})
}

// TestRegistry_ConcurrentGet 验证构造完成后的注册表可被并发只读访问
// （配合 -race 检测数据竞争）。
func TestRegistry_ConcurrentGet(t *testing.T) {
	registry := NewRegistry(
		&stubProvider{platform: service.PlatformAnthropic},
		&stubProvider{platform: service.PlatformAntigravity},
		&stubProvider{platform: service.PlatformGemini},
	)

	platforms := []string{
		service.PlatformAnthropic,
		service.PlatformAntigravity,
		service.PlatformGemini,
		service.PlatformOpenAI, // miss 路径同样并发安全
	}

	var wg sync.WaitGroup
	for i := 0; i < 32; i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			for j := 0; j < 200; j++ {
				platform := platforms[(i+j)%len(platforms)]
				got := registry.Get(platform)
				if platform == service.PlatformOpenAI {
					require.Nil(t, got)
				} else {
					require.NotNil(t, got)
					require.Equal(t, platform, got.Platform())
				}
			}
		}(i)
	}
	wg.Wait()
}

// TestAdapters_PlatformIdentity 锁定三个 adapter 的注册键 = service.Platform* 常量
// （Registry 查找键与 :444/:794 调用点的查找表达式一致）。
func TestAdapters_PlatformIdentity(t *testing.T) {
	require.Equal(t, service.PlatformAnthropic, NewAnthropicProvider(nil).Platform())
	require.Equal(t, service.PlatformAntigravity, NewAntigravityProvider(nil).Platform())
	require.Equal(t, service.PlatformGemini, NewGeminiProvider(nil).Platform())
}
