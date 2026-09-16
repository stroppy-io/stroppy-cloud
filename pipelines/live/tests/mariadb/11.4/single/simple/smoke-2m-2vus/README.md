# mariadb 11.4 / single / simple / smoke-2m-2vus

Область проверки: functional smoke and recorded telemetry checks; not full performance or fault readiness

Состояние исполнения: `completed`. Наблюдалось: `2026-09-15T23:36:40.846351+00:00`.

[Паспорт случая](case.json) · [Проверки](checks.json)

[Вход](input.json) — Full original run input, potentially containing multiple workloads. Preset is the historical label; actual segment parameters take precedence.

| Проверка | Статус | Основание |
|---|---|---|
| compile | passed | [доказательство](../../../../../platform/catalog/mysql-family-check.json) (`/catalog_cells/1`) |
| infrastructure_provisioning | unknown | No explicit retained verification for this check. |
| smoke | passed | [доказательство](../../../../../platform/catalog/mysql-family-check.json) (`/catalog_cells/1`) |
| native_tps | not_applicable | No logical transactions for this workload. |
| native_otlp_metrics | passed | [доказательство](../../../../../platform/catalog/mysql-family-check.json) (`/catalog_cells/1`) |
| component_metrics | passed | [доказательство](../../../../../platform/catalog/mysql-family-check.json) (`/catalog_cells/1`) |
| component_logs | passed | [доказательство](../../../../../platform/catalog/mysql-family-check.json) (`/catalog_cells/1`) |
| pipeline_traces | passed | [доказательство](../../../../../platform/catalog/mysql-family-check.json) (`/catalog_cells/1`) |
| database_metric_distributions | not_run | Not verified in the retained report. |
| managed_database_metrics | not_applicable | Not applicable to this database/topology; inherited scope is preserved. |
| artifacts | passed | [доказательство](../../../../../platform/catalog/mysql-family-check.json) (`/catalog_cells/1`) |
| replication | not_applicable | Not applicable to this database/topology; inherited scope is preserved. |
| cleanup | passed | [доказательство](../../../../../platform/catalog/mysql-family-check.json) (`/catalog_cells/1`) |

Прогоны:

- [runs/matrix-maria-monitor-e4a0fdb4-mariadb-single-114](runs/matrix-maria-monitor-e4a0fdb4-mariadb-single-114/manifest.json)

Предыдущие материалы (не текущее подтверждение):

- [maria114-single.artifacts.json](history/maria114-single.artifacts.json)
- [maria114-single.logs.json](history/maria114-single.logs.json)
- [maria114-single.metrics.json](history/maria114-single.metrics.json)
- [maria114-single.native.json](history/maria114-single.native.json)
- [maria114-single.result.json](history/maria114-single.result.json)
- [maria114-single.traces.json](history/maria114-single.traces.json)
- [mariadb114-single.artifacts.json](history/mariadb114-single.artifacts.json)
- [mariadb114-single.collector-check.json](history/mariadb114-single.collector-check.json)
- [mariadb114-single.load-progress.json](history/mariadb114-single.load-progress.json)
- [mariadb114-single.logs.json](history/mariadb114-single.logs.json)
- [mariadb114-single.metrics.json](history/mariadb114-single.metrics.json)
- [mariadb114-single.native.json](history/mariadb114-single.native.json)
- [mariadb114-single.result.json](history/mariadb114-single.result.json)
- [mariadb114-single.traces.json](history/mariadb114-single.traces.json)

[Единая таблица](../../../../../progress.csv)
