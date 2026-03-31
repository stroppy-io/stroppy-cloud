---
sidebar_position: 2
---

# MySQL

Stroppy Cloud supports MySQL versions 8.0 and 8.4 with three topology presets: single, replica, and group.

## Topology: MySQLTopology

```go
type MySQLTopology struct {
    Primary   MachineSpec       `json:"primary"`
    Replicas  []MachineSpec     `json:"replicas,omitempty"`
    ProxySQL  *MachineSpec      `json:"proxysql,omitempty"`
    GroupRepl bool              `json:"group_replication"`
    SemiSync  bool              `json:"semi_sync"`
    Options   map[string]string `json:"options,omitempty"`
}
```

| Field | Type | Description |
|-------|------|-------------|
| `primary` | MachineSpec | The primary MySQL node. Required. |
| `replicas` | []MachineSpec | Replication secondaries. Optional. |
| `proxysql` | *MachineSpec | Dedicated ProxySQL node(s). Optional. |
| `group_replication` | bool | Enable MySQL Group Replication (multi-primary or single-primary). |
| `semi_sync` | bool | Enable semi-synchronous replication. Used when `group_replication` is false. |
| `options` | map | Additional MySQL configuration parameters for my.cnf. |

## Presets

### single

A standalone MySQL instance.

```json
{
  "database": {
    "kind": "mysql",
    "version": "8.0",
    "mysql": {
      "primary": {"role": "database", "count": 1, "cpus": 2, "memory_mb": 4096, "disk_gb": 50}
    }
  }
}
```

### replica

Primary with 2 semi-synchronous replicas and a ProxySQL node.

```json
{
  "database": {
    "kind": "mysql",
    "version": "8.0",
    "mysql": {
      "primary": {"role": "database", "count": 1, "cpus": 4, "memory_mb": 8192, "disk_gb": 100},
      "replicas": [{"role": "database", "count": 2, "cpus": 4, "memory_mb": 8192, "disk_gb": 100}],
      "proxysql": {"role": "proxy", "count": 1, "cpus": 2, "memory_mb": 2048, "disk_gb": 20},
      "semi_sync": true
    }
  }
}
```

### group

MySQL Group Replication with 3 nodes and 2 ProxySQL instances.

```json
{
  "database": {
    "kind": "mysql",
    "version": "8.0",
    "mysql": {
      "primary": {"role": "database", "count": 1, "cpus": 8, "memory_mb": 16384, "disk_gb": 200},
      "replicas": [{"role": "database", "count": 2, "cpus": 8, "memory_mb": 16384, "disk_gb": 200}],
      "proxysql": {"role": "proxy", "count": 2, "cpus": 2, "memory_mb": 2048, "disk_gb": 20},
      "group_replication": true
    }
  }
}
```

## Full Example

From `examples/run-mysql-group.json`:

```json
{
  "id": "test-mysql-gr-001",
  "provider": "docker",
  "network": {"cidr": "10.10.0.0/24"},
  "machines": [
    {"role": "database", "count": 3, "cpus": 4, "memory_mb": 8192, "disk_gb": 100},
    {"role": "proxy",    "count": 2, "cpus": 2, "memory_mb": 2048, "disk_gb": 20},
    {"role": "monitor",  "count": 1, "cpus": 1, "memory_mb": 2048, "disk_gb": 20},
    {"role": "stroppy",  "count": 1, "cpus": 2, "memory_mb": 4096, "disk_gb": 20}
  ],
  "database": {
    "kind": "mysql",
    "version": "8.0",
    "mysql": {
      "primary": {"role": "database", "count": 1, "cpus": 4, "memory_mb": 8192, "disk_gb": 100},
      "replicas": [{"role": "database", "count": 2, "cpus": 4, "memory_mb": 8192, "disk_gb": 100}],
      "proxysql": {"role": "proxy", "count": 2, "cpus": 2, "memory_mb": 2048, "disk_gb": 20},
      "group_replication": true
    }
  },
  "monitor": {},
  "stroppy": {"version": "3.1.0", "workload": "simple", "duration": "10s", "workers": 4}
}
```

## DAG Phases

1. `install_db` -- installs MySQL packages on all database machines.
2. `configure_db` -- configures my.cnf, sets up replication or group replication.
3. `install_proxy` / `configure_proxy` -- if ProxySQL is specified, installs and configures ProxySQL routing rules.

## Default Packages

Version 8.0:
```json
{
  "apt": ["mysql-server-8.0", "mysql-client"],
  "rpm": ["mysql-community-server", "mysql-community-client"]
}
```

Version 8.4:
```json
{
  "apt": ["mysql-server-8.4", "mysql-client"],
  "rpm": ["mysql-community-server", "mysql-community-client"]
}
```
