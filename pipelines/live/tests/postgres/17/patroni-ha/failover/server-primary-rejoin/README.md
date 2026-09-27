# postgres 17 / patroni-ha / execute/sql / server-primary-rejoin

Область проверки: real YC run launched and observed through the local server API

Состояние исполнения: `completed`. Наблюдалось: `2026-09-27T20:27:01.342858+00:00`.

[Паспорт случая](case.json) · [Проверки](checks.json)

[Вход](input.json) — Compiled RunSpec from the server API; operator OTLP headers omitted.

| Проверка | Статус | Основание |
|---|---|---|
| compile | passed | [доказательство](runs/4790c0e6-69b1-4ad1-8e3f-a5a06594827d/run.json) (`/body`) |
| infrastructure_provisioning | passed | [доказательство](../../../../../platform/server/resilience/ha-failover-gated-20260927/postgres/final.json) (`/body/result`) |
| smoke | passed | [доказательство](../../../../../platform/server/resilience/ha-failover-gated-20260927/postgres/ha-verification.json) (`/verified`) |
| native_tps | not_applicable | execute_sql reports native queries/s rather than logical transaction TPS. |
| native_otlp_metrics | passed | [доказательство](../../../../../platform/server/resilience/ha-failover-gated-20260927/postgres/telemetry.json) (`/native_verified`) |
| component_metrics | passed | [доказательство](../../../../../platform/server/resilience/ha-failover-gated-20260927/postgres/cpu-tail.json) (`/verified`) |
| component_logs | passed | [доказательство](../../../../../platform/server/resilience/ha-failover-gated-20260927/postgres/telemetry.json) (`/logs_verified`) |
| pipeline_traces | not_run | No explicit acceptance evidence recorded for this check. |
| database_metric_distributions | not_run | No explicit acceptance evidence recorded for this check. |
| managed_database_metrics | not_applicable | Self-managed database. |
| artifacts | passed | [доказательство](../../../../../platform/server/resilience/ha-failover-gated-20260927/postgres/artifacts.json) (`/verified`) |
| replication | passed | [доказательство](../../../../../platform/server/resilience/ha-failover-gated-20260927/postgres/ha-verification.json) (`/verified`) |
| cleanup | passed | [доказательство](../../../../../platform/server/resilience/ha-failover-gated-20260927/postgres/cleanup.json) (`/verified`) |

Прогоны:

- [runs/48021b46-d718-4dde-b0fb-a2aca859ec8f](runs/48021b46-d718-4dde-b0fb-a2aca859ec8f/manifest.json)
- [runs/4790c0e6-69b1-4ad1-8e3f-a5a06594827d](runs/4790c0e6-69b1-4ad1-8e3f-a5a06594827d/manifest.json)

[Единая таблица](../../../../../progress.csv)
