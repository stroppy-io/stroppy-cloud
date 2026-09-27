# postgres 17 / single / tpcds / server-sf001-generated

Область проверки: real YC run launched and observed through the local server API

Состояние исполнения: `completed`. Наблюдалось: `2026-09-27T19:11:09.985044+00:00`.

[Паспорт случая](case.json) · [Проверки](checks.json)

[Вход](input.json) — Compiled RunSpec from the server API; operator OTLP headers omitted.

| Проверка | Статус | Основание |
|---|---|---|
| compile | passed | [доказательство](runs/aa231dc7-16ec-44e9-9d3c-00d0235d02bc/run.json) (`/body`) |
| infrastructure_provisioning | passed | [доказательство](runs/aa231dc7-16ec-44e9-9d3c-00d0235d02bc/run.json) (`/body/result`) |
| smoke | passed | [доказательство](../../../../../platform/server/resilience/analytical-generated-q88-20260927/verification.json) (`/workload_verified`) |
| native_tps | not_applicable | Analytical workload reports queries/s and iterations/s rather than logical transaction TPS. |
| native_otlp_metrics | passed | [доказательство](../../../../../platform/server/resilience/analytical-generated-q88-20260927/telemetry.json) (`/native_verified`) |
| component_metrics | passed | [доказательство](../../../../../platform/server/resilience/analytical-generated-q88-20260927/database-metrics.json) (`/series_count`) |
| component_logs | passed | [доказательство](../../../../../platform/server/resilience/analytical-generated-q88-20260927/telemetry.json) (`/facets`) |
| pipeline_traces | not_run | No explicit acceptance evidence recorded for this check. |
| database_metric_distributions | not_run | No explicit acceptance evidence recorded for this check. |
| managed_database_metrics | not_applicable | Self-managed PostgreSQL. |
| artifacts | passed | [доказательство](../../../../../platform/server/resilience/analytical-generated-q88-20260927/artifacts.json) (`/verified`) |
| replication | not_applicable | Single PostgreSQL instance. |
| cleanup | passed | [доказательство](../../../../../platform/server/resilience/analytical-generated-q88-20260927/cleanup.json) (`/verified`) |

Прогоны:

- [runs/7b3d8ae2-1b43-4b41-81ca-64f9e522ea86](runs/7b3d8ae2-1b43-4b41-81ca-64f9e522ea86/manifest.json)
- [runs/aa231dc7-16ec-44e9-9d3c-00d0235d02bc](runs/aa231dc7-16ec-44e9-9d3c-00d0235d02bc/manifest.json)

[Единая таблица](../../../../../progress.csv)
