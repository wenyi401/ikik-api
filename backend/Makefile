.PHONY: build generate test test-unit test-integration test-invariants test-e2e

VERSION ?= $(shell ./scripts/resolve-version.sh)
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

# Plugin-refactor regression safety net. Keep this as a focused, independently
# named gate so CI can report characterization/invariant regressions separately
# from the complete unit-test job.
test-invariants:
	go test -tags=unit -count=1 -run 'Characterization|Invariant' ./internal/...

# The old scripts/e2e-test.sh entrypoint was removed; run the maintained Go
# e2e suite directly instead.
test-e2e:
	go test -tags=e2e -v -timeout=300s ./internal/integration/...

test-e2e-local:
	go test -tags=e2e -v -timeout=300s ./internal/integration/...
