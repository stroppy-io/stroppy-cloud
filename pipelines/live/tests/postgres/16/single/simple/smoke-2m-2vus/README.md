# postgres 16 / single / simple / smoke-2m-2vus

Область проверки: functional smoke and recorded telemetry checks; not full performance or fault readiness

Состояние исполнения: `completed`. Наблюдалось: `2026-09-15T23:36:40.846351+00:00`.

[Паспорт случая](case.json) · [Проверки](checks.json)

[Вход](input.json) — Full original run input, potentially containing multiple workloads. Preset is the historical label; actual segment parameters take precedence.

| Проверка | Статус | Основание |
|---|---|---|
| compile | passed | [доказательство](../../../../../platform/catalog/postgres-matrix-check.json) (`/cells/5`) |
| infrastructure_provisioning | unknown | No explicit retained verification for this check. |
| smoke | passed | [доказательство](../../../../../platform/catalog/postgres-matrix-check.json) (`/cells/5`) |
| native_tps | not_applicable | No logical transactions for this workload. |
| native_otlp_metrics | not_run | Not verified in the retained report. |
| component_metrics | passed | [доказательство](../../../../../platform/catalog/postgres-matrix-check.json) (`/cells/5`) |
| component_logs | not_run | Not verified in the retained report. |
| pipeline_traces | passed | [доказательство](../../../../../platform/catalog/postgres-matrix-check.json) (`/cells/5`) |
| database_metric_distributions | not_run | Not verified in the retained report. |
| managed_database_metrics | not_applicable | Not applicable to this database/topology; inherited scope is preserved. |
| artifacts | not_run | Not verified in the retained report. |
| replication | not_run | Not verified in the retained report. |
| cleanup | passed | [доказательство](../../../../../platform/catalog/postgres-matrix-check.json) (`/cells/5`) |

Прогоны:

- [runs/matrix-pg-retry-daaf3554-pg16-single](runs/matrix-pg-retry-daaf3554-pg16-single/manifest.json)

[Единая таблица](../../../../../progress.csv)
