---
sidebar_position: 1
slug: /intro
---

# Stroppy Cloud

Stroppy Cloud is a database testing orchestrator that automates the provisioning, configuration, and load-testing of database clusters. It supports PostgreSQL, MySQL, and Picodata across multiple topology presets -- from single-node development setups to multi-node high-availability deployments with connection pooling, proxying, and monitoring.

## What It Does

1. **Provisions infrastructure** -- VMs on Yandex Cloud or Docker containers for local testing.
2. **Deploys database clusters** -- installs and configures the database engine, replication, connection poolers (PgBouncer, ProxySQL), load balancers (HAProxy), and consensus stores (etcd/Patroni).
3. **Deploys monitoring** -- node_exporter, postgres_exporter, vmagent, and OpenTelemetry Collector on every machine, shipping metrics to VictoriaMetrics.
4. **Runs load tests** -- deploys [stroppy](https://github.com/stroppy-io/stroppy) (a k6-based test runner) against the cluster and streams results via OpenTelemetry.
5. **Tears down** -- cleans up all provisioned resources after the test completes.

All of these steps are expressed as a **directed acyclic graph (DAG)** that is built from a single `RunConfig` JSON document. Independent branches of the DAG execute in parallel; the entire execution is resumable if the server restarts.

## Key Concepts

| Concept | Description |
|---------|-------------|
| **RunConfig** | A JSON document describing the full test run: provider, machines, database topology, monitoring, and stroppy settings. |
| **DAG** | The execution plan. Each node is a phase (e.g., `install_db`, `configure_db`, `run_stroppy`). Nodes declare dependencies; the executor runs independent nodes concurrently. |
| **Agent** | A lightweight HTTP server deployed on each provisioned machine. The orchestrator sends `Command` messages to agents; agents report back with `Report` messages. |
| **Phase** | A named step in the run lifecycle. Examples: `network`, `machines`, `install_db`, `configure_db`, `install_monitor`, `run_stroppy`, `teardown`. |
| **Topology** | The cluster layout for a database -- number of nodes, replication mode, proxy layer, connection pooling. Each database kind has named presets (`single`, `ha`, `scale`, etc.). |

## Supported Databases

- **PostgreSQL** (versions 16, 17) -- single, HA (Patroni + etcd + PgBouncer + HAProxy), scale
- **MySQL** (versions 8.0, 8.4) -- single, replica (semi-sync + ProxySQL), group replication
- **Picodata** (version 25.3) -- single, cluster, scale (multi-tier)

## Project Layout

```
cmd/cli/            CLI entrypoint (serve, agent subcommands)
internal/
  core/dag/         DAG graph, executor, storage, node context
  core/logger/      Structured logging (zap)
  core/shutdown/    Graceful shutdown
  domain/api/       HTTP server, routes, WebSocket hub, admin handlers
  domain/agent/     Agent server, protocol, cloud-init, deployer, setup scripts
  domain/run/       DAG builder, per-database task implementations
  domain/types/     RunConfig, ServerSettings, PackageSet, topology types
  domain/metrics/   VictoriaMetrics collector and comparison
  infrastructure/   Badger storage, Victoria client
examples/           Sample RunConfig JSON files
deployments/        Dockerfiles and compose files
```
