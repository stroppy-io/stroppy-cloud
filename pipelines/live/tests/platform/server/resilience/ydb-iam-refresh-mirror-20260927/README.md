# IAM rotation через зеркало с лимитом 1000 RU/s

Один runner, managed YDB serverless, 1 VU, SELECT 1, 15 минут,
query_timeout=10s, error_rate=0, keep=0. `throttling_rcu_limit=1000`,
reserved capacity=0. Полный новый запуск, не resume.

Тот же Stroppy `dev-iam-rows-beea29efd327` используется через существующее
зеркало `docker.stroppy.io`; анонимно проверен точный digest, совпадающий с
GHCR ([проверка](mirror-verification.json)). В пайплайне кешируются только
immutable digest images, mutable tags обновляются. Временные сетевые ошибки
pull повторяются максимум три раза в рамках deadline; auth/manifest ошибки
возвращаются сразу. Heartbeat не зависит от вывода загрузчика/нагрузки.

Runtime `0.0.0-dev-f658fd8b-iam-pull2-20260927`, контракт 2.0.2.
Событие обновления IAM содержит только состояние replaced/unchanged и
сроки действия; токен или его фрагмент не журналируется.

Run `d740c071-1aae-462d-a014-559d9cb143a8`, test
`bda3186d-5257-4627-9105-662f24e7c31f`: **completed/done**.
Нагрузка 21:10:29–21:25:31 UTC: 78391 запрос, 0 ошибок, native 87.1 queries/s,
900.007 measurement seconds. Нулевой error threshold сохранён.

В 21:20:29 UTC повторный запрос к YC вернул тот же действующий токен:
`credential_state=unchanged`, срок 2026-09-28T08:26:42Z не изменился.
После ответа счётчик запросов вырос с 51361 до 77398. Это подтверждает
работу renewal и продолжение нагрузки, **не замену токена или продление
его срока в этом run**. [Машиночитаемое доказательство](rotation.json)
содержит renewal_verified=true и rotation_verified=false. Настоящая замена
и повторное чтение Stroppy подтверждены в [предыдущей попытке](../ydb-iam-refresh-retry-20260927/rotation-partial.json),
которая целиком failed из-за уже исправленного учёта SQL stream.
Ожидание реального 12-часового истечения токена не выполнялось; ошибки
issuer и истечение проверены локальными тестами.

Два публичных config-артефакта не содержат runtime authToken/authTokenFile.
[Артефакты](artifacts.json) проверены по размеру/SHA, [native metrics и логи](telemetry.json)
доступны через сервер. [CPU](cpu-tail.json) проверен также до двух минут
после завершения. [Managed resource](managed-ready.json) явно ready во время
нагрузки. [Cleanup](cleanup.json) удалил всю инфраструктуру, включая YDB;
[статус](final-status.json): running=0, pending=0, kept_stands=0.
[Свежие квоты](final-quotas.json): исходные 16 VM / 3 сети.
Предыдущие четыре попытки сохранены в истории одной строки CSV.
