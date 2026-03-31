---
sidebar_position: 1
---

# PostgreSQL

Stroppy Cloud supports PostgreSQL versions 16 and 17 with three topology presets: single, HA, and scale.

## Topology: PostgresTopology

```go
type PostgresTopology struct {
    Master       MachineSpec       `json:"master"`
    Replicas     []MachineSpec     `json:"replicas,omitempty"`
    HAProxy      *MachineSpec      `json:"haproxy,omitempty"`
    PgBouncer    bool              `json:"pgbouncer"`
    Patroni      bool              `json:"patroni"`
    Etcd         bool              `json:"etcd"`
    SyncReplicas int               `json:"sync_replicas"`
    Options      map[string]string `json:"options,omitempty"`
}
```

| Field | Type | Description |
|-------|------|-------------|
| `master` | MachineSpec | The primary PostgreSQL node. Required. |
| `replicas` | []MachineSpec | Streaming replication replicas. Optional. |
| `haproxy` | *MachineSpec | Dedicated HAProxy node(s) for load balancing. Optional. |
| `pgbouncer` | bool | Whether to deploy PgBouncer colocated on each PG node. |
| `patroni` | bool | Whether to use Patroni for automatic failover. |
| `etcd` | bool | Whether to deploy etcd (colocated on PG nodes, up to 3). Required when `patroni` is true. |
| `sync_replicas` | int | Number of synchronous replicas. 0 = fully asynchronous. |
| `options` | map | Additional PostgreSQL configuration parameters passed to postgresql.conf. |

## Presets

### single

A standalone PostgreSQL instance with no replication.

```json
{
  "database": {
    "kind": "postgres",
    "version": "16",
    "postgres": {
      "master": {"role": "database", "count": 1, "cpus": 2, "memory_mb": 4096, "disk_gb": 50}
    }
  }
}
```

Resources: 1 node, 2 vCPU, 4 GB RAM, 50 GB disk.

### ha

A 3-node cluster with Patroni, etcd, PgBouncer, and HAProxy.

```json
{
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
  }
}
```

Resources: 3 database nodes (4 vCPU, 8 GB each) + 1 HAProxy node. PgBouncer runs colocated on each PG node. Etcd runs colocated on the 3 PG nodes. 1 synchronous replica ensures zero data loss on failover.

### scale

A 5-node cluster designed for throughput testing.

```json
{
  "database": {
    "kind": "postgres",
    "version": "17",
    "postgres": {
      "master": {"role": "database", "count": 1, "cpus": 8, "memory_mb": 16384, "disk_gb": 200},
      "replicas": [{"role": "database", "count": 4, "cpus": 8, "memory_mb": 16384, "disk_gb": 200}],
      "haproxy": {"role": "proxy", "count": 2, "cpus": 2, "memory_mb": 2048, "disk_gb": 20},
      "pgbouncer": true,
      "patroni": true,
      "etcd": true,
      "sync_replicas": 2
    }
  }
}
```

Resources: 5 database nodes (8 vCPU, 16 GB each) + 2 HAProxy nodes. 2 synchronous replicas.

## DAG Phases

For a single-node setup, the Postgres-related phases are:

1. `install_db` -- installs PostgreSQL packages via `apt` or `rpm` on the database machine(s).
2. `configure_db` -- initializes the cluster, configures `postgresql.conf` and `pg_hba.conf`.

For HA setups, additional phases are added:

3. `install_etcd` / `configure_etcd` -- etcd cluster for Patroni consensus.
4. `install_pgbouncer` / `configure_pgbouncer` -- connection pooling on each PG node.
5. `install_proxy` / `configure_proxy` -- HAProxy pointing to database backends.

All `configure_*` phases for proxy components depend on `configure_db` completing first (they need the running database endpoints).

## Default Packages

Version 16:
```json
{
  "apt": ["postgresql-16", "postgresql-client-16"],
  "pre_install_apt": [
    "sh -c 'echo \"deb http://apt.postgresql.org/pub/repos/apt $(lsb_release -cs)-pgdg main\" > /etc/apt/sources.list.d/pgdg.list'",
    "wget --quiet -O - https://www.postgresql.org/media/keys/ACCC4CF8.asc | apt-key add -",
    "apt-get update"
  ],
  "rpm": ["postgresql16-server", "postgresql16"],
  "pre_install_rpm": [
    "dnf install -y https://download.postgresql.org/pub/repos/yum/reporpms/EL-$(rpm -E %rhel)-x86_64/pgdg-redhat-repo-latest.noarch.rpm"
  ]
}
```

These defaults can be overridden per-run via the `packages` field in `RunConfig` or globally via the admin API.
