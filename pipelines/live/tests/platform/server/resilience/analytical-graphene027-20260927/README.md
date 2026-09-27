# PostgreSQL TPC-H/TPC-DS с обновлённым SDK

Run `eb2a84f5-aaa8-4ca2-97fb-0a6c05679208`, test
`c6ba5023-2501-435d-918c-a5c5e013e542`. Запущен исключительно через сервер.
Graphene 0.2.27; pipeline SDK 0.2.13, Docker library 0.3.4.
Опубликованный образ stroppy-run: `c9c6030b5daf9460`.

Две машины S, PostgreSQL 17, SF=0.01, 1 VU, 1 shared iteration, keep=0.
Образы БД/exporters закреплены digest через зеркало, входы сохранены в test.json.
Stroppy dev-image не менялся; generated TPC-DS здесь не проверяется.

Run завершился в 18:27:26 UTC. Native workload counters: 22 TPC-H и 103
SQL-операции TPC-DS (99 запросов с составными). Ошибок запросов/итераций нет.
Конфиги и логи скачаны после cleanup, размер и SHA-256 совпали.
Все 12 ресурсов удалены; native OTLP, логи и component metrics прошли проверку.
Проверка CPU на расширенном интервале сохраняется отдельно в cpu-tail.json.

Доказательства: final.json, verification.json, telemetry.json, cleanup.json,
artifacts.json, query-metrics.json, database-metrics.json. Это функциональная
приёмка на малом масштабе, не сертифицированный TPC benchmark.
