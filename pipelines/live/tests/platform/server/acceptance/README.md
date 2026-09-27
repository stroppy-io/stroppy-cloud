# Server acceptance

Current-source status (2026-09-27): recovery changes and Graphene 0.2.24 are under
[resilience verification](../resilience/README.md). The acceptance results below
belong to the prior source digest. Full integration/live acceptance must be rerun;
`capabilities.json` correctly reports local/race verification as false for the
changed source.

This acceptance checks the server/API boundary without repeating every database
and workload combination on YC. The machine-readable verdict is
[capabilities.json](capabilities.json). The project still has one progress table:
[progress.csv](../../../progress.csv).

Verified on 2026-09-25: **215 local tests/subtests passed, one explicitly skipped;
16 selected tests/subtests passed with `-race`; all sampled live scenarios passed
their expected outcomes.** The final tenant scan has zero active runs and zero
kept stands. The optional-feature exclusions below still apply.

## Scope and evidence

| Capability | Local assertions | Real YC/server evidence |
| --- | --- | --- |
| OpenAPI surface | All 171 operations inventoried; 169 have successful HTTP responses; two Grafana operations explicitly return 503 | Public catalog and anonymous shared metrics |
| Catalog | Every deployable database/version/topology; compatible scripts on each default database | Representative PostgreSQL and noop runs |
| Complete input transport | All runtime categories; all nine workload variants and their parameters; unrelated edits preserve fields | PostgreSQL config file sets `max_connections=123`; every SQL query asserts it |
| Lossless data | Browser/HTTP/PostgreSQL/RunSpec preserve uint64, explicit zero/false/empty/null, native result envelopes | Native metrics and artifacts read through server |
| Invalid inputs | Impossible capacity, invalid disk sizes, missing roles, unsafe paths, conflicting SQL fail before launch | No invalid preflight case is deliberately sent to YC |
| Launch admission | Concurrent same-key requests return one run; six distinct requests respect limit three; suite admission uses the same transaction lock | Two-cell suite |
| Recovery | Observer cursor survives replacement; expiry/lost release acknowledgement reconciles kept state | Server restarted during PostgreSQL workload; same Graphene run completes |
| Failure and cancellation | Terminal scenarios retain partial results and enforce access checks | Workload cancellation; SQL-error suite cell; full rerun of immutable failing snapshot |
| Keep and artifacts | Stand expiry/release excludes retained artifacts | Extend/release, resource tree cleanup, config/log download size and SHA-256 |
| Telemetry and shares | Run/tenant scope, typed/raw metrics, log pagination/facets, share scope/revocation | Native and node metrics, database logs, anonymous metrics then revoked link |
| Application workflows | Library clone/import/export/diff, tenants, tokens, roles, invites, providers, schedules, webhooks, comparisons, ratings, WebSocket | Provider and pipeline synchronization status |

`local.json` is the full integration result, including named tests, route/status
observations and the source digest before/after execution. PostgreSQL and HTTP are
real; Graphene RPC and cloud activities are simulated. `races.json` adds the race
detector for concurrency, recovery, sharing and suite orchestration. A route's
successful response alone is not semantic acceptance: `capabilities.json` links
each capability to its assertion groups.

`runtime-fixture.json` and `workloads-fixture.json` are executable fixtures read by
the integration tests. New workload variant parameters fail the test until they
are represented in the fixture. The runtime fixture explicitly exercises disks,
files, container settings, network, host preparation, flows, scrapes, driver
settings, segments, baseline and observability labels.

## Live attempts

All cloud runs are launched, observed and cleaned through Stroppy Cloud HTTP.
The only direct local runtime operation is restarting the Compose server.
Cleanup evidence asserts the resource state returned by Graphene through the
server; it does not use a separate YC or Kubernetes inspection channel.

- `pg-*`: PostgreSQL config assertion, 90 seconds, server restart, keep/extend/release,
  telemetry, artifacts and cleanup. `pg-fixture-*` is an earlier, cancelled setup
  mistake with the wrong PostgreSQL config path; its cleanup is recorded separately.
- `cancel-*`: cancel a live noop workload and confirm terminal state/cleanup.
- `suite-*`, `success-test*`, `sql-failure-test*`, `failure-*`: a two-cell suite with
  one successful noop cell and one PostgreSQL SQL-error cell. With
  `continue_on_failure`, the suite completes while reporting one failed cell.
- `rerun-*`: repeat the failed snapshot under a new server/Graphene identity;
  the expected result is the same SQL failure, with partial results retained.
- `public-*`: public catalog and temporary anonymous metrics share. The share is
  revoked; its access token is not written to evidence.
- `server-status.json`: running server revision and namespace pipeline sync.
- `final-resources.json`: final paginated API filters contain no active or kept runs.

The initial live phases ran on `observe3`/`acceptance1`; the rerun uses
`0.0.0-dev-f658fd8b-acceptance2-20260925`, Graphene 0.2.23 and contract 2.0.1.
Stroppy is pinned to the temporary development build digest
`sha256:d1083cac1321793911e39b00043afd8167ed2aa69fec5c0429fa17c7ef2ce945`.

## Fixes covered

- Public catalog is implemented instead of falling through to 501.
- Public metrics resolve only the shared run and published time window via
  Graphene; overview-only, revoked and expired links do not grant metrics access.
- Kept-state reconciliation handles expiry and a caller disconnecting while
  Graphene finishes stand release. Config/log artifacts remain independent.
- Tenant admission serializes limit checks and insertion in PostgreSQL.
  Concurrent idempotent requests replay the admitted run. Suite and child rows
  are persisted atomically before contacting Graphene.
- The run pipeline announces teardown before Graphene starts its cleanup
  cascade, including workload failure. UI no longer labels cleanup as collection.
- The live observer retries transient read failures during local server restart.

## Explicit limits

- `createRunGrafanaSession` and `createPublicShareGrafanaSession` remain unavailable
  (503). They are not included in a claim that all advertised optional features work.
- Pipeline trace retrieval is not accepted by this check.
- A sampled YC acceptance does not requalify every database/topology/workload,
  every cloud quota edge or a five-hour load. AWS is excluded.
- The official Stroppy release still needs qualification after the temporary
  development build is replaced.
- The local `TestLocalStand` test is intentionally skipped without its explicit
  environment flag; live evidence is collected by the HTTP-only tools instead.

## Reproduce

From the repository root, with Docker available:

```sh
python3 pipelines/live/tools/server_acceptance.py --name local
python3 pipelines/live/tools/server_acceptance.py --name races --race \
  --run '^TestE2E(ConcurrentLaunch|KeptStandRecovery|ObserverRecovery|SharedMetrics|SuitesAndSchedules)$'
make contract-check
make lint
make lint-pipelines
python3 pipelines/live/tools/server_acceptance_report.py
python3 pipelines/live/tools/live.py validate
```

Raw integration logs stay under ignored `.local/server-acceptance/` with mode
0600. Public reports retain only test outcomes, operation IDs and status counts.
The focused `added.json`, `surface.json`, `recovery.json` and `admission.json`
reports describe intermediate checks; `local.json` is authoritative for the final
source state. The live tools under `live/tools/` accept the server URL and bearer
token via `STROPPY_TEST_URL` / `STROPPY_TEST_TOKEN`.

The recorded suite can be observed again with
`python3 pipelines/live/tools/server_acceptance_suite.py --watch`. `--start`
creates paid YC resources and refuses to overwrite an existing suite launch.
