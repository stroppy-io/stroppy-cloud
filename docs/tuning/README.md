# Тюнинг баз данных: план

Сводка по семи отчётам в этом каталоге (`postgres.md`, `orioledb.md`,
`mysql.md`, `mariadb.md`, `cockroach.md`, `ydb.md`, `picodata.md`). В отчётах
есть ссылки на документацию, метки уверенности и пути в коде; здесь собраны
выводы, план работ и решения, которые нужно утвердить.

Эталонная машина матрицы: 8 vCPU / 32 ГБ / 200 ГБ `network-ssd`, TPC-C
100 складов, 128 VU, 10 минут, `pacing=false`, `warmup=0s`, `retry_attempts=3`.

---

## 0. Почему матрица сейчас нечестная

Причина не одна. Цифры сдвигают пять вещей, и тюнинг только одна из них.

| Фактор | Что сейчас | Кого задевает |
|---|---|---|
| **Тюнинг** | автоподбор есть только у MySQL/MariaDB, и он ставит лишь buffer pool (70% RAM) и io_capacity для MariaDB. У остальных стоковые дефолты: PG `shared_buffers=128MB`, `max_wal_size=1GB`; OrioleDB `main_buffers=64MB`; YDB `cpu_count: 4`; Cockroach статические проценты; Picodata memtx 2 ГБ (в матрице 20 ГБ вписали руками) | PG, OrioleDB, YDB, Cockroach |
| **Надёжность** | у MariaDB `sync_binlog=0`, у Picodata `wal_mode=write` (без fsync), у PG/MySQL fsync на каждый коммит. MariaDB `semi_sync` на деле async. MySQL semi-sync молча уходит в async через 10 с | MariaDB, Picodata получают фору |
| **Изоляция** | stroppy запрашивает RR для pg/mysql/cockroach. Cockroach с `repeatable_read_isolation.enabled=false`, скорее всего, повышает до SERIALIZABLE. YDB всегда serializable, Picodata без уровня | Cockroach, YDB на ретраях |
| **Нагрузка** | 128 VU без пауз, прогрева нет, 3 попытки на транзакцию. Движкам с оптимистичными блокировками (YDB, Cockroach) трёх попыток мало, неудавшиеся транзакции теряются | YDB, Cockroach |
| **Диск** | 200 ГБ `network-ssd` = 7 блоков × 32 ГБ ≈ 7000 IOPS и ~105 МБ/с на всё сразу: данные, WAL/redo, binlog, компакции. Cockroach по документации хочет 240 МБ/с на 8 vCPU. У YDB pdisk — файл 40 ГБ (≈2000 IOPS, 30 МБ/с) | все; после тюнинга станет главным потолком |

Сюда же: `/dev/shm` в контейнере 64 МБ, нет `ulimit nofile`, хостовые
sysctl применяются только для YDB/Picodata, диск монтируется ext4 `defaults`
(схема `cfg.host.disks` обещает xfs `noatime`, но её никто не читает).

---

## 1. Единый механизм тюнинга (платформа)

### 1.1 Параметр `tuning: auto | stock`

У каждого kind'а в `dbparams` одно поле `tuning` (Group «Tuning»):

- `auto` (по умолчанию): компилятор считает параметры из машины роли (vCPU,
  RAM, тип и размер диска), роли (primary/replica/member/compute/storage),
  топологии и нагрузки (сумма VU по сегментам);
- `stock`: только дефолты схемы, как сейчас.

Правила для всех движков:

1. Ключ, который пользователь явно задал в конфиге роли (`explicitConfig`),
   побеждает всегда, по каждому ключу.
2. Результат пишется в `effective_configs`: UI и отчёт показывают итоговые
   числа, а не формулы.
3. Формулы живут в `internal/domain/compile/tune_<kind>.go`, по файлу на
   семейство. Общие входы в `compile/machine.go`: `cpuOf`, `memoryOf`,
   `diskOf` → лимиты диска.
4. У MySQL/MariaDB `buffer_pool: auto|fixed|stock` вливается в `tuning`:
   `fixed` = явный `innodb_buffer_pool_size` в конфиге.

### 1.2 Лимиты дисков в каталоге

В `internal/domain/catalog` у `DiskType` появляются лимиты производителя:
IOPS и МБ/с на блок, размер блока, потолок диска, потолок ВМ на vCPU. Функция
`DiskLimits(type, GB, vCPU) → {readIOPS, writeIOPS, MBps}` берёт min(диск, ВМ).
Через неё считаются `innodb_io_capacity`, `effective_io_concurrency`, Cockroach
`provisioned_bandwidth`, оценки в UI. Значения Yandex Cloud:

| Тип | Блок | На блок | Потолок диска |
|---|---|---|---|
| `network-ssd` | 32 ГБ | 1000 IOPS, 15 МБ/с | 40000 wIOPS / 20000 rIOPS, 450 МБ/с |
| `network-ssd-nonreplicated` | 93 ГБ | 5600 wIOPS / 28000 rIOPS, 82/110 МБ/с | 75000 IOPS, 1 ГБ/с |
| `network-ssd-io-m3` | 93 ГБ | как nonreplicated, wIOPS до 40000 | — |

ВМ: `network-ssd` не больше 3500 IOPS и 45 МБ/с на vCPU,
nonreplicated/io-m3 — 10000 IOPS и 100 МБ/с на vCPU.

### 1.3 Хост и контейнер для всех БД

- `spec.Container`: новые поля `ShmSizeMB`, `SecurityOpt`; `Ulimits` уже
  есть, но рецепты его не заполняют. Проброс в `pipelines/internal/run/deploy.go`.
- Для всех db-ролей: `nofile=1048576`; Postgres/OrioleDB — shm `min(RAM/4, 8 ГБ)`;
  MariaDB — `memlock=-1` плюс seccomp для io_uring (иначе InnoDB молча
  выключает native AIO).
- `HostPrepSysctl` для всех db-ролей из `cfg.host.sysctl`: swappiness 1,
  `fs.aio-max-nr` 1048576, somaxconn, THP `never`. Исключение: у YDB THP
  `madvise` и `overcommit_memory=1` (их требование для TCMalloc). У PG ещё
  `vm.nr_hugepages` под `shared_buffers`.
- Диск данных: xfs + `noatime,nodiratime` (`pipelines/spec/diskprep.go`,
  `mount-data.sh`) — то, что схема `cfg.host.disks` уже обещает.

Всё под `pipelines/` — отдельный Go-модуль, ему нужен новый образ пайплайна.

### 1.4 Отчёт прогона показывает тюнинг

В сводке прогона появляется блок «Настройки»: режим тюнинга, кэш движка
(размер и доля RAM), надёжность коммита (fsync на коммит / без fsync /
реплика sync/async), фактическая изоляция, лимиты диска. Плюс флаги
недостоверности, которые собираются после замера:

- MySQL `Rpl_semi_sync_source_status=OFF` → «semi-sync ушёл в async»;
- реальный уровень изоляции у Cockroach (`SHOW transaction_isolation`);
- чекпоинты PG по размеру ≫ по таймеру (`pg_stat_checkpointer.num_requested`);
- Galera `wsrep_flow_control_paused`.

Значения надёжности и флагов — enum'ы контракта (правило «Словари»), тексты
в локалях. Это меняет контракт, поэтому релиз минорный: 0.4.0.

---

## 2. Профиль `auto` по движкам

Числа даны для эталонной машины 8/32/200. Полные таблицы с источниками — в
файле движка.

### 2.1 PostgreSQL (15–18, single / replica / Patroni / PgBouncer)

| Параметр | auto | Сейчас |
|---|---|---|
| `shared_buffers` | 25–40% RAM (см. решение Р3) | 128MB |
| `effective_cache_size` | 75% RAM = 24 ГБ | 4 ГБ |
| `work_mem` / `maintenance_work_mem` / `autovacuum_work_mem` | 16 МБ / min(2 ГБ, RAM/16) / 1 ГБ | 4 МБ / 64 МБ / — |
| `huge_pages` | `try` + хостовые `vm.nr_hugepages` | молча выключено |
| `max_wal_size` / `min_wal_size` | от диска, clamp(4% диска, 2, 32 ГБ) → 8 ГБ / 2 ГБ | 1 ГБ |
| `checkpoint_timeout` / `completion_target` | 15 мин / 0.9 | 5 мин |
| `wal_compression` | `lz4` (если образ собран с lz4) | off |
| `random_page_cost` | 1.1 | 4 |
| `effective_io_concurrency` / `maintenance_io_concurrency` | 200 (PG18: 64) | 1 |
| `jit` | off | on |
| `max_parallel_workers_per_gather` | 0 для OLTP | 2 |
| `max_worker_processes` / `max_parallel_workers` | vCPU | 8 |
| autovacuum | naptime 10s, scale_factor 0.05, analyze 0.02, workers 5, cost_limit 2000 | дефолты |
| `max_connections` | ceil((VU+20)/50)·50 → 200 | 100 (< 128 VU) |

Баги, которые чиним вместе с профилем:

- **Patroni** игнорирует `max_connections`, `max_worker_processes`,
  `max_locks_per_transaction`, `max_wal_senders` и т.п. из postgresql.conf —
  они берутся только из DCS. Компилятор заполняет
  `bootstrap.dcs.postgresql.parameters`: счётчики идут туда, память и IO
  остаются в `custom_conf`.
- **`sync_replicas` без Patroni не работает**: нет `application_name` в
  `primary_conninfo` и нет `synchronous_standby_names`. Добавляем.
- **PgBouncer**: `max_client_conn=100` < 128 VU. Ставим max(1000, 4·VU),
  `default_pool_size = clamp(4·vCPU, 20, 100)`, `min_pool_size` равным ему,
  `max_prepared_statements=200` явно.
- `wal_archive` в схеме обещан, но не реализован. Либо реализовать, либо
  убрать из формы (решение Р11).

### 2.2 OrioleDB (beta, pg16/17/18)

У OrioleDB кэш таблиц — `orioledb.main_buffers`, а `shared_buffers` нужен
только heap'у (каталоги).

| Параметр | auto | Сейчас |
|---|---|---|
| `orioledb.main_buffers` | 30% RAM ≈ 9.6 ГБ (потолок 45%) | 64 МБ |
| `shared_buffers` | 256 МБ | 128 МБ |
| `orioledb.undo_buffers` | max(пол от max_procs, main/16) ≈ 600 МБ | дефолт |
| `orioledb.xid_buffers` | 8 МБ | 1 МБ |
| `orioledb.temp_buffers` | 8 МБ | 64 МБ на backend |
| `orioledb.bgwriter_num_workers` | clamp(vCPU/4, 1, 4) = 2 | 1 |
| `orioledb.checkpoint_completion_ratio` | 1.0 (только Oriole-таблицы, рекомендация доки) | 0.5 |
| `max_worker_processes` | max(8, 2·vCPU) = 16 | 8 |
| общие с PG | `max_wal_size`, `checkpoint_timeout`, `random_page_cost`, `effective_cache_size`, `jit`, `max_connections` — из общего PG-кода | — |

Баги:

- `shared_buffers_mb` в параметрах до конфига не доходит, а по смыслу должен
  управлять `main_buffers`. Переименовываем в `main_buffers_mb`, оценку
  памяти в `planPostgres` считаем от него.
- Описание `max_io_concurrency` в схеме неверное: 0 значит «выключено».
- В каталоге только `beta17`; на Docker Hub уже есть `beta18` и `beta19`.
  Добавляем (решение Р9).

Общий код PG и OrioleDB: `compile/tune_postgres.go`; Oriole добавляет
только свои ключи сверху.

### 2.3 MySQL (8.0 / 8.4; single, async, semi-sync, Group Replication, ProxySQL)

| Параметр | auto | Сейчас |
|---|---|---|
| `innodb_buffer_pool_size` | 70% RAM, округление **вниз** до chunk·instances | 70%, округление вверх |
| `innodb_buffer_pool_instances` | clamp(ceil(vCPU/2), 2, 16), не меньше 1 ГБ на instance → 4 | 1 (схема перебивает сервер) |
| `innodb_redo_log_capacity` | clamp(pool/4, 2 ГБ, 32 ГБ) и ≤10% диска ≈ 5.6 ГБ | **100 МБ** — главный тормоз |
| `innodb_log_buffer_size` | 64 МБ | 16 МБ на 8.0 |
| `innodb_io_capacity` / `_max` | от диска: 2600 / 4500 | 200/2000 (8.0), 10000/20000 (8.4) |
| `innodb_page_cleaners` / `purge_threads` | = instances / 4 | разные дефолты 8.0 и 8.4 |
| AHI / change buffering / `use_fdatasync` / `flush_neighbors` | OFF / none / ON / 0 на обеих версиях | расходятся 8.0 и 8.4 |
| `binlog_transaction_dependency_tracking` | WRITESET и на 8.0 (вне GR тоже) | только в GR-блоке |
| `group_replication_consistency` / `exit_state_action` | EVENTUAL / READ_ONLY явно | у 8.0 и 8.4 разные дефолты |
| `max_connections` | max(500, 1.5·VU+100) | 151 |
| `skip_name_resolve`, `open_files_limit` | ON, от max_connections | — / 5000 |
| `binlog_expire_logs_seconds` | 1 сутки | 30 суток |
| ProxySQL `threads` | от vCPU прокси | 4 |

`innodb_dedicated_server` не используем: у 8.0 и 8.4 разные алгоритмы, и
итоговые числа не видны в `effective_configs`.

Semi-sync: `wait_point=AFTER_SYNC`, `super_read_only` на репликах; по
`Rpl_semi_sync_source_status` прогон получает флаг (решение Р12).

### 2.4 MariaDB (10.11 / 11.4 / 11.8; single, async, semi-sync, Galera, ProxySQL, MaxScale)

| Параметр | auto | Сейчас |
|---|---|---|
| `innodb_buffer_pool_size` | 70% RAM | 70% |
| `innodb_log_file_size` | clamp(pool/4, 2 ГБ, 16 ГБ) | **96 МБ** |
| `innodb_log_buffer_size` | 64 МБ | 16 МБ |
| `innodb_io_capacity` / `_max` | от диска (общая формула с MySQL) | 10000/20000 |
| `sync_binlog` | **1** | **0** (дефолт upstream) |
| `max_connections` | от VU, ~300 | 151 |
| `skip_name_resolve` | ON | — |
| `innodb_snapshot_isolation` | OFF на всех версиях (на 11.8 по умолчанию ON, меняет конфликты) — решение Р13 | расходится |
| Galera `wsrep_slave_threads` | vCPU = 8 | 1 |
| Galera `wsrep_provider_options` | `gcache.size=2G; gcs.fc_limit=128; gcs.fc_factor=0.8; gcs.fc_single_primary=yes; evs.suspect_timeout=PT10S; evs.inactive_timeout=PT30S; evs.install_timeout=PT15S` | не задаётся |
| Galera `wsrep_sync_wait` | **0 глобально** | 1 глобально |

Баги:

- **`semi_sync` на MariaDB не включает полусинхронность**: оверлей
  `rpl_semi_sync_*` есть только в ветке MySQL (`recipe_mysql.go:49-61`).
  Добавляем `rpl_semi_sync_master_enabled`, `slave_enabled`, `AFTER_SYNC`.
- **Глобальный `wsrep_sync_wait=1`** (`library/service.go:143`) наследуют
  мониторы ProxySQL/MaxScale. Во время TOI-DDL (`create_indexes`) они висят,
  узел уходит в SHUNNED, и соединения рвутся. Это главный кандидат на
  `invalid connection`. Ставим 0 и на время подготовки шлём всё на writer.
- MaxScale `causal_reads=global` с galeramon не работает → `none`.
- ProxySQL `server_version=8.0.36` перед MariaDB → настоящая версия.

### 2.5 CockroachDB (24.x / 25.4 / 26.x; single, three)

| Параметр | auto | Сейчас |
|---|---|---|
| `--cache` | 35% (RAM ≥ 32 ГБ), иначе 30% | 25% |
| `--max-sql-memory` | 20% (формула доки: 2·sql + cache ≤ 80%) | 25% |
| `--store size` + `--max-disk-temp-storage` | 80% + min(32 GiB, 15% диска) | **90% + 32 GiB не влезает в 200 ГБ** |
| кластерные настройки (из официального TPC-C гайда) | `kv.snapshot_rebalance.max_rate=128MiB`, `kv.range_merge.queue_enabled=false`, `diagnostics.reporting.enabled=false`, `gc.ttlseconds=600` на default range; `rocksdb.min_wal_sync_interval=500us` только как A/B | пусто |
| изоляция | `sql.txn.repeatable_read_isolation.enabled=true`, чтобы RR от stroppy был настоящим RR (решение Р5) | false → SERIALIZABLE |
| HAProxy | `httpchk GET /health?ready=1` на 8080 | голый TCP |
| healthcheck контейнера | `/health?ready=1` | `/health` |
| после загрузки | ждать `ranges_underreplicated=0` (three) | не ждёт |
| `insecure` | брать из параметра | захардкожен `true` |

Схема флагов: `--wal-failover`, `--locality-advertise-addr`,
`--max-disk-temp-storage` есть уже в 24.1. Разделение «только с @25» неверно.

### 2.6 YDB (25.4–26.3; single, mirror-3-dc)

YDB отстаёт больше всех, и причин несколько сразу.

| Параметр | auto | Сейчас |
|---|---|---|
| `actor_system_config.cpu_count` | storage: vCPU−1 (≤8) / vCPU−2; compute: vCPU−1 при ≥8, при 4 — 4 | **4 на всех узлах** (хуже, чем не задавать) |
| размер compute по умолчанию | 8 vCPU / 32 ГБ (сейчас 4/8) | ядра распределены наоборот: storage 8, compute 4 |
| pdisk | файл `max(80, 0.8·(disk−10)/pdisks)` ГБ (реализует мёртвый `auto_size_pdisks`) | 40 ГБ, ниже минимума доки (80) |
| `storage_groups` | single: 4–8 (A/B 1/4/8); mirror-3-dc: от числа compute-ядер | 1 |
| `memory_controller_config.hard_limit_bytes` | RAM − 2 ГБ | не задан |
| shared cache (compute) | min 25% / max 60%, target 60% | 20/50/50 |
| `resource_broker` `queue_build_index` | cpu max(1, vCPU/4) на время загрузки | 10 потоков |
| хост | THP `madvise`, overcommit 1, aio-max-nr, `nofile`/`memlock` в контейнере | THP `never` (против рекомендации) |
| сеть контейнера | `network_mode: host` (сейчас docker-proxy на interconnect) | проброс портов |

Мёртвые параметры (`disk_type`, `failure_domain`, `grpcs`, `auto_size_pdisks`)
либо подключаем, либо убираем из формы. Вариант по умолчанию — подключить
`disk_type` и `auto_size_pdisks`, а `grpcs` убрать до появления сертификатов.

Со стороны нагрузки (решения Р6, Р8):

- пул сессий `database/sql` без лимита. Документация YDB: 5–10 сессий на
  ядро compute → `max_conns = clamp(10·ядра compute, 32, VU)`;
- `retry_attempts` ≥ 10 для serializable;
- схема stroppy `ydb.sql`: индексы `idx_order`/`idx_customer_name` стартуют
  одной партицией (параметры партиционирования к индексам не применяются),
  у `history` монотонный ключ (горячий хвост), `item` — одна партиция до
  автосплита. Скорее всего, отсюда p99 20–30 с. Правка в репо stroppy.
  Сначала проверяем на стенде через `.sys/partition_stats`.

Контроль: на том же кластере прогнать `ydb workload tpcc`. Если он даёт
кратно больше, виноваты схема и драйвер stroppy; если столько же — кластер.

Managed YDB (dedicated `PermissionDenied`, serverless `Unavailable`) — отдельная
задача, в этот цикл не входит (см. `ydb.md` §6).

### 2.7 Picodata (25.3 / 26.1 / 26.2)

| Параметр | auto | Сейчас |
|---|---|---|
| `memtx.memory` | (RAM − max(2 ГБ, 8%) − 0.5 ГБ) / 1.15 / инстансов ≈ 25 ГБ | 2 ГБ (в матрице 20 вручную) |
| `wal_mode` тира (26.2) | `fsync` при strict-надёжности | `write`, из параметров недоступен |
| `sql_runtime_concurrency_max` | max(50, ceil(1.5·VU / pg-эндпоинтов)) → 192 на single | **50 < 128 VU** |
| `sql_storage_cache_count_max` / `size_max` | 200 / 32 МБ (A/B) | 50 / 5 МБ |
| `default_bucket_count` | пробрасывать в YAML | мёртвый параметр |
| `sql_motion_row_max` | 50000 при W ≥ 100 (страховка) | 5000 |

Структурное ограничение: memtx однопоточный, один инстанс на 8 vCPU
загружает 1–2 ядра. Нужен параметр `instances_per_machine` (порты, каталоги,
имена контейнеров, failure_domain по ВМ) — решение Р7. Плюс в отчёте пишем,
что 25.3/26.1 работают без fsync и иначе настроить нельзя.

`three` с RF3 — это один шард из трёх реплик, а не масштабирование. Для
сравнения масштабирования нужен вариант RF1 × 3.

---

## 3. Мелкие баги, которые всплыли в матрице

1. pico262: `bufio.Scanner: token too long` — сканер в `streamAndWait`
   (`pipelines/internal/activities/stroppy.go`) с буфером 1 МБ. Поднимаем
   до 16 МБ, длинную строку режем, а не роняем шаг.
2. noop TPC-C падает на `validate_population` — у noop валидацию выключаем.
3. pgnoop: расхождение bulk insert — разбираемся в stroppy (pg_noop).
4. Cockroach `_bucket`-гистограммы: проверить на новом образе пайплайна.

---

## 4. Порядок работ

| Этап | Что | Где |
|---|---|---|
| Э1 | Общая основа: `tuning`, `machine.go` (cpu/mem/disk), лимиты дисков в каталоге, `explicitConfig` на каждый ключ, `Container.ShmSizeMB/SecurityOpt/Ulimits`, `HostPrepSysctl` для db-ролей, xfs `noatime` | `compile/`, `catalog/`, `pipelines/spec`, `pipelines/internal/run` |
| Э2 | PG + OrioleDB: `tune_postgres.go`, Patroni DCS, sync-реплики без Patroni, PgBouncer, Oriole `main_buffers`, образы beta18/19 | `compile/recipe_postgres.go`, `recipe_patroni.go`, `dbparams/{postgres,orioledb}.go`, `cfg/*` |
| Э3 | MySQL + MariaDB: `tune_innodb.go` (общая часть и ветки), redo, instances, io_capacity от диска, `sync_binlog=1`, semi-sync MariaDB, Galera provider options, `wsrep_sync_wait=0`, ProxySQL/MaxScale | `compile/innodb.go` → `tune_innodb.go`, `recipe_mysql*.go`, `library/service.go`, `cfg/{mysql,mariadb,proxysql,maxscale}.go` |
| Э4 | Cockroach: память, store/temp, кластерные настройки, RR, HAProxy httpchk, ready-healthcheck, ожидание репликации, `insecure` | `cfg/cockroach_flags.go`, `recipe_distributed.go` |
| Э5 | YDB: `cpu_count`, pdisk от диска, `storage_groups`, память, хост, host network, размеры compute; мёртвые параметры | `recipe_distributed.go`, `dbparams/ydb.go`, `cfg/ydb_config_yaml.go`, `dbkind/ydb.go` |
| Э6 | Picodata: memtx auto, `wal_mode`, `bucket_count`, SQL-лимиты через `ALTER SYSTEM`, `instances_per_machine` | `recipe_distributed.go`, `dbparams/picodata.go`, `dbkind/picodata.go` |
| Э7 | Отчёт: блок «Настройки» + флаги недостоверности; enum'ы контракта и локали; контракт 0.4.0 | `openapi/parts`, `internal/api`, `web/` |
| Э8 | Мелкие баги из §3 | `pipelines/internal/activities`, workload |
| Э9 | Тесты: таблица «машина → значения» по каждому движку (4/16, 8/32, 16/64), explicitConfig побеждает, `stock` = дефолты, golden-файлы схем; `make test lint`, `make schemas-export`, `make contract-check` | — |
| Э10 | Релиз v0.4.0 (сервер + образ пайплайна), затем перезапуск всей матрицы на локальном стенде | — |

Отдельно, после первого перезапуска: правки схемы stroppy (YDB, split/scatter
у Cockroach) — в репо stroppy, со своим релизом. Так эффект платформы и
эффект схемы видны по отдельности.

### Как проверяем на перезапуске

По каждому прогону, не на глаз:

- значения из `effective_configs` = то, что реально стоит в БД (`SHOW ALL`,
  `SHOW GLOBAL VARIABLES`, `SHOW CLUSTER SETTINGS`, `/actors` YDB,
  `ALTER SYSTEM` Picodata);
- флаги недостоверности пустые;
- PG: `num_requested` у чекпоинтов ≈ 0; MySQL: checkpoint age < 75% redo;
  YDB: загрузка пулов (System/User/IC) и число партиций индексов; Picodata:
  загрузка ядер (`top -H`);
- сравнение «до / после» по каждой строке матрицы.

---

## 5. Решения, которые нужно утвердить

| № | Вопрос | Рекомендация |
|---|---|---|
| Р1 | Один параметр `tuning: auto\|stock` у всех kind'ов, `auto` по умолчанию; `buffer_pool` MySQL/MariaDB вливается в него | да |
| Р2 | Надёжность: `auto` = fsync на каждый коммит везде (MariaDB `sync_binlog=1`, Picodata 26.2 `wal_mode=fsync`). Ослабить можно только явной правкой конфига, отчёт это помечает. Picodata 25.3/26.1 помечается «без fsync, не настраивается» | да |
| Р3 | PG `shared_buffers`: 25% RAM (старт по доке) или 40% (весь датасет TPC-C 100 складов помещается в кэш) | 25% в `auto`; на перезапуске A/B 25 против 40, по результату поправить формулу |
| Р4 | Диск матрицы: оставить 200 ГБ `network-ssd` (~105 МБ/с) или взять 512 ГБ `network-ssd` (240 МБ/с, рекомендация Cockroach) / 186 ГБ `nonreplicated` | 512 ГБ `network-ssd`: дороже, но иначе после тюнинга все упрутся в диск и сравнение снова покажет диск |
| Р5 | Изоляция: включить RR у Cockroach, чтобы он работал на том же уровне, что pg/mysql; фактический уровень каждого движка писать в отчёт | да |
| Р6 | Нагрузка перезапуска: `warmup` 3 мин, `retry_attempts` 10, `max_conns` от движка (YDB 10/ядро compute, Cockroach ~4×vCPU кластера), остальное как было (128 VU, без пауз) | да |
| Р7 | Picodata: параметр `instances_per_machine` (по умолчанию 1); в матрицу добавить вариант с 4 инстансами на ВМ | да |
| Р8 | Правки схемы stroppy (YDB индексы/history/item, Cockroach split/scatter) — отдельным шагом после первого перезапуска | да |
| Р9 | OrioleDB: добавить beta18/beta19, матрицу гонять на beta19 (beta17 оставить в каталоге) | да |
| Р10 | YDB: compute по умолчанию 8 vCPU / 32 ГБ (сейчас 4/8) | да |
| Р11 | PG `wal_archive`: убрать из формы до реализации | убрать |
| Р12 | MySQL semi-sync: таймаут оставить 10 с, но помечать прогон, если источник ушёл в async | да |
| Р13 | MariaDB 11.8 `innodb_snapshot_isolation`: выключить в `auto` (паритет с 11.4) или оставить ON как свойство версии | оставить ON и показывать в отчёте: это поведение версии, его и надо мерить |
| Р14 | Galera: на время подготовки данных всё шлём на writer (вариант A), чтения на другие узлы — отдельный эксперимент | да |
