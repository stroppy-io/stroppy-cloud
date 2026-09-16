# Реальные прогоны YC

[CSV прогресса](progress.csv): 65 сочетаний БД / версии / топологии
и дополнительные проверки native-экспорта (текущее число строк — в CSV),
отдельные статусы компиляции, smoke, полных workload, baseline, сегментов,
телеметрии, native TPS/OTLP и отказных сценариев. `passed` означает наличие
сохранённого подтверждения; `pending` — проверка не завершена; `blocked` —
компилятор пока не поддерживает топологию. Матрица относится к YC; AWS исключён.
Каталог описан preset `smoke-2m-2vus`, native-экспорт — `native-otlp-20s-2vus`.
Для финального TPC-C smoke явно задано `retry_attempts=50`: preset
`native-otlp-20s-2vus-retry50`. Стандартный лимит Stroppy остаётся равен трём.
Это не все возможные параметры нагрузки. `not_applicable` у TPS означает, что
workload не определяет логические транзакции; его собственные скорости доступны отдельно.
Run ID, дата и файл подтверждения позволяют проверить каждый успешный smoke.
Обновление: `python3 scripts/live-progress.py` из корня репозитория.
`current-campaigns.json` добавляет заранее все нагрузки активных очередей;
`run_phase` и `run_status_checked_at` показывают состояние workflow и свежесть
наблюдения. `run_stage` и `run_stage_checked_at` показывают последний
наблюдаемый этап, например workload или cleanup. Непринятые строки привязаны
к последнему повтору той же комбинации; принятые сохраняют исходное
доказательство (`progress-retry-tracking-check.json`).
`queued` / `running` означают очередь / исполнение,
`verification_pending` — workflow завершён, доказательства ещё не приняты.
Только сохранённая полная проверка выставляет `passed`. CSV обновляется атомарно.
Число подтверждённых строк включает каталог и отдельные native workloads;
оно не является числом полных performance-матриц.

PostgreSQL 15–18 × single / primary-replica / PgBouncer / Patroni HA:
**16/16 успешных функциональных smoke-прогонов** в YC с отдельными SSD.
Первые 12 ячеек описаны в `postgres-matrix-check.json`; четыре Patroni HA —
в `patroni-matrix-check.json`. Все тестовые YC-ресурсы этих матриц удалены;
проверены скачивание и SHA-256 24 + 8 артефактов. Конфиги — до явного удаления,
логи — 30 дней; общий retention не менялся.

Patroni HA проверен на Graphene 0.2.8: три PostgreSQL, три etcd, HAProxy и
runner. В каждой ячейке подтверждены SQL-репликация, отдельные диски, метрики
и логи всех 18 компонентов, pipeline traces и отсутствие переподключений
агентов во время наблюдения нагрузки. Переключение при отказе отдельно не
проверялось. Двухминутный simple — функциональный smoke, не полный workload.

Текущий Graphene server — **0.2.14**, worker — `stroppy-run:f53623d265c372b2`,
Graphene SDK — **0.2.5**, Docker library — **0.2.3**.
Проверки опубликованных исправлений — `graphene-kind-retirement-check.json`,
`sdk-metrics-chunk-check.json` и `docker-install-heartbeat-check.json`.
Чтение исторических метрик описано в `graphene-metrics-read-check.json`.
Сроки общей телеметрии не изменены. Полные workload/baseline-матрицы и
отказные сценарии остаются отдельными этапами. Нативный TPS/OTLP
подтверждён для пяти сегментов PostgreSQL без terminal errors после исправления TPC-C
(`native-export-check.json`). Проверки MySQL/MariaDB описаны в
`mysql-family-check.json`: **7/7 сочетаний, 35/35 функциональных сегментов**
на исправленной dev-сборке Stroppy. Три MariaDB single повторно проверены
после добавления `SLAVE MONITOR`; collectors, 70 артефактов выбранных прогонов
и cleanup подтверждены. `mysql-clusters-check.json`: **5/5 сочетаний,
25/25 сегментов** MySQL GR и MariaDB Galera, с SQL-проверкой всех членов.
Текущий прогресс OrioleDB и следующих СУБД — в `catalog-functional-check.json`
и CSV. Все шесть OrioleDB-сочетаний (PG16/17/18, single и primary-replica)
прошли по пять функциональных сегментов: simple, TPC-B tx/procs, TPC-C tx/procs.
Проверены реальный OrioleDB table access method, репликация тестовых строк,
нативные метрики, логи, traces, скачивание артефактов и cleanup.
Причина повторных прогонов и исправление штатного Docker entrypoint описаны
в `catalog-pipeline-runtime-check.json`.
AWS исключён.
Ограничение покрытия первых 45 секунд метрик Galera 10.11 и наблюдения
из логов описаны ниже; это не полная performance/failover-матрица.

Входы и результаты для тестирования Graphene-пайплайнов на настоящих VM.
Проверено 2026-09-15. Секретов в этом каталоге нет.

## Контур и версии

- Graphene: `graphene.stroppy.io:443`, namespace `t-stroppy-live`.
- Kubernetes: `cat0s5bdqi9cmj8sp9hc`, namespace воркеров `graphene`.
- YC: cloud `b1gt6m4l9gfhaobcgb81`, folder `b1ghttqg66t14ldkvfcq`, zones `ru-central1-d`, `ru-central1-a`.
- Graphene server **0.2.12** (`3450317`); Argo CD Synced/Healthy,
  cloud commit `c3f14d6`. Image digest:
  `sha256:9c649fa44fe32abe6c00505ea409940aff1e35e8672ea545b5668a7ed14ac00e`.
- Pipeline SDK **0.2.4**, docker library **0.2.2**, k8s library **0.2.0**; серверный модуль Stroppy
  подтянул Graphene **0.2.3**. Изменения Stroppy Server не развёрнуты этим тестом.
- YC Ubuntu image `fd8d6s0blceqbto92ss8` (`ubuntu-24-04-lts-v20260907`).
  Docker автоматически устанавливается в deploy; проверена версия 29.1.3 из Ubuntu.
- Stroppy в ранних каталожных smoke: `docker.stroppy.io/stroppy-io/stroppy:v6.0.0.62`.
  Нативный экспорт проверен на development-образах; точные digest и хеши
  исходников сохранены в `native-export-check.json` (`images`).

Опубликованные образы в `graphene.stroppy.io:443/t-stroppy-live/`:

| Pipeline | Tag |
|---|---|
| stroppy-run | 4617c6bf8d542b06 |
| stroppy-suite | f29ab4baa3811fc5 |
| stroppy-provider-verify | 40cedcc6a1e9e620 |
| stroppy-quotas | fa2be471a320fc17 |

## Проверенные входы

| Вход | Graphene run | Результат |
|---|---|---|
| `verify-yandex.json` | `live-yc-verify-023` | Completed, ok=true, четыре проверки доступа |
| `verify-missing-secret.json` | `live-yc-missing-secret-001` | Completed, ok=false, ожидаемый NotFound |
| `quotas-yandex.json` | `live-yc-quotas-023` | Completed, permission_denied на квоты cloud; запуск ресурсов доступен |
| `run-yandex-missing-credentials.json` | `live-yc-missing-creds-003` | Failed, ожидаемый NotFound YC-секрета; пустое дерево |
| `run-yandex-postgres.json` | `live-yc-postgres-007` | Completed, 28 481 итерация, без ошибок, артефакты доступны после cleanup |
| `run-yandex-keep.json` | `live-yc-keep-008` | Completed, noop без ошибок, VM сохранена на 2 минуты и удалена по TTL |
| `suite-yandex-noop.json` | `live-yc-suite-009` | Completed, total=2, done=2, failed=0; обе ячейки без ошибок |

Результаты лежат рядом: `verify-yandex.result.json`,
`quotas-yandex-unavailable.result.json`, `run-yandex-postgres.result.json`,
`run-yandex-keep.result.json`, `suite-yandex-noop.result.json`. Входы и сохранённые результаты проходят
продуктовые схемы в `spec/manifest_test.go`. Исключение — SuiteResult:
для него продуктовая схема пока отсутствует, сохранённый результат
проверяется через отражённую выходную схему Graphene.

Noop использует `execute_sql`, SQL `--= smoke\nSELECT 1;`, один VU, 10 секунд,
`log_level: warn`. Он проверен реальным образом локально и в YC. `simple`
с noop-драйвером не подходит: проверяет 100 строк, драйвер возвращает одну.
PostgreSQL использует `simple`, два VU, 15 секунд и отдельные DB/runner VM
по 2 vCPU / 4 GiB. PostgreSQL 17.6 использует trust-auth только внутри сети
прогона; внешних ingress-правил нет. Все VM имеют 40 GiB boot SSD.

Suite содержит две независимые noop-ячейки, `concurrency: 1`,
`continue_on_failure: false`. По истории Temporal вторая ячейка стартовала
через 0.401 секунды ПОСЛЕ завершения первой, включая её cleanup.

## Жизненный цикл и артефакты

Run: ProviderConfig → сеть/подсеть/SG/VM → агент и исполнитель → host_prep →
Docker на первом runner и машинах с контейнерами → pull/контейнеры/health →
Stroppy → сбор метрик и upload артефактов → keep либо cleanup.

Docker готовится параллельно по машинам. Установка идемпотентна, timeout
15 минут; heartbeat отключён для этой activity, поскольку тело Install
его не отправляет. Ошибка установки прерывает deploy и вызывает cleanup.
Docker pull разбирает JSON progress stream: HTTP 200 с `error`/`errorDetail`
тоже считается ошибкой, сообщение «image pulled» при этом не выдаётся.

`keep: 0s` удаляет инфраструктуру после run. `keep: 2m` передаёт корневую
сеть stand с TTL, затем Graphene удаляет дерево. Для `live-yc-keep-008`
проверены RUNNING после завершения run и автоматическое удаление VM,
дисков, сети, подсети, SG после keepUntil `2026-09-12T13:56:18Z`.
Все ресурсы PostgreSQL и обеих ячеек suite также удалены из YC.
В дереве suite остаются две записи Completed дочерних run без дочерних
ресурсов; это история выполнения, а не оставшаяся инфраструктура.
Четыре артефакта suite скачаны после завершения с проверкой SHA-256.

Конфиг и лог каждого сегмента передаются `stand/stroppy-run` с отдельным
без TTL для конфигов (до явного удаления) и с TTL **30 дней** для логов
сегментов и baseline. Срок не зависит от keep инфраструктуры. После cleanup
PostgreSQL оба артефакта скачаны через Graphene GetBlob, SHA-256 совпали:

- config: `3bacbc7f5d5958202727a56f60989882e3ffeee49755959621c2f8bf4e50a013`;
- log: `eba7441a764e33de34197ca8292cd41d8782acb335052f76b949b066e4c358c9`.

После TTL noop-стенда его лог остаётся ready и доступен. Симуляция проверяет сохранность обоих типов до 30 дней и удаление
только логов после 30 дней. Тридцать реальных суток ещё не прошли.
У пяти ранее загруженных конфигов TTL снят; для пяти логов срок
обновлён до 30 дней от создания артефакта и проверен через Graphene API.

## Доступ и квоты

Graphene-секрет `yc-sa-key` хранит объект `{"sa_key_json":"<authorized key JSON>"}`.
Секрет `kubeconfig` содержит YAML kubeconfig SA `graphene/stroppy-live`.
Его используют и EnsureProviderConfig, и Crossplane-ресурсы; pod identity
не подменяет явно заданную учётку. `bootstrap.yaml` задаёт эту SA и RBAC.
Crossplane YC CRD имеют cluster scope. Доступ к существующим Secret и
ProviderConfig ограничен именем `t-stroppy-live`; Kubernetes не ограничивает
create через resourceNames. Эти права применены в данном тестовом контуре.

`bootstrap.py --kubeconfig <operator-config> --graphene-config <graphene-config>`
воспроизводит настройку с постоянным токеном SA. Манифест
`bootstrap-token.yaml` описывает Secret `graphene/stroppy-live-api-token` типа
`kubernetes.io/service-account-token`; значение заполняет контроллер Kubernetes,
в исходниках токена нет. Bootstrap создаёт Secret только при его отсутствии,
ждёт заполнения и при повторных вызовах использует тот же токен. Проверяет тип,
имя и UID ServiceAccount, JWT subject и отсутствие `exp` перед передачей в Graphene.

`--sync-token-only` синхронизирует этот kubeconfig в Graphene без применения
RBAC и выдачи облачных прав. Прежнее имя `--refresh-token-only` сохранено как
алиас; теперь оно также использует постоянный токен и не запрашивает восьмичасовой.
Оба клиента Kubernetes — EnsureProviderConfig и библиотека Crossplane-ресурсов —
читают один Graphene-секрет `t-stroppy-live/kubeconfig`.

Токен не имеет срока истечения. Для явного отзыва удаляют именно token Secret;
следующая настройка создаст новый и синхронизирует его в Graphene. Bootstrap
сам не удаляет и не ротирует существующий Secret. Не добавляйте этот Secret
в поле ServiceAccount.secrets: это вручную управляемый токен, а не автоматически
созданный legacy token. Механизм описан в
[документации Kubernetes](https://kubernetes.io/docs/tasks/configure-pod-container/configure-service-account/#manually-create-a-long-lived-api-token-for-a-serviceaccount).
Бессрочный токен при утечке требует явного отзыва; существующие права SA сохранены.

`persistent-kubeconfig-check.json` фиксирует успешную аутентификацию новым
токеном, чтение Crossplane API, неизменность RBAC и одинаковый kubeconfig при
двух последовательных bootstrap. Нагрузочные пайплайны в этой проверке не
запускались. Прежний восьмичасовой токен вызывал `Unauthorized` в
`stroppy.provider.ensure-config` ещё до создания VM; это отдельный доступ к
Kubernetes, не токен и не права YC.

Чтение квот облака работает: прогон `live-yc-quotas-access-1789381137302`
от 2026-09-14 вернул 11 квот Compute/VPC с лимитами и использованием.
Снимок сохранён в `quotas-yandex.result.json`. Ранее доступ был только на
каталог; [Quota Manager требует cloud scope](https://yandex.cloud/en/docs/quota-manager/security/).
При PermissionDenied пайплайн возвращает
`unavailable_reason: permission_denied`, `scope: cloud:<id>`, `quotas: []`.
Частичный снимок отбрасывается, квоты не объявляются нулевыми. Таймаут,
ошибка сети и Unauthenticated остаются ошибками.

Сервер сохраняет причину в кэше профиля, отдаёт её через QuotaReport и
пропускает предварительную проверку неизвестных облачных лимитов. Лимиты
тенанта и валидация профиля действуют. YC проверяет квоты при создании
ресурсов. Успешное обновление снимка очищает причину и возвращает preflight.
Миграция `quota_visibility` и оба перехода проверены через HTTP → сервис →
реальный Postgres → имитация Graphene (`TestE2EQuotaVisibility`, также race).
Это локальная проверка Stroppy Server; публикация pipeline сервер не обновляет.

В `main-v0` Quota Manager также запрашивался на cloud scope, с OAuth-токеном
из настроек провайдера. PermissionDenied был ошибкой; исключение делалось
для NotFound у сервисов без квот. Нет доказательства, что прежний код работал
с текущим сервисным аккаунтом. Выдавать дополнительные cloud-права для
продолжения тестов не требуется. При необходимости администратор облака
может использовать `bootstrap.py --grant-quota-viewer`.

## Повторение и публикация

Каждому запуску нужны уникальный Graphene run ID и новый UUID в RunSpec.
Первые восемь символов UUID входят в имена облачных ресурсов. Подготовка
копии автоматически меняет UUID run, suite и вложенных ячеек:

```sh
python3 pipelines/live/prepare.py pipelines/live/run-yandex-postgres.json \
  --output /tmp/postgres-request.json
graphenectl run start stroppy-run --run-id <unique-run-id> \
  --params-file /tmp/postgres-request.json
graphenectl run result <unique-run-id> -o json
graphenectl tree run/<unique-run-id> -o json
```

Контекст graphenectl должен указывать namespace `t-stroppy-live`.
prepare.py не перезаписывает существующий файл. Verify/quotas не создают VM.
После запуска проверяются результат, ошибки сегмента, артефакты, дерево
Graphene и фактическое отсутствие ресурсов в YC.

Для `push` обязательно используйте отдельный connection context с
`namespace: t-stroppy-live` и `push --context <name>`: флаг `-n` команды
`graphenectl` не влияет на namespace самостоятельного pipeline-бинаря.
Текущий пользовательский контекст может указывать `default`.

`make build-pipelines` собирает четыре бинаря. В SDK 0.2.1 `push` пересобирает
main package; запускать его нужно из `pipelines/` с исходниками и Go toolchain.
Registry Location исправлен в Graphene 0.2.2: push через обычный port-forward
работает без переписывающего прокси. После локальной публикации публичный
image reference должен быть `graphene.stroppy.io:443/...`, а не localhost.

## Исправленные отказы и границы проверки

- Graphene 0.2.2 (`b1a079f`): HTTPS bootstrap, TLS исполнителей, отдельный
  внутренний h2c, относительный registry Location. Cloud commit `472708b`.
- Graphene 0.2.3 (`7530f57`): токен namespace/run для VM-исполнителя при
  запуске и восстановлении. Пустой legacy run_token приводил к plaintext
  Temporal на порт 443 и HTTP 404. Реальный Started Worker подтвердил исправление.
- Stroppy: kubeconfig из секрета, omitempty рядом с omitzero для отражённой
  схемы, soft PermissionDenied квот, Docker preparation/progress errors,
  сохранение артефактов и корректные noop-входы.
- Диагностические `live-yc-noop-tls-004` и `live-yc-noop-token-005`
  завершены/отменены, их ресурсы удалены. Во втором Docker pull был принят за успех без проверки JSON
  stream, затем ContainerCreate получил No such image. На следующем
  прогоне образ загрузился; исходная причина сбоя registry не установлена.
- `live-yc-noop-pull-006` — Completed со старым simple, 2 505 616 ошибочных
  итераций. Без thresholds Completed не означает нулевой error rate.
- Собственный расчёт TPS из iterations/activity duration удалён: он включал
  подготовку контейнера и не был показателем Stroppy. Headline копирует только
  явно экспортированный Stroppy `tps`; при его отсутствии поле не публикуется.
  TPC-C `tpm_c` и throughput по типам транзакций остаются в исходном compliance
  отчёте. Образ Stroppy 6.0.0.62 не экспортирует универсальный `tps`.
  Development-образ нативного Stroppy расширяет экспорт TPS/OTLP; его вход
  для проверки — `suite-native-exports.json`.
- Логи и трассировка Graphene проходят через текущий backend: trace API
  PostgreSQL-прогона вернул 2 трейса / 69 spans. Экспорт
  workload OTLP через внешний bearer endpoint проверяется отдельным набором
  `suite-native-exports.json`; его актуальный результат — `native-export-check.json`. Остальные DB/topology и
  широкий fault-injection пока не входят в подтверждённую матрицу. Baseline
  проверен в PostgreSQL-прогоне ниже; AWS исключён из области тестирования.
- Все Go-тесты обоих модулей, pipeline lint и сборки прошли. Полный lint
  сервера имеет три прежние проблемы: форматирование application/config.go
  и два unnamedResult в application/schedules.go. make generate-ts не работает:
  в web/package.json нет generate; схемы экспортируются через schemas-export.

Сроки общих хранилищ не меняются: VictoriaMetrics 90d, VictoriaLogs 30d,
VictoriaTraces 14d. Артефакты используют отдельную политику: конфиги до
явного удаления, файлы логов — 30 дней.

## Полная матрица YC

В область проверки входят все версии и топологии каталога, включая реализацию
Patroni HA, MySQL Group Replication, MariaDB Galera и Managed YDB. AWS исключён.
Сначала проходят успешные сценарии, baseline, сегменты, телеметрия всех
компонентов и параллельные suite; после них — отдельная матрица отказов.

`scripts/live-inputs` использует серверные DeriveDatabase, DeriveWorkload и
Compile, проверяет результат через spec.run@1 и записывает только входы:

```sh
go run ./scripts/live-inputs --output /tmp/yc-matrix --all-versions \
  --registry-mirror docker.stroppy.io
go run ./scripts/live-inputs --output /tmp/pg-tpcc --database postgres \
  --topology single --script tpcc/tx --duration 1m --baseline
```

`matrix/inventory.json` содержит 65 сочетаний версии и топологии для нагрузки
simple: 58 компилируются, 7 требуют реализации. Это инвентаризация входов,
а не результаты реальных прогонов. External требует замены демонстрационного
DSN на реально подготовленную БД; simple на noop/pg-noop не проходит проверку
строк и должен получить отдельный совместимый сценарий. Набор workload,
executors, baseline и сегментов расширяет эту базовую матрицу.

Секреты OTLP добавляются только в приватную копию входа при запуске. Секреты
облака и Kubernetes передаются именами Graphene-секретов.

## Отдельный диск YC

Компилятор добавляет подготовку `/dev/disk/by-id/virtio-data` для ролей с
диском данных. Скрипт прекращает работу при ошибках проверки, наличии разделов,
использовании диска или неожиданных сигнатурах; повтор использует существующий
ext4. Файлы БД размещаются под `/data` на хосте. Нужны Graphene agent >= 0.2.1
и pipeline SDK >= 0.2.4: доступ к устройствам и исполнение команд в mount
namespace хоста.

Изолированная проверка в YC записана в `runtime-disk-check.result.json`:
настоящий runc, PostgreSQL 17, 10 000 строк после рестарта, сохранение данных
при повторной подготовке и перемонтировании, отказ на ошибке проверки и
загрузочном диске. Это отдельная проверка runtime.
Тестовая VM и оба диска удалены.

Установка Docker использует `library/docker v0.2.1`: внешний установщик имеет
общий таймаут 3 минуты, после которого применяется пакет дистрибутива.
Healthcheck поддерживает `CMD`, `CMD-SHELL` и прямой argv; ошибки Docker exec
сохраняются в диагностике.

Node-exporter в компилируемых входах читает host rootfs, procfs, sysfs и udev
через `/host`: `--path.rootfs=/host`, `--path.procfs=/host/proc`,
`--path.sysfs=/host/sys`, `--path.udev.data=/host/run/udev/data`.
Версия node-exporter закреплена на 1.12.1: она использует `ID_SERIAL`
для virtio-дисков YC; 1.9.1 оставляет serial пустым даже при доступном udev.

Полный прогон `matrix-pg-disk-a-bcd2581e` завершился **Completed** в
`ru-central1-a`: PostgreSQL 17 использовал отдельный **100 GiB network-ssd**,
`/dev/vdb`, ext4, `/data`; boot-диск — 40 GiB. Вход:
`postgres-disk-zone-a.json`, результат: `postgres-disk-zone-a.result.json`.
Stroppy выполнил **60 516 итераций** в 30-секундном сегменте, `exit_code=0`;
в результате нет полей ошибок. TPS не вычислялся пайплайном и отсутствует
в headline, поскольку этот workload не экспортировал его из Stroppy.

Baseline: `ok=true`, failed_iterations=0 в обоих tiers, но три предупреждения:
масштабирование noop 43% и wire 56% ниже порога 60%, wire p99 2.5 ms выше
порога 1 ms. Это успешное функциональное выполнение с предупреждениями,
а не подтверждение достаточной производительности стенда.

После завершения дерево run пусто; списки YC VM, дисков, сетей, подсетей и SG
не содержат ресурсов четырёх пробных прогонов и двух изолированных тестовых
VM. Все три артефакта остаются ready/verified у `stand/stroppy-run`: конфиг
без TTL, логи сегмента и baseline до 2026-10-14. Readiness сервера — Healthy.

Heartbeat передачи ресурсов исправлен в Graphene 0.2.5: повторный прогон
`matrix-pg-host-a240e0ec` выполнил 63 921 итерацию, `exit_code=0`, все передачи
ресурсов наблюдались на первой попытке. Вход, результат и снимок сохранённых
метрик: `postgres-host-metrics.json`, `postgres-host-metrics.result.json`,
`postgres-host-metrics.metrics.json`. Исправленные host-пути node-exporter
дают PCI path диска и метрики ext4 `/data`; PostgreSQL exporter отдаёт `pg_up=1`
и `pg_exporter_last_scrape_error=0`. Baseline завершён с предупреждениями по
масштабированию и задержке. Полная матрица и проверка телеметрии всех
компонентов остаются незавершёнными.

## Идентичность контейнеров и suite

Логические имена контейнеров остаются в RunSpec, зависимостях и событиях.
Физическое имя Docker и Graphene ref включают полный UUID прогона:
`docker/<run-UUID>-<logical-name>`. Healthcheck и flow-ссылки используют ту же
идентичность. Это предотвращает столкновения двух одинаковых топологий в
одном Graphene namespace.

Отмена родительского suite возвращает Canceled при любом
`continue_on_failure`. Cleanup Graphene отменяет дочерние run; их собственный
teardown может продолжаться после закрытия родителя. Окончание suite само по
себе не доказывает отсутствие VM: проверяются дочерние run и фактические
ресурсы YC.

Тестовые входы матрицы используют `--registry-mirror docker.stroppy.io`:
Nexus group проксирует Docker Hub, GHCR и Quay. Генератор сохраняет tag/digest,
добавляет `library/` для официальных Docker Hub образов и не переписывает
частные/неподдерживаемые registry. Сам пайплайн использует явно заданный image.
Прямой Docker Hub на части YC VM недоступен (таймаут `auth.docker.io`), поэтому
доступ к нему не является условием успешного теста через настроенный Nexus.

Параллельный suite `matrix-pg-mirror-df957f8d` завершён: **total=2, done=2,
failed=0**. Две независимые PostgreSQL 17 с отдельными 100 GiB data-дисками
и двумя runner VM выполнили 244 618 и 246 176 итераций; оба `exit_code=0`.
Сегменты настроены на 2 минуты. Интервалы между окончанием `load_data` и
`bench summary` в сохранённых логах Stroppy перекрываются на **114.267 s**;
это подтверждает одновременную нагрузку, а не только одновременный provisioning.
TPS пайплайн не вычисляет. Baseline в этом suite отключён и проверен отдельно.

Снимки метрик каждой ячейки содержат только её run, два её агента и три её
экспортёра. На обеих DB видны ext4 `/data`, `vdb` с `serial=data`, boot serial
совпадает с YC disk ID; PostgreSQL scrape error равен нулю. Передачи ресурсов
наблюдались только на первой попытке. Файлы результата, метрик, логов и проверки
очистки лежат рядом с `suite-yandex-postgres-parallel.json` с суффиксами
`.result.json`, `-pg-{1,2}.metrics.json`, `.evidence.json`, `.cleanup.json`.

Исправление статуса родительской отмены проверено локальной Temporal-симуляцией
для обоих значений `continue_on_failure`; отдельный live-тест отмены нового
образа suite пока не проводился. Прогон с прямым Docker Hub дал done=1/failed=1
из-за таймаута registry; этот результат сохранён отдельно и не засчитывается
как полностью успешный suite.

После параллельного suite проверено отсутствие всех созданных VM, boot/data
дисков, сетей, подсетей и security groups в YC; оба дерева дочерних run пусты.
Четыре артефакта после cleanup остаются ready/verified у `stand/stroppy-run`:
конфиги без TTL, логи до 2026-10-14. Readiness Graphene — Healthy.

## Версии конфигурации PostgreSQL

DeriveDatabase и компилятор выбирают `cfg.postgresql.conf@<major>` по
DatabaseSpec.version; для OrioleDB используется PostgreSQL major из image tag.
Параметры пользователя применяются к выбранной схеме, seed расширений сохраняется.
Явный override конфигурации другой major-версии отклоняется, чтобы настройки
не терялись молча. Шаблон роли каталога описывает конфиг версии по умолчанию;
EffectiveConfigs содержит конкретные схемы выбранной версии.

HAProxy использует закреплённую линию 3.0, сокет `/tmp/haproxy.sock` и
healthcheck через Bash TCP: официальный образ работает без root и не содержит
nc. Метрики HAProxy собираются с отдельного локального listener
`127.0.0.1:8405/metrics`. Рецепт PgBouncer добавляет exporter 0.12.1 на 9127,
подключённый к служебной БД pgbouncer; `extra_float_digits` включён в список
игнорируемых startup-параметров для клиента экспортёра.

Первый suite PostgreSQL 18 (`matrix-pg18-97b9ee5e`) проверил single и
primary/replica: done=2, failed=0, 254 621 и 246 047 итераций. Он использовал
старый конфиг @17; отдельный suite `suite-postgres-versions.json` проверяет
исправленные конфиги всех четырёх версий. Репликация 18 дополнительно проверена
SQL: 100 строк обычной таблицы на primary доступны на replica; recovery=true,
WAL receiver=streaming. SQL-доказательство: `postgres18-replication.evidence.json`.
`postgres_probe.py --seed` создаёт эту тестовую таблицу на primary; запуск без
флага только читает её и состояние репликации. Скрипт предназначен для локального
trust-доступа внутри изолированных тестовых VM.

В suite версий используется `defaults.continue_on_failure=false`: при ошибке
ячейки новые ячейки не запускаются, после исправления проходят оставшиеся.
Ключ верхнего уровня `continue_on_failure` не является полем SuiteSpec;
строгая проверка входа по spec.suite@1 обязательна до отправки в Graphene.

Матрица `matrix-pg-versions-ed460e6f` завершила pg15-single, pg17-single и
pg18-pgbouncer. Во время обновления сервера загрузка agent binary в cloud-init
PG16 получила HTTP 503 без повторной попытки; эта ячейка отменена, fail-fast
остановил suite. Результаты завершённых ячеек сохранены в `pg*.result.json`,
метрики — в `pg*.metrics.json`. `suite-postgres-versions.cleanup.json` проверяет
отсутствие ресурсов в YC, включая записанные ID безымянных boot disks.
`postgres-versions.artifacts.json` содержит проверки скачивания и SHA-256.

Graphene 0.2.6 добавляет до 10 попыток загрузки agent binary с сетевыми
таймаутами и атомарную установку. Вход `suite-postgres-versions-retry.json`
содержит девять оставшихся ячеек с новыми run UUID и теми же исправленными
конфигами; успешные ячейки первой матрицы в него не включены.

Метрики первого PG18 suite с конфигом @17 хранятся отдельно в
`pg18-initial-single.metrics.json` и `pg18-initial-primary-replica.metrics.json`.
Файлы без `initial` относятся к итоговым ячейкам матрицы версий.

Graphene 0.2.7 устраняет OOM при одновременной загрузке артефактов в S3:
точный размер файла передаётся MinIO SDK, потоки без Seek временно сохраняются
на диск. Ранее размер -1 выделял 528 MiB на загрузку даже маленького файла.
Диагностика и проверка исправления: `graphene-memory-check.json`.
Worker `stroppy-run:166453f8f98a639b` обновляет heartbeat сразу после запуска
Stroppy: `running workload` вместо устаревшего `pulling image` при числе строк
вывода меньше 50. Текущие запущенные worker продолжают прежний образ;
новые дочерние run получают опубликованный образ.

На 0.2.7 оставшиеся шесть сочетаний вынесены в
`suite-postgres-versions-final.json` (`matrix-pg-final-951a642e`).
Повторный suite дал успешные pg16-single, pg18-single и pg15-primary-replica.
PG16 primary/replica потерял heartbeat во время обновления сервера и повторяется
на стабильной версии; исходная попытка не считается успешной.
Инцидент сохранён в `pg16-replica-rollout-failure.json`. Предыдущий suite отменён
после завершения PG15, чтобы не запускать ячейки перед ожидаемым fail-fast.

`inspect_traces.py` проверяет сохранённые pipeline traces через Jaeger API
VictoriaTraces с явными `--start`/`--end` и `--endpoint`. Для доступа используется
локальный port-forward к observability. Стандартный Graphene Trace(ref) пока
ограничен 20 traces и не передаёт временной диапазон в backend; для старых прогонов его результат
может быть неполным из-за диапазона backend по умолчанию. Server/UI потребуется
явный диапазон поиска. Retention хранилища при этой проверке не меняется.
Эти spans подтверждают действия pipeline, а не OTLP-экспорт внутри Stroppy.

`inspect_metrics.py` по умолчанию использует Graphene CLI и его окно последнего
часа. Для итоговой проверки старых ячеек передаются `--endpoint`, `--start`,
`--end`: запрос к VictoriaMetrics сохраняет точный run/namespace selector.
Graphene Metrics API поддерживает start/end; CLI >= 0.2.9 выставляет их
через --start/--end в RFC3339. Это необходимо учитывать при реализации Server/UI.
`graphene-s3-concurrent-check.json` подтверждает две одновременные повторные
загрузки существующих байтов через Graphene в реальный S3 на 0.2.7; идентичность
блобов сохранена, перезапуска сервера после проверки нет.

Проверенный YC-профиль для генерации входов: `provider-yandex-zone-a.json`.
Он содержит ссылки на секрет и cloud/folder/zone, без ключа service account.
Чтобы повторить компиляцию PostgreSQL 15–18 без ручной правки зоны:

```sh
go run ./scripts/live-inputs --database postgres --all-versions \
  --provider pipelines/live/provider-yandex-zone-a.json \
  --registry-mirror docker.stroppy.io --duration 2m --output /tmp/pg-inputs-new
```

UUID прогонов генерируются заново.

## Patroni HA

Рецепт `patroni-ha` компилирует PostgreSQL 15–18 с Patroni 4.1.5,
тремя отдельными etcd 3.5.33 и HAProxy. PostgreSQL и etcd используют отдельные
диски `/data`; PGDATA находится в подкаталоге `pgdata`, чтобы Patroni мог
перемещать повреждённый каталог, не переименовывая точку монтирования.
Schema `cfg.patroni.yml@4` доступна для ролей db и db-replica; @3 сохраняется
для совместимости. Версионный postgresql.conf подключается как custom_conf,
HBA задаётся через postgresql.parameters.hba_file. Bootstrap создаёт расширение
pg_stat_statements и выполняет init_sql в стандартной БД postgres.

Все PostgreSQL-узлы могут стать лидером. HAProxy проверяет `/primary` для
RW:5000 и `/replica` для RO:5001 на REST-порту 8008 всех трёх узлов.
Сетевые правила разрешают связи между etcd peers и между PostgreSQL/Patroni
на всех узлах. При sync_replicas > 0 включён строгий синхронный режим:
Patroni управляет synchronous_standby_names.

Метрики собираются с node-exporter, postgres-exporter, HAProxy и собственных
`/metrics` Patroni и etcd. Проверенный локально трёхузловой кластер и запросы
через оба порта HAProxy: `patroni-local-check.json`. Локальная проверка
не подтверждает облачный provisioning или переключение при отказе.

Образы собираются из `images/patroni/Dockerfile`, базовый PostgreSQL —
официальный образ через Nexus. Push: `registry.stroppy.io/stroppy-io/patroni`,
pull: `docker.stroppy.io/stroppy-io/patroni:pg<major>-4.1.5`.
Входы: `matrix/postgres-patroni-ha-*-simple.json`; последовательный suite:
`suite-postgres-patroni.json`. YC квоты проверены отдельным read-only прогоном,
результат: `patroni-quota-preflight.result.json`.

Cluster-wide `bootstrap.dcs` берётся из `configs.db.cfg.patroni.yml@4`
для всех членов, поэтому первичная гонка лидера не меняет пользовательские
TTL/лимиты. Узловые параметры роли db-replica остаются локальными.
Patroni без HAProxy отклоняется компилятором: фиксированный адрес db-1
не гарантирует соединение с текущим лидером.

Первый suite `matrix-patroni-e1cc9a0c` отменён до workload: проверка контракта
обнаружила, что `Scrapes.Job` должен совпадать с точным именем контейнера.
Исходный вход сохранён в `suite-postgres-patroni-initial.json` и не считается
успешным. Исправленный `suite-postgres-patroni.json` привязывает Patroni к
каждому DB-контейнеру, а etcd использует собственный Scrape на порту 2379.
Контракт закреплён в TestPatroniVersions.

При запуске восьми VM обнаружен общий транспортный дефект: входной Traefik
обрывал bidirectional gRPC AgentAPI.Session через 60 секунд (`RST_STREAM
INTERNAL_ERROR`). Распаковка executor на proxy-1 была отменена с context
canceled, а сервер продолжал ждать ответ команды от завершённой сессии.
Исходный вход — `suite-postgres-patroni-stream-failure.json`, диагностика —
`patroni-agent-stream-failure.json`. До workload этот suite не дошёл.

В инфраструктуре websecure readTimeout выставлен в 0s (cloud `f455ddd`);
в Graphene ожидание команды прерывается при закрытии соответствующей сессии,
чтобы повтор activity использовал новое соединение. Проверены EOF и отмена
контекста с последующим успешным повтором, включая race detector.

Graphene 0.2.8 развёрнут из cloud `b1a171c`: образ GHCR и зеркала имеет digest
`sha256:f5d9b9adbfe14d5008152999679b110aa4a0ab624718fc422aa322ac6945e1a0`.
`graphene-session-recovery-check.json` содержит CI/релиз и проверки исправления.
`inspect_component_logs.py` считает сохранённые Docker-логи через VictoriaLogs
с точным namespace/run selector и отвергает потенциально усечённый ответ.
Исходные сообщения в отчёт не копируются; TTL общего хранилища не меняется.

На большом стенде общий ответ метрик может превышать 8 MiB. Graphene 0.2.8
молча обрезает такой JSON; исправление в 0.2.9 возвращает явную ошибку без
частичного snapshot. Лимит остаётся 8 MiB. Запросы отдельных ресурсов или
выбранных метрик укладываются в ограничение; CLI поддерживает явный интервал
для обеих форм команды и raw PromQL. Проверка полной телеметрии этой матрицы
использует VictoriaMetrics с точным namespace/run и интервалом workload.

`patroni-docker-fallback-check.json` фиксирует реальное срабатывание
внешнего installer timeout: загрузка Docker apt signing key на одной VM
PostgreSQL 16 не завершилась за 180 секунд. Пайплайн самостоятельно установил
Ubuntu docker.io и продолжил запуск контейнеров; ручного вмешательства не было.
Такой же автоматический fallback подтверждён на proxy-1 в PostgreSQL 15.
`patroni-native-summary-check.json` сверяет iterations_total из скачанных
журналов Stroppy с результатами pipeline. Явный нулевой счётчик неуспешных
итераций в этой версии вывода отсутствует; он не подставляется проверкой.

Итог Patroni suite `matrix-patroni-96794a09`: **4/4 Completed**.

| PostgreSQL | Итерации Stroppy | Метрики / логи компонентов | Pipeline spans |
|---|---:|---:|---:|
| 15 | 144852 | 18 / 18 | 608 |
| 16 | 137621 | 18 / 18 | 610 |
| 17 | 132109 | 18 / 18 | 613 |
| 18 | 134147 | 18 / 18 | 613 |

SQL-доказательства находятся в `pg*-patroni-ha.replication.json`,
непрерывность соединений агентов — в `pg*-patroni-ha.agents.json`.
`patroni.artifacts.json` проверяет восемь артефактов после завершения;
`patroni.cleanup.json` проверяет отсутствие VM, boot/data disks, сетей,
подсетей и security groups, включая две отменённые подготовительные попытки.
На протяжении четырёх итоговых прогонов Graphene 0.2.8 не перезапускался.

## Нативный экспорт Stroppy

`native-export-check.json` содержит актуальное подтверждение TPS/OTLP и состояния
нагрузок. `native-exports-contention-check.json` сохраняет исходную проверку с
конфликтами TPC-C при трёх попытках. Совпадение экспорта и успешность workload —
разные проверки: ненулевые ошибки сохраняются и не превращаются в успешный smoke.

Stroppy сам считает TPS успешно завершённых логических транзакций и отдельно
скорости итераций/запросов. Окно общее для всех VU, исключает setup/teardown;
время повторов и неуспешных попыток входит в окно. Сохраняются счётчики, гистограммы,
метрики загрузки и исходный отчёт TPC-C. Для каждого сегмента задаётся отдельный
`service.instance.id`, а run/namespace/segment присутствуют на точках OTLP.

`inspect_native_metrics.py` читает сохранённые данные через Graphene Metrics API
и сравнивает их с итогами Stroppy. Ключ `--allow-workload-errors` используется
для диагностики точности экспорта с ненулевыми ошибками и не означает успешный
workload. Remote write добавляет единицу скорости к имени: `tps` становится
`stroppy_tps_per_second`, а `iterations_per_second` и `queries_per_second` —
`stroppy_iterations_per_second_per_second` и `stroppy_queries_per_second_per_second`.

`native-graphene-rollout.json` подтверждает Graphene 0.2.10, digest сервера,
Argo Synced/Healthy и SHA-256 опубликованной CLI. Образы native Stroppy —
development-сборки с digest, SHA-256 бинаря и хешами исходников; это не релиз Stroppy.

Подтверждено 5/5 сегментов на PostgreSQL 17 single: simple, TPC-B tx/procs и
TPC-C tx/procs. Для simple TPS не публикуется; есть iterations/s и queries/s.
Итоговые значения совпали с сохранёнными точками Graphene Metrics API; пять
отправителей имеют разные instance ID. Полные итоговые сводки, перечни
экспортированных метрик и compliance TPC-C находятся в `native-tpcb.metrics.json`
и `native-tpcc.metrics.json`. Временные ряды и histogram buckets хранятся
в VictoriaMetrics и доступны через Graphene Metrics API.

Повторный TPC-C (`suite-native-tpcc-retry.json`) сохраняет Repeatable Read и
2 VU: 4 874 / 9 576 успешных логических транзакций, 866 / 2 678 повторов,
25/25 и 51/51 предусмотренных откатов, без terminal errors. Это 20-секундный
функциональный unpaced smoke, а не сертифицированный результат TPC-C.
Подтверждены метрики и логи компонентов и pipeline traces; нативные трейсы
Stroppy этим тестом не подтверждаются.

Для двух завершённых native suites проверены скачивание и SHA-256 14 артефактов
(включая исходные TPC-C результаты с ошибками). Удалены их 6 VM, 9 дисков,
3 сети, 3 подсети и 3 SG; исходный набор YC не изменился. Конфиги сохраняются
до явного удаления, артефакты логов — 30 дней, shared retention не менялся.
`native-local-checks.json` описывает локальные проверки, включая настоящий
PostgreSQL и распознавание ожидаемого rollback. Изменения Stroppy и Stroppy Cloud
не закоммичены; Graphene и его GitOps-обновление опубликованы.

### Временное решение: dev-сборки до релиза Stroppy

Для проверки исправлений нативного Stroppy на реальном YC используется
dev-образ из локального рабочего дерева, включая незакоммиченные изменения.
Это согласованный временный способ тестирования до переноса исправлений
через PR в `stroppy-io/stroppy`, слияния в `main` и публикации официального образа.

Исправления сохранены в коммите `cdfb7b4` ветки
`fix/native-metrics-and-database-compatibility` и опубликованы в
[Stroppy PR #166](https://github.com/stroppy-io/stroppy/pull/166).
PR открыт; переход на официальный образ ещё не выполнен.

Порядок сборки и проверки:

1. Собрать бинарь из рабочего дерева командой
   `make build VERSION=dev-native-<source-hash>`.
2. На основе существующего runtime-образа Stroppy создать отдельный OCI-образ,
   заменив `/usr/local/bin/stroppy` собранным бинарём. Опубликовать его в
   `registry.stroppy.io/stroppy-io/stroppy` с отдельным тегом
   `dev-native-<binary-sha-prefix>`.
3. Проверить загрузку через используемый в YC Nexus mirror
   `docker.stroppy.io`, digest образа, SHA-256 и версию бинаря внутри контейнера.
4. Указать в тестовом входе образ по неизменяемому digest. Сохранить базовый
   Git-коммит, хеши изменённых и новых исходников, версию и SHA-256 бинаря,
   digest образа, вход прогона без секретов и результаты проверки.

Точные идентификаторы уже проверенных сборок находятся в поле `images`
файла [`native-export-check.json`](native-export-check.json); входы —
[`suite-native-exports.json`](suite-native-exports.json) и
[`suite-native-tpcc-retry.json`](suite-native-tpcc-retry.json).

Ограничение: хеши подтверждают идентичность файлов, но не сохраняют их содержимое.
Для этих сборок отдельный patch или архив исходников не сохранён; чистого клона
базового коммита недостаточно, чтобы повторить сборку. Текущее состояние исходников
сохранено в PR #166; оно не восстанавливает автоматически каждую промежуточную сборку. Проверка
dev-образа не является проверкой официального Git-релиза.

Временное решение снимается после ревью и слияния PR, публикации официального
образа, замены dev-digest в тестовых входах на digest этого образа и повторения
соответствующих проверок. Само создание PR ещё не завершает переход.

## Проверки MySQL и MariaDB

`mysql-family-check.json` содержит подтверждённые каталожные smoke и отдельные
нативные проверки TPC-B/TPC-C. Набор этапа: MySQL 8.0/8.4 single и semi-sync,
MariaDB 10.11/11.4/11.8 single. Проверено **7/7**, на каждую ячейку — simple (2 минуты,
2 VU), TPC-B tx/procs и TPC-C tx/procs (по 20 секунд, 2 VU, SF=1).
Для TPC-C явно задано 50 попыток; изоляция и стандартный лимит Stroppy не меняются.
Group Replication и Galera реализованы; их облачная проверка описана ниже и учитывается отдельно.

Все семь итоговых ячеек используют один digest из `mysql-native-image.json`;
каждая имеет пять сегментов без terminal errors и корректные сохранённые
счётчики загрузки. Первоначальный MySQL 8.4 сохранён отдельно как
`mysql84-single-initial.*`. В CSV — 98 строк и 56 подтверждённых проверок
вместе с предыдущим PostgreSQL-этапом; полные workload, baseline и отказы
остаются pending. Функциональный smoke их не заменяет.

`mysql-family-cleanup.json` подтверждает удаление ресурсов всех попыток этапа:
36 VM, 57 дисков, 16 сетей, 16 подсетей и 16 security groups. Исходные объекты
YC сохранены: 12 VM, 20 дисков, 3 сети, 4 подсети и 6 security groups.


Входы `suite-mysql-fixed.json`, `suite-mysql-family-final.json`,
`suite-mysql84-single-fixed.json` и `suite-mariadb-fixed.json` получены через
компилятор каталога с конфигурацией выбранной версии. Схема MySQL 8.0 имеет
канонический ID `cfg.my.cnf@8`. Для semi-sync включаются плагины source/replica
и заданное число подтверждений; готовность реплики требует работающих receiver
и applier и появления исходной базы. Проверки версий и пользовательских
переопределений находятся в `internal/domain/compile/mysql_test.go`.

`mysql-family-failures.json` сохраняет найденные ошибки и исправления.
Исходный TPC-C на MariaDB 11.8 имел конечные ошибки 1020, поэтому его результаты
`maria118-initial.*` не считаются успешным workload, даже при точном OTLP-экспорте.
`mysql-native-image.json` фиксирует новую dev-сборку с исправлением классификации
1020 и двойного учёта остатка SQL-пакета. Для неё сохранён patch рабочего дерева
в приватном каталоге проверки; исправление теперь включено в Stroppy PR #166.

Общий словарь результата допускает до 64 × 256 метрик: лимит одного сегмента
не применяется к сумме всех сегментов. Список артефактов вмещает конфиг и лог
каждого из 64 сегментов и один лог baseline. Итоговые метрики не обрезаются.
Генератор входов поддерживает `--stroppy-image <image@sha256:...>` для проверки
конкретной dev-сборки через тот же путь компиляции.

MySQL при `--initialize` игнорирует `plugin_load_add`: параметры semi-sync
рендерятся с префиксом `loose-`, чтобы создать системные таблицы на пустом диске.
После запуска healthcheck требует включённый плагин; на реплике — также активный
статус semi-sync. Это предотвращает зачёт тихого перехода к обычной репликации
при недоступном плагине. Семантика префикса описана в
[документации MySQL](https://dev.mysql.com/doc/refman/8.0/en/option-modifiers.html).
SQL-проверка `mysql_probe.py` читает состояние через обычное соединение с тестовой
БД; не требует Docker socket или sudo для интерактивного пользователя агента.

Реплика получает `read_only=ON` в постоянном конфиге. `SET GLOBAL` в initdb SQL
сам по себе не переживает остановку временного сервера официального entrypoint.
Готовность проверяет `@@GLOBAL.read_only=1`; live SQL-проверка сохраняет значения
обоих узлов. Неудачный запуск до исправления записан отдельно в
`mysql84-semi-sync-initial.replication.json` и не засчитывается в матрицу.

Для MySQL/MariaDB CSV также содержит `database_actual_version`, `started_at`,
`finished_at`, `component_logs`, `artifacts` и `replication`. Статус `passed`
добавляется после проверки сохранённых результатов и cleanup. Поля без
подтверждения остаются `pending`; функциональный smoke не закрывает полные
матрицы нагрузки, baseline и отказов.

В MariaDB 10.11 поле `tx_isolation` сохраняется в схеме для совместимости входов,
но рендерится в `transaction-isolation` в option file. SQL-имя до 11.1.1 не
совпадает с именем параметра запуска; см.
[SET TRANSACTION](https://mariadb.com/docs/server/reference/sql-statements/administrative-sql-statements/set-commands/set-transaction).
Финальная очередь `suite-mysql-family-final.json` объединяет два MySQL semi-sync
и исправленную MariaDB 10.11, с concurrency=2.

Graphene 0.2.11 (`6cb3b28`) использует имена managed Deployment с хешем полной
пары namespace/run и ищет действующие воркеры по двум меткам identity.
Это устраняет коллизию длинных ID параллельных ячеек. Сервер развёрнут через
GitOps (`cloud` commit `a237dc0`); `graphene211-check.json` фиксирует digest
образа, CI и rollout, `graphene211-workers.json` — два разных готовых worker.
Отсутствовавший worker неудачной очереди восстановился автоматически и обработал
ожидавшую отмену (`graphene211-cancel-recovery.json`).

Для крупных стендов `inspect_metrics.py --entity docker/<id>` читает каждый
exporter отдельным запросом: это соблюдает лимит ответа Graphene 8 MiB без
обрезания данных. `inspect_mysql_replication_metrics.py <evidence-prefix>`
проверяет semi-sync, подтверждения, отсутствие асинхронного fallback, ошибки
репликации и итоговую задержку по сохранённым сериям. Учитываются старые и новые
имена MySQL `slave/replica` и `master/source`.

Доступность MySQL и состояние репликации проверяются от начала первого сегмента
до 15 секунд после последнего (один шаг запроса). Полный снимок сохраняется
отдельно и включает штатное отключение БД во время cleanup; его `mysql_up=0`
после нагрузки не означает падение workload. В отчёте это поле
`workload_observation` и явный `health_scope_note`.

## MySQL Group Replication и MariaDB Galera

`mysql-clusters-check.json` и `suite-mysql-clusters.json` описывают пять
сочетаний: MySQL 8.4/8.0 Group Replication и MariaDB 11.8/11.4/10.11 Galera.
Каждое включает три узла БД, ProxySQL 2.7.3 и runner, отдельные диски БД,
node-exporter на каждой VM и mysqld-exporter на каждом узле БД. ProxySQL
экспортирует собственные метрики на 6070/metrics. Облачная suite выполняется
с concurrency=2 и теми же пятью функциональными сегментами.

Локально все пять сочетаний прошли bootstrap на пустых дисках, готовность
трёх узлов и ProxySQL, запись через ProxySQL и чтение этой строки на каждом
узле. Это поле `local_cluster` в CSV; оно не означает успешный YC workload.
В YC все пять сочетаний прошли по пять функциональных нагрузок — 25 сегментов
без терминальных ошибок Stroppy. Проверены сохранённая телеметрия, артефакты
и SQL всех членов. У Galera 10.11 первые 45 секунд метрик состава имеют
отдельно описанное ограничение покрытия; полные данные сохранены ниже.
Все три MariaDB single прошли повторную YC-проверку exporter.

MySQL получает уникальные server_id, общий UUID группы, полный набор XCOM
адресов, TLS для recovery и групповых соединений. Init recovery-пользователя
не пишет локальные GTID; таблицы часовых поясов загружаются только на seed
и передаются остальным репликацией. Plugin-параметры используют `loose-` для
инициализации пустого datadir. SQL настройки recovery SSL задаются переменной
Group Replication, а не неподдерживаемым SOURCE_SSL для recovery-channel.

Galera использует поставляемые официальным образом mariabackup и provider.
Системные таблицы сначала создаются с wsrep disabled; затем первый узел
создаёт компонент, остальные присоединяются через SST. Имя группы — UUID
без дефисов (32 символа); более длинное имя вызывало отказ gcomm handshake
в локальной проверке. SQL readiness требует Primary, Synced и полный размер
кластера перед нагрузкой. Сохранённый bootstrap marker запрещает повторное
автоматическое создание группы после рестарта. Полное восстановление после
потери кворума и отказные сценарии ещё не проверены.

ProxySQL получает нативный блок hostgroups выбранного кластера и ожидает все
три узла. Для Galera preset задаёт `writer_is_also_reader=2` до применения
пользовательских настроек; явное значение, в том числе 0, сохраняется.
Монитор репликации управляет writer/reader/backup/offline группами;
четыре hostgroup ID должны различаться. `mysql_cluster_probe.py` читает
состояние членов и тестовую таблицу через обычный SQL от пользователя агента,
без sudo или Docker socket. Сохранённые GR/wsrep/ProxySQL серии сохраняют
метки каждого участника, чтобы не смешивать состояние разных узлов.

Источники: [MySQL bootstrap](https://dev.mysql.com/doc/refman/8.4/en/group-replication-bootstrap.html),
[официальный MySQL entrypoint](https://github.com/docker-library/mysql/blob/master/8.4/docker-entrypoint.sh),
[MariaDB Galera](https://mariadb.com/docs/galera-cluster/galera-cluster-quickstart-guides/mariadb-galera-cluster-guide),
[ProxySQL Group Replication](https://proxysql.com/documentation/group-replication-configuration/),
[ProxySQL Galera](https://proxysql.com/documentation/galera-configuration/),
[ProxySQL metrics](https://proxysql.com/documentation/prometheus-exporter/).

Кластерные preset используют согласованные чтения: MySQL
`group_replication_consistency=BEFORE`, Galera `wsrep_sync_wait=1`. Это seed
перед пользовательским override, поэтому preview и скомпилированный конфиг
совпадают, а явный выбор другой согласованности сохраняется. SQL isolation
не изменяется. Первая YC-попытка MySQL 8.4 с eventual-репликами получила
семь `row count mismatch` в simple, при нулевых terminal errors в четырёх
TPC-B/TPC-C сегментах. Она не засчитывается как успешный smoke.

`mysql-clusters-initial-cleanup.json` подтверждает удаление первых десяти VM
и шестнадцати дисков. Повторная suite использует исправленную collation
ProxySQL и повышает только мягкий лимит файлов процесса до уже разрешённого
жёсткого лимита. Состояние очереди и её run ID — в `mysql-clusters-check.json`.

Docker library **0.2.2**, commit `3ceccfe`, ожидает штатную dpkg-блокировку
до 120 секунд через временный APT_CONFIG. Проверены реальный fcntl lock,
unit-тесты, lint, CI и опубликованный Go module; `docker022-check.json`
содержит доказательства и образ нового worker. Сервер Graphene развёрнут в версии 0.2.13; проверка — `graphene-long-run-id-check.json`.

`inspect_native_metrics.py` сравнивает итоговый экспорт Stroppy в коротком
30-секундном окне после завершения сегмента. Это проверка финальных значений,
не выгрузка всех временных рядов за весь сегмент. Полный объём телеметрии
остаётся в backend с прежним retention; лимит ответа API не увеличен.

MariaDB создаёт пользователя exporter с `SLAVE MONITOR` в дополнение к
`PROCESS`, `REPLICATION CLIENT`, `SELECT`; право нужно для `SHOW SLAVE STATUS`
на поддерживаемых версиях. Сбор проверяется через
`mysql_exporter_collector_success{collector=...}=1` для каждого включённого
collector и каждого узла за время нагрузки. Старый
`mysql_exporter_last_scrape_error` отсутствует в mysqld-exporter 0.19.0,
поэтому пустой список этой метрики не подтверждает отсутствие ошибок.
Исходный отчёт сохранён в `mysql-family-before-collector-audit.json`, ошибки
старых прогонов — в `*.collector-check.json`. Исправленная очередь из шести
сочетаний — `suite-mariadb-monitor-fixed.json`.

Для Galera также сохраняются `mysql_galera_status_info` и метрики задержки
групповой репликации: проверяются общий state UUID, Primary/Synced, размер
кластера, ready/connected и прохождение запросов через ProxySQL.
См. [exporter collector status](https://github.com/prometheus/mysqld_exporter/blob/v0.19.0/collector/exporter.go)
и [MariaDB GRANT](https://mariadb.com/docs/server/reference/sql-statements/account-management-sql-statements/grant).

Рендер `rpl_semi_sync_source_timeout` использует пользовательское значение
в `loose-rpl_semi_sync_source_timeout`. Регрессионный тест проверяет отличное
от default значение 1733 мс на обеих схемах MySQL. В YC пока проверены
каталожные preset; отдельный эксперимент с нестандартным timeout не выполнен.

Для финальных MariaDB-проверок `workload_observation` использует исходные
samples `/api/v1/export` с точными границами времени коллекции OTLP. Обычный range query
может подставить последний bootstrap-замер до начала workload; это дало
ложный `cluster_size=2` при уже готовом кластере из трёх узлов. Исходные
samples и проверка границ сохранены в
`mariadb-galera-118.bootstrap-samples.json` и `raw-metrics-check.json`.
`first_at`/`last_at` каждого collector показывают покрытие коллекциями OTLP;
готовность всех участников до workload отдельно проверяется SQL healthcheck.
Полный обзор метрик и финальные native measurements по-прежнему читаются
через Graphene API. Raw export выполняется через кратковременный локальный
port-forward к хранилищу; после чтения он закрывается. Retention не меняется.

`component_log_observations` в CSV содержит наблюдения из логов за весь run
(поэтому повторяется у его сегментов); `retry_attempts` — нативный счётчик
Stroppy конкретного сегмента. `not_checked` означает, что дополнительный
анализ severity для старого прогона не выполнялся. Неизвестные ERROR/FATAL
блокируют проверку. Известные `HA_ERR_RECORD_CHANGED` MariaDB и таймауты
Galera-monitor ProxySQL сохраняются отдельными наблюдениями; успешность
workload, collectors и репликации проверяется независимо. Присутствие этих
наблюдений не означает, что error-log пуст или что таймаутов не было.

Для MariaDB 11.8 запись `Got error 123` связана с конфликтом snapshot isolation;
см. [MDEV-37085](https://jira.mariadb.org/browse/MDEV-37085).
В Galera 11.4 наблюдались три таймаута monitor по 803 мс во время TPC-C,
при успешных workload и проверках кластера. Isolation, monitor timeout и
порог исключения узлов из пула не изменяются ради устранения сообщений.

У Galera 10.11 выявлена дополнительная задержка: синхронный OTEL gauge
повторно отдаёт последнее значение с временем очередной коллекции, даже без
нового scrape. Это воспроизведено на SDK 1.46.0. Значение `cluster_size=2`
в 12:05:24 UTC относится к запуску: журналы всех трёх БД фиксируют состав
из трёх участников в 12:04:57 и синхронизацию последнего в 12:05:09,
до старта workload в 12:05:14; дополнительный SQL probe подтвердил состояние.
Доказательства: `mariadb-galera-1011.startup-telemetry.json`.

Полное окно сохранено в `workload_observation`; для этой ячейки строгая
проверка метрик состава начинается спустя 45 секунд (период scrape 30 с
плюс коллекция OTLP 15 с), в `cluster_stable_observation`. Это явно ограниченное
покрытие метрик, а не доказательство непрерывного состояния кластера.
Нагрузка, collectors и mysql_up проверяются за исходное окно. В будущем
для точного сопоставления событий требуется сохранять время scrape источника;
текущее время OTLP-коллекции ему не эквивалентно. Настройки retention прежние.


Накопление артефактов выявило превышение лимита Temporal у `stand/stroppy-run`:
кеш 100 ответов accept содержал 100 копий растущего списка holdings.
Последний MariaDB single 10.11 остановился на transfer артефакта до окончания
pipeline. Исправление Graphene **0.2.12** возвращает компактный `count` для
accept/extend/release; полный список читается через describe. Состояние, TTL
и артефакты сохраняются. `graphene212-check.json` показывает статус релиза
и восстановления. `graphene212-stand-recovery.json` подтверждает сохранность
254 holdings и 255 артефактов: payload нового workflow — 41 108 байт вместо
2 458 311. Передача разблокировалась в том же прогоне; suite завершилась
успешно: 6/6 ячеек, 30/30 сегментов (`suite-mariadb-monitor-fixed.result.json`).
Лимит Temporal не повышается.


Итоговая сверка `mysql-clusters-cleanup.json` охватывает три попытки этой
кампании, включая повторные single: удалены 51 VM, 81 загрузочный/отдельный
диск, 12 сетей, 12 подсетей и 12 security groups. Все исходные ID ресурсов
сохранены; managed workers этих прогонов отсутствуют. Скачаны и проверены
SHA-256 80 артефактов восьми принятых свежих прогонов (два MySQL GR,
три Galera, три MariaDB single). Конфиги остаются до явного удаления,
логи — 30 дней, общий retention не менялся.

У Galera 10.11 также сохранены два таймаута ProxySQL Galera-monitor.
Счётчики таких наблюдений видны в CSV; они не скрываются утверждением
об отсутствии терминальных ошибок workload.

## Текущая проверка распределённых СУБД

Все шесть сочетаний Picodata (25.3, 26.1, 26.2 × single/three-node) прошли
YC-матрицу `suite-picodata-metrics.json`: 18 native-сегментов simple/TPC-B/TPC-C
без terminal errors. TPC-C работает без транзакций; два VU используют два склада.
Проверены SQL-состав на каждом участнике, сохранённые native OTLP и скалярные
метрики компонентов, логи, трейсы, скачивание артефактов с SHA256 и удаление
ресурсов стендов. `catalog-functional-check.json` и `progress.csv` содержат
принятые результаты. Это функциональные проверки, не полная нагрузочная или
отказная матрица; graceful decommissioning отдельно не проверялся.

Picodata membership probe завершает pgwire-сессию сообщением Terminate.
Точные EOF старого probe и стартовые сообщения репликации, доставленные
Graphene после начала нагрузки, сохраняются в `*.probe-diagnostics.json`
с run ID, entity, исходным временем и SHA256. Только эти конкретные сообщения
исключаются из ошибок workload; остальные ERROR/FATAL блокируют проверку.
Время доставки логов Graphene пока не заменяет исходное время Docker.

`container.scrape_port` задаёт HTTP-порт для `scrape`: Picodata — 8081,
CockroachDB — 8080, YDB — 8765. При отсутствии поля старые RunSpec используют
первый порт контейнера. YDB использует `/counters/prometheus` на каждом storage
и compute контейнере. `database-metric-coverage.json` фиксирует пробел:
Graphene scrape сохраняет скалярные серии, пропускает histogram/summary
и теряет тип counter. Полный экспорт метрик БД остаётся открытым; распределения
native Stroppy OTLP проверяются отдельно. В CSV это отдельная колонка
`database_metric_distributions`, которая не помечается успешной по скалярам.

CockroachDB локально проверен на всех шести версиях и топологии three-node:
30 сегментов simple, TPC-B tx/procs, TPC-C tx/procs и состав на каждом участнике
(`cockroach-cluster-local-check.json`). YC-матрица из 12 сочетаний single/three-node
запущена как `matrix-cockroach-74ee56ed`, входы — `suite-cockroach-matrix.json`.
Проверка участников использует обычный SQL и HTTP, без sudo и Docker socket.
Все 12 YC-ячеек приняты: по пять нагрузок без terminal errors, SQL/HTTP,
сохранённая телеметрия, артефакты и очистка подтверждены.

Self-hosted YDB локально прошёл simple/TPC-B/TPC-C на четырёх версиях
25.4, 26.1, 26.2, 26.3 (`ydb-local-check.json`). Readiness compute выполняет
`scheme ls <database_path>`, чтобы дождаться готовности SchemeShard к DDL.
Восемь YC-входов single/mirror3dc — `suite-ydb-matrix.json`; multi-DC использует
зоны a/b/d. Все восемь сочетаний single/mirror приняты по функциональным проверкам.
Mirror 26.1–26.3 повторяются с нуля после исправления runtime YC provider,
чтобы отдельно проверить provisioning без ручного восстановления ссылок. Для 26.1 проверено полное скачивание
копии неизменённого upstream-образа из собственного hosted registry;
`ydb-26.1-mirror-check.json` содержит фактический статус проверки digest.

Для Managed YDB роль `stroppy-live-yandex` разрешает жизненный цикл двух
Crossplane-ресурсов (`managed-ydb-access-check.json`). Временный IAM-конфиг
создаётся для реального `drivers["0"]` с правами 0600 и удаляется после запуска,
в том числе при ошибке; исходный конфиг не получает IAM-токен.
Прежний Serverless завершил workflow с терминальными ошибками workload и
не засчитан. Stroppy `dev-native-6177d97e8fbc` исправляет классификацию ошибок:
сохраняет решение retry SDK YDB, когда query stream также возвращает cancellation.
Реальная отмена вызывающим кодом останавливает повторы; слишком большие сообщения
не повторяются. `ydbretry-native-image.json` фиксирует исходники, бинарь и digest.
Изменения Stroppy сохранены в PR #166; dev-сборка остаётся временным решением до официального релиза.

Graphene 0.2.13 восстановил Dedicated run с ID длиннее 63 символов:
полный ID сохраняется в annotations, Kubernetes labels используют безопасный хеш.
`graphene-long-run-id-check.json` подтверждает релиз и восстановление.
БД достигла YC RUNNING, но соединение Stroppy с TCP 2135 завершилось таймаутом.
Все ресурсы этой попытки удалены. Новый worker разрешает TCP 2135 только
источнику `loadbalancer_healthchecks` для dedicated; клиентский ingress
остаётся в подсети стенда. Повтор подтвердил TCP-доступ с первой попытки. В полном логе обнаружена
следующая ошибка клиента: внутренняя CA добавляется только при fallback,
который одновременно заменяет явный IAM-токен на недоступную metadata identity.
`ydb-dedicated-auth-check.json` описывает исправление выбора CA и авторизации;
его проверяет текущая повторная YC-матрица.

До Managed YDB workload новая проверка требует TCP-доступ и успешный SELECT 1
из того же образа Stroppy с той же авторизацией на реальном runner. Пять минут
ограничивают всё ожидание; отдельная попытка ограничена 20 секундами. Повторяются
только ошибки недоступности/перегрузки и Database not found от Discovery после
создания новой БД. Ошибки прав и неизвестные ошибки останавливают запуск.
Проверка не входит в измерения нагрузки и не экспортирует native OTLP.
Её конфиг и полный журнал попыток передаются stand с обычным сроком хранения:
конфиг бессрочно, лог 30 дней. Worker `09898c34bb5de279` опубликован из текущего
исходного entrypoint: серверный манифест содержит `stroppy.database.ready` и оба
типа Managed YDB. Реальный локальный SELECT проверен в
`ydb-managed-readiness-local-check.json`. Повтор обоих Managed режимов — `matrix-ydbretry-578bcce5`, входы —
`suite-ydbretry.json`. Serverless прошёл simple/TPC-B, но TPC-C остановился
на пяти ResourceExhausted в read-only validate_population до измерения нагрузки.
`ydb-serverless-population-retry-check.json` сохраняет точные причины.
Dev-сборка `populationretry-native-image.json` добавляет ограниченные повторы
этих SELECT с отдельным `tpcc_population_retry_attempts`; проверки соответствия
данных не ослаблены. Сборка прошла race/lint, PostgreSQL/MySQL integration
и локальную YDB 26.3. Следующий Serverless вход — `suite-ydb-populationretry.json`;
повтор обоих режимов запущен как `matrix-ydbpopulation-f4bd4b38`.
Он использует `dedicatedauth-native-image.json`: проверочные SELECT с retry,
CA Dedicated до первого подключения и сохранение явно выбранной авторизации.
Все ресурсы предыдущей попытки удалены.
Текущий worker `stroppy-run:932541bfa871c3f2`, suite `81bba44e13ff1ef7`.
В предыдущем Serverless запуске Discovery ещё не видел только что созданную БД;
`ydb-serverless-discovery-check.json` фиксирует ошибку до измерения нагрузки.
Повтор `matrix-ydbready-cd74e12d` с SELECT-readiness прошёл simple/TPC-B/TPC-C
без terminal errors; native OTLP, артефакты (включая проверку готовности),
логи/трейсы и полная очистка проверены. Вход — `suite-ydb-queryready.json`.
Dedicated из `matrix-ydbpopulation-f4bd4b38` завершил simple/TPC-B/TPC-C без
terminal errors; сохранённая телеметрия, артефакты и полная очистка проверены.
Одновременно выполняются не более трёх стендов, включая очистку; ограничения описаны ниже.

Метрики `tx_queries_per_tx` имеют единицу `queries/transaction`; только
распределения длительности помечаются `ms`. Stroppy сохраняет причины SQL-ошибок
проверки TPC-C population и не выдаёт фиктивное время измерения, если setup
завершился ошибкой до старта нагрузки. Конфиги хранятся до явного удаления,
артефакты логов — 30 дней; общий retention телеметрии не изменяется.

В CSV `managed_database_metrics` отдельно отмечает отсутствие проверки собственных
метрик Managed YDB. Метрики node-exporter на runner не подтверждают телеметрию
управляемой БД. Для многозонных стендов поле `zone` перечисляет a/b/d.

Публикация pipeline выполняется через текущий entrypoint (`go run ./cmd/run push`),
чтобы worker и отправляемый манифест соответствовали одним исходникам.
Старый вспомогательный бинарник может собрать новый образ, но отправить старую схему.

`noop` и `pg_noop` проверяются сценарием `execute_sql`: одна операция на
итерацию, native iterations/s и queries/s, без заявления о TPS или хранении
данных. Каталог разрешает этот сценарий всем драйверам, включая noop и YDB.
Для pg-noop 0.1.2 используется опубликованная упаковка релизного статического
бинарника (`images/pg-noop`, `pg-noop-image-check.json`); SHA-256 бинарника
проверен. Ненулевые latency_ms/error_rate компилятор отклоняет: upstream
0.1.2 эти режимы не реализует. Локальные проверки — `noop-execute-local-check.json`
и `pg-noop-local-check.json`; YC-проверки обоих режимов приняты в
`catalog-functional-check.json`, исправление auto-workers — `pg-noop-workers-check.json`.

Docker library v0.2.3 и worker `932541bfa871c3f2` включают heartbeat установки
Docker: сразу и каждые 15 секунд, таймаут в pipeline — минута. Установка по-прежнему
имеет 15 минут на попытку. `docker-install-heartbeat-check.json` содержит проверки
и опубликованные версии; метрики распределений БД этим релизом не исправлены.

Self-hosted YDB: четыре single-прогона `matrix-ydbmetrics-4f28bec5` приняты
с worker `f53623d265c372b2`. Очередь остановлена на первом mirror-3-dc:
в конфигурации отсутствовал обязательный ChannelProfileConfig.
Исправленный V1-конфиг содержит static_erasure и профиль 0 с тремя каналами
системных таблеток, соответствующими storage pool топологии. Все восемь
локальных запусков storage-процесса прошли (`ydb-channel-local-check.json`);
это не проверка распределённого кворума. Повтор только четырёх mirror-комбинаций
(a/b/d) описан в `suite-ydb-channelretry.json`. Mirror 25.4 завершил три
нагрузки с нулём terminal errors и прошёл HTTP-проверки всех 12 DB-узлов;
итоговая приёмка и cleanup пройдены. Проверены 25 exporter-ов и 12
файловых систем /data. Версии 26.1–26.3 перенесены
в `suite-ydb-parallel.json` с concurrency=3. Параллельная очередь запущена после
освобождения первого стенда и свежей проверки квот на все 39 VM
(`ydb-parallel-quota-check.json`).
Текущее состояние — в CSV; сохранение результата при отмене очереди
во время cleanup подтверждено в `ydb-cleanup-cancellation-check.json`.
Предыдущая `matrix-ydb-02736dbb` остановлена после обнаружения потери метрик;
все её ресурсы удалены.
Для объёмной телеметрии YDB verifier читает исходные samples из постоянной
VictoriaMetrics по точным namespace/run/entity/segment и времени; это обходит
лимит размера CLI-ответа без изменения серверного лимита. Сверка трёх уже
принятых Dedicated segments дала те же финальные native значения.

## Параллельность текущей YC-матрицы

Лимит — три одновременных стенда, включая удаляемые. Проверка актуальных
compute/VPC-квот сохранена в `concurrency-quota-check.json`. В облаке разрешено
шесть сетей, три существующие сети сохраняются, каждый тестовый стенд создаёт
одну сеть. Четвёртый стенд не запускается. Резерв учитывает YDB mirror-3-dc
(13 VM), трёхузловой CockroachDB (5 VM) и вспомогательный стенд (до 2 VM).
Перед использованием свободного слота квоты перечитываются из YC.

`suite-auxiliary.json` проверяет noop и pg-noop последовательно в третьем слоте.
Оба выполняют execute_sql: native queries/s и iterations/s, нулевые terminal
errors, OTLP, логи, трейсы, артефакты и cleanup. Для них TPS, репликация и
прогресс загрузки данных не применимы. Публичный вход не содержит OTLP headers;
реальные credentials заполняются только в приватной копии.

## Проверка больших OTLP-экспортов и pg-noop

Self-hosted YDB 25.4 завершил simple/TPC-B/TPC-C без terminal errors,
но экспорт метрик DB-агентов потерян: сообщения 8–9 MB превышают gRPC
лимит 4 MiB. Матрица остановлена с cleanup; успешная нагрузка сама по себе
не закрывает ячейку. Диагностика: `ydb-metrics-transport-check.json`.

Graphene SDK v0.2.5 делит готовые OTLP-метрики на запросы до 2 MiB с
сохранением целых точек и метаданных. Проверены большой реальный gRPC-export,
типы gauge/sum/histogram/exponential histogram/summary, partial rejection и
race. Также синхронизирован буфер stdout/stderr в RunTail. Тесты SDK и lint
пройдены; live-проверка worker подтверждена в `sdk-metrics-chunk-check.json`.
Это исправление размера передачи; histogram/summary из Prometheus scrape БД
по-прежнему требуют отдельного исправления (`database-metric-coverage.json`).

Для pg-noop значение workers=0 остаётся автоматическим выбором числа CPU.
Компилятор не передаёт PGNOOP_WORKERS в этом режиме: сам сервер отвергает
буквальный ноль. Положительное число передаётся явно. Новый вариант проверен
локально с native execute_sql; YC-повтор отмечается отдельно в
`pg-noop-workers-check.json`.

Повтор pg-noop `matrix-pgnoop-f4eb20b2-pg-noop-single` принят: native
execute_sql, scalar node metrics, логи/трейсы, скачивание двух артефактов
с проверкой SHA/TTL и полная очистка YC. Noop также принят; обоим не
приписывается TPS. Для YDB 25.4 worker f53623d265c372b2 подтвердил
45 262 сохранённые series от всех пяти exporter-ов и оба /data-диска;
полная приёмка single 25.4, 26.1, 26.2 и 26.3 завершена. Остальные ячейки — в CSV.
Точные хеши сообщений YDB, исходное время bootstrap, границы DROP IF EXISTS,
конфликты и повторяемые отказы при разделении шарда сохранены в
`ydb-single-*.probe-diagnostics.json`. Наблюдения не исключаются из отчёта;
успешность требует native zero terminal errors и независимых проверок членов.

Генератор внешних входов выбирает драйвер из валидированного protocol
подключения, а не первый протокол каталога. `external-protocol-input-check.json`
подтверждает компиляцию всех шести протоколов с сохранением DSN и только
runner-машиной. Это проверка входов с placeholder DSN; живой тест ниже
проверяет только PostgreSQL-протокол.

Проверка external DSN использует только собственную временную PostgreSQL БД:
контрольную строку и роль с SELECT без INSERT/UPDATE/DELETE/TRUNCATE.
`external_canary_probe.py` проверяет строку и права через loopback обычного агента.
Реальный локальный PostgreSQL прошёл положительный и два отрицательных случая
(`external-canary-local-check.json`). В YC внешний runner завершил 2m/2VU
execute_sql: 857 831 итерация, native 7 148.562 queries/s, ноль terminal errors.
Native OTLP, scalar node metrics, логи, трейсы и два артефакта проверены;
контрольная строка, права роли и ресурсы fixture сохранились после cleanup
runner. Полное удаление fixture подтверждено в `external-dsn-lifecycle-check.json`.
Внешний runner и fixture имеют разные сети и владельцев; доступ открывается
только от публичного IP runner `/32` на TCP 5432. После удаления runner повторно проверены
контрольная строка и неизменность ресурсов fixture; затем fixture удалён
обычным каскадным cleanup. Оба стенда освобождены.

CockroachDB: все 12 сочетаний шести версий и single/three-node приняты
в YC. Каждый прогон выполнил simple, TPC-B tx/procs и TPC-C tx/procs;
проверены native OTLP, scalar-метрики всех узлов, репликация, логи, трейсы,
артефакты и удаление ресурсов. Histogram/summary БД и длинные baseline/
fault-кампании остаются отдельными незавершёнными проверками.

VictoriaLogs verifier читает поток до EOF без прежнего ограничения 20 000 строк.
`component-log-stream-check.json` подтверждает совпадение с прежней проверкой
на реальном YDB run и обнаружение ошибки в строке 25 001 локального потока.

YC provider v0.12.0 получил отдельный runtime через GitOps `28827f6`:
requests=limits памяти 2Gi, CPU request 250m, max-reconcile-rate=1.
При одновременном создании трёх mirror-стендов прежний runtime без requests
выселялся из-за memory pressure на infra-нодах. Новая настройка проверена
на трёх свежих стендах (`yc-provider-memory-check.json`); она ограничивает reconcile,
но не задаёт строгий глобальный лимит асинхронных операций YC.

После прежних выселений provider 21 VM существовала в YC без сохранённого
external-name в Crossplane. Ссылки восстановлены только после однозначной
сверки run labels, полного bootstrap metadata, CPU/RAM, загрузочного и data
дисков, подсети и security groups; базовые ресурсы исключены из кандидатов.
Каждый patch проверял UID и resourceVersion. Все 39 VM стали Ready без
пересоздания (`yc-vm-recovery-check.json`). Это проверенное операционное
восстановление, не автоматическая функция pipeline.

CSV-колонки `infrastructure_recovery` и `recovery_evidence` явно отмечают
прогоны, которым потребовалось ручное восстановление ссылок VM. Успешность
нагрузок и телеметрии таких прогонов не доказывает автоматическое прохождение
provisioning с нуля после изменения runtime. Пустая колонка означает отсутствие
записи о таком восстановлении, а не доказанное отсутствие любых вмешательств.

Проверка логов mirror 26.1–26.3 выделяет ABORTED при чтении завершённой
транзакции, ABORTED при разделении shard и UNAVAILABLE при обращении к shard
после split. Общие сообщения требуют совпадения member/session actor/table
или transaction/trace/session с исходным сообщением и ограниченного времени.
`ydb-parallel-log-filter-check.json` проверяет три реальных набора и шесть
отрицательных изменений (неизвестная ошибка, другая trace/session actor,
код ошибки, время вне DROP-окна, terminal error). Для schema-drop допуск между
часами VM — 50ms: наблюдаемое расхождение конца шага и сообщения 26.3 равно
26.9ms. Смещение часов отдельно не измерено; точные missing-path коды и
количества 1/4/9 остаются обязательными.

Все 253 строки текущей функциональной CSV-матрицы имеют smoke=passed,
compile=passed и cleanup=passed с сохранёнными доказательствами. Это не
закрывает full_workloads, длинные baseline/fault-кампании и histogram/summary
БД. Native OTLP подтверждён для 237 строк; 16 PostgreSQL simple-строк версий
15–18 (single, primary-replica, patroni-ha, pgbouncer) остаются pending
в этой отдельной колонке. Повтор `matrix-ydbautonomous-73a81601` полностью принят:
YDB mirror-3-dc 26.1, 26.2 и 26.3 созданы одновременно с нуля и прошли
simple, TPC-B tx и TPC-C tx без конечных ошибок. Native TPS/OTLP,
scalar-метрики компонентов, логи, трейсы, артефакты и 12 member-проб каждого
стенда подтверждены сохранёнными данными (`ydb-autonomous-workload-check.json`).
Использованы `suite-ydb-autonomous.json`, закреплённые сборки Stroppy и worker;
квоты перечитаны перед запуском (`ydb-autonomous-quota-check.json`).

Все 39 VM получили external-name от provider без ручного восстановления.
Provider не перезапускался; наблюдаемый максимум памяти при provisioning —
1122MiB при лимите 2Gi. Один ответ YC ResourceExhausted успешно обработан
автоматическим повтором (`yc-autonomous-provisioning-check.json`). CSV отмечает
12 строк этих трёх стендов как `infrastructure_provisioning=automatic_verified`.
Поля `recheck_run_id`, `recheck_phase`, `recheck_stage`, `recheck_verification`
описывают только ещё не принятый повтор и теперь пусты.

После завершения всех workflows свежие списки YC подтверждают отсутствие
39 VM, 36 data-дисков, всех 39 boot-дисков, 3 сетей, 9 подсетей и 3 security
groups стендов (`ydb-autonomous-cleanup-check.json`). Cleanup занял
22.2–25.7 минуты (`ydb-autonomous-cleanup-timing-check.json`).

Инвентаризация файловых систем объединяет только одинаковые записи
entity/device/fstype/size: повтор docker.container.observe на двух узлах
YDB 26.3 создал 14 серий для 12 дисков. Конфликтующие размеры или устройства
остаются отдельными записями, отсутствующий диск не маскируется.
`filesystem-series-check.json` подтверждает replay реальных сохранённых
samples и три регрессионные проверки (`test_inspect_metrics.py`).

`preserved-resources-check.json` отличает исходные ручные ресурсы от VM
managed Kubernetes: пустая infra-нода и её boot disk удалены штатным
cluster-autoscaler, что подтверждено событиями ScaleDownEmpty. Остальные
исходные идентификаторы сохранены; полное сравнение конфигураций не заявляется.
