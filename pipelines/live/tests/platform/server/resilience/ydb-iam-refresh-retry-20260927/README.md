# IAM rotation: полный повтор с heartbeat исходной activity

Run `81cda0f2-8db1-44c8-8d12-cb83ccf91f98`, test
`46c0a87a-10e7-4a9f-a81d-553e60da94f7`. Запуск через Stroppy Cloud;
managed YDB serverless и один runner. 1 VU, SELECT 1, 15 минут,
keep=0. Пользовательский SA key разрешается только в activity на runner.

Compose `0.0.0-dev-f658fd8b-iam-heartbeat2-20260927`, контракт 2.0.2,
resync synced перед launch. Исправление сохраняет исходный heartbeat context
при вложенной проверке версии/readiness; регулярные heartbeat не зависят
от количества логов. [Локальные проверки](local-validation.json).

Run failed после полного 15-минутного сегмента: 35 query errors,
нулевой error threshold сработал. Cleanup и артефакты проверены.
Heartbeat исправлен, сегмент прошёл прежнюю минутную точку обрыва. Обнаружены редкие ложные `context canceled` после успешного закрытия
SQL stream. Причина воспроизведена локальным regression-тестом и исправлена
в Stroppy: outcome курсора фиксируется до освобождения query timeout.
[Доказательство](../../../images/ydb-iam-rows-20260927/rows-regression.json).
[Ротация токена](rotation-partial.json) прошла, весь run — failed.
Нулевой error threshold сохранён.

Приёмка требует выдачи нового YC IAM-токена, повторного чтения
файла тем же workload-процессом, роста native query counter после обновления,
нулевых query errors, доступности артефактов без IAM-токена, telemetry и cleanup.
Предыдущая failed-попытка сохранена отдельно; это полный rerun, не resume.
Реальное истечение 12-часового токена не проверяется этим 15-минутным тестом:
expiry и временные ошибки issuer проверены локально с коротким TTL.
