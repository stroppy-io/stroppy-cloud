# postgres 17 / single / tpch-tpcds / server-sf001-power

Область проверки: real YC run launched and observed through the local server API

Состояние исполнения: `completed`. Наблюдалось: `2026-09-27T18:28:20.681030+00:00`.

[Паспорт случая](case.json) · [Проверки](checks.json)

[Вход](input.json) — Compiled RunSpec from the server API; operator OTLP headers omitted.

| Проверка | Статус | Основание |
|---|---|---|
| compile | passed | [доказательство](runs/eb2a84f5-aaa8-4ca2-97fb-0a6c05679208/run.json) (`/body`) |
| infrastructure_provisioning | passed | [доказательство](runs/eb2a84f5-aaa8-4ca2-97fb-0a6c05679208/run.json) (`/body/result`) |
| smoke | passed | [доказательство](../../../../../platform/server/resilience/analytical-graphene027-20260927/verification.json) (`/workload_verified`) |
| native_tps | not_applicable | Analytical workload reports queries/s and iterations/s rather than logical transaction TPS. |
| native_otlp_metrics | passed | [доказательство](../../../../../platform/server/resilience/analytical-graphene027-20260927/telemetry.json) (`/native_verified`) |
| component_metrics | passed | [доказательство](../../../../../platform/server/resilience/analytical-graphene027-20260927/database-metrics.json) (`/series_count`) |
| component_logs | passed | [доказательство](../../../../../platform/server/resilience/analytical-graphene027-20260927/telemetry.json) (`/facets`) |
| pipeline_traces | not_run | No explicit acceptance evidence recorded for this check. |
| database_metric_distributions | not_run | No explicit acceptance evidence recorded for this check. |
| managed_database_metrics | not_applicable | Self-managed PostgreSQL. |
| artifacts | passed | [доказательство](../../../../../platform/server/resilience/analytical-graphene027-20260927/artifacts.json) (`/verified`) |
| replication | not_applicable | Single PostgreSQL instance. |
| cleanup | passed | [доказательство](../../../../../platform/server/resilience/analytical-graphene027-20260927/cleanup.json) (`/verified`) |

Прогоны:

- [runs/0c3d8b9e-2c1c-4547-9dc8-fe2b7f656054](runs/0c3d8b9e-2c1c-4547-9dc8-fe2b7f656054/manifest.json)
- [runs/f019770c-bb58-4436-9bcb-8d28cced6fd2](runs/f019770c-bb58-4436-9bcb-8d28cced6fd2/manifest.json)
- [runs/09e86d6e-3eab-4e5d-ac47-621d19accff3](runs/09e86d6e-3eab-4e5d-ac47-621d19accff3/manifest.json)
- [runs/eb2a84f5-aaa8-4ca2-97fb-0a6c05679208](runs/eb2a84f5-aaa8-4ca2-97fb-0a6c05679208/manifest.json)

[Единая таблица](../../../../../progress.csv)
