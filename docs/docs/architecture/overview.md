---
sidebar_position: 1
---

# Architecture Overview

Stroppy Cloud follows a server-agent architecture. A central orchestration server builds and executes a DAG of tasks. Each task sends commands to lightweight agents running on target machines (VMs or Docker containers).

## Components

### Orchestration Server

The server (`internal/domain/api/`) exposes three API surfaces:

- **External API** (`/api/v1/`) -- for users and CI systems to submit runs, check status, and query metrics.
- **Agent API** (`/api/agent/`) -- for agents to register themselves and report command results.
- **WebSocket** (`/ws/logs/`) -- for real-time log streaming to UI clients.

Internally, the server is composed of:

- **App** (`api.App`) -- the application facade that wires together the DAG builder, executor, storage, and agent client.
- **DAG Builder** (`run.Build`) -- translates a `RunConfig` into a `dag.Graph` with typed task nodes.
- **DAG Executor** (`dag.Executor`) -- walks the graph, runs nodes as their dependencies complete, persists state to BadgerDB.
- **Agent Client** (`agent.Client`) -- sends HTTP commands to agents and waits for completion reports.

### Agent

The agent (`internal/domain/agent/`) runs on each provisioned machine. It:

1. Starts an HTTP server with `/health` and `/execute` endpoints.
2. Registers itself with the orchestration server via `POST /api/agent/register`.
3. Receives `Command` messages and dispatches them to action handlers (install/configure database, monitoring, stroppy, etc.).
4. Reports results back to the server.

### Storage

Execution state is persisted in BadgerDB (`internal/infrastructure/badger/`). Each run's DAG graph and per-node status is saved as a `Snapshot` after every node completion. This enables run resumption after server restarts.

## Request Flow

```
User                    Server                      Agent(s)
  |                       |                            |
  |-- POST /api/v1/run -->|                            |
  |<-- 202 {run_id} ------|                            |
  |                       |-- Build DAG from RunConfig |
  |                       |-- Execute DAG              |
  |                       |                            |
  |                       |-- POST /execute (install_db)-->|
  |                       |<-- Report {completed} ---------|
  |                       |                            |
  |                       |-- POST /execute (config_db)--->|
  |                       |<-- Report {completed} ---------|
  |                       |                            |
  |                       |   ... (parallel branches)  |
  |                       |                            |
  |-- GET /run/{id}/status->|                          |
  |<-- Snapshot ------------|                          |
```

## Concurrency Model

The DAG executor runs independent branches in parallel using goroutines. Within each wave, all "ready" nodes (those whose dependencies are satisfied) start simultaneously. The executor uses a fail-fast strategy: on first error, no new nodes are started, but currently running nodes are allowed to finish. State is saved to storage, and the run can be resumed later with `POST /api/v1/run/{runID}/resume`.

## Providers

| Provider | Value | Description |
|----------|-------|-------------|
| Yandex Cloud | `yandex` | Provisions VMs via Yandex Cloud API. Requires cloud settings (folder ID, subnet, SSH key, etc.) configured via the admin API. |
| Docker | `docker` | Creates containers on the local Docker daemon. Uses a shared bridge network. Agents call back via `host.docker.internal`. |
