---
sidebar_position: 4
---

# Topologies

Each database kind defines a set of topology presets -- pre-configured cluster layouts with sensible defaults. You can use a preset as-is or provide a fully custom topology in the `RunConfig`.

## MachineSpec

Every topology references `MachineSpec` to define hardware for each role:

```go
type MachineSpec struct {
    Role     MachineRole `json:"role"`
    Count    int         `json:"count"`
    CPUs     int         `json:"cpus"`
    MemoryMB int         `json:"memory_mb"`
    DiskGB   int         `json:"disk_gb"`
}
```

## Machine Roles

| Role | Value | Description |
|------|-------|-------------|
| Database | `database` | Runs the database engine (PG, MySQL, or Picodata) |
| Monitor | `monitor` | Runs the monitoring stack (node_exporter, vmagent, etc.) |
| Stroppy | `stroppy` | Runs the stroppy load test runner |
| Etcd | `etcd` | Dedicated etcd node (or colocated on database nodes) |
| Proxy | `proxy` | HAProxy or ProxySQL load balancer |
| PgBouncer | `pgbouncer` | PgBouncer connection pooler (typically colocated) |

## Preset Summary

### PostgreSQL

| Preset | Nodes | Replication | Proxy | Pooler | Failover |
|--------|-------|-------------|-------|--------|----------|
| `single` | 1 master (2 vCPU, 4 GB) | None | None | None | None |
| `ha` | 1 master + 2 replicas (4 vCPU, 8 GB each) | Streaming, 1 sync | HAProxy x1 | PgBouncer (colocated) | Patroni + etcd |
| `scale` | 1 master + 4 replicas (8 vCPU, 16 GB each) | Streaming, 2 sync | HAProxy x2 | PgBouncer (colocated) | Patroni + etcd |

### MySQL

| Preset | Nodes | Replication | Proxy |
|--------|-------|-------------|-------|
| `single` | 1 primary (2 vCPU, 4 GB) | None | None |
| `replica` | 1 primary + 2 replicas (4 vCPU, 8 GB each) | Semi-synchronous | ProxySQL x1 |
| `group` | 1 primary + 2 secondaries (8 vCPU, 16 GB each) | Group Replication | ProxySQL x2 |

### Picodata

| Preset | Nodes | Shards | Replication Factor | Proxy | Tiers |
|--------|-------|--------|--------------------|-------|-------|
| `single` | 1 instance (2 vCPU, 4 GB) | 1 | 1 | None | None |
| `cluster` | 3 instances (4 vCPU, 8 GB each) | 3 | 2 | HAProxy x1 | None |
| `scale` | 6 instances (8 vCPU, 16 GB each) | 6 | 3 | HAProxy x2 | compute(3) + storage(3) |

## Listing Presets via API

```bash
curl http://localhost:8080/api/v1/presets | jq
```

Response structure:

```json
{
  "postgres": {
    "single": { "master": {...} },
    "ha": { "master": {...}, "replicas": [...], "haproxy": {...}, ... },
    "scale": { ... }
  },
  "mysql": {
    "single": { "primary": {...} },
    "replica": { ... },
    "group": { ... }
  },
  "picodata": {
    "single": { "instances": [...] },
    "cluster": { ... },
    "scale": { ... }
  }
}
```

## Custom Topologies

You are not limited to presets. Pass any valid topology in the `RunConfig`:

```json
{
  "database": {
    "kind": "postgres",
    "version": "16",
    "postgres": {
      "master": {"role": "database", "count": 1, "cpus": 16, "memory_mb": 65536, "disk_gb": 500},
      "replicas": [
        {"role": "database", "count": 3, "cpus": 16, "memory_mb": 65536, "disk_gb": 500}
      ],
      "haproxy": {"role": "proxy", "count": 2, "cpus": 4, "memory_mb": 4096, "disk_gb": 20},
      "pgbouncer": true,
      "patroni": true,
      "etcd": true,
      "sync_replicas": 2,
      "options": {
        "shared_buffers": "16GB",
        "effective_cache_size": "48GB",
        "max_connections": "500"
      }
    }
  }
}
```

The `options` map is passed through to the database configuration. For PostgreSQL, these become `postgresql.conf` parameters. For MySQL, they go into `my.cnf`. For Picodata, they are passed as instance configuration options.

## Conditional DAG Nodes

The topology determines which DAG phases are included:

- **PgBouncer** phases are added only when `postgres.pgbouncer` is `true`.
- **Proxy** phases (HAProxy/ProxySQL) are added only when `haproxy` or `proxysql` is non-nil in the topology.
- **Etcd** phases are added only when `postgres.etcd` is `true`.
- All configure phases for proxy/pooler components depend on `configure_db` completing first.
- `run_stroppy` depends on all configure phases completing.
