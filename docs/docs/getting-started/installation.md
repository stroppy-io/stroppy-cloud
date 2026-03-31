---
sidebar_position: 1
---

# Installation

## Prerequisites

- Go 1.21 or later
- Docker and Docker Compose (for local testing with the `docker` provider)
- Make

## Building from Source

Clone the repository and build all binaries:

```bash
git clone https://github.com/stroppy-io/stroppy-cloud.git
cd stroppy-cloud
make build
```

This produces a single binary in `./bin/` that serves as both the orchestration server and the agent. The binary supports two subcommands:

- `serve` -- starts the HTTP orchestration server
- `agent` -- starts the agent HTTP server on a target machine

## Binary Usage

Start the server:

```bash
./bin/cli serve \
  --addr=:8080 \
  --victoria-url=http://localhost:8428 \
  --data-dir=/var/lib/stroppy/badger
```

Server flags:

| Flag | Description | Default |
|------|-------------|---------|
| `--addr` | Listen address for the HTTP server | `:8080` |
| `--victoria-url` | VictoriaMetrics URL for metrics collection | (empty, disables metrics) |
| `--data-dir` | Directory for BadgerDB state persistence | (empty, in-memory) |
| `--api-key` | API key for authentication | (empty, auth disabled) |
| `--settings-path` | Path to persist admin settings JSON | (empty, no persistence) |

Start an agent (typically done automatically via cloud-init):

```bash
./bin/cli agent \
  --server-addr=http://orchestrator:8080 \
  --machine-id=db-node-1 \
  --port=9090
```

## Environment Variables

The server also reads from a `.env` file if present in the working directory. Key variables:

| Variable | Description |
|----------|-------------|
| `LOG_MOD` | Logging mode: `development` or `production` |
| `LOG_LEVEL` | Log level: `debug`, `info`, `warn`, `error` |
| `STROPPY_BINARY_HOST_PATH` | Host path where the server binary is located (for Docker volume mounts) |
