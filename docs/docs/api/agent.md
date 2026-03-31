---
sidebar_position: 2
---

# Agent API

The agent API is used by agent processes running on target machines to communicate with the orchestration server. These endpoints are under `/api/agent/` and are exempt from API key authentication.

## Endpoints

### POST /api/agent/register

Called by each agent on startup to announce its presence.

**Request:**

```bash
curl -X POST http://server:8080/api/agent/register \
  -H "Content-Type: application/json" \
  -d '{
    "machine_id": "db-node-1",
    "host": "10.10.0.5",
    "port": 9090
  }'
```

**Request body:**

| Field | Type | Description |
|-------|------|-------------|
| `machine_id` | string | Unique identifier for this machine (assigned during provisioning) |
| `host` | string | IP address or hostname where the agent is reachable |
| `port` | int | Port the agent HTTP server is listening on |

**Response (200 OK):**

```json
{"status": "ok"}
```

After registration, the server adds the agent to its in-memory registry. DAG tasks look up agents by machine ID to dispatch commands.

### POST /api/agent/report

Called by agents to report the outcome of a command execution. In the current implementation, reports are delivered synchronously as HTTP responses to `/execute` calls. This endpoint is used for asynchronous reporting and is broadcast to WebSocket clients.

**Request:**

```bash
curl -X POST http://server:8080/api/agent/report \
  -H "Content-Type: application/json" \
  -d '{
    "command_id": "cmd-install-pg-001",
    "status": "completed",
    "output": "postgresql-16 installed"
  }'
```

**Request body:**

| Field | Type | Description |
|-------|------|-------------|
| `command_id` | string | ID of the command this report is for |
| `status` | string | One of: `running`, `completed`, `failed` |
| `error` | string | Error message (when status is `failed`) |
| `output` | string | Command output (stdout) |

**Response (200 OK):**

```json
{"status": "ok"}
```

Reports are broadcast to all connected WebSocket clients as messages with `type: "report"`.

### POST /api/agent/log

Stream a single log line from agent to server during command execution.

**Request:**

```bash
curl -X POST http://server:8080/api/agent/log \
  -H "Content-Type: application/json" \
  -d '{
    "command_id": "cmd-install-pg-001",
    "line": "Reading package lists...",
    "stream": "stdout"
  }'
```

**Request body:**

| Field | Type | Description |
|-------|------|-------------|
| `command_id` | string | ID of the command producing this log |
| `line` | string | The log line content |
| `stream` | string | Either `stdout` or `stderr` |

**Response:** 204 No Content

Log lines are broadcast to WebSocket clients as messages with `type: "agent_log"`.

## Agent Binary Download

### GET /agent/binary

Download the agent binary. Used by cloud-init scripts to bootstrap agents on newly provisioned VMs. No authentication required.

**Request:**

```bash
curl -o stroppy-agent http://server:8080/agent/binary
chmod +x stroppy-agent
```

**Response:** Binary file with `Content-Type: application/octet-stream`.

## Package Upload and Serving

### POST /api/v1/upload/deb

Upload a `.deb` package file. The file is stored on the server and made available at `/packages/{filename}` for agents to download.

**Request:**

```bash
curl -X POST http://localhost:8080/api/v1/upload/deb \
  -H "Authorization: Bearer my-secret-key" \
  -F "file=@my-custom-package.deb"
```

**Response (200 OK):**

```json
{
  "filename": "my-custom-package.deb",
  "url": "http://localhost:8080/packages/my-custom-package.deb",
  "size": "15728640"
}
```

Only `.deb` files are accepted. Maximum upload size is 500 MB.

### GET /packages/{filename}

Serve uploaded package files. Used by agents to download custom packages during installation phases.

```bash
curl -o package.deb http://server:8080/packages/my-custom-package.deb
```
