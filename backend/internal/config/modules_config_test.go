package config

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"
)

// TestLoadModulesSubtreeDefaultEmpty 验证不配置 modules: 子树时，
// cfg.Modules 为空（非 nil）map，解析结果与现状完全等价。
func TestLoadModulesSubtreeDefaultEmpty(t *testing.T) {
	resetViperWithJWTSecret(t)

	cfg, err := Load()
	require.NoError(t, err)
	require.NotNil(t, cfg.Modules)
	require.Empty(t, cfg.Modules)
}

// TestLoadModulesSubtreeParsing 验证 modules: 子树解析：
// 含点的模块 ID 必须作为完整 key 保留（不被 viper 按 "." 拆分为多级嵌套）。
func TestLoadModulesSubtreeParsing(t *testing.T) {
	resetViperWithJWTSecret(t)

	tempDir := t.TempDir()
	configPath := filepath.Join(tempDir, "config.yaml")
	yaml := `
modules:
  job.hello:
    enabled: true
    greeting: "hi"
  payment.provider.stripe:
    enabled: false
  job.bare:
`
	require.NoError(t, os.WriteFile(configPath, []byte(yaml), 0o644))
	t.Setenv("DATA_DIR", tempDir)

	cfg, err := Load()
	require.NoError(t, err)
	require.Len(t, cfg.Modules, 3)

	require.Equal(t, map[string]any{"enabled": true, "greeting": "hi"}, cfg.Modules["job.hello"])
	require.Equal(t, map[string]any{"enabled": false}, cfg.Modules["payment.provider.stripe"])
	// 空模块项规整为空 map（提及但无私有配置）。
	require.NotNil(t, cfg.Modules["job.bare"])
	require.Empty(t, cfg.Modules["job.bare"])
}

// TestLoadModulesSubtreeInvalidShape 验证 modules 子树形状非法时报错。
func TestLoadModulesSubtreeInvalidShape(t *testing.T) {
	resetViperWithJWTSecret(t)

	tempDir := t.TempDir()
	configPath := filepath.Join(tempDir, "config.yaml")
	require.NoError(t, os.WriteFile(configPath, []byte("modules:\n  job.hello: true\n"), 0o644))
	t.Setenv("DATA_DIR", tempDir)

	_, err := Load()
	require.Error(t, err)
	require.Contains(t, err.Error(), "modules")
}

// TestNormalizeModulesSubtree 直接覆盖规整函数的边界分支。
func TestNormalizeModulesSubtree(t *testing.T) {
	t.Run("nil input", func(t *testing.T) {
		out, err := normalizeModulesSubtree(nil)
		require.NoError(t, err)
		require.NotNil(t, out)
		require.Empty(t, out)
	})
	t.Run("non-map top level", func(t *testing.T) {
		_, err := normalizeModulesSubtree([]any{"x"})
		require.Error(t, err)
	})
	t.Run("non-map entry", func(t *testing.T) {
		_, err := normalizeModulesSubtree(map[string]any{"job.hello": 42})
		require.Error(t, err)
		require.Contains(t, err.Error(), "job.hello")
	})
	t.Run("nil entry becomes empty map", func(t *testing.T) {
		out, err := normalizeModulesSubtree(map[string]any{"job.hello": nil})
		require.NoError(t, err)
		require.NotNil(t, out["job.hello"])
		require.Empty(t, out["job.hello"])
	})
}
