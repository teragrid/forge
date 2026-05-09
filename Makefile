# Forge — Makefile (DEV-M0-01)
#
# Single entry point for contributors and CI. Per ADR-001 the toolchain is Go
# (CGO_ENABLED=0 default). Per DEV-M0-01 acceptance, `make build` must succeed
# on a fresh clone with no extra setup beyond Go + the tools listed in
# `make tools`.

GO            ?= go
GOFLAGS       ?=
PKG           ?= ./...
BIN_DIR       ?= dist
BIN_NAME      ?= forge
VERSION       ?= $(shell git describe --tags --always --dirty 2>/dev/null || echo 0.0.0-dev)
LDFLAGS       ?= -s -w -X main.Version=$(VERSION)
CGO_ENABLED   ?= 0

export CGO_ENABLED

.PHONY: all
all: lint test build

.PHONY: build
build:
	$(GO) build $(GOFLAGS) -ldflags '$(LDFLAGS)' -o $(BIN_DIR)/$(BIN_NAME) ./cmd/forge

.PHONY: test
test:
	$(GO) test -race -count=1 $(PKG)

.PHONY: test-short
test-short:
	$(GO) test -race -short -count=1 $(PKG)

.PHONY: cover
cover:
	$(GO) test -race -coverprofile=coverage.out -covermode=atomic $(PKG)
	$(GO) tool cover -func=coverage.out | tail -n 1

.PHONY: lint
lint:
	golangci-lint run ./...

.PHONY: fmt
fmt:
	gofmt -s -w .
	goimports -w .

.PHONY: fmt-check
fmt-check:
	@if [ -n "$$(gofmt -s -l .)" ]; then \
		echo 'gofmt -s violations:'; gofmt -s -l .; exit 1; \
	fi
	@if [ -n "$$(goimports -l .)" ]; then \
		echo 'goimports violations:'; goimports -l .; exit 1; \
	fi

.PHONY: vuln
vuln:
	govulncheck ./...

.PHONY: tidy
tidy:
	$(GO) mod tidy
	$(GO) mod verify

# Cross-compile matrix (DEV-M0-01 TC-01-06): 6 binaries, CGO_ENABLED=0.
.PHONY: cross
cross:
	@set -e; \
	for target in linux/amd64 linux/arm64 darwin/amd64 darwin/arm64 windows/amd64 windows/arm64; do \
		os=$${target%/*}; arch=$${target#*/}; \
		ext=""; if [ "$$os" = "windows" ]; then ext=".exe"; fi; \
		out=$(BIN_DIR)/$(BIN_NAME)-$$os-$$arch$$ext; \
		echo "==> $$out"; \
		GOOS=$$os GOARCH=$$arch CGO_ENABLED=0 \
		  $(GO) build $(GOFLAGS) -ldflags '$(LDFLAGS)' -o $$out ./cmd/forge; \
	done

.PHONY: tools
tools:
	$(GO) install github.com/golangci/golangci-lint/cmd/golangci-lint@latest
	$(GO) install golang.org/x/vuln/cmd/govulncheck@latest
	$(GO) install golang.org/x/tools/cmd/goimports@latest
	$(GO) install gotest.tools/gotestsum@latest

# Register the repo's own git hooks (runs once per clone after `make tools`).
.PHONY: hooks
hooks:
	git config core.hooksPath .githooks
	@echo "pre-push hook active — checks: fmt, vet, lint, build, test, vuln, mod verify"

# Run every quality gate locally in the same order as the pre-push hook.
.PHONY: check
check: fmt-check
	$(GO) vet $(PKG)
	golangci-lint run ./...
	CGO_ENABLED=0 $(GO) build $(PKG)
	$(GO) test -count=1 $(PKG)
	govulncheck ./...
	$(GO) mod verify

.PHONY: docs
docs:
	$(GO) run ./cmd/gen-errors --out docs/ERROR_CODES.md

.PHONY: docs-check
docs-check:
	$(GO) run ./cmd/gen-errors --out docs/ERROR_CODES.md --check

.PHONY: bench
bench:
	$(GO) test -bench=. -benchmem -run='^$$' ./...

.PHONY: clean
clean:
	rm -rf $(BIN_DIR) coverage.out
