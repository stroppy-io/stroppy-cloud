---
sidebar_position: 1
---

# Building

## Prerequisites

- Go 1.21+
- Docker and Docker Compose
- Make
- `zap-pretty` (optional, for readable dev logs): `go install github.com/maoueh/zap-pretty@latest`

## Build Commands

### Build all binaries

```bash
make build
```

Produces `./bin/cli` -- a single binary that acts as both the orchestration server and the agent.

### Build and run the server in dev mode

```bash
make up-dev
```

This builds a Docker image and starts the full stack (VictoriaMetrics + server) with Docker Compose. The dev compose file uses a build context from the repository root.

### Run without rebuilding

```bash
make up-dev-no-build
```

### Stop dev environment

```bash
make down-dev
# or with volume cleanup:
make clean-dev
```

### Build the Docker image for the agent

```bash
make build-edge-worker-image
```

## Project Structure

```
cmd/cli/                     CLI entrypoint
internal/
  core/
    build/build.go           Build metadata (version, service name)
    dag/                     DAG graph, executor, storage, node context
    defaults/                String default helpers
    ips/                     IP address allocation utilities
    logger/                  Structured logging with zap
    protoyaml/               Protobuf-to-YAML conversion
    shutdown/                Graceful shutdown handler
    uow/                     Unit-of-work pattern
    utils/                   Embed file utilities
  domain/
    api/                     HTTP server, routes, admin, auth, WebSocket, upload
    agent/                   Agent server, protocol, cloud-init, deployer, setup_*
    run/                     DAG builder, task implementations per database/component
    types/                   Domain types (RunConfig, settings, packages, topologies)
    metrics/                 VictoriaMetrics collector and comparison
  infrastructure/
    badger/                  BadgerDB storage implementation
    victoria/                VictoriaMetrics HTTP client
```

## Dependencies

Key Go dependencies (from go.mod):

- `github.com/go-chi/chi/v5` -- HTTP router
- `github.com/gorilla/websocket` -- WebSocket support
- `github.com/dgraph-io/badger/v4` -- Embedded key-value storage
- `go.uber.org/zap` -- Structured logging
- Docker SDK -- Container management for the docker provider

## Versioning

The Makefile derives versions from git tags:

```bash
# Stable version from latest tag
VERSION := $(git describe --tags)

# Dev version with auto-incrementing suffix
DEV_VERSION := $(VERSION)-dev$(INCREMENT)
```

Dev releases are published to GitHub Releases:

```bash
make release-dev-edge
```

## Cross-compilation

The binary is a standard Go project and supports cross-compilation:

```bash
GOOS=linux GOARCH=amd64 go build -o ./bin/cli ./cmd/cli
```
