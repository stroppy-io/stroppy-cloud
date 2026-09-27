# TPC-H и TPC-DS через сервер

Run `09e86d6e-3eab-4e5d-ac47-621d19accff3` завершён успешно. PostgreSQL 17 single,
DB/runner размера S, SF=0.01, один VU и одна shared iteration на сегмент.

- TPC-H: 22 запроса на шаге workload, ошибок 0; native 97.778 queries/s.
- TPC-DS: 103 SQL-запроса базового набора на шаге workload, ошибок 0;
  native 7.944 queries/s. Загружены 24 таблицы, 2 658 555 строк по native summary.
- Через сервер получены 1916 native OTLP series, host CPU/RAM, PostgreSQL
  exporter metrics, журналы и facets. Числа throughput скопированы из Stroppy.
- Четыре артефакта (конфиг/лог каждого сегмента) скачаны после завершения;
  размеры и SHA-256 совпали. Инфраструктура удалена, kept stand отсутствует.

Нагрузка принята, но полная телеметрия — нет: повторная проверка после окончания
teardown выявила CPU runner до −75.17%. `component_metrics=failed`, при этом
native OTLP, SQL-результат, логи, артефакты и cleanup подтверждены.
[Бриф Graphene](../graphene-0.2.26-blockers/README.md).

Итог по отдельным проверкам: [verification.json](verification.json), [cleanup](cleanup.json),
[artifacts](artifacts.json), [telemetry](telemetry.json),
[native workload counters](query-metrics.json).

Образы БД/exporters закреплены digest и получены через публичное зеркало
`mirror.gcr.io`. Node-exporter и postgres-exporter имеют те же manifest digest,
что и оригинальные реестры. Stroppy — прежняя dev-сборка с native telemetry.
Две предыдущие попытки отменены до workload из-за таймаутов публичных registries;
их входы и cleanup сохранены в той же строке общей CSV как отдельные attempts.

Это функциональная проверка малого масштаба; она не подтверждает TPC benchmark
certification, большой масштаб, generated query streams или все СУБД.
Generated TPC-DS проверяется отдельным case.
