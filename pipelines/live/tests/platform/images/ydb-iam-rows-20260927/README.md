# Stroppy: обновление IAM и стабильный результат закрытого SQL stream

Сборка `dev-iam-rows-beea29efd327` содержит Q88/all99, token-file credentials
и исправление ложных `context canceled` после успешного закрытия SQL stream.
Результат курсора фиксируется до освобождения query timeout; реальные ошибки
итерации/закрытия и реальное истечение deadline сохраняются.

[Регрессия](rows-regression.json): старый код упал на пятом успешном запросе,
исправленный прошёл с race detector. Полный short/race suite и lint прошли;
обязательная интеграция PostgreSQL/MySQL/OTEL прошла (45.867 s).
Локальный baseline harness удалён после проверки.

Источник — незакоммиченное рабочее дерево Stroppy. `source-changes.tar.gz`
содержит точные изменённые файлы; base commit, source и binary SHA указаны
в `image.json`. Публикация выполнена локальной Docker-учёткой в существующий
GHCR, digest проверен анонимным чтением. Это временная dev-сборка до PR и
официального образа; исходники не коммитились и не пушились.
