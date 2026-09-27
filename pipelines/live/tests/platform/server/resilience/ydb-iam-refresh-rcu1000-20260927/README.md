# IAM rotation: нагрузка ниже лимита serverless

Run `546827b6-29e3-4974-bf7e-ad81dbf75cda`, test
`1001572e-ddcd-4879-b7c3-6127435f7359`. Через сервер, managed YDB serverless,
один runner, 1 VU, SELECT 1, 15 минут, query_timeout=10s, error_rate=0, keep=0.
Потолок `throttling_rcu_limit=1000`, reserved capacity=0. Это ограничение
максимального потребления, не предоплата 1000 RU/s. Прошлый потолок 100 RU/s
привёл к реальному throttling после нескольких минут непрерывной нагрузки.

Stroppy `dev-iam-rows-beea29efd327`, digest `sha256:7e58c713d13387b2e82a85ffd909daf184b10b06c64ae724c17d126475e1f25c`.
Включает Q88, token-file refresh и исправление ложных cursor cancellation.
Пайплайн `0.0.0-dev-f658fd8b-iam-heartbeat2-20260927`, контракт 2.0.2.

Run failed до readiness SQL: GHCR HEAD завершился сетевым i/o timeout. Managed база и VM удалены; нагрузка не запускалась. Доказательства должны подтвердить реальную выдачу и чтение
нового IAM-токена тем же процессом, запросы после refresh, ноль ошибок,
сохранность config/log артефактов без токена, телеметрию и cleanup.
Предыдущие неудачные попытки сохраняются в истории одной строки CSV.
