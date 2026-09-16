# postgres 17 / single / tpcc/procs / native-otlp-20s-2vus-retry50

Область проверки: functional smoke and recorded telemetry checks; not full performance or fault readiness

Состояние исполнения: `completed`. Наблюдалось: `2026-09-15T23:36:40.846351+00:00`.

[Паспорт случая](case.json) · [Проверки](checks.json)

[Вход](input.json) — Full original run input, potentially containing multiple workloads. Preset is the historical label; actual segment parameters take precedence.

| Проверка | Статус | Основание |
|---|---|---|
| compile | passed | [доказательство](../../../../../platform/catalog/native-export-check.json) (`/cells/4`) |
| infrastructure_provisioning | unknown | No explicit retained verification for this check. |
| smoke | passed | [доказательство](../../../../../platform/catalog/native-export-check.json) (`/cells/4`) |
| native_tps | passed | [доказательство](../../../../../platform/catalog/native-export-check.json) (`/cells/4`) |
| native_otlp_metrics | passed | [доказательство](../../../../../platform/catalog/native-export-check.json) (`/cells/4`) |
| component_metrics | passed | [доказательство](../../../../../platform/catalog/native-export-check.json) (`/cells/4`) |
| component_logs | not_run | Not verified in the retained report. |
| pipeline_traces | passed | [доказательство](../../../../../platform/catalog/native-export-check.json) (`/cells/4`) |
| database_metric_distributions | not_run | Not verified in the retained report. |
| managed_database_metrics | not_applicable | Not applicable to this database/topology; inherited scope is preserved. |
| artifacts | not_run | Not verified in the retained report. |
| replication | not_run | Not verified in the retained report. |
| cleanup | passed | [доказательство](../../../../../platform/catalog/native-export-check.json) (`/cells/4`) |

Прогоны:

- [runs/native-tpcc-77fa0211-tpcc](runs/native-tpcc-77fa0211-tpcc/manifest.json)

Предыдущие материалы (не текущее подтверждение):

- [native-tpcc-contention.components.json](history/native-tpcc-contention.components.json)
- [native-tpcc-contention.logs.json](history/native-tpcc-contention.logs.json)
- [native-tpcc-contention.metrics.json](history/native-tpcc-contention.metrics.json)
- [native-tpcc-contention.result.json](history/native-tpcc-contention.result.json)
- [native-tpcc-contention.traces.json](history/native-tpcc-contention.traces.json)

[Единая таблица](../../../../../progress.csv)
