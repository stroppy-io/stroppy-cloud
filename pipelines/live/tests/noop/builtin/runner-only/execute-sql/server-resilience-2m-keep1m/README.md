# noop builtin / runner-only / execute/sql / server-resilience-2m-keep1m

Область проверки: real YC run launched and observed through the local server API

Состояние исполнения: `cancelled`. Наблюдалось: `2026-09-27T16:10:12.436494+00:00`.

[Паспорт случая](case.json) · [Проверки](checks.json)

[Вход](input.json) — Compiled RunSpec from the server API; operator OTLP headers omitted.

| Проверка | Статус | Основание |
|---|---|---|
| compile | passed | [доказательство](runs/e3e18f98-f2b4-40dc-93da-e035ff75814f/run.json) (`/body`) |
| infrastructure_provisioning | passed | [доказательство](runs/e3e18f98-f2b4-40dc-93da-e035ff75814f/events.json) (`/body/data`) |
| smoke | blocked | [доказательство](../../../../../platform/server/resilience/live-suite-20260927/cancel-intent.json) (`/reason`) |
| native_tps | not_applicable | The workload has no logical transaction TPS. |
| native_otlp_metrics | blocked | [доказательство](../../../../../platform/server/resilience/live-suite-20260927/cancel-intent.json) (`/reason`) |
| component_metrics | blocked | [доказательство](../../../../../platform/server/resilience/live-suite-20260927/cancel-intent.json) (`/reason`) |
| component_logs | blocked | [доказательство](../../../../../platform/server/resilience/live-suite-20260927/cancel-intent.json) (`/reason`) |
| pipeline_traces | not_run | No explicit acceptance evidence recorded for this check. |
| database_metric_distributions | not_applicable | Runner-only test has no database. |
| managed_database_metrics | not_applicable | Runner-only test has no database. |
| artifacts | blocked | [доказательство](../../../../../platform/server/resilience/live-suite-20260927/cancel-intent.json) (`/reason`) |
| replication | not_applicable | Runner-only test has no database. |
| cleanup | passed | [доказательство](../../../../../platform/server/resilience/live-suite-20260927/cleanup-e3e18f98-f2b4-40dc-93da-e035ff75814f.json) (`/verified`) |

Прогоны:

- [runs/e3e18f98-f2b4-40dc-93da-e035ff75814f](runs/e3e18f98-f2b4-40dc-93da-e035ff75814f/manifest.json)

[Единая таблица](../../../../../progress.csv)
