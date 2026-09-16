# mariadb 10.11 / galera / tpcc/procs / native-otlp-20s-2vus-retry50

Область проверки: functional smoke and recorded telemetry checks; not full performance or fault readiness

Состояние исполнения: `completed`. Наблюдалось: `2026-09-15T23:36:40.846351+00:00`.

[Паспорт случая](case.json) · [Проверки](checks.json)

[Вход](input.json) — Full original run input, potentially containing multiple workloads. Preset is the historical label; actual segment parameters take precedence.

| Проверка | Статус | Основание |
|---|---|---|
| compile | passed | [доказательство](../../../../../platform/catalog/mysql-clusters-check.json) (`/native_cells/19`) |
| infrastructure_provisioning | unknown | No explicit retained verification for this check. |
| smoke | passed | [доказательство](../../../../../platform/catalog/mysql-clusters-check.json) (`/native_cells/19`) |
| native_tps | passed | [доказательство](../../../../../platform/catalog/mysql-clusters-check.json) (`/native_cells/19`) |
| native_otlp_metrics | passed | [доказательство](../../../../../platform/catalog/mysql-clusters-check.json) (`/native_cells/19`) |
| component_metrics | passed | [доказательство](../../../../../platform/catalog/mysql-clusters-check.json) (`/native_cells/19`) |
| component_logs | passed | [доказательство](../../../../../platform/catalog/mysql-clusters-check.json) (`/native_cells/19`) |
| pipeline_traces | passed | [доказательство](../../../../../platform/catalog/mysql-clusters-check.json) (`/native_cells/19`) |
| database_metric_distributions | not_run | Not verified in the retained report. |
| managed_database_metrics | not_applicable | Not applicable to this database/topology; inherited scope is preserved. |
| artifacts | passed | [доказательство](../../../../../platform/catalog/mysql-clusters-check.json) (`/native_cells/19`) |
| replication | passed | [доказательство](../../../../../platform/catalog/mysql-clusters-check.json) (`/native_cells/19`) |
| cleanup | passed | [доказательство](../../../../../platform/catalog/mysql-clusters-check.json) (`/native_cells/19`) |

Прогоны:

- [runs/matrix-maria-monitor-e4a0fdb4-mariadb-galera-1011](runs/matrix-maria-monitor-e4a0fdb4-mariadb-galera-1011/manifest.json)

[Единая таблица](../../../../../progress.csv)
