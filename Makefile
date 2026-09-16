# stroppy-cloud Makefile
.PHONY: help configure build build-pipelines test lint fmt openapi generate-go generate-ts schemas-export schemas-test \
        tools db-gen migrate-generate migrate-clear \
        web-install web-dev web-build docs-install docs-dev docs-build \
        docker-build clean

# ============================================================
# Variables
# ============================================================
VERSION ?= $(shell git describe --tags --always --dirty 2>/dev/null || echo "dev")
COMMIT  ?= $(shell git rev-parse --short HEAD 2>/dev/null || echo "unknown")
MODULE  := github.com/stroppy-io/stroppy-cloud
BINARY  := stroppy-server
LDFLAGS := -w -s -X $(MODULE)/internal/build.Version=$(VERSION) -X $(MODULE)/internal/build.Commit=$(COMMIT)
GOFLAGS := -trimpath -ldflags="$(LDFLAGS)"

DOCKER_IMAGE := docker.stroppy.io/stroppy-io/stroppy-server
DOCKER_TAG   := $(VERSION)

# ============================================================
# Help
# ============================================================
help: ## Show this help
	@grep -E '^[a-zA-Z_-]+:.*?## .*$$' $(MAKEFILE_LIST) | sort | awk 'BEGIN {FS = ":.*?## "}; {printf "\033[36m%-20s\033[0m %s\n", $$1, $$2}'

# ============================================================
# Configure — check all dependencies
# ============================================================
configure: ## Check that all required tools are installed
	@echo "Checking dependencies..."
	@command -v go >/dev/null 2>&1 || { echo "ERROR: go is not installed"; exit 1; }
	@echo "  go $$(go version | awk '{print $$3}')"
	@command -v node >/dev/null 2>&1 && echo "  node $$(node --version)" || echo "  WARNING: node not found (needed for web)"
	@command -v yarn >/dev/null 2>&1 && echo "  yarn $$(yarn --version)" || echo "  WARNING: yarn not found (needed for web)"
	@command -v golangci-lint >/dev/null 2>&1 && echo "  golangci-lint $$(golangci-lint --version 2>/dev/null | awk '{print $$4}')" || echo "  WARNING: golangci-lint not found"
	@echo "All required dependencies OK"

# ============================================================
# Build
# ============================================================
build: web-build ## Build the stroppy-server binary (with embedded SPA)
	@mkdir -p bin
	CGO_ENABLED=0 go build $(GOFLAGS) -o bin/$(BINARY) ./cmd/stroppy-cloud/

build-pipelines: ## Build the graphene pipeline binaries (linux/amd64, shipped in the server image)
	@mkdir -p bin
	cd pipelines && GOOS=linux GOARCH=amd64 CGO_ENABLED=0 go build -trimpath -o ../bin/stroppy-run ./cmd/run/
	cd pipelines && GOOS=linux GOARCH=amd64 CGO_ENABLED=0 go build -trimpath -o ../bin/stroppy-suite ./cmd/suite/
	cd pipelines && GOOS=linux GOARCH=amd64 CGO_ENABLED=0 go build -trimpath -o ../bin/stroppy-provider-verify ./cmd/provider-verify/
	cd pipelines && GOOS=linux GOARCH=amd64 CGO_ENABLED=0 go build -trimpath -o ../bin/stroppy-quotas ./cmd/quotas/

# ============================================================
# OpenAPI → code
# ============================================================
OPENAPI    := openapi/openapi.yaml
OPENAPI_30 := openapi/.build/openapi.3.0.yaml
OGEN       := go run github.com/ogen-go/ogen/cmd/ogen@v1.20.3

openapi: ## Merge openapi/parts/*.yaml into openapi/openapi.yaml
	python3 scripts/openapi_merge.py

generate-go: openapi ## Generate the ogen server/client into internal/oas
	@mkdir -p $(dir $(OPENAPI_30))
	python3 scripts/openapi_to_30.py $(OPENAPI) $(OPENAPI_30)
	$(OGEN) --config .ogen.yaml --target internal/oas --package oas --clean $(OPENAPI_30)

generate-ts: openapi ## Generate TypeScript API types into web/src/api
	cd web && yarn generate

.PHONY: contract-check
contract-check: ## Verify the reviewed contract and the browser JSON boundary
	python3 pipelines/live/tools/contract_lock.py
	cd web && yarn test:contract

schemas-export: ## Export schemapb schemas (protoJSON + TS types) into web/src/schemas
	cd pipelines && go run ./cmd/schemas-export -out ../web/src/schemas

schemas-test: ## Run schema tests (add ARGS=-update to refresh goldens)
	cd pipelines && go test ./schemas/... $(ARGS)

# ============================================================
# Postgres store codegen (sqld toolchain)
# ============================================================
SQLD_CFG := internal/infrastructure/postgres/sqld.yaml

tools: ## Install the sqld code generators into ./bin
	GOFLAGS=-mod=mod GOBIN=$$(pwd)/bin go install github.com/gopherex/sqld/cmd/sqld@v1.1.1
	GOFLAGS=-mod=mod GOBIN=$$(pwd)/bin go install github.com/gopherex/sqld/cmd/sqld-gen-go@v1.1.1
	GOFLAGS=-mod=mod GOBIN=$$(pwd)/bin go install github.com/gopherex/sqld/cmd/sqld-gen-bob@v1.1.1

db-gen: tools ## Generate gen/db + gen/bob from schema.sql + queries/*.sql
	./bin/sqld generate -c $(SQLD_CFG)

migrate-generate: tools ## Generate a migration by schema diff (usage: make migrate-generate name=add_table)
	@test -n "$(name)" || (echo "usage: make migrate-generate name=add_table" && exit 2)
	./bin/sqld migrate generate $(name) -c $(SQLD_CFG)

migrate-clear: tools ## Regenerate the single bootstrap migration from schema.sql, then regenerate code
	rm -f internal/infrastructure/postgres/migrations/*.sql
	./bin/sqld migrate generate bootstrap -c $(SQLD_CFG)
	@for f in internal/infrastructure/postgres/migrations/*.sql; do \
		awk 'index(tolower($$0), "-- sqld:" "up") == 1 { next } index(tolower($$0), "-- sqld:" "down") == 1 { exit } { print }' "$$f" > "$$f.tmp"; \
		mv "$$f.tmp" "$$f"; \
	done
	$(MAKE) db-gen

# ============================================================
# Test / lint
# ============================================================
test-db: tools ## Run integration tests (testcontainers Postgres + fake Graphene; needs Docker)
	go test -tags=integration ./cmd/... -count=1 -p 1

test: ## Run unit tests
	go test ./... -count=1 -race

lint: ## Run linters
	golangci-lint run ./...

fmt: ## Format Go code
	gofmt -s -w .
	go vet ./...

# ============================================================
# Web
# ============================================================
web-install: ## Install web dependencies
	cd web && yarn install --frozen-lockfile

web-dev: ## Start web dev server (proxies API to localhost:8080)
	cd web && yarn dev

web-build: ## Build web for production
	cd web && yarn build

# ============================================================
# Docs
# ============================================================
docs-install: ## Install docs dependencies
	cd docs && yarn install

docs-dev: ## Start docs dev server
	cd docs && yarn start

docs-build: ## Build docs static site
	cd docs && yarn build

# ============================================================
# Docker
# ============================================================
docker-build: ## Build the server image (server + pipeline binaries)
	docker build --build-arg VERSION=$(VERSION) --build-arg COMMIT=$(COMMIT) -t $(DOCKER_IMAGE):$(DOCKER_TAG) -f deployments/Dockerfile .

# ============================================================
# Clean
# ============================================================
clean: ## Clean build artifacts
	rm -rf bin web/dist
