# mariadb 10.11 / single / simple / smoke-2m-2vus

Область проверки: functional smoke and recorded telemetry checks; not full performance or fault readiness

Состояние исполнения: `completed`. Наблюдалось: `2026-09-15T23:36:40.846351+00:00`.

[Паспорт случая](case.json) · [Проверки](checks.json)

[Вход](input.json) — Full original run input, potentially containing multiple workloads. Preset is the historical label; actual segment parameters take precedence.

| Проверка | Статус | Основание |
|---|---|---|
| compile | passed | [доказательство](../../../../../platform/catalog/mysql-family-check.json) (`/catalog_cells/3`) |
| infrastructure_provisioning | unknown | No explicit retained verification for this check. |
| smoke | passed | [доказательство](../../../../../platform/catalog/mysql-family-check.json) (`/catalog_cells/3`) |
| native_tps | not_applicable | No logical transactions for this workload. |
| native_otlp_metrics | passed | [доказательство](../../../../../platform/catalog/mysql-family-check.json) (`/catalog_cells/3`) |
| component_metrics | passed | [доказательство](../../../../../platform/catalog/mysql-family-check.json) (`/catalog_cells/3`) |
| component_logs | passed | [доказательство](../../../../../platform/catalog/mysql-family-check.json) (`/catalog_cells/3`) |
| pipeline_traces | passed | [доказательство](../../../../../platform/catalog/mysql-family-check.json) (`/catalog_cells/3`) |
| database_metric_distributions | not_run | Not verified in the retained report. |
| managed_database_metrics | not_applicable | Not applicable to this database/topology; inherited scope is preserved. |
| artifacts | passed | [доказательство](../../../../../platform/catalog/mysql-family-check.json) (`/catalog_cells/3`) |
| replication | not_applicable | Not applicable to this database/topology; inherited scope is preserved. |
| cleanup | passed | [доказательство](../../../../../platform/catalog/mysql-family-check.json) (`/catalog_cells/3`) |

Прогоны:

- [runs/matrix-maria-monitor-e4a0fdb4-mariadb-single-1011](runs/matrix-maria-monitor-e4a0fdb4-mariadb-single-1011/manifest.json)

Предыдущие материалы (не текущее подтверждение):

- [mariadb1011-single.artifacts.json](history/mariadb1011-single.artifacts.json)
- [mariadb1011-single.collector-check.json](history/mariadb1011-single.collector-check.json)
- [mariadb1011-single.load-progress.json](history/mariadb1011-single.load-progress.json)
- [mariadb1011-single.logs.json](history/mariadb1011-single.logs.json)
- [mariadb1011-single.metrics.json](history/mariadb1011-single.metrics.json)
- [mariadb1011-single.native.json](history/mariadb1011-single.native.json)
- [mariadb1011-single.result.json](history/mariadb1011-single.result.json)
- [mariadb1011-single.traces.json](history/mariadb1011-single.traces.json)

[Единая таблица](../../../../../progress.csv)
