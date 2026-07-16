// Command newmodule 生成插件模块骨架（纯开发工具，不进生产二进制）。
//
// 用法：
//
//	go run ./tools/newmodule -id job.foo
//	make new-module ID=job.foo
//
// 在 internal/modules/<模块名>/ 下生成模块文件与基于 plugintest 的测试文件
// （模板从 internal/modules/hello 提炼：四个可选生命周期接口骨架、编译期
// 断言、EnabledByDefault=false、示例私有配置与校验），并打印后续插装步骤。
//
// 生成器不会自动修改 internal/modules/standard/imports.go——
// 显式插装可 review 是设计原则，import 行由作者手工加入。
package main

import (
	"bytes"
	"embed"
	"errors"
	"flag"
	"fmt"
	"go/format"
	"go/token"
	"io"
	"os"
	"path/filepath"
	"text/template"

	"ikik-api/internal/plugin"
)

// modulePath 是本仓库的 Go module 路径，用于生成插装 import 行。
const modulePath = "ikik-api"

// authorGuidePath 是模块作者指南的仓库相对路径（next steps 提示用）。
const authorGuidePath = "docs/plugin-architecture/MODULE-AUTHOR-GUIDE.md"

//go:embed templates/module.go.tmpl templates/module_test.go.tmpl
var templatesFS embed.FS

// templateData 是模板渲染数据。
type templateData struct {
	// ID 是完整模块 ID，如 "job.foo"。
	ID string
	// Package 是模块包名（ID 最后一段），如 "foo"。
	Package string
}

func main() {
	id := flag.String("id", "", "模块 ID（点分层级命名，如 job.foo）")
	dir := flag.String("dir", ".", "backend 根目录（须包含 internal/modules）")
	flag.Parse()

	if err := run(*id, *dir, os.Stdout); err != nil {
		fmt.Fprintf(os.Stderr, "newmodule: %v\n", err)
		os.Exit(1)
	}
}

// run 执行完整的生成流程：校验 ID 与包名 → 渲染模板 → 写盘 → 打印后续步骤。
// root 是 backend 根目录（包含 internal/modules），out 接收 next steps 输出。
func run(id, root string, out io.Writer) error {
	if id == "" {
		return errors.New(`missing module ID: usage "go run ./tools/newmodule -id job.foo" or "make new-module ID=job.foo"`)
	}
	if err := validateModuleID(id); err != nil {
		return err
	}
	pkg := plugin.ModuleID(id).Name()
	if err := validatePackageName(pkg); err != nil {
		return err
	}

	modulesDir := filepath.Join(root, "internal", "modules")
	if info, err := os.Stat(modulesDir); err != nil || !info.IsDir() {
		return fmt.Errorf("%s not found: run from the backend root (or pass -dir)", modulesDir)
	}
	targetDir := filepath.Join(modulesDir, pkg)
	if _, err := os.Stat(targetDir); err == nil {
		return fmt.Errorf("target directory %s already exists; refusing to overwrite", targetDir)
	} else if !errors.Is(err, os.ErrNotExist) {
		return fmt.Errorf("stat %s: %w", targetDir, err)
	}

	data := templateData{ID: id, Package: pkg}
	moduleSrc, err := render("module.go.tmpl", data)
	if err != nil {
		return err
	}
	testSrc, err := render("module_test.go.tmpl", data)
	if err != nil {
		return err
	}

	if err := os.Mkdir(targetDir, 0o755); err != nil {
		return fmt.Errorf("create %s: %w", targetDir, err)
	}
	files := []struct {
		name    string
		content []byte
	}{
		{pkg + ".go", moduleSrc},
		{pkg + "_test.go", testSrc},
	}
	for _, f := range files {
		path := filepath.Join(targetDir, f.name)
		if err := os.WriteFile(path, f.content, 0o644); err != nil {
			// 写盘失败时回滚刚创建的目标目录，避免留下半成品。
			if rmErr := os.RemoveAll(targetDir); rmErr != nil {
				return errors.Join(fmt.Errorf("write %s: %w", path, err), rmErr)
			}
			return fmt.Errorf("write %s: %w", path, err)
		}
	}

	printNextSteps(out, id, pkg)
	return nil
}

// validateModuleID 通过内核导出口径 plugin.ParseConfig 复用 ModuleID.validate()
// 的格式规则（见 internal/plugin/module.go）：非空、点分、各段为非空的
// [a-z0-9_-]+。不在生成器中复制规则，内核规则演进时自动跟随。
func validateModuleID(id string) error {
	if _, err := plugin.ParseConfig(map[string]map[string]any{id: {}}); err != nil {
		return fmt.Errorf("invalid module ID %q (kernel rule, see internal/plugin/module.go ModuleID.validate): %w", id, err)
	}
	return nil
}

// validatePackageName 校验模块名（ID 最后一段）可用作 Go 包名。
// 内核 ID 规则允许 [a-z0-9_-]，但包含 '-'、数字开头、Go 关键字与 "_"
// 都不是合法包名，必须在生成前拒绝。
func validatePackageName(name string) error {
	if name == "_" || !token.IsIdentifier(name) {
		return fmt.Errorf("module name %q (the last ID segment, used as the Go package name) is not a valid package name: it must be a non-keyword Go identifier (no '-', no leading digit, not a Go keyword, not %q)", name, "_")
	}
	return nil
}

// render 渲染指定模板并经 go/format 规范化，保证产物 gofmt 干净；
// 渲染结果无法通过 gofmt（模板自身腐化）时立即报错，不写盘。
func render(name string, data templateData) ([]byte, error) {
	tmpl, err := template.ParseFS(templatesFS, "templates/"+name)
	if err != nil {
		return nil, fmt.Errorf("parse template %s: %w", name, err)
	}
	var buf bytes.Buffer
	if err := tmpl.Execute(&buf, data); err != nil {
		return nil, fmt.Errorf("execute template %s: %w", name, err)
	}
	src, err := format.Source(buf.Bytes())
	if err != nil {
		return nil, fmt.Errorf("gofmt rendered %s (template is corrupted, fix tools/newmodule/templates): %w", name, err)
	}
	return src, nil
}

// printNextSteps 打印生成结果与后续插装步骤（生成器不自动改 imports.go）。
func printNextSteps(out io.Writer, id, pkg string) {
	fmt.Fprintf(out, `module %[1]s generated:
  internal/modules/%[2]s/%[2]s.go
  internal/modules/%[2]s/%[2]s_test.go

Next steps (the generator never edits imports.go -- explicit instrumentation keeps it reviewable):

  1. 编译期插装：在 internal/modules/standard/imports.go 的 import 块中加入：

	// %[1]s：TODO 一句话描述模块用途（默认 disabled）。
	_ "%[3]s/internal/modules/%[2]s"

  2. 配置启用（config.yaml；新模块默认 disabled，未显式 enabled 时零行为变更）：

	modules:
	  %[1]s:
	    enabled: true
	    interval: 30s

  3. 填充代码中的 TODO 后运行测试：

	go test -count=1 ./internal/modules/%[2]s/

生命周期契约与完整示例见 %[4]s（蓝本实现：internal/modules/hello/）。
`, id, pkg, modulePath, authorGuidePath)
}
