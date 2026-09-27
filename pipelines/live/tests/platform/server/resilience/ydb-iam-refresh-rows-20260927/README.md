# IAM rotation с исправлением SQL stream

Run `50faedbf-236c-4145-8c3b-055fd652e84a`, test
`7aecd8aa-61eb-480f-a42c-a113fb97f05f`. Полный серверный rerun, 15 минут,
1 VU, managed YDB serverless, SELECT 1, query_timeout=10s, error_rate=0,
keep=0. Настройки нагрузки и нулевой порог ошибок не ослаблены.

Образ Stroppy `dev-iam-rows-beea29efd327`, immutable digest и исходники
в [паспорте сборки](../../../images/ydb-iam-rows-20260927/README.md).
Пайплайн — `0.0.0-dev-f658fd8b-iam-heartbeat2-20260927`, контракт 2.0.2.
Предыдущий run подтвердил замену token во время нагрузки, но завершился
неуспешно из-за дефекта учёта SQL-ошибок. Обе неудачные попытки сохранены
в истории общей строки CSV.

Run failed после полного сегмента: 83 реальные query errors при 100 RU/s; cleanup подтверждён. Через 7 минут возникли реальные `ResourceExhausted` от YDB:
тестовый потолок 100 RU/s оказался ниже устойчивого потребления 1 VU.
Ложные canceled после закрытия курсора не наблюдаются. Нулевой threshold
сохранён; статус этого run — failed.
[Следующий полный запуск](../ydb-iam-refresh-rcu1000-20260927/README.md)
меняет только потолок serverless на 1000 RU/s (reserved capacity остаётся 0).

Приёмка требует реального refresh/reload IAM внутри сегмента,
запросов после смены токена, нулевых ошибок, сохранности артефактов,
метрик/логов и удаления инфраструктуры.
