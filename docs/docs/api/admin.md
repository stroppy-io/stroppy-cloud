---
sidebar_position: 3
---

# Admin API

The admin API manages server settings, package defaults, and database topology presets. All endpoints are under `/api/v1/admin/` and require API key authentication.

## Endpoints

### GET /api/v1/admin/settings

Returns the current server settings, including cloud provider credentials, monitoring stack versions, package defaults, and stroppy defaults.

**Request:**

```bash
curl http://localhost:8080/api/v1/admin/settings \
  -H "Authorization: Bearer my-secret-key"
```

**Response (200 OK):**

```json
{
  "cloud": {
    "yandex": {
      "folder_id": "",
      "zone": "ru-central1-b",
      "subnet_id": "",
      "service_account_id": "",
      "ssh_public_key": "",
      "image_id": ""
    },
    "server_addr": "",
    "binary_url": ""
  },
  "monitoring": {
    "node_exporter_version": "1.9.1",
    "postgres_exporter_version": "0.16.0",
    "otel_col_version": "0.127.0",
    "vmagent_version": "1.115.0",
    "victoria_metrics_url": "",
    "victoria_metrics_user": "",
    "victoria_metrics_password": ""
  },
  "packages": {
    "postgres": {"16": {...}, "17": {...}},
    "mysql": {"8.0": {...}, "8.4": {...}},
    "picodata": {"25.3": {...}},
    "monitoring": {},
    "stroppy": {}
  },
  "stroppy_defaults": {
    "version": "3.1.0",
    "otlp_exporter_type": "http",
    "otlp_endpoint": "",
    "otlp_url_path": "/insert/multitenant/opentelemetry/v1/metrics",
    "otlp_insecure": false,
    "otlp_headers": "",
    "otlp_metric_prefix": "stroppy_",
    "otlp_service_name": ""
  }
}
```

### PUT /api/v1/admin/settings

Replace the entire server settings. The payload must be a complete `ServerSettings` object.

**Request:**

```bash
curl -X PUT http://localhost:8080/api/v1/admin/settings \
  -H "Content-Type: application/json" \
  -H "Authorization: Bearer my-secret-key" \
  -d '{
    "cloud": {
      "yandex": {
        "folder_id": "b1g...",
        "zone": "ru-central1-b",
        "subnet_id": "e9b...",
        "service_account_id": "aje...",
        "ssh_public_key": "ssh-ed25519 AAAA...",
        "image_id": "fd8..."
      },
      "server_addr": "http://203.0.113.10:8080",
      "binary_url": ""
    },
    "monitoring": {
      "node_exporter_version": "1.9.1",
      "postgres_exporter_version": "0.16.0",
      "otel_col_version": "0.127.0",
      "vmagent_version": "1.115.0",
      "victoria_metrics_url": "https://metrics.example.com",
      "victoria_metrics_user": "admin",
      "victoria_metrics_password": "secret"
    },
    "packages": {...},
    "stroppy_defaults": {
      "version": "3.1.0",
      "otlp_exporter_type": "http",
      "otlp_endpoint": "https://otel.example.com",
      "otlp_url_path": "/insert/multitenant/opentelemetry/v1/metrics",
      "otlp_insecure": false,
      "otlp_headers": "Authorization=Basic dXNlcjpwYXNz",
      "otlp_metric_prefix": "stroppy_",
      "otlp_service_name": "stroppy-cloud"
    }
  }'
```

**Response (200 OK):**

```json
{"status": "ok"}
```

Settings are persisted to disk at the path specified by `--settings-path`. If no path is configured, settings live only in memory and reset on restart.

### GET /api/v1/admin/packages

Returns only the `packages` section of the server settings.

**Request:**

```bash
curl http://localhost:8080/api/v1/admin/packages \
  -H "Authorization: Bearer my-secret-key"
```

**Response (200 OK):**

```json
{
  "postgres": {
    "16": {
      "apt": ["postgresql-16", "postgresql-client-16"],
      "pre_install_apt": ["..."],
      "rpm": ["postgresql16-server", "postgresql16"],
      "pre_install_rpm": ["..."]
    },
    "17": {...}
  },
  "mysql": {"8.0": {...}, "8.4": {...}},
  "picodata": {"25.3": {...}},
  "monitoring": {},
  "stroppy": {}
}
```

### PUT /api/v1/admin/packages

Replace the package defaults.

**Request:**

```bash
curl -X PUT http://localhost:8080/api/v1/admin/packages \
  -H "Content-Type: application/json" \
  -H "Authorization: Bearer my-secret-key" \
  -d '{
    "postgres": {
      "16": {
        "apt": ["my-custom-postgres-16"],
        "custom_repo_apt": "deb https://my-registry.corp/apt jammy main",
        "custom_repo_key": "https://my-registry.corp/key.gpg"
      }
    },
    "mysql": {},
    "picodata": {},
    "monitoring": {},
    "stroppy": {}
  }'
```

**Response (200 OK):**

```json
{"status": "ok"}
```

### GET /api/v1/admin/db-defaults/{kind}

Returns all topology presets for a given database kind.

**Request:**

```bash
curl http://localhost:8080/api/v1/admin/db-defaults/postgres \
  -H "Authorization: Bearer my-secret-key"
```

**Response (200 OK):** Same as the corresponding section from `GET /api/v1/presets`.

Valid `{kind}` values: `postgres`, `mysql`, `picodata`.

### GET /api/v1/admin/db-defaults/{kind}/{version}

Returns topology presets for a specific database kind and version.

**Request:**

```bash
curl http://localhost:8080/api/v1/admin/db-defaults/postgres/16 \
  -H "Authorization: Bearer my-secret-key"
```

**Response (200 OK):**

```json
{
  "kind": "postgres",
  "version": "16",
  "presets": {
    "single": {...},
    "ha": {...},
    "scale": {...}
  }
}
```
