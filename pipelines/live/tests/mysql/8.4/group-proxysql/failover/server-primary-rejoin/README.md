# mysql 8.4 / group-proxysql / execute/sql / server-primary-rejoin

Область проверки: real YC run launched and observed through the local server API

Состояние исполнения: `completed`. Наблюдалось: `2026-09-27T19:57:52.426862+00:00`.

[Паспорт случая](case.json) · [Проверки](checks.json)

[Вход](input.json) — Compiled RunSpec from the server API; operator OTLP headers omitted.

| Проверка | Статус | Основание |
|---|---|---|
| compile | passed | [доказательство](runs/d1c531ed-2199-4049-98b2-44ccfa07a05e/run.json) (`/body`) |
| infrastructure_provisioning | passed | [доказательство](../../../../../platform/server/resilience/ha-failover-20260927/mysql/final.json) (`/body/result`) |
| smoke | passed | [доказательство](../../../../../platform/server/resilience/ha-failover-20260927/mysql/ha-verification.json) (`/verified`) |
| native_tps | not_applicable | execute_sql reports native queries/s rather than logical transaction TPS. |
| native_otlp_metrics | passed | [доказательство](../../../../../platform/server/resilience/ha-failover-20260927/mysql/telemetry.json) (`/native_verified`) |
| component_metrics | passed | [доказательство](../../../../../platform/server/resilience/ha-failover-20260927/mysql/cpu-tail.json) (`/verified`) |
| component_logs | passed | [доказательство](../../../../../platform/server/resilience/ha-failover-20260927/mysql/telemetry.json) (`/logs_verified`) |
| pipeline_traces | not_run | No explicit acceptance evidence recorded for this check. |
| database_metric_distributions | not_run | No explicit acceptance evidence recorded for this check. |
| managed_database_metrics | not_applicable | Self-managed database. |
| artifacts | passed | [доказательство](../../../../../platform/server/resilience/ha-failover-20260927/mysql/artifacts.json) (`/verified`) |
| replication | passed | [доказательство](../../../../../platform/server/resilience/ha-failover-20260927/mysql/ha-verification.json) (`/verified`) |
| cleanup | passed | [доказательство](../../../../../platform/server/resilience/ha-failover-20260927/mysql/cleanup.json) (`/verified`) |

Прогоны:

- [runs/d1c531ed-2199-4049-98b2-44ccfa07a05e](runs/d1c531ed-2199-4049-98b2-44ccfa07a05e/manifest.json)

[Единая таблица](../../../../../progress.csv)
