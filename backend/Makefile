.PHONY: build generate test test-unit test-integration test-e2e test-invariants new-module

VERSION ?= $(shell tr -d '\r\n' < ./cmd/server/VERSION)
LDFLAGS ?= -s -w -X main.Version=$(VERSION)

build:
	CGO_ENABLED=0 go build -ldflags="$(LDFLAGS)" -trimpath -o bin/server ./cmd/server

generate:
	go generate ./ent
	go generate ./cmd/server

test:
	go test ./...
	golangci-lint run ./...

test-unit:
	go test -tags=unit ./...

test-integration:
	go test -tags=integration ./...

# 插件化改造回归安全网（特征化/不变量测试，详见 .claude/plugin-refactor/INVARIANTS.md）
test-invariants:
	go test -tags=unit -count=1 -run 'Characterization|Invariant' ./internal/...

# 生成插件模块骨架：make new-module ID=job.foo
# （生成后按提示在 internal/modules/standard/imports.go 手工加 import 行）
new-module:
ifndef ID
	$(error ID is required: make new-module ID=job.foo)
endif
	go run ./tools/newmodule -id "$(ID)"

# scripts/e2e-test.sh 已不存在，test-e2e 与 test-e2e-local 等价（env 驱动的真实服务 e2e）
test-e2e:
	go test -tags=e2e -v -timeout=300s ./internal/integration/...

test-e2e-local:
	go test -tags=e2e -v -timeout=300s ./internal/integration/...
