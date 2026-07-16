package main

import (
	"bytes"
	"go/format"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"
)

// newRoot 构造一个带 internal/modules 结构的临时 backend 根目录。
func newRoot(t *testing.T) string {
	t.Helper()
	root := t.TempDir()
	require.NoError(t, os.MkdirAll(filepath.Join(root, "internal", "modules"), 0o755))
	return root
}

// TestRunGeneratesGofmtCleanModule 验证生成产物：文件路径正确、gofmt 干净
// （format.Source 幂等）、关键骨架内容齐全，且 next steps 含确切的插装 import 行。
func TestRunGeneratesGofmtCleanModule(t *testing.T) {
	root := newRoot(t)
	var out bytes.Buffer
	require.NoError(t, run("job.foo", root, &out))

	moduleFile := filepath.Join(root, "internal", "modules", "foo", "foo.go")
	testFile := filepath.Join(root, "internal", "modules", "foo", "foo_test.go")

	for _, path := range []string{moduleFile, testFile} {
		content, err := os.ReadFile(path)
		require.NoError(t, err, "generated file should exist: %s", path)
		formatted, err := format.Source(content)
		require.NoError(t, err, "generated file must be valid Go: %s", path)
		require.Equal(t, string(formatted), string(content), "generated file must be gofmt-clean: %s", path)
		require.Contains(t, string(content), "package foo")
	}

	moduleSrc, err := os.ReadFile(moduleFile)
	require.NoError(t, err)
	require.Contains(t, string(moduleSrc), `const ID plugin.ModuleID = "job.foo"`)
	require.Contains(t, string(moduleSrc), "EnabledByDefault: false")
	require.Contains(t, string(moduleSrc), "plugin.RegisterModule(&Module{})")
	// 四个可选生命周期接口的编译期断言齐全。
	for _, iface := range []string{"plugin.Provisioner", "plugin.Validator", "plugin.Starter", "plugin.Stopper"} {
		require.Contains(t, string(moduleSrc), iface)
	}
	require.Contains(t, string(moduleSrc), "TODO", "module template should mark author fill-in points")

	testSrc, err := os.ReadFile(testFile)
	require.NoError(t, err)
	require.Contains(t, string(testSrc), "internal/plugin/plugintest", "test template must use plugintest fixtures")
	require.Contains(t, string(testSrc), "plugintest.RunLifecycle")

	// next steps：确切 import 行、不自动插装的提示、配置示例与作者指南链接。
	nextSteps := out.String()
	require.Contains(t, nextSteps, `_ "ikik-api/internal/modules/foo"`)
	require.Contains(t, nextSteps, "internal/modules/standard/imports.go")
	require.Contains(t, nextSteps, "enabled: true")
	require.Contains(t, nextSteps, "docs/plugin-architecture/MODULE-AUTHOR-GUIDE.md")
}

// TestRunRejectsInvalidModuleID 验证不符合内核规则的 ID 被拒绝，
// 错误信息引用内核规则来源，且不留下任何产物目录。
func TestRunRejectsInvalidModuleID(t *testing.T) {
	tests := []struct {
		name string
		id   string
	}{
		{"empty", ""},
		{"uppercase", "Job.Foo"},
		{"empty segment", "job..foo"},
		{"illegal char", "job.foo!"},
		{"trailing dot", "job.foo."},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			root := newRoot(t)
			err := run(tt.id, root, &bytes.Buffer{})
			require.Error(t, err)
			if tt.id != "" {
				require.Contains(t, err.Error(), "internal/plugin/module.go",
					"error should reference the kernel rule source")
			}
			entries, readErr := os.ReadDir(filepath.Join(root, "internal", "modules"))
			require.NoError(t, readErr)
			require.Empty(t, entries, "rejected ID must not leave artifacts")
		})
	}
}

// TestRunRejectsInvalidPackageName 验证 ID 符合内核规则但最后一段
// 不是合法 Go 包名（关键字、含 '-'、数字开头、下划线）时被拒绝。
func TestRunRejectsInvalidPackageName(t *testing.T) {
	tests := []struct {
		name string
		id   string
	}{
		{"go keyword", "job.type"},
		{"contains dash", "job.foo-bar"},
		{"leading digit", "job.1foo"},
		{"blank identifier", "job._"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			root := newRoot(t)
			err := run(tt.id, root, &bytes.Buffer{})
			require.Error(t, err)
			require.Contains(t, err.Error(), "package name")
			entries, readErr := os.ReadDir(filepath.Join(root, "internal", "modules"))
			require.NoError(t, readErr)
			require.Empty(t, entries, "rejected package name must not leave artifacts")
		})
	}
}

// TestRunRejectsExistingTargetDir 验证目标目录已存在时拒绝覆盖，且原内容不被触碰。
func TestRunRejectsExistingTargetDir(t *testing.T) {
	root := newRoot(t)
	existing := filepath.Join(root, "internal", "modules", "foo")
	require.NoError(t, os.MkdirAll(existing, 0o755))
	sentinel := filepath.Join(existing, "keep.go")
	require.NoError(t, os.WriteFile(sentinel, []byte("package foo\n"), 0o644))

	err := run("job.foo", root, &bytes.Buffer{})
	require.Error(t, err)
	require.Contains(t, err.Error(), "already exists")

	content, readErr := os.ReadFile(sentinel)
	require.NoError(t, readErr)
	require.Equal(t, "package foo\n", string(content), "existing files must be untouched")
}

// TestRunRequiresModulesDir 验证 root 下没有 internal/modules 时给出明确提示。
func TestRunRequiresModulesDir(t *testing.T) {
	err := run("job.foo", t.TempDir(), &bytes.Buffer{})
	require.Error(t, err)
	require.Contains(t, err.Error(), "internal")
	require.Contains(t, err.Error(), "backend root")
}

// TestRunRequiresID 验证缺少 -id 参数时报用法错误。
func TestRunRequiresID(t *testing.T) {
	err := run("", newRoot(t), &bytes.Buffer{})
	require.Error(t, err)
	require.Contains(t, err.Error(), "make new-module ID=job.foo")
}
