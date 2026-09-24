# Контракт входов pipeline для server/UI

Согласованная версия, правила совместимости и передача агенту:
[HANDOFF.md](HANDOFF.md).

Источник форм, ограничений и defaults — `pipelines/schemas`. Server и pipeline
используют один Go-модуль, UI получает protoJSON и TypeScript через
`make schemas-export`. Второго каталога полей нет: параметры каталога Stroppy
строятся из `workload.segment@1`.

| Уровень | Схемы | Исполнение и проверка |
|---|---|---|
| Библиотечный workload | `workload.stroppy@1`, `workload.segment@1` | Compiler → RunSpec → native Stroppy config |
| Полный стенд | `spec.run@1` | VM, secondary disks, host_prep, containers, dependencies, workload, telemetry, artifacts, keep |
| Матрица | `spec.suite@1` | Вложенные RunSpec проходят ту же проверку до запуска дочерних run |
| Провайдер | `provider.*.settings@1` | Общие определения подключены к RunSpec; настройки сети профиля можно опустить в уже скомпилированном RunSpec |
| Проверка прав и квоты | `spec.provider_verify@1`, `spec.quotas@1` | Go-входы и выходы проверяются на соответствие схемам |

## Что доступно во входах

Девять сценариев: `simple`, `execute_sql`, `baseline`, `tpcb/tx`, `tpcb/procs`,
`tpcc/tx`, `tpcc/procs`, `tpch/tx`, `tpcds`. Все workload-параметры из сохранённого
`stroppy probe` представлены типизированными полями. Executor, VUs, duration,
iterations и query timeout находятся в `run`. `baseline` как сегмент и
`stroppy baseline` как self-check машины — разные возможности.

Доступны native pool/PostgreSQL/SQL options, insert progress, подключение и TLS,
seed, deadline сегмента, ожидание warmup, steps/no_steps, inline SQL и файлы,
Graphene artifact references, thresholds. В RunSpec настройки драйвера имеют
native lowerCamel, в библиотечной форме — snake_case; compiler преобразует их.
`sslmode`, `application_name`, MySQL `tls` и `charset` добавляются в DSN.

`seed` сохраняется как uint64 без округления через float64. Отсутствующий
TPC-DS `query_stream` означает baked queries, явный `0` — generated stream 0.
`error_rate: 0` запрещает ошибки. `allow_public_ips: false` и
`continue_on_failure: false` сохраняются после нормализации.

`warmup` — ожидание без нагрузки перед сегментом. `timeout` ограничивает
исполнение после этого ожидания, включая подготовку контейнера и загрузку
данных. Сегмент не возит с собой файлов: `sql_file` и `schema_file` — это
идентификаторы пресетов самого Stroppy (`tpcc/pico`), а произвольный SQL
задаётся `sql_body` прямо в сегменте.

## Ограничения, которые нельзя обещать в UI

- Один runner на RunSpec. Параллельные независимые run выражаются через suite;
  распределённый запуск одного Stroppy на нескольких runner пока отсутствует.
- Pipeline создаёт собственную сеть. Режим `network.kind=existing` отвергается:
  adoption существующей сети и её отдельный lifecycle не реализованы.
- Docker использует host network; `ports.host` должен равняться `ports.container`.
- `flows` описывает топологию, не ACL. SG разрешает внутренний трафик, дополнительные
  открытия задаёт `network.ingress`. Рёбра контейнеров используют первый target роли.
- `disks.mount` задаёт автоматическую подготовку ext4/XFS для YC; общий
  валидатор RunSpec формирует `host_prep` и при прямом запуске без сервера.
- `scrapes` привязывается к контейнеру: `job` равен его имени, `role` совпадает.
  Собственные `container.scrape`/`scrape_port` имеют приоритет.
- Stored-procedure workloads: PostgreSQL, MySQL/MariaDB, CockroachDB и noop.
  Picodata TPC-DS: только загрузка, `workload` надо исключить. YDB TPC-DS:
  baked query set, без generated stream и multi-stream.
- `extra_params` — явный CLI escape hatch для выбранной сборки. Его значения
  проверяет Stroppy; успешная проверка схемы не гарантирует поддержку нового
  флага старым образом. Для полей server/UI использовать типизированные параметры.
- Пользовательский SQL, файлы SQL, внешний DSN и произвольные Docker-образы
  требуют проверки с реальной БД/образом. Schema не доказывает корректность SQL.

## Автоматические проверки

`pipelines/spec/contract_test.go` сравнивает наборы полей Go и schemas в обе
стороны, включая nested objects, lists, refs и provider variants. Для динамических
workload parameters используется отдельная проверка против native probe.

`pipelines/stroppycfg/contract_test.go` сравнивает все сценарии и параметры
с независимым `stroppy probe`, поля драйверов — с JSON Schema, генерируемой самим
Stroppy. Fixtures и происхождение находятся в `pipelines/stroppycfg/testdata`.
Хеш бинарника и его VCS metadata записаны отдельно от commit исходников.

Команда из корня репозитория:

```sh
python3 pipelines/live/tools/check_contract.py --binary /path/to/stroppy
```

Она сравнивает fixtures с локальным Stroppy, проверяет передачу входов workflow,
запускает девять native workloads на noop (одна iteration, только шаг workload).
`--refresh` обновляет fixtures; их diff нужно рассматривать как изменение
контракта, а не как доказательство совместимости.

Отдельно выполняются тесты всего root-модуля и `pipelines`, lint, экспорт schemas,
сборка UI и четырёх pipeline binaries с recording manifests. Сохранённые live-input
проверяются на совместимость без редактирования исторических результатов.

Эти проверки подтверждают форму контракта, передачу параметров и локальный
native dispatch. Они не заменяют матрицы реальных БД, долгие YC-прогоны,
квоты, отказоустойчивость и проверку опубликованного Docker digest. Их прогресс
остаётся в единственной таблице `live/tests/progress.csv`.

Настройки VM, сырые файлы БД, приоритеты overrides и границы resource preflight:
[configuration.md](configuration.md).
