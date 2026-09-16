# mariadb 10.11 / single / tpcc/tx / native-otlp-20s-2vus-retry50

Область проверки: functional smoke and recorded telemetry checks; not full performance or fault readiness

Состояние исполнения: `completed`. Наблюдалось: `2026-09-15T23:36:40.846351+00:00`.

[Паспорт случая](case.json) · [Проверки](checks.json)

[Вход](input.json) — Full original run input, potentially containing multiple workloads. Preset is the historical label; actual segment parameters take precedence.

| Проверка | Статус | Основание |
|---|---|---|
| compile | passed | [доказательство](../../../../../platform/catalog/mysql-family-check.json) (`/native_cells/26`) |
| infrastructure_provisioning | unknown | No explicit retained verification for this check. |
| smoke | passed | [доказательство](../../../../../platform/catalog/mysql-family-check.json) (`/native_cells/26`) |
| native_tps | passed | [доказательство](../../../../../platform/catalog/mysql-family-check.json) (`/native_cells/26`) |
| native_otlp_metrics | passed | [доказательство](../../../../../platform/catalog/mysql-family-check.json) (`/native_cells/26`) |
| component_metrics | passed | [доказательство](../../../../../platform/catalog/mysql-family-check.json) (`/native_cells/26`) |
| component_logs | passed | [доказательство](../../../../../platform/catalog/mysql-family-check.json) (`/native_cells/26`) |
| pipeline_traces | passed | [доказательство](../../../../../platform/catalog/mysql-family-check.json) (`/native_cells/26`) |
| database_metric_distributions | not_run | Not verified in the retained report. |
| managed_database_metrics | not_applicable | Not applicable to this database/topology; inherited scope is preserved. |
| artifacts | passed | [доказательство](../../../../../platform/catalog/mysql-family-check.json) (`/native_cells/26`) |
| replication | not_applicable | Not applicable to this database/topology; inherited scope is preserved. |
| cleanup | passed | [доказательство](../../../../../platform/catalog/mysql-family-check.json) (`/native_cells/26`) |

Прогоны:

- [runs/matrix-maria-monitor-e4a0fdb4-mariadb-single-1011](runs/matrix-maria-monitor-e4a0fdb4-mariadb-single-1011/manifest.json)

[Единая таблица](../../../../../progress.csv)
