# Stroppy Cloud

Database benchmarking orchestrator. One binary that deploys databases, configures HA topologies, runs [Stroppy](https://github.com/stroppy-io/stroppy) benchmarks, and collects metrics. Supports local Docker runs and Yandex Cloud VMs.

## Supported databases

| Database   | Topologies                          | Versions    |
|------------|-------------------------------------|-------------|
| PostgreSQL | `single`, `ha`, `scale`             | 16, 17      |
| MySQL      | `single`, `replica`, `group`        | 8.0, 8.4    |
| Picodata   | `single`, `cluster`, `scale`        | 25.3        |

## Quick start

```bash
# Build (compiles Go binary with embedded SPA)
make build

# Start the full stack (server + VictoriaMetrics + VictoriaLogs + Grafana)
docker compose -f docker-compose.yaml up -d --build

# Open the web UI
open http://localhost:8080
```

Default login: `admin` / `admin`.

### CLI usage

```bash
# Start a run from JSON config (no server needed)
./bin/stroppy-cloud run -c examples/run-postgres-single.json

# Validate a config
./bin/stroppy-cloud validate -c examples/run-postgres-ha.json

# Print execution DAG
./bin/stroppy-cloud dry-run -c examples/run-mysql-group.json

# Start server with all integrations
./bin/stroppy-cloud serve \
  --addr :8080 \
  --victoria-url http://localhost:8428 \
  --victoria-logs-url http://localhost:9428 \
  --api-key my-secret-key
```

### API usage

```bash
TOKEN=$(curl -s -X POST http://localhost:8080/api/v1/auth/login \
  -H 'Content-Type: application/json' \
  -d '{"username":"admin","password":"admin"}' | jq -r .token)

# Start a run
curl -X POST http://localhost:8080/api/v1/run \
  -H "Authorization: Bearer $TOKEN" \
  -H 'Content-Type: application/json' \
  -d @examples/run-postgres-single.json

# Check status
curl -H "Authorization: Bearer $TOKEN" \
  http://localhost:8080/api/v1/run/my-run/status

# Compare two runs
curl -H "Authorization: Bearer $TOKEN" \
  'http://localhost:8080/api/v1/compare?a=run-1&b=run-2'

# Upload custom .deb package
curl -X POST http://localhost:8080/api/v1/upload/deb \
  -H "Authorization: Bearer $TOKEN" \
  -F file=@my-custom-postgres.deb
```

## Architecture

```
Server (Go binary)          Agent (same binary, agent mode)
  |                            |
  |- HTTP API (/api/v1/)       |- Receives commands via HTTP
  |- WebSocket (/ws/logs/)     |- Executes shell scripts
  |- Embedded SPA              |- Reports status back
  |- DAG executor              |- Manages background daemons
  |- BadgerDB state            |
  |                            |
  +-- VictoriaMetrics (metrics storage)
  +-- VictoriaLogs (log persistence)
  +-- Grafana (dashboards)
```

The server deploys agents on target machines (Docker containers or Yandex Cloud VMs) via cloud-init. Each agent receives commands from the server, executes them, and reports back. The server orchestrates the entire run as a DAG with automatic retries and backoff.

## CI Integration

See [docs/docs/ci-integration.md](docs/docs/ci-integration.md) for a complete guide on:
- Building custom database packages (.deb/.rpm)
- Uploading packages to the server
- Starting test runs via API
- Comparing results between runs
- Extracting metrics for CI reporting

## Configuration

Example run configs in `examples/`:

- `run-postgres-single.json` -- single-node PostgreSQL
- `run-postgres-ha.json` -- PostgreSQL HA with Patroni + etcd
- `run-mysql-group.json` -- MySQL Group Replication

## Development

```bash
make configure        # Check dependencies
make build            # Build binary (with embedded SPA)
make test             # Unit tests
make test-integration # Integration tests (requires Docker)
make lint             # Run linters
make web-dev          # Start SPA dev server
make docs-dev         # Start docs dev server
```

## Links

- [Stroppy](https://github.com/stroppy-io/stroppy) -- the K6-based benchmark tool
- [Documentation](docs/) -- full API reference, configuration guide, architecture

## License

See the [LICENSE](LICENSE) file for details.
