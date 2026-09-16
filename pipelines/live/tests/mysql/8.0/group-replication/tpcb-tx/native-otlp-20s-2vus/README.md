# mysql 8.0 / group-replication / tpcb/tx / native-otlp-20s-2vus

Область проверки: functional smoke and recorded telemetry checks; not full performance or fault readiness

Состояние исполнения: `completed`. Наблюдалось: `2026-09-15T23:36:40.846351+00:00`.

[Паспорт случая](case.json) · [Проверки](checks.json)

[Вход](input.json) — Full original run input, potentially containing multiple workloads. Preset is the historical label; actual segment parameters take precedence.

| Проверка | Статус | Основание |
|---|---|---|
| compile | passed | [доказательство](../../../../../platform/catalog/mysql-clusters-check.json) (`/native_cells/4`) |
| infrastructure_provisioning | unknown | No explicit retained verification for this check. |
| smoke | passed | [доказательство](../../../../../platform/catalog/mysql-clusters-check.json) (`/native_cells/4`) |
| native_tps | passed | [доказательство](../../../../../platform/catalog/mysql-clusters-check.json) (`/native_cells/4`) |
| native_otlp_metrics | passed | [доказательство](../../../../../platform/catalog/mysql-clusters-check.json) (`/native_cells/4`) |
| component_metrics | passed | [доказательство](../../../../../platform/catalog/mysql-clusters-check.json) (`/native_cells/4`) |
| component_logs | passed | [доказательство](../../../../../platform/catalog/mysql-clusters-check.json) (`/native_cells/4`) |
| pipeline_traces | passed | [доказательство](../../../../../platform/catalog/mysql-clusters-check.json) (`/native_cells/4`) |
| database_metric_distributions | not_run | Not verified in the retained report. |
| managed_database_metrics | not_applicable | Not applicable to this database/topology; inherited scope is preserved. |
| artifacts | passed | [доказательство](../../../../../platform/catalog/mysql-clusters-check.json) (`/native_cells/4`) |
| replication | passed | [доказательство](../../../../../platform/catalog/mysql-clusters-check.json) (`/native_cells/4`) |
| cleanup | passed | [доказательство](../../../../../platform/catalog/mysql-clusters-check.json) (`/native_cells/4`) |

Прогоны:

- [runs/matrix-mysql-clusters-fixed-babb0afe-mysql-gr-80](runs/matrix-mysql-clusters-fixed-babb0afe-mysql-gr-80/manifest.json)

[Единая таблица](../../../../../progress.csv)
