---
sidebar_position: 1
---

# External API

The external API is the primary interface for users and CI systems. All endpoints are under `/api/v1/`.

## Authentication

The server uses token-based authentication. Obtain a token via the login endpoint, then pass it in subsequent requests.

### POST /api/v1/auth/login

```bash
curl -X POST http://localhost:8080/api/v1/auth/login \
  -H 'Content-Type: application/json' \
  -d '{"username":"admin","password":"admin"}'
```

**Response:**

```json
{"token": "eyJhbG...", "username": "admin"}
```

### GET /api/v1/auth/me

Returns the authenticated user's info.

```bash
curl http://localhost:8080/api/v1/auth/me \
  -H "Authorization: Bearer $TOKEN"
```

All subsequent endpoints require `Authorization: Bearer <token>`.

Exempt paths (no auth required): `/health`, `/agent/binary`, `/api/agent/*`.

## Run Management

### POST /api/v1/run

Start a new test run. The request body is a full `RunConfig` JSON document. The `machines` array can be empty -- the server auto-generates it from the topology.

```bash
curl -X POST http://localhost:8080/api/v1/run \
  -H "Authorization: Bearer $TOKEN" \
  -H "Content-Type: application/json" \
  -d '{
    "id": "run-001",
    "provider": "docker",
    "network": {"cidr": "10.10.0.0/24"},
    "machines": [],
    "database": {
      "kind": "postgres",
      "version": "16",
      "postgres": {
        "master": {"role": "database", "count": 1, "cpus": 2, "memory_mb": 4096, "disk_gb": 50}
      }
    },
    "monitor": {},
    "stroppy": {
      "version": "3.1.0",
      "workload": "tpcb",
      "duration": "5m",
      "workers": 4
    }
  }'
```

**Response (202):**

```json
{"run_id": "run-001", "status": "started"}
```

### GET /api/v1/run/{runID}/status

Get the current execution state including DAG snapshot, timestamps, and run state.

```bash
curl http://localhost:8080/api/v1/run/run-001/status \
  -H "Authorization: Bearer $TOKEN"
```

**Response:**

```json
{
  "graph": "...",
  "nodes": [
    {"id": "network", "status": "done"},
    {"id": "machines", "status": "done"},
    {"id": "install_db", "status": "pending"}
  ],
  "started_at": "2026-03-31T14:00:00Z",
  "finished_at": "2026-03-31T14:05:00Z",
  "state": {
    "provider": "docker",
    "run_config": {...}
  }
}
```

### DELETE /api/v1/run/{runID}

Delete a run and clean up its Docker resources (containers, networks).

```bash
curl -X DELETE http://localhost:8080/api/v1/run/run-001 \
  -H "Authorization: Bearer $TOKEN"
```

### GET /api/v1/runs

List all runs with summary info (status counts, timestamps, db_kind, provider).

```bash
curl http://localhost:8080/api/v1/runs \
  -H "Authorization: Bearer $TOKEN"
```

### POST /api/v1/validate

Validate a RunConfig without executing it.

```bash
curl -X POST http://localhost:8080/api/v1/validate \
  -H "Authorization: Bearer $TOKEN" \
  -H "Content-Type: application/json" \
  -d @examples/run-postgres-ha.json
```

### POST /api/v1/dry-run

Build the DAG and return its structure as JSON without executing it.

```bash
curl -X POST http://localhost:8080/api/v1/dry-run \
  -H "Authorization: Bearer $TOKEN" \
  -H "Content-Type: application/json" \
  -d @examples/run-postgres-ha.json
```

## Metrics & Comparison

### GET /api/v1/run/{runID}/metrics

Fetch aggregated metrics for a run. Time range is optional -- defaults to run timestamps.

```bash
curl 'http://localhost:8080/api/v1/run/run-001/metrics?start=2026-03-31T14:00:00Z&end=2026-03-31T14:10:00Z' \
  -H "Authorization: Bearer $TOKEN"
```

Returns `MetricSummary[]` with avg/min/max/last for each metric (db_tps, cpu_usage, memory_usage, stroppy_ops, etc).

### GET /api/v1/compare

Compare metrics between two runs. Time range is optional -- auto-resolved from run timestamps if omitted.

```bash
# Auto time range (recommended)
curl 'http://localhost:8080/api/v1/compare?a=run-001&b=run-002' \
  -H "Authorization: Bearer $TOKEN"

# Explicit time range
curl 'http://localhost:8080/api/v1/compare?a=run-001&b=run-002&start=2026-03-31T14:00:00Z&end=2026-03-31T15:00:00Z' \
  -H "Authorization: Bearer $TOKEN"
```

**Response:**

```json
{
  "run_a": "run-001",
  "run_b": "run-002",
  "start": "2026-03-31T13:59:30Z",
  "end": "2026-03-31T14:10:30Z",
  "metrics": [
    {
      "key": "db_tps",
      "name": "DB Transactions Per Second",
      "unit": "txn/s",
      "avg_a": 1250.5,
      "avg_b": 1340.2,
      "diff_avg_pct": 7.2,
      "verdict": "better"
    }
  ],
  "summary": {"better": 3, "worse": 1, "same": 12}
}
```

## Logs

### GET /api/v1/run/{runID}/logs

Fetch historical logs from VictoriaLogs (NDJSON format). Returns 503 if VictoriaLogs is not configured.

```bash
curl http://localhost:8080/api/v1/run/run-001/logs \
  -H "Authorization: Bearer $TOKEN"
```

## Presets

### GET /api/v1/presets

List all topology presets for all database kinds.

```bash
curl http://localhost:8080/api/v1/presets \
  -H "Authorization: Bearer $TOKEN"
```

## Packages & Upload

### POST /api/v1/upload/deb

Upload a custom .deb package. The server stores it and returns a URL that agents can download.

```bash
curl -X POST http://localhost:8080/api/v1/upload/deb \
  -H "Authorization: Bearer $TOKEN" \
  -F file=@postgresql-custom-16_1.0_amd64.deb
```

**Response:**

```json
{
  "filename": "postgresql-custom-16_1.0_amd64.deb",
  "url": "http://server:8080/packages/postgresql-custom-16_1.0_amd64.deb",
  "size": "52428800"
}
```

Use the returned URL in the RunConfig `packages.deb_files` array.

## Admin

### GET/PUT /api/v1/admin/settings

Read or update server settings (cloud config, monitoring stack, stroppy defaults, Grafana).

### GET/PUT /api/v1/admin/packages

Read or update default package sets per database kind and version.

### GET /api/v1/admin/db-defaults/{kind}

Get default database configuration parameters for a database kind.

### GET /api/v1/admin/grafana

Get Grafana integration settings (URL, embed flag, dashboard UIDs).

## WebSocket

### GET /ws/logs

Stream all run logs in real time (all runs).

### GET /ws/logs/{runID}

Stream logs for a specific run.

Messages are JSON with `type` field: `"log"`, `"agent_log"`, `"report"`.

## Health

### GET /health

Health check (no auth required). Returns `{"status": "ok"}`.
