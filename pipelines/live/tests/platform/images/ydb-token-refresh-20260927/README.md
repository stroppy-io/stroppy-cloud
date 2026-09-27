# Временная сборка Stroppy: IAM refresh и полный generated TPC-DS

Образ `ghcr.io/stroppy-io/stroppy@sha256:9dd1a203fc94d53df223e724362c20678b1c38b6dc6cc92a9d4889473286110f`,
версия `dev-iam-e621e2612d93`. Собран из рабочего дерева без commit/push
исходников; локальными Docker credentials опубликован в существующий GHCR.
Digest проверен анонимным чтением registry. Это временное решение до PR,
официальной сборки и повторной проверки её digest.

`source-changes.tar.gz` сохраняет патч и исходники относительно base commit,
`image.json` — хеши исходников и бинарника, `version.json` — capability
`ydb_token_file=1`. Изменения: исправление Q88/запрет неполного generated набора
и credentials из атомарно обновляемого приватного файла. Заданный файл
не допускает переключения на credentials metadata при ошибке чтения.

Проверки: полный `make tests TEST_FLAGS=-short` с race detector,
`make linter` (0 issues), обязательный `make integration` с PostgreSQL,
MySQL и OTEL (PASS). Локальная тестовая инфраструктура удалена.
Живое доказательство ротации хранится отдельно в
[кампании IAM](../../server/resilience/ydb-iam-refresh-20260927/).
