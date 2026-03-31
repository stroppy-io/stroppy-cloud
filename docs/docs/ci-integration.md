---
sidebar_position: 10
---

# CI Integration

This guide shows how to use Stroppy Cloud in CI pipelines for automated database performance testing.

## Overview

A typical CI flow:

1. **Build** your custom database package (.deb/.rpm)
2. **Upload** the package to the Stroppy Cloud server
3. **Start** a test run referencing the uploaded package
4. **Wait** for the run to complete
5. **Compare** results against a baseline run
6. **Report** pass/fail based on performance regression thresholds

## Prerequisites

- Stroppy Cloud server running and accessible from CI (e.g., `https://stroppy.internal:8080`)
- Authentication token or credentials
- `curl` and `jq` available in CI environment

## Step 1: Authenticate

```bash
export STROPPY_URL="http://stroppy.internal:8080"

TOKEN=$(curl -sf -X POST "$STROPPY_URL/api/v1/auth/login" \
  -H 'Content-Type: application/json' \
  -d '{"username":"ci-user","password":"$CI_PASSWORD"}' | jq -r .token)

export AUTH="Authorization: Bearer $TOKEN"
```

## Step 2: Build a Custom Package

Build your database package so it conforms to the standard layout. Stroppy Cloud agents install packages via `dpkg -i` (for .deb) or `rpm -i` (for .rpm).

### .deb package requirements

The package should:
- Install the database binary to a standard path (`/usr/bin/`, `/usr/lib/postgresql/`)
- Include systemd service files if needed
- Not start the service on install (agents manage startup)

Example build (for a patched PostgreSQL):

```bash
# Build from source
./configure --prefix=/usr/lib/postgresql/16
make -j$(nproc)
make DESTDIR=./pkg install

# Package
fpm -s dir -t deb \
  -n postgresql-custom-16 \
  -v "16.0-$(git rev-parse --short HEAD)" \
  -C ./pkg \
  --deb-no-default-config-files \
  usr/
```

### .rpm package requirements

Same as .deb but in RPM format:

```bash
fpm -s dir -t rpm \
  -n postgresql-custom-16 \
  -v "16.0.$(git rev-parse --short HEAD)" \
  -C ./pkg \
  usr/
```

## Step 3: Upload the Package

```bash
UPLOAD_RESULT=$(curl -sf -X POST "$STROPPY_URL/api/v1/upload/deb" \
  -H "$AUTH" \
  -F file=@postgresql-custom-16_16.0-abc1234_amd64.deb)

PACKAGE_URL=$(echo "$UPLOAD_RESULT" | jq -r .url)
echo "Package uploaded: $PACKAGE_URL"
```

## Step 4: Start a Test Run

Create a RunConfig JSON that references your uploaded package:

```bash
RUN_ID="ci-$(date +%s)-${CI_COMMIT_SHORT_SHA:-manual}"

cat > /tmp/run-config.json << EOF
{
  "id": "$RUN_ID",
  "provider": "docker",
  "network": {"cidr": "10.10.0.0/24"},
  "machines": [],
  "database": {
    "kind": "postgres",
    "version": "16",
    "postgres": {
      "master": {"role": "database", "count": 1, "cpus": 2, "memory_mb": 4096, "disk_gb": 50}
    }
  },
  "monitor": {},
  "stroppy": {
    "version": "3.1.0",
    "workload": "tpcb",
    "duration": "5m",
    "workers": 8
  },
  "packages": {
    "deb_files": ["$PACKAGE_URL"]
  }
}
EOF

curl -sf -X POST "$STROPPY_URL/api/v1/run" \
  -H "$AUTH" \
  -H "Content-Type: application/json" \
  -d @/tmp/run-config.json
```

## Step 5: Wait for Completion

Poll the status endpoint until the run finishes:

```bash
while true; do
  STATUS=$(curl -sf "$STROPPY_URL/api/v1/run/$RUN_ID/status" -H "$AUTH")
  PENDING=$(echo "$STATUS" | jq '[.nodes[] | select(.status == "pending")] | length')
  FAILED=$(echo "$STATUS" | jq '[.nodes[] | select(.status == "failed")] | length')

  if [ "$FAILED" -gt 0 ]; then
    echo "FAIL: Run has $FAILED failed nodes"
    echo "$STATUS" | jq '.nodes[] | select(.status == "failed")'
    exit 1
  fi

  if [ "$PENDING" -eq 0 ]; then
    echo "Run completed successfully"
    break
  fi

  echo "Waiting... ($PENDING nodes pending)"
  sleep 10
done
```

## Step 6: Compare Against Baseline

Compare the current run against a known-good baseline run:

```bash
BASELINE_RUN="baseline-postgres-16"  # a previously saved successful run ID

COMPARE=$(curl -sf "$STROPPY_URL/api/v1/compare?a=$BASELINE_RUN&b=$RUN_ID" \
  -H "$AUTH")

echo "Comparison results:"
echo "$COMPARE" | jq '.metrics[] | {name, avg_a, avg_b, diff_avg_pct, verdict}'

# Check for regressions
WORSE=$(echo "$COMPARE" | jq '.summary.worse')
BETTER=$(echo "$COMPARE" | jq '.summary.better')

echo "Better: $BETTER, Worse: $WORSE"

if [ "$WORSE" -gt 2 ]; then
  echo "FAIL: Performance regression detected ($WORSE metrics worse)"
  exit 1
fi

echo "PASS: No significant regressions"
```

### Comparison verdict logic

The compare endpoint uses a 5% threshold by default:
- **better**: metric improved by >5%
- **worse**: metric degraded by >5%
- **same**: within 5% of baseline

Metrics where "higher is better": `db_qps`, `stroppy_ops`
Metrics where "lower is better": `db_latency_p99`, `cpu_usage`, `memory_usage`, `stroppy_latency_p99`, `stroppy_errors`

## Step 7: Cleanup

Delete the run after extracting results:

```bash
curl -sf -X DELETE "$STROPPY_URL/api/v1/run/$RUN_ID" -H "$AUTH"
```

## Full CI Script Example

```bash
#!/bin/bash
set -euo pipefail

STROPPY_URL="${STROPPY_URL:-http://stroppy.internal:8080}"
BASELINE_RUN="${BASELINE_RUN:-baseline-postgres-16}"
WORKLOAD="${WORKLOAD:-tpcb}"
DURATION="${DURATION:-5m}"
WORKERS="${WORKERS:-8}"

# 1. Auth
TOKEN=$(curl -sf -X POST "$STROPPY_URL/api/v1/auth/login" \
  -H 'Content-Type: application/json' \
  -d "{\"username\":\"$CI_USER\",\"password\":\"$CI_PASSWORD\"}" | jq -r .token)
AUTH="Authorization: Bearer $TOKEN"

# 2. Upload package
PKG_URL=$(curl -sf -X POST "$STROPPY_URL/api/v1/upload/deb" \
  -H "$AUTH" -F file=@"$DEB_PATH" | jq -r .url)

# 3. Start run
RUN_ID="ci-$(date +%s)"
curl -sf -X POST "$STROPPY_URL/api/v1/run" \
  -H "$AUTH" -H "Content-Type: application/json" \
  -d "{
    \"id\": \"$RUN_ID\",
    \"provider\": \"docker\",
    \"network\": {\"cidr\": \"10.10.0.0/24\"},
    \"machines\": [],
    \"database\": {\"kind\": \"postgres\", \"version\": \"16\",
      \"postgres\": {\"master\": {\"role\": \"database\", \"count\": 1, \"cpus\": 2, \"memory_mb\": 4096, \"disk_gb\": 50}}},
    \"monitor\": {},
    \"stroppy\": {\"version\": \"3.1.0\", \"workload\": \"$WORKLOAD\", \"duration\": \"$DURATION\", \"workers\": $WORKERS},
    \"packages\": {\"deb_files\": [\"$PKG_URL\"]}
  }"

# 4. Wait
while true; do
  S=$(curl -sf "$STROPPY_URL/api/v1/run/$RUN_ID/status" -H "$AUTH")
  F=$(echo "$S" | jq '[.nodes[] | select(.status=="failed")] | length')
  P=$(echo "$S" | jq '[.nodes[] | select(.status=="pending")] | length')
  [ "$F" -gt 0 ] && { echo "FAIL"; echo "$S" | jq '.nodes[] | select(.status=="failed")'; exit 1; }
  [ "$P" -eq 0 ] && break
  sleep 10
done

# 5. Compare
C=$(curl -sf "$STROPPY_URL/api/v1/compare?a=$BASELINE_RUN&b=$RUN_ID" -H "$AUTH")
echo "$C" | jq '.metrics[] | {name, diff_avg_pct, verdict}'
WORSE=$(echo "$C" | jq '.summary.worse')
[ "$WORSE" -gt 2 ] && { echo "Regression: $WORSE metrics worse"; exit 1; }

echo "OK: no regressions"
```

## ROADMAP

The following CI features are not yet implemented:

### `stroppy-cloud compare` CLI command
**Status**: Not implemented
**What**: A CLI command to compare two runs locally without the server API. Would allow `stroppy-cloud compare --run-a X --run-b Y --threshold 5 --format json|table`.
**Workaround**: Use the `/api/v1/compare` HTTP endpoint via `curl`.

### `stroppy-cloud upload` CLI command
**Status**: Not implemented
**What**: A CLI command to upload packages. Would allow `stroppy-cloud upload --file my.deb --server http://...`.
**Workaround**: Use `curl -F file=@my.deb $URL/api/v1/upload/deb`.

### `stroppy-cloud wait` CLI command
**Status**: Not implemented
**What**: A CLI command that polls a run until completion with configurable timeout. Would allow `stroppy-cloud wait --run-id X --timeout 30m`.
**Workaround**: Poll `/api/v1/run/{id}/status` in a shell loop (see Step 5 above).

### Configurable comparison threshold
**Status**: Hardcoded at 5%
**What**: Allow passing `--threshold` to the compare API/CLI for custom regression sensitivity.
**Workaround**: Post-process the JSON response and apply custom thresholds in CI script.

### JUnit/TAP output for CI
**Status**: Not implemented
**What**: Output comparison results in JUnit XML or TAP format for CI system integration (GitLab, Jenkins, GitHub Actions).
**Workaround**: Parse JSON output with `jq` and generate the report in CI script.

### Baseline management
**Status**: Not implemented
**What**: Ability to mark a run as "baseline" and have the compare endpoint use it automatically. Would allow `stroppy-cloud baseline set --run-id X --name postgres-16`.
**Workaround**: Store baseline run IDs in CI environment variables.

### Webhook notifications
**Status**: Not implemented
**What**: Configure webhooks to notify external systems (Slack, CI) when a run completes or fails.
**Workaround**: Poll the status endpoint.

### RPM upload support
**Status**: Only .deb is supported
**What**: Extend upload endpoint to accept `.rpm` files.
**Workaround**: Serve RPM files from an external HTTP server and reference them in `packages.rpm_files`.

### Per-run stroppy_run_id in K6 OTEL metrics
**Status**: Not implemented
**What**: Stroppy K6 OTEL metrics currently use `service_name="stroppy"` without a per-run label. This means stroppy metrics in compare dashboards show the same data for both runs.
**Workaround**: Use the API compare endpoint (which uses run timestamps to isolate data) instead of Grafana dashboards for stroppy-specific comparison.
