# mariadb 11.8 / single / simple / smoke-2m-2vus

Область проверки: functional smoke and recorded telemetry checks; not full performance or fault readiness

Состояние исполнения: `completed`. Наблюдалось: `2026-09-15T23:36:40.846351+00:00`.

[Паспорт случая](case.json) · [Проверки](checks.json)

[Вход](input.json) — Full original run input, potentially containing multiple workloads. Preset is the historical label; actual segment parameters take precedence.

| Проверка | Статус | Основание |
|---|---|---|
| compile | passed | [доказательство](../../../../../platform/catalog/mysql-family-check.json) (`/catalog_cells/6`) |
| infrastructure_provisioning | unknown | No explicit retained verification for this check. |
| smoke | passed | [доказательство](../../../../../platform/catalog/mysql-family-check.json) (`/catalog_cells/6`) |
| native_tps | not_applicable | No logical transactions for this workload. |
| native_otlp_metrics | passed | [доказательство](../../../../../platform/catalog/mysql-family-check.json) (`/catalog_cells/6`) |
| component_metrics | passed | [доказательство](../../../../../platform/catalog/mysql-family-check.json) (`/catalog_cells/6`) |
| component_logs | passed | [доказательство](../../../../../platform/catalog/mysql-family-check.json) (`/catalog_cells/6`) |
| pipeline_traces | passed | [доказательство](../../../../../platform/catalog/mysql-family-check.json) (`/catalog_cells/6`) |
| database_metric_distributions | not_run | Not verified in the retained report. |
| managed_database_metrics | not_applicable | Not applicable to this database/topology; inherited scope is preserved. |
| artifacts | passed | [доказательство](../../../../../platform/catalog/mysql-family-check.json) (`/catalog_cells/6`) |
| replication | not_applicable | Not applicable to this database/topology; inherited scope is preserved. |
| cleanup | passed | [доказательство](../../../../../platform/catalog/mysql-family-check.json) (`/catalog_cells/6`) |

Прогоны:

- [runs/matrix-maria-monitor-e4a0fdb4-mariadb-single-118](runs/matrix-maria-monitor-e4a0fdb4-mariadb-single-118/manifest.json)

Предыдущие материалы (не текущее подтверждение):

- [maria118-initial.artifacts.json](history/maria118-initial.artifacts.json)
- [maria118-initial.logs.json](history/maria118-initial.logs.json)
- [maria118-initial.metrics.json](history/maria118-initial.metrics.json)
- [maria118-initial.native.json](history/maria118-initial.native.json)
- [maria118-initial.result.json](history/maria118-initial.result.json)
- [maria118-initial.traces.json](history/maria118-initial.traces.json)
- [mariadb118-single.artifacts.json](history/mariadb118-single.artifacts.json)
- [mariadb118-single.collector-check.json](history/mariadb118-single.collector-check.json)
- [mariadb118-single.load-progress.json](history/mariadb118-single.load-progress.json)
- [mariadb118-single.logs.json](history/mariadb118-single.logs.json)
- [mariadb118-single.metrics.json](history/mariadb118-single.metrics.json)
- [mariadb118-single.native.json](history/mariadb118-single.native.json)
- [mariadb118-single.result.json](history/mariadb118-single.result.json)
- [mariadb118-single.traces.json](history/mariadb118-single.traces.json)

[Единая таблица](../../../../../progress.csv)
