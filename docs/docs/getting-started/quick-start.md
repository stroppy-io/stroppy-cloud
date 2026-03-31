---
sidebar_position: 2
---

# Quick Start

This guide runs a single-node PostgreSQL test locally using Docker containers.

## 1. Start the Infrastructure

Launch VictoriaMetrics and the Stroppy Cloud server:

```bash
docker compose -f docker-compose.test.yaml up -d
```

This starts:
- **VictoriaMetrics** on port 8428 (metrics storage)
- **Stroppy Cloud server** on port 8080 (orchestration)

The server container mounts the Docker socket so it can create agent containers for the `docker` provider.

## 2. Submit a Run

Send a `RunConfig` to start a single-node Postgres test:

```bash
curl -X POST http://localhost:8080/api/v1/run \
  -H "Content-Type: application/json" \
  -d '{
    "id": "my-first-run",
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
    "monitor": {
      "metrics_endpoint": "http://victoria:8428/api/v1/write"
    },
    "stroppy": {
      "version": "3.1.0",
      "workload": "tpcb",
      "duration": "30s",
      "workers": 4
    }
  }'
```

The server returns immediately with a `202 Accepted`:

```json
{"run_id": "my-first-run", "status": "started"}
```

## 3. Monitor Progress

Connect to the WebSocket endpoint to stream logs in real time:

```bash
websocat ws://localhost:8080/ws/logs/my-first-run
```

Or check the run status via the REST API:

```bash
curl http://localhost:8080/api/v1/run/my-first-run/status
```

The response contains the DAG snapshot with per-node status (`pending`, `done`, `failed`).

## 4. Validate Before Running

Use the `/validate` or `/dry-run` endpoints to check a config without executing it:

```bash
# Validate only (returns 200 or 422)
curl -X POST http://localhost:8080/api/v1/validate \
  -H "Content-Type: application/json" \
  -d @examples/run-postgres-single.json

# Dry run (returns the DAG structure as JSON)
curl -X POST http://localhost:8080/api/v1/dry-run \
  -H "Content-Type: application/json" \
  -d @examples/run-postgres-single.json
```

## 5. View Available Presets

List all built-in topology presets for all database kinds:

```bash
curl http://localhost:8080/api/v1/presets | jq
```

## 6. Tear Down

Stop the infrastructure:

```bash
docker compose -f docker-compose.test.yaml down -v
```
