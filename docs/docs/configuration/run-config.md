---
sidebar_position: 1
---

# RunConfig

The `RunConfig` is the primary input to Stroppy Cloud. It is a JSON document that fully describes a test run: infrastructure provider, machines, database topology, monitoring, and load test settings. The server uses it to build the execution DAG.

Defined in `internal/domain/types/run.go`.

## Schema

```go
type RunConfig struct {
    ID       string         `json:"id"`
    Provider Provider       `json:"provider"`
    Network  NetworkConfig  `json:"network"`
    Machines []MachineSpec  `json:"machines"`
    Database DatabaseConfig `json:"database"`
    Monitor  MonitorConfig  `json:"monitor"`
    Stroppy  StroppyConfig  `json:"stroppy"`
    Packages *PackageSet    `json:"packages,omitempty"`
}
```

## Fields

### id

**Type:** string (required)

A unique identifier for this run. Used as the key in storage, in WebSocket messages, and in metrics correlation.

```json
{"id": "pg-ha-benchmark-2026-03-31"}
```

### provider

**Type:** string (required)

The infrastructure provider. Determines how machines are created.

| Value | Description |
|-------|-------------|
| `yandex` | Provision VMs on Yandex Cloud |
| `docker` | Create Docker containers locally |

```json
{"provider": "docker"}
```

### network

**Type:** NetworkConfig (required)

Network configuration for the run.

```go
type NetworkConfig struct {
    CIDR string `json:"cidr"`
    Zone string `json:"zone,omitempty"`
}
```

| Field | Type | Description |
|-------|------|-------------|
| `cidr` | string | Subnet CIDR for the run (e.g., `10.10.0.0/24`) |
| `zone` | string | Cloud availability zone (optional, used by Yandex provider) |

```json
{"network": {"cidr": "10.10.0.0/24", "zone": "ru-central1-b"}}
```

### machines

**Type:** []MachineSpec (required)

List of machines to provision. Each entry specifies a role, count, and resource allocation.

```go
type MachineSpec struct {
    Role     MachineRole `json:"role"`
    Count    int         `json:"count"`
    CPUs     int         `json:"cpus"`
    MemoryMB int         `json:"memory_mb"`
    DiskGB   int         `json:"disk_gb"`
}
```

Machine roles: `database`, `monitor`, `stroppy`, `etcd`, `proxy`, `pgbouncer`.

```json
{
  "machines": [
    {"role": "database", "count": 3, "cpus": 4, "memory_mb": 8192, "disk_gb": 100},
    {"role": "proxy",    "count": 1, "cpus": 2, "memory_mb": 2048, "disk_gb": 20},
    {"role": "monitor",  "count": 1, "cpus": 1, "memory_mb": 2048, "disk_gb": 20},
    {"role": "stroppy",  "count": 1, "cpus": 2, "memory_mb": 4096, "disk_gb": 20}
  ]
}
```

### database

**Type:** DatabaseConfig (required)

Database specification. Exactly one topology field must be set, matching `kind`.

```go
type DatabaseConfig struct {
    Kind     DatabaseKind      `json:"kind"`
    Version  string            `json:"version"`
    Postgres *PostgresTopology `json:"postgres,omitempty"`
    MySQL    *MySQLTopology    `json:"mysql,omitempty"`
    Picodata *PicodataTopology `json:"picodata,omitempty"`
}
```

| Field | Type | Description |
|-------|------|-------------|
| `kind` | string | Database kind: `postgres`, `mysql`, or `picodata` |
| `version` | string | Database version (e.g., `16`, `8.0`, `25.3`) |
| `postgres` | PostgresTopology | Postgres topology (required when kind is `postgres`) |
| `mysql` | MySQLTopology | MySQL topology (required when kind is `mysql`) |
| `picodata` | PicodataTopology | Picodata topology (required when kind is `picodata`) |

See the [Databases](/docs/databases/postgres) section for topology details.

### monitor

**Type:** MonitorConfig (required, may be empty `{}`)

Monitoring export targets.

```go
type MonitorConfig struct {
    MetricsEndpoint string `json:"metrics_endpoint,omitempty"`
    LogsEndpoint    string `json:"logs_endpoint,omitempty"`
}
```

| Field | Type | Description |
|-------|------|-------------|
| `metrics_endpoint` | string | Prometheus remote_write URL (e.g., `http://victoria:8428/api/v1/write`) |
| `logs_endpoint` | string | Loki push URL for log aggregation |

```json
{"monitor": {"metrics_endpoint": "http://victoria:8428/api/v1/write"}}
```

Monitoring is always deployed regardless of whether endpoints are specified. When endpoints are empty, agents collect metrics locally but do not forward them.

### stroppy

**Type:** StroppyConfig (required)

Stroppy test runner settings.

```go
type StroppyConfig struct {
    Version  string            `json:"version"`
    Workload string            `json:"workload"`
    Duration string            `json:"duration"`
    Workers  int               `json:"workers"`
    Options  map[string]string `json:"options,omitempty"`
}
```

| Field | Type | Description |
|-------|------|-------------|
| `version` | string | Stroppy binary version to install |
| `workload` | string | Workload name (e.g., `tpcb`, `simple`) |
| `duration` | string | Test duration (Go duration format: `30s`, `5m`, `1h`) |
| `workers` | int | Number of concurrent virtual users |
| `options` | map | Additional stroppy CLI options |

```json
{
  "stroppy": {
    "version": "3.1.0",
    "workload": "tpcb",
    "duration": "5m",
    "workers": 16,
    "options": {"scale": "100"}
  }
}
```

### packages

**Type:** *PackageSet (optional)

Override the default packages for this run. When nil, the server uses `DefaultPackages()` or the admin-configured defaults. See the [Packages](/docs/configuration/packages) page for details.

## Complete Example

From `examples/run-postgres-ha.json`:

```json
{
  "id": "test-pg-ha-001",
  "provider": "docker",
  "network": {"cidr": "10.10.0.0/24"},
  "machines": [
    {"role": "database", "count": 3, "cpus": 4, "memory_mb": 8192, "disk_gb": 100},
    {"role": "proxy",    "count": 1, "cpus": 2, "memory_mb": 2048, "disk_gb": 20},
    {"role": "monitor",  "count": 1, "cpus": 1, "memory_mb": 2048, "disk_gb": 20},
    {"role": "stroppy",  "count": 1, "cpus": 2, "memory_mb": 4096, "disk_gb": 20}
  ],
  "database": {
    "kind": "postgres",
    "version": "16",
    "postgres": {
      "master": {"role": "database", "count": 1, "cpus": 4, "memory_mb": 8192, "disk_gb": 100},
      "replicas": [{"role": "database", "count": 2, "cpus": 4, "memory_mb": 8192, "disk_gb": 100}],
      "haproxy": {"role": "proxy", "count": 1, "cpus": 2, "memory_mb": 2048, "disk_gb": 20},
      "pgbouncer": true,
      "patroni": true,
      "etcd": true,
      "sync_replicas": 1
    }
  },
  "monitor": {},
  "stroppy": {
    "version": "3.1.0",
    "workload": "tpcb",
    "duration": "30s",
    "workers": 4
  }
}
```
