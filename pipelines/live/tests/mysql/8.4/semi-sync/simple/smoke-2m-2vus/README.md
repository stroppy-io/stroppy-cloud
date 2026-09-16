# mysql 8.4 / semi-sync / simple / smoke-2m-2vus

Область проверки: functional smoke and recorded telemetry checks; not full performance or fault readiness

Состояние исполнения: `completed`. Наблюдалось: `2026-09-15T23:36:40.846351+00:00`.

[Паспорт случая](case.json) · [Проверки](checks.json)

[Вход](input.json) — Full original run input, potentially containing multiple workloads. Preset is the historical label; actual segment parameters take precedence.

| Проверка | Статус | Основание |
|---|---|---|
| compile | passed | [доказательство](../../../../../platform/catalog/mysql-family-check.json) (`/catalog_cells/4`) |
| infrastructure_provisioning | unknown | No explicit retained verification for this check. |
| smoke | passed | [доказательство](../../../../../platform/catalog/mysql-family-check.json) (`/catalog_cells/4`) |
| native_tps | not_applicable | No logical transactions for this workload. |
| native_otlp_metrics | passed | [доказательство](../../../../../platform/catalog/mysql-family-check.json) (`/catalog_cells/4`) |
| component_metrics | passed | [доказательство](../../../../../platform/catalog/mysql-family-check.json) (`/catalog_cells/4`) |
| component_logs | passed | [доказательство](../../../../../platform/catalog/mysql-family-check.json) (`/catalog_cells/4`) |
| pipeline_traces | passed | [доказательство](../../../../../platform/catalog/mysql-family-check.json) (`/catalog_cells/4`) |
| database_metric_distributions | not_run | Not verified in the retained report. |
| managed_database_metrics | not_applicable | Not applicable to this database/topology; inherited scope is preserved. |
| artifacts | passed | [доказательство](../../../../../platform/catalog/mysql-family-check.json) (`/catalog_cells/4`) |
| replication | passed | [доказательство](../../../../../platform/catalog/mysql-family-check.json) (`/catalog_cells/4`) |
| cleanup | passed | [доказательство](../../../../../platform/catalog/mysql-family-check.json) (`/catalog_cells/4`) |

Прогоны:

- [runs/matrix-mysql-family-19fb11ff-mysql84-semi-sync](runs/matrix-mysql-family-19fb11ff-mysql84-semi-sync/manifest.json)

Предыдущие материалы (не текущее подтверждение):

- [mysql84-semi-sync-initial.replication.json](history/mysql84-semi-sync-initial.replication.json)

[Единая таблица](../../../../../progress.csv)
