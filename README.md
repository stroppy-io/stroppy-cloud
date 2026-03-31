# Stroppy Cloud

Database benchmarking orchestrator. One binary that deploys databases, configures HA topologies, and runs [Stroppy](https://github.com/stroppy-io/stroppy) benchmarks. Supports local Docker runs and Yandex Cloud VMs.

## Supported databases

| Database   | Topologies                          |
|------------|-------------------------------------|
| PostgreSQL | `single`, `ha`, `scale`             |
| MySQL      | `single`, `replica`, `group`        |
| Picodata   | `single`, `cluster`, `scale`        |

## Quick start

```bash
# Build
make build

# Run locally with Docker
docker compose -f docker-compose.test.yaml up -d --build
curl -X POST http://localhost:8080/api/v1/run \
  -H 'Content-Type: application/json' \
  -d @examples/run-postgres-single.json

# Or run directly from CLI (no server needed)
./bin/stroppy-cloud run -c examples/run-postgres-single.json
```

## Configuration

Example run configs are in the `examples/` directory:

- `run-postgres-single.json` -- single-node PostgreSQL
- `run-postgres-ha.json` -- PostgreSQL HA with Patroni + etcd
- `run-mysql-group.json` -- MySQL Group Replication

See the [docs site](docs/) for the full configuration reference, API documentation, and topology details.

## Development

```bash
make configure        # Check dependencies
make build            # Build binary
make test             # Unit tests
make test-integration # Integration tests (requires Docker)
make lint             # Run linters
```

## Links

- [Stroppy](https://github.com/stroppy-io/stroppy) -- the benchmark tool
- [Documentation](docs/) -- full API reference, configuration guide, architecture

## License

See the [LICENSE](LICENSE) file for details.
