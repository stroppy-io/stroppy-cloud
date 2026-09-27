# noop builtin / runner-only / execute/sql / server-api-90s-1vu

Область проверки: real YC run launched and observed through the local server API

Состояние исполнения: `completed`. Наблюдалось: `2026-09-25T13:50:43.351825+00:00`.

[Паспорт случая](case.json) · [Проверки](checks.json)

[Вход](input.json) — Compiled RunSpec from the server API; operator OTLP headers omitted.

| Проверка | Статус | Основание |
|---|---|---|
| compile | passed | [доказательство](runs/d92548ed-af3a-425f-a896-14a90f58bf01/run.json) (`/body`) |
| infrastructure_provisioning | passed | [доказательство](runs/d92548ed-af3a-425f-a896-14a90f58bf01/run.json) (`/body/result`) |
| smoke | passed | [доказательство](runs/d92548ed-af3a-425f-a896-14a90f58bf01/run.json) (`/body/result`) |
| native_tps | not_applicable | The workload has no logical transaction TPS. |
| native_otlp_metrics | passed | [доказательство](../../../../../platform/server/2026-09-25/noop-90s-final-metrics.json) (`/verified`) |
| component_metrics | passed | [доказательство](../../../../../platform/server/2026-09-25/noop-90s-rerun-telemetry.json) (`/metric_series`) |
| component_logs | passed | [доказательство](../../../../../platform/server/2026-09-25/noop-90s-rerun-telemetry.json) (`/http`) |
| pipeline_traces | not_run | No explicit acceptance evidence recorded for this check. |
| database_metric_distributions | not_applicable | Runner-only test has no database. |
| managed_database_metrics | not_applicable | Runner-only test has no database. |
| artifacts | passed | [доказательство](../../../../../platform/server/2026-09-25/noop-90s-rerun-artifact-downloads.json) (`/verified`) |
| replication | not_applicable | Runner-only test has no database. |
| cleanup | passed | [доказательство](runs/d92548ed-af3a-425f-a896-14a90f58bf01/tree.json) (`/body`) |

Прогоны:

- [runs/feb7be00-c574-40ab-aae7-b183b85ba793](runs/feb7be00-c574-40ab-aae7-b183b85ba793/manifest.json)
- [runs/d92548ed-af3a-425f-a896-14a90f58bf01](runs/d92548ed-af3a-425f-a896-14a90f58bf01/manifest.json)

[Единая таблица](../../../../../progress.csv)
