---
sidebar_position: 1
---

# External API

The external API is the primary interface for users and CI systems. All endpoints are under `/api/v1/`.

## Authentication

When the server is started with `--api-key`, all external API requests must include an `Authorization` header:

```
Authorization: Bearer <api-key>
```

Raw key format is also accepted:

```
Authorization: <api-key>
```

The following paths are exempt from authentication:
- `/health`
- `/agent/binary`
- `/api/agent/*` (agent-to-server communication uses machine-to-machine trust)

## Endpoints

### POST /api/v1/run

Start a new test run. The request body is a full `RunConfig` JSON document. The server builds a DAG, starts execution asynchronously, and returns immediately.

**Request:**

```bash
curl -X POST http://localhost:8080/api/v1/run \
  -H "Content-Type: application/json" \
  -H "Authorization: Bearer my-secret-key" \
  -d '{
    "id": "run-001",
    "provider": "docker",
    "network": {"cidr": "10.10.0.0/24"},
    "machines": [
      {"role": "database", "count": 1, "cpus": 2, "memory_mb": 4096, "disk_gb": 50},
      {"role": "monitor",  "count": 1, "cpus": 1, "memory_mb": 2048, "disk_gb": 20},
      {"role": "stroppy",  "count": 1, "cpus": 2, "memory_mb": 4096, "disk_gb": 20}
    ],
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
      "duration": "30s",
      "workers": 4
    }
  }'
```

**Response (202 Accepted):**

```json
{"run_id": "run-001", "status": "started"}
```

### POST /api/v1/run/{runID}/resume

Resume a previously failed or interrupted run. The server loads the saved DAG state from storage, marks completed nodes as done, and continues execution from where it stopped. Failed nodes are retried.

**Request:**

```bash
curl -X POST http://localhost:8080/api/v1/run/run-001/resume \
  -H "Content-Type: application/json" \
  -H "Authorization: Bearer my-secret-key" \
  -d @examples/run-postgres-single.json
```

**Response (202 Accepted):**

```json
{"run_id": "run-001", "status": "resumed"}
```

### POST /api/v1/validate

Validate a `RunConfig` without executing it. Checks that the config produces a valid DAG (no unknown database kinds, no missing fields, no dependency cycles).

**Request:**

```bash
curl -X POST http://localhost:8080/api/v1/validate \
  -H "Content-Type: application/json" \
  -d @examples/run-postgres-ha.json
```

**Response (200 OK):**

```json
{"status": "valid"}
```

**Response (422 Unprocessable Entity):**

```json
{"error": "unsupported database kind \"cassandra\""}
```

### POST /api/v1/dry-run

Build the DAG and return its structure as JSON without executing it. Useful for visualizing the execution plan.

**Request:**

```bash
curl -X POST http://localhost:8080/api/v1/dry-run \
  -H "Content-Type: application/json" \
  -d @examples/run-postgres-ha.json
```

**Response (200 OK):**

```json
{
  "nodes": [
    {"id": "network", "type": "network"},
    {"id": "machines", "type": "machines", "deps": ["network"]},
    {"id": "install_db", "type": "install_db", "deps": ["machines"]},
    {"id": "configure_db", "type": "configure_db", "deps": ["install_db"]},
    {"id": "install_monitor", "type": "install_monitor", "deps": ["machines"]},
    {"id": "configure_monitor", "type": "configure_monitor", "deps": ["install_monitor"]},
    {"id": "install_stroppy", "type": "install_stroppy", "deps": ["machines"]},
    {"id": "run_stroppy", "type": "run_stroppy", "deps": ["configure_db", "configure_monitor", "install_stroppy"]},
    {"id": "teardown", "type": "teardown", "deps": ["run_stroppy"]}
  ]
}
```

### GET /api/v1/run/{runID}/status

Get the current execution state of a run. Returns the full DAG snapshot with per-node status.

**Request:**

```bash
curl http://localhost:8080/api/v1/run/run-001/status \
  -H "Authorization: Bearer my-secret-key"
```

**Response (200 OK):**

```json
{
  "graph": "...",
  "nodes": [
    {"id": "network", "status": "done"},
    {"id": "machines", "status": "done"},
    {"id": "install_db", "status": "done"},
    {"id": "configure_db", "status": "failed", "error": "pg_ctl init failed: exit 1"},
    {"id": "install_monitor", "status": "done"},
    {"id": "configure_monitor", "status": "pending"},
    {"id": "install_stroppy", "status": "done"},
    {"id": "run_stroppy", "status": "pending"},
    {"id": "teardown", "status": "pending"}
  ]
}
```

**Response (404 Not Found):**

```
not found
```

### GET /api/v1/presets

List all built-in topology presets for all database kinds.

**Request:**

```bash
curl http://localhost:8080/api/v1/presets \
  -H "Authorization: Bearer my-secret-key"
```

**Response (200 OK):**

```json
{
  "postgres": {
    "single": {"master": {"role": "database", "count": 1, "cpus": 2, "memory_mb": 4096, "disk_gb": 50}},
    "ha": {"master": {...}, "replicas": [...], "haproxy": {...}, "pgbouncer": true, "patroni": true, "etcd": true, "sync_replicas": 1},
    "scale": {...}
  },
  "mysql": {
    "single": {...},
    "replica": {...},
    "group": {...}
  },
  "picodata": {
    "single": {...},
    "cluster": {...},
    "scale": {...}
  }
}
```

### GET /health

Health check endpoint (no authentication required).

**Request:**

```bash
curl http://localhost:8080/health
```

**Response (200 OK):**

```json
{"status": "ok"}
```
