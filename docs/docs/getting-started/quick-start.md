---
sidebar_position: 2
---

# Quick Start

This guide runs a single-node PostgreSQL test locally using Docker containers.

## 1. Start the Infrastructure

Launch the full stack:

```bash
docker compose -f docker-compose.test.yaml up -d --build
```

This starts:
- **Stroppy Cloud server** on port 8080 (orchestration + embedded SPA)
- **VictoriaMetrics** on port 8428 (metrics storage)
- **VictoriaLogs** on port 9428 (log persistence)
- **Grafana** on port 3001 (dashboards)

The server uses `network_mode: host` so it can communicate directly with agent containers.

## 2. Open the Web UI

Navigate to [http://localhost:8080](http://localhost:8080).

Default login: `admin` / `admin`.

The UI provides:
- **Runs** -- table of all runs with filters, sorting, pagination
- **New Run** -- form-based run creation with topology visualization
- **Compare** -- side-by-side metric comparison of two runs
- **Monitoring** -- embedded Grafana dashboards
- **Presets** -- browse available topologies
- **Settings** -- server configuration

## 3. Create a Run via UI

1. Click **New Run** in the sidebar
2. Select **PostgreSQL**, version **16**, provider **Docker**
3. Choose topology: **single**
4. Set workload: **SIMPLE**, duration: **5m**, workers: **4**
5. Click **Launch Run**

You'll be redirected to the run detail page with live DAG progress, log streaming, and metrics.

## 4. Create a Run via API

```bash
# Authenticate
TOKEN=$(curl -s -X POST http://localhost:8080/api/v1/auth/login \
  -H 'Content-Type: application/json' \
  -d '{"username":"admin","password":"admin"}' | jq -r .token)

# Start a run (machines array can be empty -- auto-generated from topology)
curl -X POST http://localhost:8080/api/v1/run \
  -H "Authorization: Bearer $TOKEN" \
  -H "Content-Type: application/json" \
  -d '{
    "id": "my-first-run",
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
      "duration": "30s",
      "workers": 4
    }
  }'
```

## 5. Monitor Progress

### WebSocket (real-time)

```bash
websocat ws://localhost:8080/ws/logs/my-first-run
```

### REST API (polling)

```bash
curl http://localhost:8080/api/v1/run/my-first-run/status \
  -H "Authorization: Bearer $TOKEN"
```

### Web UI

Open [http://localhost:8080/runs/my-first-run](http://localhost:8080/runs/my-first-run) to see the DAG, live logs, phases, metrics, and Grafana dashboards.

## 6. Compare Two Runs

After running two or more tests:

### Via UI

1. Go to the **Runs** table
2. Check two completed runs
3. Click **Compare**
4. View the side-by-side metric diff

Or navigate to **Compare** in the sidebar and enter run IDs manually.

### Via API

```bash
curl "http://localhost:8080/api/v1/compare?a=run-001&b=run-002" \
  -H "Authorization: Bearer $TOKEN"
```

Time range is auto-resolved from run timestamps.

## 7. Validate Before Running

```bash
# Validate only
curl -X POST http://localhost:8080/api/v1/validate \
  -H "Authorization: Bearer $TOKEN" \
  -H "Content-Type: application/json" \
  -d @examples/run-postgres-single.json

# Dry run (returns execution DAG)
curl -X POST http://localhost:8080/api/v1/dry-run \
  -H "Authorization: Bearer $TOKEN" \
  -H "Content-Type: application/json" \
  -d @examples/run-postgres-single.json
```

## 8. Tear Down

```bash
docker compose -f docker-compose.test.yaml down -v
```
