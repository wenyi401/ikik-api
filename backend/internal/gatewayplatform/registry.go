package gatewayplatform

import "fmt"

// Registry 是 platform → Provider 的只读查找表：构造期一次性注册，
// 构造完成后并发只读（无锁）。
type Registry struct {
	providers map[string]Provider
}

// NewRegistry 构造注册表。providers 中出现 nil Provider、空 Platform() 或
// 重复平台时 panic——装配错误必须在启动期（Wire 装配）暴露，而非请求期。
func NewRegistry(providers ...Provider) *Registry {
	m := make(map[string]Provider, len(providers))
	for _, p := range providers {
		if p == nil {
			panic("gatewayplatform: nil provider")
		}
		platform := p.Platform()
		if platform == "" {
			panic("gatewayplatform: provider with empty platform")
		}
		if _, dup := m[platform]; dup {
			panic(fmt.Sprintf("gatewayplatform: duplicate provider for platform %q", platform))
		}
		m[platform] = p
	}
	return &Registry{providers: m}
}

// Get 返回平台对应的 Provider；未注册（或 nil Registry）返回 nil。
// 调用点须保证所查平台已在装配期注册（Wire 注册全集，miss 属编程错误）。
func (r *Registry) Get(platform string) Provider {
	if r == nil {
		return nil
	}
	return r.providers[platform]
}
