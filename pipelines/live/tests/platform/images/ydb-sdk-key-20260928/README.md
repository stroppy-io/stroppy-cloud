# Stroppy: штатная авторизация YDB по JSON-ключу

Сборка `dev-ydb-key-4972e5f35f1f` содержит Q88/all99, исправление SQL stream
и `serviceAccountKeyFile` вместо собственного `authTokenFile`.
Получение/кеширование/обновление IAM выполняет `ydb-go-yc v0.12.5`.
IAM TLS verification включён явно, metadata fallback при заданном ключе
не выполняется. Capability: `ydb_service_account_key_file=1`.

[Проверки](validation.json): race, short suite, lint и обязательная
интеграция PostgreSQL/MySQL/OTEL (49.391 s) прошли. Локальный TLS IAM
проверяет подпись JWT, короткий TTL, кеш, повторную выдачу и восстановление
после отказа issuer. Это не 12-часовая живая проверка YC.

[Паспорт](image.json) фиксирует base commit, SHA изменённых исходников и
бинарника; `source-changes.tar.gz` содержит точные исходники сборки.
Публикация — через существующую локальную Docker-учётку в GHCR;
[анонимное чтение зеркала](mirror-verification.json) подтвердило тот же digest.
Сборка временная, исходники не коммитились и не пушились.
