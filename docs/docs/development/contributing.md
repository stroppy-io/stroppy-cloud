---
sidebar_position: 3
---

# Contributing

## Development Setup

1. Clone the repository:

```bash
git clone https://github.com/stroppy-io/stroppy-cloud.git
cd stroppy-cloud
```

2. Install Go 1.21+ and Docker.

3. Build the project:

```bash
make build
```

4. Start the dev environment:

```bash
make up-dev
```

## Code Organization

The project follows a clean architecture pattern:

- **`internal/core/`** -- Framework-level code with no domain dependencies. The DAG engine, logger, shutdown handler, and utilities live here. These packages are reusable and have minimal external dependencies.

- **`internal/domain/`** -- Business logic. Contains the API server, agent protocol, run builder, type definitions, and metrics collection. Domain packages depend on core packages but not on infrastructure.

- **`internal/infrastructure/`** -- External system integrations. BadgerDB storage implementation, VictoriaMetrics client. These implement interfaces defined in core/domain packages.

## Adding a New Database

To add support for a new database engine:

1. **Define the topology type** in `internal/domain/types/run.go`:

```go
type NewDBTopology struct {
    Primary  MachineSpec       `json:"primary"`
    Replicas []MachineSpec     `json:"replicas,omitempty"`
    Options  map[string]string `json:"options,omitempty"`
}
```

2. **Add the database kind** constant:

```go
const DatabaseNewDB DatabaseKind = "newdb"
```

3. **Add the topology field** to `DatabaseConfig`:

```go
type DatabaseConfig struct {
    // ...existing fields...
    NewDB *NewDBTopology `json:"newdb,omitempty"`
}
```

4. **Define presets** (optional):

```go
var NewDBPresets = map[NewDBPreset]NewDBTopology{
    NewDBSingle: { Primary: MachineSpec{...} },
}
```

5. **Add default packages** in `DefaultPackages()` in `internal/domain/types/packages.go`.

6. **Create task implementations** in `internal/domain/run/`:
   - `task_newdb.go` with `newdbInstallTask` and `newdbConfigTask` structs implementing `dag.Task`.

7. **Create agent setup handlers** in `internal/domain/agent/`:
   - `setup_newdb.go` with the install and config logic.

8. **Add protocol actions** in `internal/domain/agent/protocol.go`:

```go
const (
    ActionInstallNewDB Action = "install_newdb"
    ActionConfigNewDB  Action = "config_newdb"
)
```

9. **Wire into the DAG builder** in `internal/domain/run/builder.go`:
   - Add a case to `dbTasks()` for the new kind.

10. **Add example configs** in `examples/`.

## Adding a New Agent Action

1. Add the action constant to `internal/domain/agent/protocol.go`.
2. Add a handler case in the agent executor (`internal/domain/agent/executor.go`).
3. Create a setup file (e.g., `setup_newcomponent.go`) with the implementation.
4. If the action is a new DAG phase, add the `Phase` constant to `internal/domain/types/run.go` and wire it into the builder.

## Code Style

- Use `zap.Logger` for all logging (no `log.Printf` in production code).
- Use `context.Context` for cancellation propagation.
- Error messages should be lowercase without trailing punctuation.
- Wrap errors with `fmt.Errorf("package: action: %w", err)` for context.
- Keep packages focused: one file per logical concern.

## Pull Requests

- Include tests for new functionality.
- Run `go test ./...` before submitting.
- Update example configs if the RunConfig schema changes.
- Keep commits atomic and messages descriptive.
