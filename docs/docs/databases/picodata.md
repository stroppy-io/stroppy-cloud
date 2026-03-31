---
sidebar_position: 3
---

# Picodata

Stroppy Cloud supports Picodata version 25.3 with three topology presets: single, cluster, and scale.

Picodata is a distributed SQL database based on Tarantool. It natively supports sharding and replication. Stroppy Cloud connects to Picodata via its pgproto-compatible interface, optionally through an HAProxy load balancer.

## Topology: PicodataTopology

```go
type PicodataTopology struct {
    Instances   []MachineSpec     `json:"instances"`
    HAProxy     *MachineSpec      `json:"haproxy,omitempty"`
    Replication int               `json:"replication_factor"`
    Shards      int               `json:"shards"`
    Tiers       []PicodataTier    `json:"tiers,omitempty"`
    Options     map[string]string `json:"options,omitempty"`
}

type PicodataTier struct {
    Name        string `json:"name"`
    Replication int    `json:"replication_factor"`
    CanVote     bool   `json:"can_vote"`
    Count       int    `json:"count"`
}
```

| Field | Type | Description |
|-------|------|-------------|
| `instances` | []MachineSpec | Picodata instance machines. Required. |
| `haproxy` | *MachineSpec | Dedicated HAProxy node(s) for pgproto load balancing. Optional. |
| `replication_factor` | int | Number of data replicas per shard. |
| `shards` | int | Number of shards in the cluster. |
| `tiers` | []PicodataTier | For multi-tier deployments (compute/storage separation). Optional. |
| `options` | map | Additional Picodata configuration parameters. |

## Presets

### single

A single Picodata instance with no replication.

```json
{
  "database": {
    "kind": "picodata",
    "version": "25.3",
    "picodata": {
      "instances": [{"role": "database", "count": 1, "cpus": 2, "memory_mb": 4096, "disk_gb": 50}],
      "replication_factor": 1,
      "shards": 1
    }
  }
}
```

### cluster

A 3-node cluster with replication factor 2, 3 shards, and HAProxy.

```json
{
  "database": {
    "kind": "picodata",
    "version": "25.3",
    "picodata": {
      "instances": [{"role": "database", "count": 3, "cpus": 4, "memory_mb": 8192, "disk_gb": 100}],
      "haproxy": {"role": "proxy", "count": 1, "cpus": 2, "memory_mb": 2048, "disk_gb": 20},
      "replication_factor": 2,
      "shards": 3
    }
  }
}
```

### scale

A 6-node multi-tier deployment with compute and storage tiers.

```json
{
  "database": {
    "kind": "picodata",
    "version": "25.3",
    "picodata": {
      "instances": [{"role": "database", "count": 6, "cpus": 8, "memory_mb": 16384, "disk_gb": 200}],
      "haproxy": {"role": "proxy", "count": 2, "cpus": 2, "memory_mb": 2048, "disk_gb": 20},
      "replication_factor": 3,
      "shards": 6,
      "tiers": [
        {"name": "compute", "replication_factor": 1, "can_vote": true, "count": 3},
        {"name": "storage", "replication_factor": 2, "can_vote": false, "count": 3}
      ]
    }
  }
}
```

The `compute` tier handles query routing and Raft voting. The `storage` tier stores data with higher replication.

## DAG Phases

1. `install_db` -- installs Picodata packages on all instance machines.
2. `configure_db` -- bootstraps the Picodata cluster, configures sharding and replication.
3. `install_proxy` / `configure_proxy` -- if HAProxy is specified, sets up load balancing across Picodata pgproto endpoints.

## Default Packages

Version 25.3:
```json
{
  "apt": ["picodata"],
  "pre_install_apt": [
    "curl -fsSL https://download.picodata.io/tarantool-picodata/picodata.gpg.key | gpg --no-default-keyring --keyring gnupg-ring:/etc/apt/trusted.gpg.d/picodata.gpg --import && chmod 644 /etc/apt/trusted.gpg.d/picodata.gpg",
    "echo \"deb https://download.picodata.io/tarantool-picodata/ubuntu/ $(lsb_release -cs) main\" > /etc/apt/sources.list.d/picodata.list",
    "apt-get update"
  ],
  "rpm": ["picodata"],
  "pre_install_rpm": [
    "sh -c 'cat > /etc/yum.repos.d/picodata.repo << REPO\n[picodata]\nname=Picodata\nbaseurl=https://binary.picodata.io/repository/picodata-rpm/el$releasever\ngpgcheck=0\nenabled=1\nREPO'"
  ]
}
```
