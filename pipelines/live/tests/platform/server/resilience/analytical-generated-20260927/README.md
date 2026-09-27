# Generated TPC-DS: неполное покрытие при completed

Run `7b3d8ae2-1b43-4b41-81ca-64f9e522ea86` завершён и очищен. PostgreSQL 17,
SF=0.01, stream 0, seed 19620718, одна shared iteration, один VU.
Использован pipeline после исправления image-pull heartbeat.

Вход исполнился, SQL-ошибок нет, native OTLP и конфиг/лог доступны. Но проверка
содержимого лога обнаружила **98 из 99 query IDs** и 102 SQL statements вместо
полного набора: **Q88 отсутствует**. Native counter шага workload также равен 102.
`smoke=failed` отражает неполноту; статус самого run остаётся честным `completed`.

Причина в Stroppy: `third_party/gotpcds/dsqgen/generate.go` сообщает неподдержанную
псевдодистрибуцию stores для Q88 в `Result.Skipped`. Функция
`workloads/tpcds/tpcds.go:generateStream` читает только `Result.Queries` и не
передаёт пропуски пользователю. Есть также отдельные structural gaps generated
MySQL; их нельзя объявлять проверенными по этому PostgreSQL-run.

Дальнейшая правка Stroppy должна обеспечить полный Q88 и явный результат для
любых оставшихся пропусков, с regression по покрытию всех query IDs. Молча
считать такой прогон полным нельзя. Stroppy в этой итерации не изменялся:
новые запуски остановлены на [блокерах Graphene](../graphene-0.2.26-blockers/README.md).

Доказательства: [query coverage](query-coverage.json), [native counters](query-metrics.json),
[фрагменты workload log](query-log-excerpts.json), [artifacts](artifacts.json),
[cleanup](cleanup.json). `component_metrics=failed`: на границе остановки runner
CPU падает до −100.32%; остальные части [телеметрии](telemetry.json) проверены отдельно.
