# postgres 17 / single / execute/sql / server-api-config-restart

Область проверки: real YC run launched and observed through the local server API

Состояние исполнения: `completed`. Наблюдалось: `2026-09-25T14:36:18.806304+00:00`.

[Паспорт случая](case.json) · [Проверки](checks.json)

[Вход](input.json) — Compiled RunSpec from the server API; operator OTLP headers omitted.

| Проверка | Статус | Основание |
|---|---|---|
| compile | passed | [доказательство](runs/43efd586-7ce9-451a-8cbe-666faaf22392/run.json) (`/body`) |
| infrastructure_provisioning | passed | [доказательство](runs/43efd586-7ce9-451a-8cbe-666faaf22392/events.json) (`/body/data`) |
| smoke | passed | [доказательство](../../../../../platform/server/acceptance/pg-run.json) (`/body/result`) |
| native_tps | not_applicable | The workload has no logical transaction TPS. |
| native_otlp_metrics | passed | [доказательство](../../../../../platform/server/acceptance/pg-telemetry.json) (`/native_series_count`) |
| component_metrics | passed | [доказательство](../../../../../platform/server/acceptance/pg-telemetry.json) (`/metric_series`) |
| component_logs | passed | [доказательство](../../../../../platform/server/acceptance/pg-telemetry.json) (`/facets`) |
| pipeline_traces | not_run | No explicit acceptance evidence recorded for this check. |
| database_metric_distributions | not_run | No explicit acceptance evidence recorded for this check. |
| managed_database_metrics | not_applicable | Self-hosted PostgreSQL, not a managed service. |
| artifacts | passed | [доказательство](../../../../../platform/server/acceptance/pg-artifacts.json) (`/artifacts`) |
| replication | not_applicable | Single-node topology. |
| cleanup | passed | [доказательство](../../../../../platform/server/acceptance/pg-cleanup.json) (`/verified`) |

Прогоны:

- [runs/43efd586-7ce9-451a-8cbe-666faaf22392](runs/43efd586-7ce9-451a-8cbe-666faaf22392/manifest.json)

[Единая таблица](../../../../../progress.csv)
