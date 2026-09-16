# cockroach 25.2 / three-node / simple / native-otlp-2m-2vus

Область проверки: functional smoke and recorded telemetry checks; not full performance or fault readiness

Состояние исполнения: `completed`. Наблюдалось: `2026-09-15T23:36:40.846351+00:00`.

[Паспорт случая](case.json) · [Проверки](checks.json)

[Вход](input.json) — Full original run input, potentially containing multiple workloads. Preset is the historical label; actual segment parameters take precedence.

| Проверка | Статус | Основание |
|---|---|---|
| compile | passed | [доказательство](../../../../../platform/catalog/catalog-functional-check.json) (`/native_cells/90`) |
| infrastructure_provisioning | unknown | No explicit retained verification for this check. |
| smoke | passed | [доказательство](../../../../../platform/catalog/catalog-functional-check.json) (`/native_cells/90`) |
| native_tps | not_applicable | No logical transactions for this workload. |
| native_otlp_metrics | passed | [доказательство](../../../../../platform/catalog/catalog-functional-check.json) (`/native_cells/90`) |
| component_metrics | passed | [доказательство](../../../../../platform/catalog/catalog-functional-check.json) (`/native_cells/90`) |
| component_logs | passed | [доказательство](../../../../../platform/catalog/catalog-functional-check.json) (`/native_cells/90`) |
| pipeline_traces | passed | [доказательство](../../../../../platform/catalog/catalog-functional-check.json) (`/native_cells/90`) |
| database_metric_distributions | not_run | Not verified in the retained report. |
| managed_database_metrics | not_applicable | Not applicable to this database/topology; inherited scope is preserved. |
| artifacts | passed | [доказательство](../../../../../platform/catalog/catalog-functional-check.json) (`/native_cells/90`) |
| replication | passed | [доказательство](../../../../../platform/catalog/catalog-functional-check.json) (`/native_cells/90`) |
| cleanup | passed | [доказательство](../../../../../platform/catalog/catalog-functional-check.json) (`/native_cells/90`) |

Прогоны:

- [runs/matrix-cockroach-74ee56ed-cockroach-three-node-25.2](runs/matrix-cockroach-74ee56ed-cockroach-three-node-25.2/manifest.json)

[Единая таблица](../../../../../progress.csv)
