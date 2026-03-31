---
sidebar_position: 5
---

# Metrics API

The metrics API provides access to collected performance data from VictoriaMetrics. These endpoints require a VictoriaMetrics URL to be configured (via `--victoria-url` on the server or the admin settings).

## Endpoints

### GET /api/v1/run/{runID}/metrics

Retrieve collected metrics for a specific run within a time range.

**Query parameters:**

| Parameter | Type | Required | Description |
|-----------|------|----------|-------------|
| `start` | string | Yes | Start time in RFC3339 format |
| `end` | string | Yes | End time in RFC3339 format |

**Request:**

```bash
curl "http://localhost:8080/api/v1/run/run-001/metrics?start=2026-03-31T12:00:00Z&end=2026-03-31T12:30:00Z" \
  -H "Authorization: Bearer my-secret-key"
```

**Response (200 OK):**

```json
{
  "cpu_usage": { "avg": 45.2, "max": 92.1, "min": 5.3 },
  "memory_usage": { "avg": 62.8, "max": 78.4, "min": 51.2 },
  "disk_io": { "read_mbps": 120.5, "write_mbps": 85.3 },
  "network": { "rx_mbps": 42.1, "tx_mbps": 38.7 },
  "db_qps": { "avg": 15230, "max": 18500, "min": 12100 },
  "db_latency_p99": { "avg_ms": 12.5 }
}
```

**Error (503 Service Unavailable):**

```
metrics not configured (no VictoriaMetrics URL)
```

**Error (400 Bad Request):**

```
query params 'start' and 'end' (RFC3339) are required
```

### GET /api/v1/compare

Compare metrics between two runs. Useful for regression testing -- checking whether a configuration change improved or degraded performance.

**Query parameters:**

| Parameter | Type | Required | Description |
|-----------|------|----------|-------------|
| `a` | string | Yes | First run ID |
| `b` | string | Yes | Second run ID |
| `start` | string | Yes | Start time in RFC3339 format |
| `end` | string | Yes | End time in RFC3339 format |

**Request:**

```bash
curl "http://localhost:8080/api/v1/compare?a=run-001&b=run-002&start=2026-03-31T12:00:00Z&end=2026-03-31T12:30:00Z" \
  -H "Authorization: Bearer my-secret-key"
```

**Response (200 OK):**

```json
{
  "metrics": {
    "cpu_usage": {
      "run_a": 45.2,
      "run_b": 52.8,
      "diff_pct": 16.8,
      "verdict": "regression"
    },
    "db_qps": {
      "run_a": 15230,
      "run_b": 18500,
      "diff_pct": 21.5,
      "verdict": "improvement"
    },
    "db_latency_p99": {
      "run_a": 12.5,
      "run_b": 11.2,
      "diff_pct": -10.4,
      "verdict": "improvement"
    }
  }
}
```

The comparison uses a 5% threshold: differences within 5% are considered equivalent ("same"). Above 5%, the verdict is either "improvement" or "regression" depending on the metric type.

## Metrics Pipeline

The metrics pipeline works as follows:

1. **Monitoring agents** (node_exporter, postgres_exporter, vmagent) are deployed on each machine during the `install_monitor` / `configure_monitor` DAG phases.
2. **vmagent** scrapes local exporters and remote-writes to VictoriaMetrics.
3. **Stroppy** exports test metrics via OpenTelemetry (K6 OTEL output) to VictoriaMetrics.
4. The **metrics collector** (`domain/metrics/`) queries VictoriaMetrics for aggregated results.

## Stroppy OTEL Configuration

Stroppy metrics are tagged with the run ID via OTEL resource attributes:

```
K6_OTEL_RESOURCE_ATTRIBUTES=stroppy_run_id=run-001
```

The OTEL exporter configuration is derived from the `stroppy_defaults` in server settings:

| Setting | Environment Variable | Description |
|---------|---------------------|-------------|
| `otlp_exporter_type` | `K6_OTEL_EXPORTER_TYPE` | `http` or `grpc` |
| `otlp_endpoint` | `K6_OTEL_HTTP_EXPORTER_ENDPOINT` | OTEL collector endpoint URL |
| `otlp_url_path` | `K6_OTEL_HTTP_EXPORTER_URL_PATH` | Path for HTTP exporter |
| `otlp_insecure` | `K6_OTEL_HTTP_EXPORTER_INSECURE` | Whether to use TLS |
| `otlp_headers` | `K6_OTEL_HEADERS` | Extra headers (e.g., auth) |
| `otlp_metric_prefix` | `K6_OTEL_METRIC_PREFIX` | Prefix for all metric names |
| `otlp_service_name` | `K6_OTEL_SERVICE_NAME` | OTEL service name |
