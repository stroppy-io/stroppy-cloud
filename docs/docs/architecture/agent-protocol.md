---
sidebar_position: 3
---

# Agent Protocol

The agent protocol defines the communication between the orchestration server and agent processes running on target machines. It is a simple HTTP-based request-response protocol.

## Agent Lifecycle

1. A machine is provisioned (VM or Docker container) with the agent binary and a cloud-init script.
2. The agent starts an HTTP server on a configured port (default: 9090).
3. The agent registers itself with the orchestration server via `POST /api/agent/register`.
4. The server sends `Command` messages to the agent via `POST /execute` on the agent's HTTP server.
5. The agent executes the command and returns a `Report` synchronously.
6. Optionally, the agent streams log lines back to the server via `POST /api/agent/log`.

## Agent HTTP Endpoints

### GET /health

Returns agent health status.

**Response:**
```json
{
  "status": "ok",
  "machine_id": "db-node-1"
}
```

### POST /execute

Executes a command on the agent machine.

**Request body:**
```json
{
  "id": "cmd-001",
  "action": "install_postgres",
  "config": {
    "version": "16",
    "packages": {
      "apt": ["postgresql-16", "postgresql-client-16"],
      "pre_install_apt": [
        "sh -c 'echo \"deb http://apt.postgresql.org/pub/repos/apt $(lsb_release -cs)-pgdg main\" > /etc/apt/sources.list.d/pgdg.list'",
        "wget --quiet -O - https://www.postgresql.org/media/keys/ACCC4CF8.asc | apt-key add -",
        "apt-get update"
      ]
    }
  }
}
```

**Response (success):**
```json
{
  "command_id": "cmd-001",
  "status": "completed",
  "output": "postgresql-16 installed successfully"
}
```

**Response (failure):**
```json
{
  "command_id": "cmd-001",
  "status": "failed",
  "error": "apt-get install failed: exit code 1",
  "output": "E: Unable to locate package postgresql-99"
}
```

## Data Types

### Command

Sent from the server to an agent:

```go
type Command struct {
    ID     string `json:"id"`
    Action Action `json:"action"`
    Config any    `json:"config"`
}
```

### Action

The `Action` field identifies what the agent should do. All supported actions:

| Action | Description |
|--------|-------------|
| `install_postgres` | Install PostgreSQL packages |
| `config_postgres` | Configure PostgreSQL (pg_hba.conf, postgresql.conf, init cluster) |
| `install_mysql` | Install MySQL packages |
| `config_mysql` | Configure MySQL (my.cnf, replication, group replication) |
| `install_picodata` | Install Picodata packages |
| `config_picodata` | Configure and bootstrap Picodata cluster |
| `install_monitor` | Install monitoring stack (node_exporter, vmagent, otel-collector) |
| `config_monitor` | Configure monitoring targets and scrape configs |
| `install_stroppy` | Install stroppy load testing binary |
| `run_stroppy` | Execute stroppy workload against the database |
| `install_etcd` | Install etcd for Patroni consensus |
| `config_etcd` | Configure etcd cluster membership |
| `install_patroni` | Install Patroni HA manager |
| `config_patroni` | Configure Patroni with etcd backend and PG settings |
| `install_pgbouncer` | Install PgBouncer connection pooler |
| `config_pgbouncer` | Configure PgBouncer pool settings and auth |
| `install_haproxy` | Install HAProxy load balancer |
| `config_haproxy` | Configure HAProxy backends pointing to database nodes |
| `install_proxysql` | Install ProxySQL for MySQL |
| `config_proxysql` | Configure ProxySQL routing rules and backends |
| `shutdown` | Gracefully stop all services on the agent |

### Report

Sent from the agent back to the server:

```go
type Report struct {
    CommandID string       `json:"command_id"`
    Status    ReportStatus `json:"status"`
    Error     string       `json:"error,omitempty"`
    Output    string       `json:"output,omitempty"`
}
```

Report statuses: `running`, `completed`, `failed`.

### LogLine

Streamed from agent to server during command execution:

```go
type LogLine struct {
    CommandID string `json:"command_id"`
    Line      string `json:"line"`
    Stream    string `json:"stream"` // "stdout" or "stderr"
}
```

## Client Interface

The server communicates with agents through the `agent.Client` interface:

```go
type Client interface {
    Send(nc *dag.NodeContext, target Target, cmd Command) error
    SendAll(nc *dag.NodeContext, targets []Target, cmd Command) error
}
```

- `Send` dispatches a command to a single agent and blocks until the report is received.
- `SendAll` dispatches the same command to all targets in parallel and fails on first error.

The production implementation is `HTTPClient`, which posts to `http://{host}:{port}/execute` on each agent.

## Target

A `Target` identifies an agent machine:

```go
type Target struct {
    ID        string
    Host      string
    AgentPort int
}
```

The server maintains a registry of connected agents, populated by agent registration requests.

## Registration Flow

When an agent starts, it POSTs to the server:

```
POST /api/agent/register
{
  "machine_id": "db-node-1",
  "host": "10.10.0.5",
  "port": 9090
}
```

The server adds the agent to its in-memory registry. DAG tasks look up agents by machine role to find the correct targets for each command.
