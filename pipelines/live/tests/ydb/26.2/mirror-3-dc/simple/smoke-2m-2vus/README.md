# ydb 26.2 / mirror-3-dc / simple / smoke-2m-2vus

Область проверки: functional smoke and recorded telemetry checks; not full performance or fault readiness

Состояние исполнения: `completed`. Наблюдалось: `2026-09-15T23:36:40.846351+00:00`.

[Паспорт случая](case.json) · [Проверки](checks.json)

[Вход](input.json) — Full original run input, potentially containing multiple workloads. Preset is the historical label; actual segment parameters take precedence.

| Проверка | Статус | Основание |
|---|---|---|
| compile | passed | [доказательство](../../../../../platform/catalog/catalog-functional-check.json) (`/native_cells/129`) |
| infrastructure_provisioning | passed | [доказательство](../../../../../platform/lifecycle/yc-autonomous-provisioning-check.json) (`/workflow_runs/1`) |
| smoke | passed | [доказательство](../../../../../platform/catalog/catalog-functional-check.json) (`/native_cells/129`) |
| native_tps | not_applicable | No logical transactions for this workload. |
| native_otlp_metrics | passed | [доказательство](../../../../../platform/catalog/catalog-functional-check.json) (`/native_cells/129`) |
| component_metrics | passed | [доказательство](../../../../../platform/catalog/catalog-functional-check.json) (`/native_cells/129`) |
| component_logs | passed | [доказательство](../../../../../platform/catalog/catalog-functional-check.json) (`/native_cells/129`) |
| pipeline_traces | passed | [доказательство](../../../../../platform/catalog/catalog-functional-check.json) (`/native_cells/129`) |
| database_metric_distributions | not_run | Not verified in the retained report. |
| managed_database_metrics | not_applicable | Not applicable to this database/topology; inherited scope is preserved. |
| artifacts | passed | [доказательство](../../../../../platform/catalog/catalog-functional-check.json) (`/native_cells/129`) |
| replication | passed | [доказательство](../../../../../platform/catalog/catalog-functional-check.json) (`/native_cells/129`) |
| cleanup | passed | [доказательство](../../../../../platform/catalog/catalog-functional-check.json) (`/native_cells/129`) |

Прогоны:

- [runs/matrix-ydbautonomous-73a81601-ydb-mirror-3-dc-26.2-autonomous](runs/matrix-ydbautonomous-73a81601-ydb-mirror-3-dc-26.2-autonomous/manifest.json)

[Единая таблица](../../../../../progress.csv)
