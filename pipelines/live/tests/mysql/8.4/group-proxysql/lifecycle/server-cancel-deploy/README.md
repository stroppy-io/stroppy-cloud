# mysql 8.4 / group-proxysql / execute/sql / server-cancel-deploy

Область проверки: real YC run launched and observed through the local server API

Состояние исполнения: `cancelled`. Наблюдалось: `2026-09-27T19:14:33.547886+00:00`.

[Паспорт случая](case.json) · [Проверки](checks.json)

[Вход](input.json) — Compiled RunSpec from the server API; operator OTLP headers omitted.

| Проверка | Статус | Основание |
|---|---|---|
| compile | passed | [доказательство](runs/421dd494-143c-43b0-8d37-62b0a4811c73/run.json) (`/body`) |
| infrastructure_provisioning | passed | [доказательство](../../../../../platform/server/resilience/cancel-deploy-graphene027-20260927/verification.json) (`/verified`) |
| smoke | passed | [доказательство](../../../../../platform/server/resilience/cancel-deploy-graphene027-20260927/verification.json) (`/verified`) |
| native_tps | not_applicable | The workload has no logical transaction TPS. |
| native_otlp_metrics | not_run | No explicit acceptance evidence recorded for this check. |
| component_metrics | not_run | No explicit acceptance evidence recorded for this check. |
| component_logs | not_run | No explicit acceptance evidence recorded for this check. |
| pipeline_traces | not_run | No explicit acceptance evidence recorded for this check. |
| database_metric_distributions | not_run | No explicit acceptance evidence recorded for this check. |
| managed_database_metrics | not_run | No explicit acceptance evidence recorded for this check. |
| artifacts | not_run | No explicit acceptance evidence recorded for this check. |
| replication | not_run | No explicit acceptance evidence recorded for this check. |
| cleanup | passed | [доказательство](../../../../../platform/server/resilience/cancel-deploy-graphene027-20260927/verification.json) (`/verified`) |

Прогоны:

- [runs/421dd494-143c-43b0-8d37-62b0a4811c73](runs/421dd494-143c-43b0-8d37-62b0a4811c73/manifest.json)

[Единая таблица](../../../../../progress.csv)
