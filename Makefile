VERSION := $(shell v=$$(git describe --tags 2>/dev/null | sed -e 's/^v//g' | awk -F "-" '{print $$1}'); [ -n "$$v" ] && echo "$$v" || echo "v0.0.0")
LAST_DEV_NUM := $(shell git tag -l "$(VERSION)-dev*" | sed 's/.*-dev//' | grep -E '^[0-9]+$$' | sort -rn | head -n1)
INCREMENT := $(shell echo $$(($(if $(LAST_DEV_NUM),$(LAST_DEV_NUM),0) + 1)))
DEV_VERSION := $(VERSION)-dev$(INCREMENT)

ifneq (,$(wildcard ./.env))
    include .env
    export
endif

.PHONY: up-infra
up-infra:
	docker compose -f docker-compose.infra.yaml up -d

.PHONY: down-infra
down-infra:
	docker compose -f docker-compose.infra.yaml down

.PHONY: clean-infra
clean-infra:
	docker compose -f docker-compose.infra.yaml down -v


.PHONY: up-dev
up-dev:
	VERSION=$(DEV_VERSION) docker compose -f docker-compose.dev.yaml up -d --build --force-recreate

.PHONY: up-dev-no-build
up-dev-no-build:
	docker compose -f docker-compose.dev.yaml up -d

.PHONY: down-dev
down-dev:
	docker compose -f docker-compose.dev.yaml down

.PHONY: clean-dev
clean-dev:
	docker compose -f docker-compose.dev.yaml down -v

.PHONY: build
build:
	mkdir -p bin
	go build -o ./bin/ ./cmd/...

.PHONY: build-all
build-all:
	mkdir -p bin
	go build -o ./bin/ ./cmd/...

.PHONY: run-master-worker
run-master-worker: build-all
	./bin/master-worker 2>&1 | zap-pretty

.PHONY: run-api
run-api: build-all
	./bin/api 2>&1 | zap-pretty

.PHONY: run-test
run-test: build-all
	./bin/run --file ./examples/test.yaml

.PHONY: build-web
build-web:
	cd web && yarn install && yarn build
	rm -rf cmd/api/web/dist
	cp -r web/dist cmd/api/web/dist

.PHONY: build-api-image
build-api-image:
	docker build -f deployments/docker/api.Dockerfile -t stroppy-api:latest .

.PHONY: build-edge-worker-image
build-edge-worker-image:
	docker build -f deployments/docker/edge-worker.Dockerfile -t stroppy-edge-worker:latest .

.PHONY: release-dev-edge
release-dev-edge:
	mkdir -p bin
	go build -ldflags "-X github.com/stroppy-io/hatchet-workflow/internal/core/build.Version=$(DEV_VERSION) -X github.com/stroppy-io/hatchet-workflow/internal/core/build.ServiceName=edge-worker" -o ./bin/ ./cmd/edge-worker
	@echo "Built version: $(DEV_VERSION)"
	gh release create "$(DEV_VERSION)" ./bin/edge-worker#edge-worker --title "$(DEV_VERSION)" --notes "dev release" --prerelease
	git fetch --tags
