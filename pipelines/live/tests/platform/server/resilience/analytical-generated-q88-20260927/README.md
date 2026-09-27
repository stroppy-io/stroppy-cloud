# Generated TPC-DS: живой повтор с Q88

Run `aa231dc7-16ec-44e9-9d3c-00d0235d02bc`, test
`11374fec-dad3-4406-b14e-1e88b187d29a`. PostgreSQL 17 single, SF=.01,
query_stream=0, query_seed=19620718, 1 VU / 1 iteration, keep=0.
Запуск только через сервер. Graphene 0.2.27, SDK 0.2.13, docker 0.3.4.

[Dev-образ и исходники](../../../images/tpcds-generated-q88-20260927/README.md):
`ghcr.io/stroppy-io/stroppy@sha256:65378b0d306301a2d80030a74230be4327b4f2c084ae37e009deda342c0b2624`.
Старый неполный run и его checks сохранены в истории кейса.

Результат: **completed / done**, 99/99 номеров query, включая Q88, 103 SQL,
без ошибок запросов и итераций. Загружены 24 таблицы / 2 658 555 строк.
Stroppy экспортировал 8.279 queries/s; значение взято из результата Stroppy.
[Полный набор запросов](query-coverage.json), [native counters](query-metrics.json),
[результат проверки](verification.json).

Native OTLP, компонентные метрики и логи доступны через сервер. Проверены
2110 PostgreSQL exporter series. Конфиг и лог скачаны после завершения,
размеры и SHA-256 совпали. CPU проверен до двух минут после teardown:
DB 16.36%, runner 0.11–76.46%, обе машины представлены, невалидных samples нет.
[Телеметрия](telemetry.json), [CPU](cpu-tail.json), [артефакты](artifacts.json).
[Cleanup](cleanup.json) подтверждает отсутствие оставшейся инфраструктуры.

Исправление Stroppy удаляет неиспользуемое определение STORE в шаблоне Q88;
SQL с `s_store_name='ese'` сохранён. Неполный generated query set теперь
отклоняется до изменения схемы БД. В частности, MySQL generator ещё не
поддерживает Q51/Q97 с FULL OUTER JOIN и возвращает явную ошибку; встроенный
MySQL-набор остаётся доступен. Это функциональная проверка малого масштаба,
не сертифицированный TPC benchmark. Изменения исходников не закоммичены.
