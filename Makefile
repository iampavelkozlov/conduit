OPENAPI_SPEC := ./api/open-api.yml
OAPI_CODEGEN_VERSION := v2.8.0
OAPI_CODEGEN := $(shell go env GOPATH)/bin/oapi-codegen
GOWRAP_VERSION := v1.4.3
GOLANGCI_LINT_VERSION := v2.13.2
GOLANGCI_LINT_VERSION_NUMBER := $(patsubst v%,%,$(GOLANGCI_LINT_VERSION))
GOLANGCI_LINT := $(shell go env GOPATH)/bin/golangci-lint
GO_ARCH_LINT_VERSION := v1.18.0
GO_ARCH_LINT := $(shell go env GOPATH)/bin/go-arch-lint
GO_TEST_COVERAGE_VERSION := v2.19.0
GO_TEST_COVERAGE := $(shell go env GOPATH)/bin/go-test-coverage
GOOSE_VERSION := v3.28.0
GOOSE := $(shell go env GOPATH)/bin/goose
SQLC_VERSION := v1.30.0
SQLC := $(shell go env GOPATH)/bin/sqlc
SQLC_CONFIGS := sqlc.yaml $(shell find services -name sqlc.yaml -type f | sort)
BUF_VERSION := v1.72.0
BUF := $(shell go env GOPATH)/bin/buf
PROTOC_GEN_GO_VERSION := v1.36.11
PROTOC_GEN_GO := $(shell go env GOPATH)/bin/protoc-gen-go
PROTOC_GEN_GO_GRPC_VERSION := v1.6.2
PROTOC_GEN_GO_GRPC := $(shell go env GOPATH)/bin/protoc-gen-go-grpc
BUF_BREAKING_AGAINST ?= .git\#branch=main,subdir=proto
MIGRATIONS_DIR := migrations
DB_DSN ?= postgres://postgres:postgres@localhost:5432/conduit?sslmode=disable
COVERAGE_PROFILE ?= coverage.out
COVERAGE_BADGE ?= docs/coverage.svg

.PHONY: oapi-codegen gen-http go-wrap goose-install migrate sqlc-install sqlc grpc-tools grpc-lint grpc-breaking gen-grpc wire mocks wrap generate golangci-lint-install lint go-arch-lint-install arch-lint arch-graph go-test-coverage-install coverage badges quality

-include .env
export

oapi-codegen:
	go install github.com/oapi-codegen/oapi-codegen/v2/cmd/oapi-codegen@$(OAPI_CODEGEN_VERSION)

gen-http: oapi-codegen
	$(OAPI_CODEGEN) --config=config.yaml $(OPENAPI_SPEC)

go-wrap:
	go install github.com/hexdigest/gowrap/cmd/gowrap@$(GOWRAP_VERSION)

wrap: go-wrap
	go generate ./internal/metrics

goose-install:
	go install github.com/pressly/goose/v3/cmd/goose@$(GOOSE_VERSION)

migrate: goose-install
	$(GOOSE) -dir $(MIGRATIONS_DIR) postgres "$(DB_DSN)" up

sqlc-install:
	go install github.com/sqlc-dev/sqlc/cmd/sqlc@$(SQLC_VERSION)

sqlc: sqlc-install
	@for config in $(SQLC_CONFIGS); do \
		echo "Generating sqlc sources from $$config"; \
		$(SQLC) generate -f "$$config" || exit 1; \
	done

grpc-tools:
	go install github.com/bufbuild/buf/cmd/buf@$(BUF_VERSION)
	go install google.golang.org/protobuf/cmd/protoc-gen-go@$(PROTOC_GEN_GO_VERSION)
	go install google.golang.org/grpc/cmd/protoc-gen-go-grpc@$(PROTOC_GEN_GO_GRPC_VERSION)

grpc-lint: grpc-tools
	$(BUF) lint

grpc-breaking: grpc-tools
	@if git ls-tree -r --name-only main -- proto | grep -q '\.proto$$'; then \
		$(BUF) breaking --against '$(BUF_BREAKING_AGAINST)'; \
	else \
		echo "No protobuf baseline on main yet; breaking check skipped"; \
	fi

gen-grpc: grpc-lint
	PATH="$(dir $(PROTOC_GEN_GO)):$(dir $(PROTOC_GEN_GO_GRPC)):$$PATH" $(BUF) generate

wire:
	go tool wire ./cmd/server

mocks:
	go generate ./internal/service/... ./internal/transport/http ./internal/transport/middleware

generate: gen-http gen-grpc sqlc mocks wrap wire

golangci-lint-install:
	@if ! $(GOLANGCI_LINT) version 2>/dev/null | grep -q "$(GOLANGCI_LINT_VERSION_NUMBER)"; then \
		curl -sSfL https://golangci-lint.run/install.sh | sh -s -- -b "$(shell go env GOPATH)/bin" $(GOLANGCI_LINT_VERSION); \
	fi

lint: golangci-lint-install
	$(GOLANGCI_LINT) config verify
	$(GOLANGCI_LINT) run ./...

go-arch-lint-install:
	@if ! $(GO_ARCH_LINT) --output-color=false version 2>/dev/null | grep -q "$(GO_ARCH_LINT_VERSION)"; then \
		go install github.com/fe3dback/go-arch-lint@$(GO_ARCH_LINT_VERSION); \
	fi

arch-lint: go-arch-lint-install
	$(GO_ARCH_LINT) check --project-path .

arch-graph: go-arch-lint-install
	mkdir -p docs
	$(GO_ARCH_LINT) graph --project-path . --type di --out docs/architecture.svg
	chmod 0644 docs/architecture.svg

go-test-coverage-install:
	@if ! $(GO_TEST_COVERAGE) --version 2>/dev/null | grep -q "$(GO_TEST_COVERAGE_VERSION)"; then \
		go install github.com/vladopajic/go-test-coverage/v2@$(GO_TEST_COVERAGE_VERSION); \
	fi

coverage: go-test-coverage-install
	go test ./... -covermode=atomic -coverprofile=$(COVERAGE_PROFILE)
	$(GO_TEST_COVERAGE) --config .testcoverage.yml --profile $(COVERAGE_PROFILE) --badge-file-name $(COVERAGE_BADGE)

badges:
	./scripts/generate-badges.sh

quality: lint arch-lint coverage
