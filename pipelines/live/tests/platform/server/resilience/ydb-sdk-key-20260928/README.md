# Managed YDB: JSON-ключ и официальный SDK

Run `6af3d1ca-3ee4-4561-8f85-e270512df99f`, test
`4d39d2f1-1d2d-4802-aee7-680ea63b3087`. **completed/done**, завершён 2026-09-27T22:51:16Z.
Readiness через SDK прошла в 22:34:09 UTC, основной сегмент стартовал в
22:34:13 UTC; [события выбранного режима](sdk-started.json).
Один runner, managed YDB serverless, 1 VU, SELECT 1, 15 минут,
query_timeout=10s, error_rate=0, keep=0. Потолок 1000 RU/s, reserved capacity=0.

Stroppy `dev-ydb-key-4972e5f35f1f`: официальный SDK получает IAM по
`serviceAccountKeyFile`; собственных refresh loop/token-file credentials нет.
[Образ и исходники](../../../images/ydb-sdk-key-20260928/README.md).
Runtime сервера/пайплайнов `0.0.0-dev-d39684a7-ydb-sdk-key-20260928`,
контракт 2.0.2, resync synced. API сервера и схема профиля не менялись.

Проверяем успешную нагрузку с SDK-аутентификацией, отсутствие старого режима,
ключа/токена/приватного пути в публичных config-артефактах, native telemetry,
CPU до двух минут после окончания и полное удаление стенда.
Перевыдача IAM после короткого TTL и отказ/восстановление issuer проверены
локально через SDK с TLS и подписанным JWT; 15 минут не доказывают переход
через 12-часовой срок реального YC-токена.

Предыдущие попытки сохранены в истории той же строки общей CSV.

## Результат

За 900.006 measurement seconds Stroppy выполнил 83330 запросов, ошибок 0;
native throughput 92.588 queries/s. [Авторизация](sdk-auth.json) подтверждает
режим SDK и отсутствие событий старого token-file механизма. Два публичных
конфига не содержат authToken, authTokenFile, serviceAccountKeyFile или ключа.
[Артефакты](artifacts.json) проверены по размеру/SHA, [native metrics и логи](telemetry.json)
доступны через сервер. [CPU](cpu-tail.json) проверен до двух минут после
завершения. [Managed YDB](managed-ready.json) явно ready во время
нагрузки. [Cleanup](cleanup.json) удалил все ресурсы, включая YDB и runner.
[Статус](final-status.json): running=0, pending=0, kept_stands=0;
[квоты](final-quotas.json) вернулись к исходным 16 VM / 3 сетям.

[Локальные проверки пайплайна](local-validation.json): activities/cloud race,
lint и контракт 2.0.2 прошли. Серверный API и схемы профиля не изменены.
Код Graphene не менялся; новые исходники Cloud/Stroppy пока без коммита.
