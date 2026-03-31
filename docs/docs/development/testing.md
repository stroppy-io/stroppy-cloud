---
sidebar_position: 2
---

# Testing

## Running Tests

Run the full test suite:

```bash
go test ./...
```

Run tests with verbose output:

```bash
go test -v ./...
```

Run tests for a specific package:

```bash
go test -v ./internal/core/dag/...
go test -v ./internal/core/logger/...
go test -v ./internal/core/ips/...
go test -v ./internal/core/shutdown/...
go test -v ./internal/domain/api/...
go test -v ./internal/domain/agent/...
```

## Test Infrastructure

The project includes a Docker Compose file for integration testing:

```bash
docker compose -f docker-compose.test.yaml up -d
```

This starts:
- **VictoriaMetrics** on port 8428
- **Stroppy Cloud server** on port 8080 with Docker socket mounted

You can then submit test runs against the running server:

```bash
curl -X POST http://localhost:8080/api/v1/run \
  -H "Content-Type: application/json" \
  -d @examples/run-postgres-single.json
```

## Key Test Files

| Package | Tests | Description |
|---------|-------|-------------|
| `internal/core/dag/` | `dag_test.go` | DAG construction, validation, cycle detection, ready node calculation |
| `internal/core/ips/` | `ips_test.go` | IP address allocation from CIDR ranges |
| `internal/core/logger/` | `logger_test.go`, `ctx_test.go` | Logger initialization and context propagation |
| `internal/core/shutdown/` | `shutdown_test.go`, `shortcut_test.go` | Graceful shutdown signal handling |
| `internal/domain/api/` | `auth_test.go` | API key authentication middleware |
| `internal/domain/agent/` | `executor_test.go` | Agent command executor |

## Example Test Configs

The `examples/` directory contains validated RunConfig files for testing:

- `examples/run-postgres-single.json` -- Single-node PostgreSQL
- `examples/run-postgres-ha.json` -- 3-node PostgreSQL HA with Patroni, PgBouncer, HAProxy
- `examples/run-mysql-group.json` -- 3-node MySQL Group Replication with ProxySQL

These can be used with both the validate and dry-run endpoints:

```bash
# Validate
curl -X POST http://localhost:8080/api/v1/validate \
  -H "Content-Type: application/json" \
  -d @examples/run-postgres-ha.json

# Dry run
curl -X POST http://localhost:8080/api/v1/dry-run \
  -H "Content-Type: application/json" \
  -d @examples/run-postgres-ha.json | jq
```

## Writing Tests

### DAG Tests

Test DAG construction by building a graph and checking its structure:

```go
func TestGraphValidation(t *testing.T) {
    g := dag.New()
    _ = g.Add(&dag.Node{ID: "a", Type: "test"})
    _ = g.Add(&dag.Node{ID: "b", Type: "test", Deps: []string{"a"}})

    err := g.Validate()
    assert.NoError(t, err)

    ready := g.Ready(map[string]bool{})
    assert.Len(t, ready, 1)
    assert.Equal(t, "a", ready[0].ID)

    ready = g.Ready(map[string]bool{"a": true})
    assert.Len(t, ready, 1)
    assert.Equal(t, "b", ready[0].ID)
}
```

### Agent Tests

Test agent command execution with the executor:

```go
func TestExecutor(t *testing.T) {
    exec := agent.NewExecutor()
    report := exec.Run(context.Background(), agent.Command{
        ID:     "test-cmd",
        Action: agent.ActionInstallPostgres,
        Config: map[string]any{"version": "16"},
    })
    // Check report.Status and report.Output
}
```
