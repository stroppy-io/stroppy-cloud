# noop builtin / runner-only / execute/sql / server-api-20s-1vu

Область проверки: real YC run launched and observed through the local server API

Состояние исполнения: `completed`. Наблюдалось: `2026-09-25T13:25:56.203052+00:00`.

[Паспорт случая](case.json) · [Проверки](checks.json)

[Вход](input.json) — Compiled RunSpec from the server API; operator OTLP headers omitted.

| Проверка | Статус | Основание |
|---|---|---|
| compile | passed | [доказательство](runs/d7003247-6d13-4c77-92b2-6267e1c63281/run.json) (`/body`) |
| infrastructure_provisioning | passed | [доказательство](runs/d7003247-6d13-4c77-92b2-6267e1c63281/run.json) (`/body/status`) |
| smoke | passed | [доказательство](runs/d7003247-6d13-4c77-92b2-6267e1c63281/run.json) (`/body/result`) |
| native_tps | not_applicable | The workload has no logical transaction TPS. |
| native_otlp_metrics | passed | [доказательство](../../../../../platform/server/2026-09-25/noop-telemetry.json) (`/native_series_count`) |
| component_metrics | failed | [доказательство](../../../../../platform/server/2026-09-25/noop-telemetry.json) (`/metric_series`) |
| component_logs | passed | [доказательство](../../../../../platform/server/2026-09-25/noop-telemetry.json) (`/log_page_sizes`) |
| pipeline_traces | not_run | No explicit acceptance evidence recorded for this check. |
| database_metric_distributions | not_applicable | Runner-only test has no database. |
| managed_database_metrics | not_applicable | Runner-only test has no database. |
| artifacts | passed | [доказательство](../../../../../platform/server/2026-09-25/noop-artifact-downloads.json) (`/verified`) |
| replication | not_applicable | Runner-only test has no database. |
| cleanup | passed | [доказательство](../../../../../platform/server/2026-09-25/noop-cleanup-tree.json) (`/body`) |

Прогоны:

- [runs/bc94e0ff-2a4f-4e76-a85c-faf9e013e528](runs/bc94e0ff-2a4f-4e76-a85c-faf9e013e528/manifest.json)
- [runs/5f04e4bc-1c83-4e7b-ad1f-88f599403e98](runs/5f04e4bc-1c83-4e7b-ad1f-88f599403e98/manifest.json)
- [runs/731d5ee3-e248-4176-bd53-8bf0b6e5223e](runs/731d5ee3-e248-4176-bd53-8bf0b6e5223e/manifest.json)
- [runs/d7003247-6d13-4c77-92b2-6267e1c63281](runs/d7003247-6d13-4c77-92b2-6267e1c63281/manifest.json)

[Единая таблица](../../../../../progress.csv)
