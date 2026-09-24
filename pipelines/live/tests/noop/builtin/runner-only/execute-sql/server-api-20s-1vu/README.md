# noop builtin / runner-only / execute/sql / server-api-20s-1vu

Область проверки: real YC run launched and observed through the local server API

Состояние исполнения: `completed`. Наблюдалось: `2026-09-24T15:39:58.088605+00:00`.

[Паспорт случая](case.json) · [Проверки](checks.json)

[Вход](input.json) — Compiled RunSpec from the server API; operator OTLP headers omitted.

| Проверка | Статус | Основание |
|---|---|---|
| compile | passed | [доказательство](runs/731d5ee3-e248-4176-bd53-8bf0b6e5223e/run.json) (`/body`) |
| infrastructure_provisioning | passed | [доказательство](runs/731d5ee3-e248-4176-bd53-8bf0b6e5223e/events.json) (`/body/data`) |
| smoke | passed | [доказательство](runs/731d5ee3-e248-4176-bd53-8bf0b6e5223e/run.json) (`/body/result`) |
| native_tps | not_applicable | The workload has no logical transaction TPS. |
| native_otlp_metrics | not_run | This acceptance run has no configured OTLP endpoint or Victoria store. Native result metrics and Stroppy log are verified separately. |
| component_metrics | not_run | This acceptance run has no configured OTLP endpoint or Victoria store. Native result metrics and Stroppy log are verified separately. |
| component_logs | not_run | This acceptance run has no configured OTLP endpoint or Victoria store. Native result metrics and Stroppy log are verified separately. |
| pipeline_traces | not_run | No explicit acceptance evidence recorded for this check. |
| database_metric_distributions | not_applicable | Runner-only test has no database. |
| managed_database_metrics | not_applicable | Runner-only test has no database. |
| artifacts | passed | [доказательство](../../../../../platform/server/2026-09-24/noop-artifact-downloads.json) (`/verified`) |
| replication | not_applicable | Runner-only test has no database. |
| cleanup | passed | [доказательство](../../../../../platform/server/2026-09-24/noop-success-cleanup.json) (`/yc_not_found`) |

Прогоны:

- [runs/bc94e0ff-2a4f-4e76-a85c-faf9e013e528](runs/bc94e0ff-2a4f-4e76-a85c-faf9e013e528/manifest.json)
- [runs/5f04e4bc-1c83-4e7b-ad1f-88f599403e98](runs/5f04e4bc-1c83-4e7b-ad1f-88f599403e98/manifest.json)
- [runs/731d5ee3-e248-4176-bd53-8bf0b6e5223e](runs/731d5ee3-e248-4176-bd53-8bf0b6e5223e/manifest.json)

[Единая таблица](../../../../../progress.csv)
