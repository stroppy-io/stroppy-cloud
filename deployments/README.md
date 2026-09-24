# Local server with Docker Compose

The packaging follows `gopherex/iam`: build the SPA with Node, embed `web/dist`
in a static Go binary, then copy the binary into
`gcr.io/distroless/static-debian12:nonroot`. One process serves API and UI.
There is no separate frontend container or runtime Node/Nginx dependency.
Stroppy also includes five pipeline binaries from the same checkout under
`/opt/stroppy/pipelines`: run, suite, provider-verify, provider-config and quotas. The server
publishes those binaries to tenant namespaces through Graphene using the SDK's
`selfbuild.PushBinary`, then registers their exported manifests. Publication
does not invoke the pipeline CLI's `push` command, which rebuilds from source
and requires a Go toolchain.

Compose starts this image and PostgreSQL 17, waits for PostgreSQL's healthcheck,
and lets the server apply migrations. PostgreSQL data lives in the `pgdata`
volume. Only the server is published, on `127.0.0.1:18347` by default.

## Start

Prerequisites: Docker with Compose v2 and an existing Graphene cluster with
workers/providers configured for the intended workloads. Local Go, Node,
Kubernetes access and a Docker socket inside the server are not required to
build or start the Compose stack.

```sh
cp .env.compose.example .env.compose
chmod 600 .env.compose
# Set STROPPY_INFRA_GRAPHENE_TOKEN in .env.compose using your editor.
make dev
```

Set Graphene's address and TLS mode alongside the token if using a different
cluster. The token must authorize the server's tenant namespace management,
pipeline publication and execution. This is a real Graphene connection;
`STROPPY_LOCAL_STAND` simulation is not enabled.

`make dev` uses `.env.compose` explicitly, leaving the legacy root `.env` out of
Compose configuration. Credentials are gitignored and excluded from the Docker
build context. For a different env file use `COMPOSE_ENV=/path/to/file` with
the Make targets. Do not use `docker compose config` without `--quiet` in shared
logs: the rendered configuration contains credentials.

Open <http://localhost:18347>. The current SPA is a placeholder; benchmark screens
and browser login are not implemented yet. API testing uses the development
bearer token `dev`, configured as the local platform administrator:

```sh
curl -fsS http://localhost:18347/healthz/liveness
curl -fsS http://localhost:18347/healthz/readiness
curl -fsS -H 'Authorization: Bearer dev' http://localhost:18347/api/v1/me
curl -fsS -H 'Authorization: Bearer dev' http://localhost:18347/api/v1/admin/status
curl -fsS -H 'Authorization: Bearer dev' http://localhost:18347/api/v1/catalog/databases
```

Readiness checks PostgreSQL and Graphene. It does not certify pipeline
publication, YC provisioning or a complete workload run. A fresh database has
no tenants or cloud providers; create them through the server API before runs.
The development token is for this loopback-only setup; a shared deployment
should use the server's IAM authentication configuration.

```sh
make dev-logs       # follow server logs
make dev           # rebuild and recreate after code/config changes
make dev-down      # stop the stack; retain PostgreSQL data
```

After rebuilding modified pipeline sources without a new Git revision, force
publication through `POST /api/v1/admin/pipelines:resync` and wait for `synced`
in `GET /api/v1/admin/status`. Development builds can share a revision string.

Stopping Compose does not cancel Graphene runs or delete YC stands. Manage
their lifecycle through the server API before stopping the server. Do not use
`down -v` unless deliberately discarding the local server database.

To change the published port, update both `STROPPY_LOCAL_PORT` and
`STROPPY_HTTP_PUBLIC_URL` in `.env.compose`.

## Run telemetry

Graphene, YC resources and run telemetry remain external dependencies.
Compose does not deploy them or alter their retention settings.
The example leaves telemetry endpoints empty; configure these before testing
complete run observability:

| Variable | Required network access |
| --- | --- |
| `STROPPY_INFRA_VICTORIA_LOGS_URL` | Server container to VictoriaLogs HTTP API |
| `STROPPY_INFRA_VICTORIA_METRICS_URL` | Server container to VictoriaMetrics HTTP API |
| `STROPPY_HTTP_GRAFANA_URL` | Browser to Grafana |
| `STROPPY_HTTP_GRAFANA_DASHBOARDS` | Dashboard mapping used for Grafana links |
| `STROPPY_INFRA_OBSERVABILITY_OTLP_ENDPOINT` | YC runner VMs to the OTLP collector |
| `STROPPY_INFRA_OBSERVABILITY_OTLP_HEADERS` | Collector authorization headers |

A Kubernetes `*.svc.cluster.local` address normally cannot be resolved from a
local container or browser. For an existing host-side tunnel the container can
use `host.docker.internal`; the tunnel must listen on a Docker-reachable host
address. A tunnel listening only on host `127.0.0.1` is not reachable there on
Linux. Do not substitute a local tunnel for the OTLP export endpoint: workload
VMs need their own network route to the collector.

## Image and frontend development

`make docker-build` builds the same server image independently of Compose.
Version and commit metadata are supplied by Make; build time defaults to UTC
at image build. The runtime runs as `nonroot`, includes CA certificates for TLS,
and has no shell. Use host HTTP tools for probes and `make dev-logs` for logs.

For frontend hot reload, keep Compose running and run `make web-install` then
`make web-dev`. Vite proxies API requests to port 18347; the Compose default CORS
origin is `http://localhost:5173`. Changes to the embedded SPA require another
`make dev` build.

## Graphene installation access

Stroppy Cloud only connects to Graphene. Do not mount a kubeconfig or grant the
server Kubernetes permissions. Trusted Graphene managed workers use the
installation's projected `graphene-crossplane` ServiceAccount. The current cluster is managed by the Helm chart in `stroppy-io/cloud/k8s/apps/graphene`.
For installations without GitOps, the operator configures RBAC and `managed.pod_template` with
`pipelines/live/tools/install_graphene_access.py --kubeconfig <operator-config> --apply`.
The supplied RBAC covers Yandex Cloud; AWS execution is outside this live scope.
Crossplane must be able to read Secrets in `stroppy-provider-credentials`.

`stroppy-provider-config` owns the explicit configuration lifecycle of a provider
profile. Its persistent resources are not children of a benchmark run. `ready`
means cloud verification and configuration both completed. `deleting` and
`delete_failed` block launches and updates; DELETE can be retried. Active runs,
kept stands and actual Crossplane usages/resources prevent cleanup. Tenant deletion
seals admission of new runs/profiles, cleans profiles, then retires its Graphene
namespace. A partial failure remains retryable without removing credentials early.

Credential rotation creates an immutable Graphene secret version before atomically
selecting it with the profile's settings and operation ID. Pending setup operations
reattach the same Graphene run after restart or transport failure; they do not launch
a second configuration run. Old credential versions are removed on profile deletion.
