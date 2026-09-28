# ydb_managed managed / serverless / execute/sql / server-iam-refresh-15m

Область проверки: real YC run launched and observed through the local server API

Состояние исполнения: `completed`. Наблюдалось: `2026-09-27T22:51:29.377579+00:00`.

[Паспорт случая](case.json) · [Проверки](checks.json)

[Вход](input.json) — Compiled RunSpec from the server API; operator OTLP headers omitted.

| Проверка | Статус | Основание |
|---|---|---|
| compile | passed | [доказательство](runs/6af3d1ca-3ee4-4561-8f85-e270512df99f/run.json) (`/body`) |
| infrastructure_provisioning | passed | [доказательство](../../../../../platform/server/resilience/ydb-sdk-key-20260928/managed-ready.json) (`/verified`) |
| smoke | passed | [доказательство](../../../../../platform/server/resilience/ydb-sdk-key-20260928/sdk-auth.json) (`/verified`) |
| native_tps | not_applicable | execute_sql reports queries/s, not logical transaction TPS. |
| native_otlp_metrics | passed | [доказательство](../../../../../platform/server/resilience/ydb-sdk-key-20260928/telemetry.json) (`/native_verified`) |
| component_metrics | passed | [доказательство](../../../../../platform/server/resilience/ydb-sdk-key-20260928/cpu-tail.json) (`/verified`) |
| component_logs | passed | [доказательство](../../../../../platform/server/resilience/ydb-sdk-key-20260928/telemetry.json) (`/logs_verified`) |
| pipeline_traces | not_run | No explicit acceptance evidence recorded for this check. |
| database_metric_distributions | not_run | No explicit acceptance evidence recorded for this check. |
| managed_database_metrics | not_run | This scenario verifies authentication renewal, not managed database telemetry. |
| artifacts | passed | [доказательство](../../../../../platform/server/resilience/ydb-sdk-key-20260928/artifacts.json) (`/verified`) |
| replication | not_applicable | Managed serverless service; internal replication is outside this test. |
| cleanup | passed | [доказательство](../../../../../platform/server/resilience/ydb-sdk-key-20260928/cleanup.json) (`/verified`) |

Прогоны:

- [runs/608b7c66-56d7-4c35-8559-800c056a953e](runs/608b7c66-56d7-4c35-8559-800c056a953e/manifest.json)
- [runs/81cda0f2-8db1-44c8-8d12-cb83ccf91f98](runs/81cda0f2-8db1-44c8-8d12-cb83ccf91f98/manifest.json)
- [runs/50faedbf-236c-4145-8c3b-055fd652e84a](runs/50faedbf-236c-4145-8c3b-055fd652e84a/manifest.json)
- [runs/546827b6-29e3-4974-bf7e-ad81dbf75cda](runs/546827b6-29e3-4974-bf7e-ad81dbf75cda/manifest.json)
- [runs/d740c071-1aae-462d-a014-559d9cb143a8](runs/d740c071-1aae-462d-a014-559d9cb143a8/manifest.json)
- [runs/6af3d1ca-3ee4-4561-8f85-e270512df99f](runs/6af3d1ca-3ee4-4561-8f85-e270512df99f/manifest.json)

[Единая таблица](../../../../../progress.csv)
