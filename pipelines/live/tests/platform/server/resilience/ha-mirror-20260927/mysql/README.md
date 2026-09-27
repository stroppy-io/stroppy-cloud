# MySQL Group Replication через сервер, Graphene 0.2.27

Run `829fe228-d91a-421f-94c7-ebc12b00da90`, test
`e3cbbaed-49be-4335-97b2-e303d41d4291`. MySQL 8.4: три DB-узла, один
ProxySQL и runner, все S, отдельные диски DB, keep=0. Образы закреплены
по digest через mirror.gcr.io, входы — test.json. SDK pipeline 0.2.13,
Docker library 0.3.4; Stroppy dev-image прежний.

Run completed/done в 18:46:11 UTC. За 120.015 s Stroppy выполнил
9919 CALL live_assert_cluster(), ошибок и ретраев нет, 82.648 queries/s
(нативная метрика Stroppy). Каждый вызов проверяет три ONLINE member и
ровно один PRIMARY, затем делает upsert. Все три exporters дают mysql_up=1.

Native OTLP и component telemetry доступны через API. CPU всех пяти машин
проверен с начала run до двух минут после его завершения; невозможных
отрицательных значений нет. Конфиг и лог доступны после cleanup, размеры
и SHA-256 совпадают. Все 28 инфраструктурных ресурсов удалены.

Доказательства: verification.json, final.json, database-metrics.json,
query-metrics.json, telemetry.json, cpu-tail.json, cleanup.json, artifacts.json.

Это не проверка failover и не сравнение строк на каждой реплике. Эти проверки
остаются not_run в CSV. Отмена во время deploy на 0.2.27 здесь не проверялась.
