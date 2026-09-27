# Server telemetry through Graphene

Stroppy Cloud uses Graphene 0.2.23 for run logs, facets and metrics. The local
Compose server has no Victoria/Grafana/workload-OTLP backend environment and
no mounts. The pipelines use pipeline 0.2.12 and library/docker 0.3.3.
The runtime supplies the local OTLP intake to Stroppy; the stored config
contains loopback HTTP and no export auth headers.

The current local build is `0.0.0-dev-f658fd8b-observe3-20260925`, built from
uncommitted Stroppy Cloud changes on top of `f658fd8b`. Graphene was not modified.
The native Stroppy development image remains pinned by digest as in each case's
input; this does not constitute acceptance of the catalog's release image.

## Evidence

- `server-boundary.json`: container image and absence of backend settings/mounts.
- `noop-launch.json`, `noop-telemetry.json`: 20s server-only run and telemetry.
- `noop-local-intake.json`: redacted exporter configuration inspected through the server.
- `noop-artifact-downloads.json`: sizes and digests after cleanup.
- `noop-cleanup-tree.json`: deleted infrastructure reported by Graphene via the server.
- `noop-90s-launch.json`, `noop-90s-telemetry.json`: first 90s attempt, including runner metrics.
- `noop-90s-artifact-downloads.json`, `noop-90s-cleanup-tree.json`: artifacts and cleanup of that failed attempt.
- `noop-90s-rerun-launch.json`: full rerun after correcting large-log summary parsing.
- `noop-90s-forward-pages.json`: live forward pagination through the server and Graphene.
- `noop-90s-rerun-telemetry.json`: native and runner metrics, facets and pagination in both directions.
- `foreign-run-metrics.json`: an explicit foreign-run selector yields no data in a window covering both runs.

The 20s run `d7003247-6d13-4c77-92b2-6267e1c63281` completed. Native workload
metrics, log pagination/facets and artifact downloads pass. The native metrics
retain the correct namespace/run. No transaction TPS exists for noop.
Node-exporter series were not observed in this short run; its component-metric
check remains failed. The 90s case records its own result and does not overwrite
that evidence. Store retention settings were not changed.

The first 90s attempt `feb7be00-c574-40ab-aae7-b183b85ba793` exported 61 native
series and runner CPU/memory metrics. Stroppy exited 0 after 15,203,535 iterations.
The pipeline incorrectly failed it because its 8 MiB diagnostic prefix did not
contain the summary at the end of the 28,460,453-byte log. Config and log remained
downloadable after cleanup. The pipeline now parses the complete file as a
stream, propagates read/write failures and keeps diagnostics bounded in memory.
The full rerun `d92548ed-af3a-425f-a896-14a90f58bf01` completed: 15,610,838
iterations, 61 native series, runner CPU/memory, logs paginated in both directions.
The 29,182,888-byte log and config download with matching digests after cleanup;
the server tree reports all resources deleted. The final OTLP iteration count
matches the native summary (`noop-90s-final-metrics.json`). Stroppy logged a
context-canceled export warning during shutdown; the final value was visible
after ingestion caught up. The original failed attempt and its checks are kept.
Pipeline traces, databases and the full scenario matrix are not accepted by this
runner-only check. Grafana remains unavailable through the server until Graphene
provides the required capability.

## Local validation

Root Go tests with race detection, pipeline tests, both module linters and
contract/browser checks passed. The complete server integration suite passed
in 388 seconds. After the catalog corrections the targeted Observe integration
and affected package tests passed again. Large-log summary/compliance/threshold
regressions and truncated-container-stream tests pass with race detection;
pipeline simulation and lint also pass after the parser fix. Real runs and artifact/telemetry reads
are initiated exclusively through Stroppy Cloud HTTP API; no direct telemetry
backend client or Kubernetes client is added to the server.
