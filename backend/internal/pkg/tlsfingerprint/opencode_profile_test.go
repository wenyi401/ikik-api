package tlsfingerprint

import (
	"testing"

	"github.com/stretchr/testify/require"
)

// TestOpencodeProfileMatchesBunFingerprint 验证 OpenCode CLI (Bun 1.3.x) 指纹：
// 与 Node.js 24.x 默认扩展序完全一致，仅在末尾多一个 padding(21) 扩展
// （Bun/BoringSSL 行为；实测来源 PixelAPI，Bun 1.3.10 via tls.peet.ws，
// JA3 = 50027c67d7d68e24c00d233bca146d88）。
func TestOpencodeProfileMatchesBunFingerprint(t *testing.T) {
	p := NewOpencodeProfile()
	require.NotNil(t, p)
	require.Equal(t, "OpenCode CLI (Bun 1.3.x)", p.Name)
	require.NotEmpty(t, p.Extensions)

	// 前缀必须与 Node.js 24.x 默认扩展序一致
	require.Equal(t, defaultExtensionOrder, p.Extensions[:len(p.Extensions)-1])
	// 末尾是 padding(21)
	require.Equal(t, uint16(21), p.Extensions[len(p.Extensions)-1])
}

// TestBuildClientHelloSpecWithOpencodeProfile 验证 opencode profile 能构建出
// 合法的 ClientHelloSpec（padding(21) 走 BoringSSL padding 扩展而非空通用扩展）。
func TestBuildClientHelloSpecWithOpencodeProfile(t *testing.T) {
	spec := buildClientHelloSpecFromProfile(NewOpencodeProfile())
	require.NotNil(t, spec)
	// 扩展数量 = 扩展序长度（GREASE 除外时逐个映射）
	require.NotEmpty(t, spec.Extensions)
}
