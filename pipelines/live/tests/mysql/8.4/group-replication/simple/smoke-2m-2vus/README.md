# mysql 8.4 / group-replication / simple / smoke-2m-2vus

Область проверки: functional smoke and recorded telemetry checks; not full performance or fault readiness

Состояние исполнения: `completed`. Наблюдалось: `2026-09-15T23:36:40.846351+00:00`.

[Паспорт случая](case.json) · [Проверки](checks.json)

[Вход](input.json) — Full original run input, potentially containing multiple workloads. Preset is the historical label; actual segment parameters take precedence.

| Проверка | Статус | Основание |
|---|---|---|
| compile | passed | [доказательство](../../../../../platform/catalog/mysql-clusters-check.json) (`/catalog_cells/0`) |
| infrastructure_provisioning | unknown | No explicit retained verification for this check. |
| smoke | passed | [доказательство](../../../../../platform/catalog/mysql-clusters-check.json) (`/catalog_cells/0`) |
| native_tps | not_applicable | No logical transactions for this workload. |
| native_otlp_metrics | passed | [доказательство](../../../../../platform/catalog/mysql-clusters-check.json) (`/catalog_cells/0`) |
| component_metrics | passed | [доказательство](../../../../../platform/catalog/mysql-clusters-check.json) (`/catalog_cells/0`) |
| component_logs | passed | [доказательство](../../../../../platform/catalog/mysql-clusters-check.json) (`/catalog_cells/0`) |
| pipeline_traces | passed | [доказательство](../../../../../platform/catalog/mysql-clusters-check.json) (`/catalog_cells/0`) |
| database_metric_distributions | not_run | Not verified in the retained report. |
| managed_database_metrics | not_applicable | Not applicable to this database/topology; inherited scope is preserved. |
| artifacts | passed | [доказательство](../../../../../platform/catalog/mysql-clusters-check.json) (`/catalog_cells/0`) |
| replication | passed | [доказательство](../../../../../platform/catalog/mysql-clusters-check.json) (`/catalog_cells/0`) |
| cleanup | passed | [доказательство](../../../../../platform/catalog/mysql-clusters-check.json) (`/catalog_cells/0`) |

Прогоны:

- [runs/matrix-mysql-clusters-fixed-babb0afe-mysql-gr-84](runs/matrix-mysql-clusters-fixed-babb0afe-mysql-gr-84/manifest.json)

Предыдущие материалы (не текущее подтверждение):

- [mysql-gr-84-initial.artifacts.json](history/mysql-gr-84-initial.artifacts.json)
- [mysql-gr-84-initial.logs.json](history/mysql-gr-84-initial.logs.json)
- [mysql-gr-84-initial.metrics.json](history/mysql-gr-84-initial.metrics.json)
- [mysql-gr-84-initial.native.json](history/mysql-gr-84-initial.native.json)
- [mysql-gr-84-initial.replication-metrics.json](history/mysql-gr-84-initial.replication-metrics.json)
- [mysql-gr-84-initial.replication.json](history/mysql-gr-84-initial.replication.json)
- [mysql-gr-84-initial.result.json](history/mysql-gr-84-initial.result.json)
- [mysql-gr-84-initial.traces.json](history/mysql-gr-84-initial.traces.json)

[Единая таблица](../../../../../progress.csv)
