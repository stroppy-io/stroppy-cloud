# Приёмка Graphene 0.2.27 и SDK

Зависимости сервера и пайплайнов обновлены: Graphene 0.2.27, pipeline 0.2.13,
Docker library 0.3.4; k8s library остаётся 0.4.0. Изменение SDK сохраняет тип
и время исходных samples вместо периодического повторения gauge.
[Локальные проверки и сборка](validation.json).

Старая отмена MySQL `22a00ddf-a390-4e89-b04a-a2eac80894bc` завершена:
`cancelled / done`, finished_at `2026-09-27T17:34:28Z`. Все ресурсы удалены;
[проверка через сервер](cleanup-verification.json). Это подтверждение
восстановленного состояния, не доказательство того, какая версия завершила
конкретную отмену: время завершения предшествует коммиту 0.2.27.

Свежий аналитический прогон `eb2a84f5-aaa8-4ca2-97fb-0a6c05679208`
[прошёл проверку](../analytical-graphene027-20260927/verification.json):
22 TPC-H + 103 SQL TPC-DS, без ошибок, native OTLP/логи/артефакты доступны,
все 12 ресурсов удалены и run completed.

[CPU, включая две минуты после завершения](../analytical-graphene027-20260927/cpu-tail.json):
DB 11.57–13.57%, runner 0.15–44.91%; обе машины представлены, отрицательных
значений нет. Тот же проверочный инструмент воспроизводит −75% на старой
сборке ([контроль](old-cpu-tail.json)). Старые samples не переписывались.
Исправление CPU подтверждено для этого свежего прогона. Старые Galera и
generated TPC-DS результаты сохраняют прежние неуспешные component checks.

Новый run завершил штатный cleanup без зависания. Отдельный свежий
[повтор отмены multi-machine deploy](../cancel-deploy-graphene027-20260927/README.md)
также прошёл: пять готовых машин, отмена в deploying, cancelled/done,
инфраструктура удалена.

Обновление квот через новый stroppy-quotas успешно: свежие данные YC без
ручного обновления credentials ([ответ](quota-refresh.json)).

## MySQL Group Replication

[Свежий пяти-машинный прогон](../ha-mirror-20260927/mysql/README.md) завершён:
9919 успешных SQL-проверок/записей, три ONLINE узла и один PRIMARY, ноль ошибок.
CPU всех пяти машин корректен также после teardown. Native OTLP, логи и
артефакты проверены через сервер; все 28 ресурсов удалены.

На окончание приёмки active/pending/kept=0; квоты вернулись к исходным
16 VM / 3 сетям. [Статус](final-status.json), [квоты](final-quota-refresh.json).
[Generated TPC-DS/Q88](../analytical-generated-q88-20260927/README.md) проверен
с исправленным dev-образом Stroppy: все 99 query / 103 SQL, cleanup успешен.
[MySQL GR и MariaDB Galera](../ha-failover-20260927/README.md) дополнительно
прошли primary stop/start и точное сравнение контрольных строк всех реплик.
[Повтор Patroni](../ha-failover-gated-20260927/README.md) также полностью прошёл,
включая CPU восьми машин после teardown и удаление всех ресурсов.
