# ydb 26.3 / single / simple / native-otlp-2m-2vus

Область проверки: functional smoke and recorded telemetry checks; not full performance or fault readiness

Состояние исполнения: `completed`. Наблюдалось: `2026-09-15T23:36:40.846351+00:00`.

[Паспорт случая](case.json) · [Проверки](checks.json)

[Вход](input.json) — Full original run input, potentially containing multiple workloads. Preset is the historical label; actual segment parameters take precedence.

| Проверка | Статус | Основание |
|---|---|---|
| compile | passed | [доказательство](../../../../../platform/catalog/catalog-functional-check.json) (`/native_cells/119`) |
| infrastructure_provisioning | unknown | No explicit retained verification for this check. |
| smoke | passed | [доказательство](../../../../../platform/catalog/catalog-functional-check.json) (`/native_cells/119`) |
| native_tps | not_applicable | No logical transactions for this workload. |
| native_otlp_metrics | passed | [доказательство](../../../../../platform/catalog/catalog-functional-check.json) (`/native_cells/119`) |
| component_metrics | passed | [доказательство](../../../../../platform/catalog/catalog-functional-check.json) (`/native_cells/119`) |
| component_logs | passed | [доказательство](../../../../../platform/catalog/catalog-functional-check.json) (`/native_cells/119`) |
| pipeline_traces | passed | [доказательство](../../../../../platform/catalog/catalog-functional-check.json) (`/native_cells/119`) |
| database_metric_distributions | not_run | Not verified in the retained report. |
| managed_database_metrics | not_applicable | Not applicable to this database/topology; inherited scope is preserved. |
| artifacts | passed | [доказательство](../../../../../platform/catalog/catalog-functional-check.json) (`/native_cells/119`) |
| replication | passed | [доказательство](../../../../../platform/catalog/catalog-functional-check.json) (`/native_cells/119`) |
| cleanup | passed | [доказательство](../../../../../platform/catalog/catalog-functional-check.json) (`/native_cells/119`) |

Прогоны:

- [runs/matrix-ydbmetrics-4f28bec5-ydb-single-26.3](runs/matrix-ydbmetrics-4f28bec5-ydb-single-26.3/manifest.json)

[Единая таблица](../../../../../progress.csv)
