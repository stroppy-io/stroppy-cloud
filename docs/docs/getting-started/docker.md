---
sidebar_position: 3
---

# Docker Provider

The `docker` provider emulates cloud infrastructure using Docker containers. Each "machine" in the `RunConfig` becomes a Docker container on a shared bridge network. This is the recommended way to develop and test locally.

## How It Works

When `provider` is set to `"docker"`:

1. The **network phase** creates (or reuses) a Docker bridge network named `stroppy-run-net`.
2. The **machines phase** creates one Docker container per machine spec. Each container runs a minimal OS image with the stroppy agent binary injected. The agent registers itself with the orchestration server at `http://host.docker.internal:8080`.
3. All subsequent phases (install, configure, run) work identically to cloud deployments -- the server sends commands to agents over HTTP.
4. The **teardown phase** removes all containers and the network.

## docker-compose.test.yaml

The project includes a compose file for local development:

```yaml
services:
  victoria:
    image: victoriametrics/victoria-metrics:v1.115.0
    ports:
      - "8428:8428"
    command:
      - "-retentionPeriod=30d"
      - "-httpListenAddr=:8428"

  server:
    build:
      context: .
      dockerfile: deployments/docker/stroppy-cloud.Dockerfile
    command:
      - serve
      - --addr=:8080
      - --victoria-url=http://victoria:8428
      - --data-dir=/data/badger
    ports:
      - "8080:8080"
    volumes:
      - /var/run/docker.sock:/var/run/docker.sock
      - /tmp/stroppy-cloud:/opt/stroppy-cloud:ro
    environment:
      LOG_MOD: development
      LOG_LEVEL: debug
```

Key points:

- The Docker socket is mounted so the server can create agent containers.
- The `stroppy-run-net` network is created by compose and shared between the server and agent containers.
- Agent containers use `host.docker.internal:8080` to call back to the server.

## Makefile Targets

```bash
# Start VictoriaMetrics + server
make up-infra

# Stop everything
make down-infra

# Stop and delete volumes
make clean-infra
```

## Limitations

- The Docker provider does not enforce CPU/memory limits from `MachineSpec`. Resource fields are used for documentation and DAG construction but not enforced at the container level.
- Disk size (`disk_gb`) is ignored; containers use the host filesystem.
- Networking uses a flat Docker bridge network rather than isolated subnets.
