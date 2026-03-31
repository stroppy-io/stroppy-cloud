---
sidebar_position: 4
---

# Cloud-Init

When using the `yandex` provider, Stroppy Cloud generates a cloud-init userdata script for each provisioned VM. This script bootstraps the stroppy agent, which then communicates with the orchestration server to receive and execute commands.

Defined in `internal/domain/agent/cloudinit.go`.

## Parameters

```go
type CloudInitParams struct {
    BinaryURL  string
    ServerAddr string
    AgentPort  int
    MachineID  string
    ExtraEnv   map[string]string
}
```

| Parameter | Description |
|-----------|-------------|
| `BinaryURL` | URL to download the agent binary (typically `http://<server>/agent/binary` or an S3 presigned URL) |
| `ServerAddr` | Callback address for the agent to register with the server (e.g., `http://203.0.113.10:8080`) |
| `AgentPort` | Port the agent listens on (default: 9090) |
| `MachineID` | Unique identifier for this machine, used in agent registration |
| `ExtraEnv` | Additional environment variables passed to the agent process |

## Generated Script

The cloud-init template produces a `#cloud-config` YAML that:

1. Creates a `stroppy` user with passwordless sudo.
2. Writes an environment file at `/etc/stroppy/agent.env`:

```bash
STROPPY_SERVER_ADDR=http://203.0.113.10:8080
STROPPY_AGENT_PORT=9090
STROPPY_MACHINE_ID=db-node-1
```

3. Creates a systemd service unit at `/etc/systemd/system/stroppy-agent.service`:

```ini
[Unit]
Description=Stroppy Agent
After=network-online.target
Wants=network-online.target

[Service]
Type=simple
EnvironmentFile=/etc/stroppy/agent.env
ExecStart=/usr/local/bin/stroppy-agent agent
Restart=on-failure
RestartSec=5

[Install]
WantedBy=multi-user.target
```

4. Runs commands to download the binary, set permissions, and start the service:

```bash
mkdir -p /etc/stroppy
curl -fsSL -o /usr/local/bin/stroppy-agent "<BinaryURL>"
chmod +x /usr/local/bin/stroppy-agent
systemctl daemon-reload
systemctl enable --now stroppy-agent
```

## Agent Registration

Once the agent starts, it:

1. Resolves the local hostname.
2. POSTs a registration request to `{ServerAddr}/api/agent/register` with its machine ID, host, and port.
3. Begins listening for commands on `POST /execute`.

The orchestration server stores the agent in its in-memory registry, making it available for DAG task execution.

## Binary URL Resolution

The `BinaryURL` is determined as follows:

1. If `ServerSettings.Cloud.BinaryURL` is set, use that (allows pointing to an S3 bucket or CDN).
2. Otherwise, default to `{ServerAddr}/agent/binary`, which serves the server's own binary (since the server and agent are compiled into the same binary).

## Docker Provider

When using the `docker` provider, cloud-init is not used. Instead, the `DockerDeployer` creates containers with the agent binary injected via a volume mount. The agent process is started directly as the container's entrypoint, and it registers with the server at `http://host.docker.internal:8080`.
