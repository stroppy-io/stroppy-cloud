---
sidebar_position: 2
---

# Server Settings

Server settings are managed via the admin API and optionally persisted to a JSON file on disk. They control cloud provider credentials, monitoring stack versions, default packages, and stroppy OTEL export configuration.

Defined in `internal/domain/types/settings.go`.

## Schema

```go
type ServerSettings struct {
    Cloud           CloudSettings   `json:"cloud"`
    Monitoring      MonitoringStack `json:"monitoring"`
    Packages        PackageDefaults `json:"packages"`
    StroppyDefaults StroppySettings `json:"stroppy_defaults"`
}
```

## Cloud Settings

```go
type CloudSettings struct {
    Yandex     YandexCloudSettings `json:"yandex"`
    ServerAddr string              `json:"server_addr"`
    BinaryURL  string              `json:"binary_url"`
}

type YandexCloudSettings struct {
    FolderID         string `json:"folder_id"`
    Zone             string `json:"zone"`
    SubnetID         string `json:"subnet_id"`
    ServiceAccountID string `json:"service_account_id"`
    SSHPublicKey     string `json:"ssh_public_key"`
    ImageID          string `json:"image_id"`
}
```

| Field | Description |
|-------|-------------|
| `yandex.folder_id` | Yandex Cloud folder ID for resource creation |
| `yandex.zone` | Availability zone (default: `ru-central1-b`) |
| `yandex.subnet_id` | Subnet ID for VM network interfaces |
| `yandex.service_account_id` | Service account for API calls |
| `yandex.ssh_public_key` | SSH public key injected into VMs |
| `yandex.image_id` | VM boot image ID |
| `server_addr` | External address of the orchestration server (used in cloud-init for agent callbacks) |
| `binary_url` | Override URL for agent binary download (defaults to the server's `/agent/binary` endpoint) |

## Monitoring Stack

```go
type MonitoringStack struct {
    NodeExporterVersion     string `json:"node_exporter_version"`
    PostgresExporterVersion string `json:"postgres_exporter_version"`
    OtelColVersion          string `json:"otel_col_version"`
    VmagentVersion          string `json:"vmagent_version"`
    VictoriaMetricsURL      string `json:"victoria_metrics_url"`
    VictoriaMetricsUser     string `json:"victoria_metrics_user"`
    VictoriaMetricsPassword string `json:"victoria_metrics_password"`
}
```

| Field | Default | Description |
|-------|---------|-------------|
| `node_exporter_version` | `1.9.1` | Version of Prometheus node_exporter |
| `postgres_exporter_version` | `0.16.0` | Version of Prometheus postgres_exporter |
| `otel_col_version` | `0.127.0` | Version of OpenTelemetry Collector |
| `vmagent_version` | `1.115.0` | Version of VictoriaMetrics vmagent |
| `victoria_metrics_url` | (empty) | VictoriaMetrics remote write URL |
| `victoria_metrics_user` | (empty) | Basic auth username for VM |
| `victoria_metrics_password` | (empty) | Basic auth password for VM |

## Stroppy Defaults

```go
type StroppySettings struct {
    Version          string `json:"version"`
    OTLPExporterType string `json:"otlp_exporter_type"`
    OTLPEndpoint     string `json:"otlp_endpoint"`
    OTLPURLPath      string `json:"otlp_url_path"`
    OTLPInsecure     bool   `json:"otlp_insecure"`
    OTLPHeaders      string `json:"otlp_headers"`
    OTLPMetricPrefix string `json:"otlp_metric_prefix"`
    OTLPServiceName  string `json:"otlp_service_name"`
}
```

These settings are converted to environment variables when running stroppy on agent machines:

| Setting | Environment Variable | Default |
|---------|---------------------|---------|
| `version` | (binary version to install) | `3.1.0` |
| `otlp_exporter_type` | `K6_OTEL_EXPORTER_TYPE` | `http` |
| `otlp_endpoint` | `K6_OTEL_HTTP_EXPORTER_ENDPOINT` | (empty) |
| `otlp_url_path` | `K6_OTEL_HTTP_EXPORTER_URL_PATH` | `/insert/multitenant/opentelemetry/v1/metrics` |
| `otlp_insecure` | `K6_OTEL_HTTP_EXPORTER_INSECURE` | `false` |
| `otlp_headers` | `K6_OTEL_HEADERS` | (empty) |
| `otlp_metric_prefix` | `K6_OTEL_METRIC_PREFIX` | `stroppy_` |
| `otlp_service_name` | `K6_OTEL_SERVICE_NAME` | (empty) |

The run ID is automatically injected as an OTEL resource attribute:

```
K6_OTEL_RESOURCE_ATTRIBUTES=stroppy_run_id=<run_id>
```

## Defaults

When no settings have been configured, the server uses these defaults:

```json
{
  "cloud": {
    "yandex": {"zone": "ru-central1-b"}
  },
  "monitoring": {
    "node_exporter_version": "1.9.1",
    "postgres_exporter_version": "0.16.0",
    "otel_col_version": "0.127.0",
    "vmagent_version": "1.115.0"
  },
  "stroppy_defaults": {
    "version": "3.1.0",
    "otlp_exporter_type": "http",
    "otlp_url_path": "/insert/multitenant/opentelemetry/v1/metrics",
    "otlp_insecure": false,
    "otlp_metric_prefix": "stroppy_"
  }
}
```

## Persistence

Settings are persisted to disk when `--settings-path` is specified:

```bash
./bin/cli serve --settings-path=/etc/stroppy/settings.json
```

The file is read on startup and written on every `PUT /api/v1/admin/settings` or `PUT /api/v1/admin/packages` call. If the file does not exist on startup, defaults are used.
