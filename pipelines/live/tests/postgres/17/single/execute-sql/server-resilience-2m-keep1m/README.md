# postgres 17 / single / execute/sql / server-resilience-2m-keep1m

Область проверки: real YC run launched and observed through the local server API

Состояние исполнения: `completed`. Наблюдалось: `2026-09-27T16:08:55.313222+00:00`.

[Паспорт случая](case.json) · [Проверки](checks.json)

[Вход](input.json) — Compiled RunSpec from the server API; operator OTLP headers omitted.

| Проверка | Статус | Основание |
|---|---|---|
| compile | passed | [доказательство](runs/be40ddab-2f06-4904-9d51-999359b45653/run.json) (`/body`) |
| infrastructure_provisioning | passed | [доказательство](runs/be40ddab-2f06-4904-9d51-999359b45653/events.json) (`/body/data`) |
| smoke | passed | [доказательство](runs/be40ddab-2f06-4904-9d51-999359b45653/run.json) (`/body/result`) |
| native_tps | not_applicable | The workload has no logical transaction TPS. |
| native_otlp_metrics | passed | [доказательство](../../../../../platform/server/resilience/live-suite-20260927/telemetry-postgres.json) (`/verified`) |
| component_metrics | passed | [доказательство](../../../../../platform/server/resilience/live-suite-20260927/database-metrics-postgres.json) (`/series_count`) |
| component_logs | passed | [доказательство](../../../../../platform/server/resilience/live-suite-20260927/telemetry-postgres.json) (`/facets`) |
| pipeline_traces | not_run | No explicit acceptance evidence recorded for this check. |
| database_metric_distributions | not_run | No explicit acceptance evidence recorded for this check. |
| managed_database_metrics | not_applicable | Self-managed PostgreSQL. |
| artifacts | passed | [доказательство](../../../../../platform/server/resilience/live-suite-20260927/artifacts-postgres.json) (`/verified`) |
| replication | not_applicable | Single PostgreSQL node; replication is absent from this case. |
| cleanup | passed | [доказательство](../../../../../platform/server/resilience/live-suite-20260927/cleanup-be40ddab-2f06-4904-9d51-999359b45653.json) (`/verified`) |

Прогоны:

- [runs/be40ddab-2f06-4904-9d51-999359b45653](runs/be40ddab-2f06-4904-9d51-999359b45653/manifest.json)

[Единая таблица](../../../../../progress.csv)
