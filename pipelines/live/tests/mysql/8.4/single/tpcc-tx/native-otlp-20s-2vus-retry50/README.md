# mysql 8.4 / single / tpcc/tx / native-otlp-20s-2vus-retry50

Область проверки: functional smoke and recorded telemetry checks; not full performance or fault readiness

Состояние исполнения: `completed`. Наблюдалось: `2026-09-15T23:36:40.846351+00:00`.

[Паспорт случая](case.json) · [Проверки](checks.json)

[Вход](input.json) — Full original run input, potentially containing multiple workloads. Preset is the historical label; actual segment parameters take precedence.

| Проверка | Статус | Основание |
|---|---|---|
| compile | passed | [доказательство](../../../../../platform/catalog/mysql-family-check.json) (`/native_cells/2`) |
| infrastructure_provisioning | unknown | No explicit retained verification for this check. |
| smoke | passed | [доказательство](../../../../../platform/catalog/mysql-family-check.json) (`/native_cells/2`) |
| native_tps | passed | [доказательство](../../../../../platform/catalog/mysql-family-check.json) (`/native_cells/2`) |
| native_otlp_metrics | passed | [доказательство](../../../../../platform/catalog/mysql-family-check.json) (`/native_cells/2`) |
| component_metrics | passed | [доказательство](../../../../../platform/catalog/mysql-family-check.json) (`/native_cells/2`) |
| component_logs | passed | [доказательство](../../../../../platform/catalog/mysql-family-check.json) (`/native_cells/2`) |
| pipeline_traces | passed | [доказательство](../../../../../platform/catalog/mysql-family-check.json) (`/native_cells/2`) |
| database_metric_distributions | not_run | Not verified in the retained report. |
| managed_database_metrics | not_applicable | Not applicable to this database/topology; inherited scope is preserved. |
| artifacts | passed | [доказательство](../../../../../platform/catalog/mysql-family-check.json) (`/native_cells/2`) |
| replication | not_applicable | Not applicable to this database/topology; inherited scope is preserved. |
| cleanup | passed | [доказательство](../../../../../platform/catalog/mysql-family-check.json) (`/native_cells/2`) |

Прогоны:

- [runs/matrix-mysql84-fixed-fc2399ff-mysql84-single](runs/matrix-mysql84-fixed-fc2399ff-mysql84-single/manifest.json)

[Единая таблица](../../../../../progress.csv)
