# Утилиты live-проверок

Все скрипты находятся здесь. Команды ниже выполняются из корня репозитория,
если явно не указано другое. Данные тестов находятся в [../tests](../tests/README.md).

## Поддерживаемые команды

| Команда / файл | Назначение |
|---|---|
| `python3 pipelines/live/tools/live.py validate` | Проверить структуру и ссылки, без сети |
| `python3 pipelines/live/tools/live.py progress` | Обновить единственную CSV и README случаев, без сети |
| `python3 pipelines/live/tools/live.py resolve старое-имя.json` | Найти перенесённый файл |
| `server_api.py /api/v1/... --evidence <файл>` | Вызвать локальный сервер на 18347, сохранить HTTP-ответ; `--method`, `--data <JSON-файл или ->`; запрос и bearer token не сохраняются |
| `verify_server_artifacts.py --tenant <slug> --run-id <UUID> --evidence <файл>` | Скачать артефакты через API, проверить размер и SHA-256; содержимое конфигов и логов не сохраняется в evidence |
| `record_server_case.py --tenant <slug> --run-id <UUID> --case <база/версия/топология/workload/preset> --workload <id>` | Добавить строку матрицы по API-прогону; `--append` сохраняет новую попытку и архивирует старые проверки, `--refresh` обновляет evidence выбранной попытки без изменения checks; непроверенные проверки не становятся успешными автоматически |
| `prepare.py <input.json> --output <новый-файл>` | Подготовить вход с новыми UUID без запуска |
| `bootstrap.py --help` | Настроить доступ по явно переданным operator/Graphene-конфигам |
| `live-inputs/` | Скомпилировать входы через реальный серверный компилятор |
| `inspect_metrics.py`, `inspect_native_metrics.py` | Проверить сохранение метрик компонентов и нативных метрик Stroppy |
| `inspect_component_logs.py`, `inspect_traces.py` | Проверить логи и трассировку |
| `inspect_mysql_replication_metrics.py`, `inspect_mysql_cluster_metrics.py` | Проверить метрики репликации и кластеров MySQL/MariaDB |
| `postgres_probe.py`, `mysql_probe.py`, `mysql_cluster_probe.py` | SQL-проверки PostgreSQL/MySQL/MariaDB |
| `picodata_probe.py`, `cockroach_probe.py`, `ydb_probe.py`, `external_canary_probe.py` | Проверки соответствующих БД |

Bootstrap-манифесты лежат в `bootstrap/`. Скрипт использует постоянный token Secret;
значение токена и приватные конфиги в репозитории не хранятся. Команды bootstrap
меняют контур только при явном запуске оператором.

Серверные проверки используют `STROPPY_TEST_URL` (по умолчанию
`http://localhost:18347`) и `STROPPY_TEST_TOKEN` (локальный `dev`). Запуск,
повтор, отмена и наблюдение идут через API сервера; утилиты не запускают
Graphene-пайплайны напрямую. Снимки первой кампании и ограничения настройки
tenant находятся в [server/2026-09-24](../tests/platform/server/2026-09-24/README.md).

`inspect_mysql_cluster_metrics.py` принимает **полный префикс файла**, например
`/tmp/check/mysql-gr-84` для `/tmp/check/mysql-gr-84.metrics.json`; остальные
инспекторы сохраняют собственные CLI-аргументы. Probe-скрипты со stdin/переменными
окружения описывают интерфейс в исходнике; это не универсальные `--help`-команды.

Компилятор входов — отдельный Go-модуль с локальными заменами серверного и
pipeline-модулей. Поддерживаются его прежние флаги; пример без запуска ресурсов:

```sh
go -C pipelines/live/tools/live-inputs run . --help
go -C pipelines/live/tools/live-inputs run . --database postgres --output /tmp/stroppy-inputs-new
```

Публичные снимки одного прогона могут включать несколько сегментов. Перед
повтором проверьте состав входа; `prepare.py` обновляет UUID, но не сокращает нагрузку.

## Скрипты прошлых прогонов

В `history/<исходный-каталог>/` сохранены 232 найденных скрипта из временных
рабочих каталогов. [inventory.json](history/inventory.json) содержит исходные
пути и SHA-256. Они охватывают запуск и наблюдение кампаний, сбор доказательств,
локальные стенды, диагностику, dev-сборки и публикацию образов.

Это исторические рецепты: часть содержит конкретные run ID, `/tmp`-пути,
зависимости от приватных `state.json`, CLI-бинарей и конфигов. Они не импортируются
автоматически и не вызываются генератором таблицы. Их перенос не означает, что
они готовы к повторному запуску без адаптации. Некоторые выполняют изменения
сразу при запуске; используйте поддерживаемые команды выше для текущего каталога.
Приватные рабочие конфиги и токены вместе с ними не переносились.

Go-фрагменты сохранены как `.go.txt`, чтобы старые самостоятельные `main` и
выдержки внешнего кода не становились пакетами production-модуля. Для выполнения
конкретного фрагмента потребуется отдельный рабочий каталог и его зависимости.

## Миграция и проверки

`../tests/platform/migration/paths.json` связывает все прежние имена с новыми путями.
`../tests/platform/migration/files.json` фиксирует исходные SHA-256; адаптации поддерживаемых
утилит отмечены отдельно. `../tests/platform/migration/legacy-progress.json` сохраняет все
46 полей прежних 253 строк без второй CSV.

`migration/legacy_progress.py` сохранён для понимания старой агрегации; его
запуск отключён. Новую таблицу создаёт только `live.py progress`.
`migration/migrate_layout.py` воспроизводит первоначальный этап переноса
из отдельного снимка в новый каталог; он не предназначен для повторного запуска
поверх текущих данных.

```sh
python3 -B -m unittest discover -s pipelines/live/tools -p 'test_*.py'
python3 pipelines/live/tools/live.py validate
go -C pipelines/live/tools/live-inputs test ./...
go -C pipelines test ./spec
```

### Доступ Graphene к Crossplane

`install_graphene_access.py --kubeconfig <operator-config>` показывает план;
`--apply` для инсталляции без GitOps устанавливает RBAC YC и назначает `graphene-crossplane` в шаблоне
воркеров Graphene. Подключение оператора используется только этой командой.
Сервер Stroppy Cloud и пользовательские Graphene namespaces не получают kubeconfig.
Манифест: `bootstrap/graphene-crossplane.yaml`. Реквизиты профилей изолированы
от системных секретов Crossplane в `stroppy-provider-credentials`.
`bootstrap.py` сохранён для воспроизводимости исторических прогонов 1.x.

В текущем кластере источник настройки — `stroppy-io/cloud/k8s/apps/graphene`.
Скрипт отказывается изменять Deployment под управлением ArgoCD.
