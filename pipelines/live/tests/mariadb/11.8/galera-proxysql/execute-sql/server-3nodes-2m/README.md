# mariadb 11.8 / galera-proxysql / execute/sql / server-3nodes-2m

Область проверки: real YC run launched and observed through the local server API

Состояние исполнения: `completed`. Наблюдалось: `2026-09-27T17:20:02.318861+00:00`.

[Паспорт случая](case.json) · [Проверки](checks.json)

[Вход](input.json) — Compiled RunSpec from the server API; operator OTLP headers omitted.

| Проверка | Статус | Основание |
|---|---|---|
| compile | passed | [доказательство](runs/5fc7ddd6-5c13-4be7-b51b-21bf19277540/run.json) (`/body`) |
| infrastructure_provisioning | passed | [доказательство](runs/5fc7ddd6-5c13-4be7-b51b-21bf19277540/run.json) (`/body/result`) |
| smoke | passed | [доказательство](../../../../../platform/server/resilience/ha-20260927/mariadb/cluster-health.json) (`/verified`) |
| native_tps | not_applicable | This test reports native queries/s and iterations/s, not logical transaction TPS. |
| native_otlp_metrics | passed | [доказательство](../../../../../platform/server/resilience/ha-20260927/mariadb/telemetry.json) (`/native_verified`) |
| component_metrics | failed | [доказательство](../../../../../platform/server/resilience/ha-20260927/mariadb/telemetry.json) (`/invalid_metric_values`) |
| component_logs | passed | [доказательство](../../../../../platform/server/resilience/ha-20260927/mariadb/telemetry.json) (`/logs_verified`) |
| pipeline_traces | not_run | No explicit acceptance evidence recorded for this check. |
| database_metric_distributions | not_run | No explicit acceptance evidence recorded for this check. |
| managed_database_metrics | not_applicable | Self-managed database. |
| artifacts | passed | [доказательство](../../../../../platform/server/resilience/ha-20260927/mariadb/artifacts.json) (`/verified`) |
| replication | not_run | [доказательство](../../../../../platform/server/resilience/ha-20260927/mariadb/cluster-health.json) (`/scope`) |
| cleanup | passed | [доказательство](../../../../../platform/server/resilience/ha-20260927/mariadb/cleanup.json) (`/verified`) |

Прогоны:

- [runs/5fc7ddd6-5c13-4be7-b51b-21bf19277540](runs/5fc7ddd6-5c13-4be7-b51b-21bf19277540/manifest.json)

[Единая таблица](../../../../../progress.csv)
