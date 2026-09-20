OPENAPI_SPEC := ./api/open-api.yml
.DEFAULT_GOAL := help

SHELL := /bin/bash

ROOT_DIR := $(abspath $(dir $(lastword $(MAKEFILE_LIST))))
TOOLS_DIR := $(ROOT_DIR)/.tools/bin
ARTIFACTS_DIR := $(ROOT_DIR)/.artifacts

COMPOSE ?= $(shell if docker compose version >/dev/null 2>&1; then echo "docker compose"; elif docker-compose version >/dev/null 2>&1; then echo "docker-compose"; else echo "docker compose"; fi)
MICROSERVICES_COMPOSE_FILE := $(ROOT_DIR)/deploy/compose/docker-compose.microservices.yml
MICROSERVICES_PROJECT ?= conduit-microservices
CONDUIT_IMAGE ?= conduit:local
GATEWAY_HOST ?= http://localhost:8000
FRONTEND_HOST ?= http://localhost:3000
LOCAL_AUTH_JWT_SECRET ?= development-only-change-me-jwt-secret
LOCAL_AUTH_PASSWORD_PEPPER ?= development-change-me-pepper
LOCAL_COMPOSE_ENV = AUTH_JWT_SECRET="$(LOCAL_AUTH_JWT_SECRET)" AUTH_PASSWORD_PEPPER="$(LOCAL_AUTH_PASSWORD_PEPPER)"
MICRO_COMPOSE = $(LOCAL_COMPOSE_ENV) CONDUIT_IMAGE=$(CONDUIT_IMAGE) $(COMPOSE) --project-name $(MICROSERVICES_PROJECT) --file $(MICROSERVICES_COMPOSE_FILE) --profile microservices

KIND_VERSION ?= v0.33.0
KUBECTL_VERSION ?= v1.37.0
KIND := $(TOOLS_DIR)/kind
KUBECTL := $(TOOLS_DIR)/kubectl
KIND_CLUSTER ?= conduit-microservices
KUBE_NAMESPACE ?= conduit
K8S_BASE := $(ROOT_DIR)/deploy/kubernetes/base
K8S_KIND := $(ROOT_DIR)/deploy/kubernetes/overlays/kind
K8S_JOBS := $(ROOT_DIR)/deploy/kubernetes/jobs
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
COVERAGE_PROFILE ?= coverage.out
COVERAGE_BADGE ?= docs/coverage.svg

.PHONY: help doctor test vet check image \
	microservices-build microservices-platform-up microservices-up microservices-wait microservices-down microservices-reset microservices-ps microservices-logs microservices-test microservices-check microservices-topics microservices-outbox \
	k8s-tools k8s-render k8s-validate kind-create kind-load k8s-up k8s-migrate k8s-status k8s-logs k8s-test k8s-check k8s-down \
	oapi-codegen gen-http go-wrap sqlc-install sqlc grpc-tools grpc-lint grpc-breaking gen-grpc wire mocks wrap generate golangci-lint-install lint go-arch-lint-install arch-lint arch-graph go-test-coverage-install coverage badges quality

-include .env
export

help: ## Show the complete local-development command list.
	@awk 'BEGIN {FS = ":.*## "; printf "Conduit development commands\n\n"} /^[a-zA-Z0-9_.-]+:.*## / {printf "  \033[36m%-30s\033[0m %s\n", $$1, $$2}' $(MAKEFILE_LIST)

doctor: ## Check the tools required for local development.
	@set -eu; \
	missing=0; \
	for tool in go git make docker curl; do \
		if command -v "$$tool" >/dev/null 2>&1; then printf "ok       %s\n" "$$tool"; else printf "missing  %s\n" "$$tool"; missing=1; fi; \
	done; \
	if $(COMPOSE) version >/dev/null 2>&1; then printf "ok       compose (%s)\n" '$(COMPOSE)'; else printf "missing  Docker Compose\n"; missing=1; fi; \
	if command -v hurl >/dev/null 2>&1; then printf "ok       hurl\n"; else printf "missing  hurl (required for contract tests)\n"; missing=1; fi; \
	exit $$missing

test: ## Run all Go unit tests.
	go test ./...

vet: ## Run go vet for every package.
	go vet ./...

check: test vet grpc-lint arch-lint ## Run fast local correctness checks without coverage.

image: ## Build the all-in-one local container image.
	docker build --tag $(CONDUIT_IMAGE) .

microservices-build: image ## Build the image shared by all local microservices.

microservices-platform-up: ## Start only PostgreSQL, Kafka, and Redis.
	@$(LOCAL_COMPOSE_ENV) CONDUIT_IMAGE=$(CONDUIT_IMAGE) $(COMPOSE) --project-name $(MICROSERVICES_PROJECT) --file $(MICROSERVICES_COMPOSE_FILE) up --detach postgres kafka redis

microservices-up: microservices-build ## Start the complete Compose microservice topology and wait for gateway.
	@$(MICRO_COMPOSE) up --detach --remove-orphans
	@$(MAKE) --no-print-directory microservices-wait

microservices-wait: ## Wait until the Compose gateway and frontend accept HTTP requests.
	@set -eu; \
	for attempt in $$(seq 1 90); do \
		if curl --fail --silent --show-error $(GATEWAY_HOST)/metrics >/dev/null 2>&1 && curl --fail --silent --show-error $(FRONTEND_HOST)/ >/dev/null 2>&1; then echo "gateway and frontend are ready"; exit 0; fi; \
		sleep 2; \
	done; \
	echo "gateway or frontend did not become ready; run 'make microservices-logs'" >&2; exit 1

microservices-down: ## Stop the Compose microservices and preserve volumes.
	@$(MICRO_COMPOSE) down --remove-orphans

microservices-reset: ## Stop Compose and DELETE all local PostgreSQL/Kafka/Redis data.
	@$(MICRO_COMPOSE) down --volumes --remove-orphans

microservices-ps: ## Show every Compose service and its health.
	@$(MICRO_COMPOSE) ps --all

microservices-logs: ## Follow all Compose microservice logs.
	@$(MICRO_COMPOSE) logs --follow --tail=200

microservices-test: microservices-wait ## Run all canonical RealWorld tests against Compose.
	HOST=$(GATEWAY_HOST) bash apitests/run-hurl-tests.sh

microservices-check: ## Build, start, and contract-test the Compose topology.
	@$(MAKE) --no-print-directory microservices-up
	@$(MAKE) --no-print-directory microservices-test

microservices-topics: ## List Kafka topics in the Compose topology.
	@$(MICRO_COMPOSE) exec --no-TTY kafka /opt/kafka/bin/kafka-topics.sh --bootstrap-server kafka:9092 --list

microservices-outbox: ## Show unpublished outbox rows for each producer database.
	@set -eu; \
	for source in subscriptions posts profile; do \
		echo "$$source:"; \
		$(MICRO_COMPOSE) exec --no-TTY postgres psql --username="conduit_$$source" --dbname="conduit_$$source" --tuples-only --command='SELECT count(*) AS unpublished FROM outbox_events WHERE published_at IS NULL;'; \
	done

$(KIND):
	@mkdir -p $(TOOLS_DIR)
	@set -eu; \
	os=$$(uname -s | tr '[:upper:]' '[:lower:]'); \
	arch=$$(uname -m); \
	case "$$arch" in x86_64) arch=amd64 ;; arm64|aarch64) arch=arm64 ;; *) echo "unsupported architecture: $$arch" >&2; exit 1 ;; esac; \
	curl --fail --location --silent --show-error "https://kind.sigs.k8s.io/dl/$(KIND_VERSION)/kind-$$os-$$arch" --output $(KIND); \
	chmod 0755 $(KIND)

$(KUBECTL):
	@mkdir -p $(TOOLS_DIR)
	@set -eu; \
	os=$$(uname -s | tr '[:upper:]' '[:lower:]'); \
	arch=$$(uname -m); \
	case "$$arch" in x86_64) arch=amd64 ;; arm64|aarch64) arch=arm64 ;; *) echo "unsupported architecture: $$arch" >&2; exit 1 ;; esac; \
	curl --fail --location --silent --show-error "https://dl.k8s.io/release/$(KUBECTL_VERSION)/bin/$$os/$$arch/kubectl" --output $(KUBECTL); \
	chmod 0755 $(KUBECTL)

k8s-tools: $(KIND) $(KUBECTL) ## Install pinned kind and kubectl into .tools/bin.

k8s-render: $(KUBECTL) ## Render production and local-kind manifests into .artifacts.
	@mkdir -p $(ARTIFACTS_DIR)/kubernetes
	$(KUBECTL) kustomize $(K8S_BASE) > $(ARTIFACTS_DIR)/kubernetes/base.yaml
	$(KUBECTL) kustomize $(K8S_KIND) > $(ARTIFACTS_DIR)/kubernetes/kind.yaml
	$(KUBECTL) kustomize $(K8S_JOBS) > $(ARTIFACTS_DIR)/kubernetes/jobs.yaml

k8s-validate: k8s-render ## Render and verify both Kubernetes variants without a cluster.
	@test -s $(ARTIFACTS_DIR)/kubernetes/base.yaml
	@test -s $(ARTIFACTS_DIR)/kubernetes/kind.yaml
	@test -s $(ARTIFACTS_DIR)/kubernetes/jobs.yaml
	@test "$$(grep -c '^kind: HorizontalPodAutoscaler' $(ARTIFACTS_DIR)/kubernetes/base.yaml)" -eq 6
	@test "$$(grep -c '^kind: HorizontalPodAutoscaler' $(ARTIFACTS_DIR)/kubernetes/kind.yaml || true)" -eq 0
	@test "$$(grep -c '^kind: Job' $(ARTIFACTS_DIR)/kubernetes/jobs.yaml)" -eq 6

kind-create: $(KIND) ## Create the local kind cluster if it does not exist.
	@if $(KIND) get clusters | grep -Fxq '$(KIND_CLUSTER)'; then echo "kind cluster $(KIND_CLUSTER) already exists"; else $(KIND) create cluster --name $(KIND_CLUSTER) --wait 180s; fi

kind-load: kind-create image ## Load the local Conduit image into kind.
	$(KIND) load docker-image $(CONDUIT_IMAGE) --name $(KIND_CLUSTER)

k8s-up: k8s-tools ## Create kind, deploy the laptop overlay, migrate, and wait for readiness.
	@$(MAKE) --no-print-directory kind-load
	-$(KUBECTL) --context kind-$(KIND_CLUSTER) --namespace $(KUBE_NAMESPACE) delete horizontalpodautoscaler --all --ignore-not-found
	$(KUBECTL) --context kind-$(KIND_CLUSTER) apply --kustomize $(K8S_KIND)
	$(KUBECTL) --context kind-$(KIND_CLUSTER) --namespace $(KUBE_NAMESPACE) rollout status statefulset/postgres --timeout=300s
	$(KUBECTL) --context kind-$(KIND_CLUSTER) --namespace $(KUBE_NAMESPACE) rollout status statefulset/redis --timeout=300s
	$(KUBECTL) --context kind-$(KIND_CLUSTER) --namespace $(KUBE_NAMESPACE) rollout status statefulset/kafka --timeout=420s
	@$(MAKE) --no-print-directory k8s-migrate
	@set -eu; for deployment in auth profile subscriptions posts comments gateway subscriptions-outbox posts-outbox profile-outbox; do $(KUBECTL) --context kind-$(KIND_CLUSTER) --namespace $(KUBE_NAMESPACE) rollout status deployment/$$deployment --timeout=420s; done

k8s-migrate: $(KUBECTL) ## Recreate and wait for database migration and Kafka topic Jobs.
	-$(KUBECTL) --context kind-$(KIND_CLUSTER) delete --kustomize $(K8S_JOBS) --ignore-not-found --wait=true
	$(KUBECTL) --context kind-$(KIND_CLUSTER) apply --kustomize $(K8S_JOBS)
	$(KUBECTL) --context kind-$(KIND_CLUSTER) --namespace $(KUBE_NAMESPACE) wait --for=condition=complete job --all --timeout=420s

k8s-status: $(KUBECTL) ## Show pods, services, deployments, jobs, HPA, and PDB in kind.
	$(KUBECTL) --context kind-$(KIND_CLUSTER) --namespace $(KUBE_NAMESPACE) get pods,services,deployments,statefulsets,jobs,horizontalpodautoscalers,poddisruptionbudgets --output=wide

k8s-logs: $(KUBECTL) ## Follow logs from all application pods in kind.
	$(KUBECTL) --context kind-$(KIND_CLUSTER) --namespace $(KUBE_NAMESPACE) logs --selector app.kubernetes.io/part-of=conduit --all-containers --prefix --follow --tail=100

k8s-test: $(KUBECTL) ## Port-forward gateway and run canonical RealWorld tests against kind.
	@mkdir -p $(ARTIFACTS_DIR)
	@set -eu; \
	$(KUBECTL) --context kind-$(KIND_CLUSTER) --namespace $(KUBE_NAMESPACE) port-forward service/gateway 8000:8000 >$(ARTIFACTS_DIR)/gateway-port-forward.log 2>&1 & \
	forward_pid=$$!; \
	trap 'kill $$forward_pid >/dev/null 2>&1 || true; wait $$forward_pid >/dev/null 2>&1 || true' EXIT; \
	for attempt in $$(seq 1 60); do curl --fail --silent http://127.0.0.1:8000/metrics >/dev/null 2>&1 && break; sleep 1; done; \
	curl --fail --silent http://127.0.0.1:8000/metrics >/dev/null; \
	HOST=http://127.0.0.1:8000 bash apitests/run-hurl-tests.sh

k8s-check: ## Deploy the complete kind topology and run contract tests.
	@$(MAKE) --no-print-directory k8s-up
	@$(MAKE) --no-print-directory k8s-test

k8s-down: $(KIND) ## Delete the local kind cluster and all data inside it.
	$(KIND) delete cluster --name $(KIND_CLUSTER)

oapi-codegen:
	go install github.com/oapi-codegen/oapi-codegen/v2/cmd/oapi-codegen@$(OAPI_CODEGEN_VERSION)

gen-http: oapi-codegen
	$(OAPI_CODEGEN) --config=config.yaml $(OPENAPI_SPEC)

go-wrap:
	go install github.com/hexdigest/gowrap/cmd/gowrap@$(GOWRAP_VERSION)

wrap: go-wrap
	go generate ./internal/metrics

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
