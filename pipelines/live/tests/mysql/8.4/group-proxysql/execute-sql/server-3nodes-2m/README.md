# mysql 8.4 / group-proxysql / execute/sql / server-3nodes-2m

Область проверки: real YC run launched and observed through the local server API

Состояние исполнения: `completed`. Наблюдалось: `2026-09-27T18:47:05.898174+00:00`.

[Паспорт случая](case.json) · [Проверки](checks.json)

[Вход](input.json) — Compiled RunSpec from the server API; operator OTLP headers omitted.

| Проверка | Статус | Основание |
|---|---|---|
| compile | passed | [доказательство](runs/829fe228-d91a-421f-94c7-ebc12b00da90/run.json) (`/body`) |
| infrastructure_provisioning | passed | [доказательство](runs/829fe228-d91a-421f-94c7-ebc12b00da90/run.json) (`/body/result`) |
| smoke | passed | [доказательство](../../../../../platform/server/resilience/ha-mirror-20260927/mysql/verification.json) (`/verified`) |
| native_tps | not_applicable | execute_sql reports queries/s, not logical transaction TPS. |
| native_otlp_metrics | passed | [доказательство](../../../../../platform/server/resilience/ha-mirror-20260927/mysql/telemetry.json) (`/native_verified`) |
| component_metrics | passed | [доказательство](../../../../../platform/server/resilience/ha-mirror-20260927/mysql/cpu-tail.json) (`/verified`) |
| component_logs | passed | [доказательство](../../../../../platform/server/resilience/ha-mirror-20260927/mysql/telemetry.json) (`/logs_verified`) |
| pipeline_traces | not_run | No explicit acceptance evidence recorded for this check. |
| database_metric_distributions | not_run | No explicit acceptance evidence recorded for this check. |
| managed_database_metrics | not_applicable | Self-managed MySQL. |
| artifacts | passed | [доказательство](../../../../../platform/server/resilience/ha-mirror-20260927/mysql/artifacts.json) (`/verified`) |
| replication | not_run | [доказательство](../../../../../platform/server/resilience/ha-mirror-20260927/mysql/verification.json) (`/scope`) |
| cleanup | passed | [доказательство](../../../../../platform/server/resilience/ha-mirror-20260927/mysql/cleanup.json) (`/verified`) |

Прогоны:

- [runs/22a00ddf-a390-4e89-b04a-a2eac80894bc](runs/22a00ddf-a390-4e89-b04a-a2eac80894bc/manifest.json)
- [runs/829fe228-d91a-421f-94c7-ebc12b00da90](runs/829fe228-d91a-421f-94c7-ebc12b00da90/manifest.json)

[Единая таблица](../../../../../progress.csv)
