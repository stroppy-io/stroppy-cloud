# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## Project Overview

Stroppy Cloud is a distributed workflow orchestration system for automated database performance testing. It uses **Hatchet** as the task-scheduling backbone with a master/edge worker architecture: `master-worker` orchestrates test workflows while ephemeral `edge-worker` instances run on provisioned VMs to execute tasks (install software, run containers, execute tests).

All inter-service contracts are defined as **Protocol Buffers** (source in `tools/proto/`, generated Go in `internal/proto/`, generated TypeScript in `web/src/proto/`).

## Build & Development Commands

### Go Backend

```bash
make build                # Build all binaries to bin/
make run-master-worker    # Build + run master worker with zap-pretty
make run-test             # Build all + run test from examples/test.yaml
go test ./...             # Run all unit tests
go test ./internal/domain/provision/...  # Run tests for a specific package
go test -run TestName ./internal/...     # Run a single test
go test -tags integration ./...          # Run integration tests (need Docker)
```

### Infrastructure (Docker Compose)

```bash
make up-infra             # Start Hatchet + Valkey + Postgres + RabbitMQ
make down-infra           # Stop infrastructure
make clean-infra          # Stop + remove volumes
make up-dev               # Build and start master + edge workers in Docker
make down-dev             # Stop dev containers
```

### Web Frontend

```bash
cd web && npm install
npm run dev               # Vite dev server
npm run build             # TypeScript check + production build
npm run lint              # ESLint
```

### Protobuf Generation

Proto sources are in `tools/proto/`. Generation is configured via `tools/proto/easyp.yaml` using easyp. Generated files go to `internal/proto/` (Go) and `web/src/proto/` (TypeScript). **Never edit generated proto files directly.**

## Architecture

### Binary Entry Points (`cmd/`)

- **`master-worker`** — Long-running daemon; registers workflows with Hatchet, orchestrates test lifecycles
- **`edge-worker`** — Ephemeral per-VM binary; accepts task kinds from env, executes containers/stroppy tasks
- **`run`** — One-shot CLI; parses YAML test spec, triggers workflow via Hatchet, waits for completion

### Internal Code Organization (`internal/`)

```
internal/
├── core/           # Shared utilities (logger, IDs, IPs, shutdown, hatchet-ext wrappers)
├── domain/         # Business logic
│   ├── workflows/  # Hatchet workflow DAG definitions
│   │   ├── test/   # Master-side: TestSuiteWorkflow, TestRunWorkflow (11-task DAG)
│   │   └── edge/   # Edge-side: stroppy install/run, container setup
│   ├── provision/  # Placement orchestration (ProvisionerService, PostgresPlacementBuilder)
│   ├── deployment/ # Cloud target backends via Registry (strategy pattern)
│   │   ├── yandex/ # Terraform + Yandex Cloud (embedded .tf files)
│   │   └── docker/ # Local Docker backend
│   ├── managers/   # Infrastructure managers (Valkey-backed network CIDR allocation)
│   └── edge/       # Edge worker helpers (container lifecycle, postgres docker-compose)
├── infrastructure/ # External service adapters (terraform executor, valkey client, S3)
└── proto/          # Generated protobuf code (DO NOT EDIT)
```

### Key Abstractions

- **`hatchet_ext.WTask[I,O]` / `PTask[PO,I,O]`** (`internal/core/hatchet-ext/task.go`) — Type-safe generic wrappers around Hatchet tasks that avoid `any` casts and auto-unmarshal parent outputs
- **`deployment.Registry`** (`internal/domain/deployment/deployment.go`) — Strategy pattern mapping `Target` enum to `Service` implementations; add new cloud providers by implementing the `Service` interface
- **`ProvisionerService`** (`internal/domain/provision/provision.go`) — Orchestrates full placement lifecycle: plan intent → build placement → deploy → destroy
- **`NetworkManager`** (`internal/domain/managers/network.go`) — Valkey-backed distributed CIDR reservation with locking

### Workflow Execution Flow

1. CLI (`bin/run --file examples/test.yaml`) parses YAML into protobuf input
2. Hatchet dispatches to `TestSuiteWorkflow` → spawns N child `TestRunWorkflow` via `RunMany`
3. Each `TestRunWorkflow` is an 11-task DAG: validate → acquire network → plan placement → build placement → deploy → wait workers → setup containers → install stroppy → run stroppy → destroy
4. Edge workers poll Hatchet and execute assigned tasks on provisioned VMs

### State & Coordination

- **Workflow state**: Hatchet's Postgres
- **Network allocation**: Valkey sets keyed by `network:{target}:{name}` with distributed locking
- **Terraform workdirs**: `/tmp/stroppy-terraform/{deploymentId}/`

## Conventions

- Go module path: `github.com/stroppy-io/hatchet-workflow`
- Protobuf types are the canonical domain model — YAML configs are unmarshaled into proto types via `protoyaml`
- Task/workflow names use kebab-case string constants (e.g., `"stroppy-test-run"`, `"validate-input"`)
- Table-driven tests with `testify` assertions; integration tests use `//go:build integration` tag
- Frontend uses React 19 + TypeScript + Vite + Tailwind CSS + @xyflow/react for topology visualization
