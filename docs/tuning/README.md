# Тюнинг баз данных

Как платформа настраивает развёрнутые движки. Исследование по каждому движку с
источниками и метками уверенности — в отчётах этого каталога (`postgres.md`,
`orioledb.md`, `mysql.md`, `mariadb.md`, `cockroach.md`, `ydb.md`,
`picodata.md`); пути и номера строк в отчётах указывают на код до тюнинга.
Код профиля: `internal/domain/compile/tune*.go`, `host.go`, `summary.go`.

Эталонная машина матрицы: 8 vCPU / 32 ГБ / 200 ГБ `network-ssd`, TPC-C
100 складов, 128 VU, 10 минут, `pacing=false`, `warmup=0s`, `retry_attempts=3`.

---

## 0. Почему матрица до тюнинга была нечестной

Причина не одна. Цифры сдвигают пять вещей, и тюнинг только одна из них.

| Фактор | Что было | Кого задевает |
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

У каждого развёртываемого kind'а в `dbparams` поле `tuning` (Group «Tuning»):

- `auto` (по умолчанию): компилятор считает параметры из машины роли (vCPU,
  RAM, тип и размер диска), роли (primary/replica/member/compute/storage),
  топологии и нагрузки (пиковое число VU сегмента, `peakVUs`);
- `stock`: дефолты схемы (движка), без хостовой подготовки.

Правила:

1. Ключ, который пользователь явно задал в конфиге роли, побеждает всегда,
   по каждому ключу (`tuner.set` в `compile/tune.go`).
2. Значения auto попадают в `effective_configs` снапшота прогона
   (`Output.Tuned` → `EffectiveWithTuned`).
3. Формулы — `internal/domain/compile/tune_<семейство>.go`; входы —
   `hostOf(machine)` (CPU, RAM, диск и его лимиты) и `peakVUs()`.
4. У MySQL/MariaDB `buffer_pool: auto|fixed` работает внутри auto;
   `buffer_pool: stock` значит `tuning: stock`.

### 1.2 Лимиты дисков в каталоге

`catalog.DiskType.Perf` — модель производителя: блок, IOPS и МБ/с на блок,
потолок диска, потолок ВМ на vCPU. `Provider.DiskLimitsOf(type, GB, vCPU)`
берёт min(диск, ВМ) по числу блоков (вверх). Через неё считаются
`innodb_io_capacity`, I/O concurrency PG и лимиты в отчёте. Yandex Cloud:

| Тип | Блок | На блок (чтение / запись) | Потолок диска (чтение / запись) |
|---|---|---|---|
| `network-hdd` | 256 ГБ | 300 / 300 IOPS, 30 / 30 МБ/с | 2000 / 11000 IOPS, 240 МБ/с |
| `network-ssd` | 32 ГБ | 1000 / 1000 IOPS, 15 / 15 МБ/с | 20000 / 40000 IOPS, 450 МБ/с |
| `network-ssd-nonreplicated` | 93 ГБ | 28000 / 5600 IOPS, 110 / 82 МБ/с | 75000 / 75000 IOPS, 1 ГБ/с |
| `network-ssd-io-m3` | 93 ГБ | 28000 / 5600 IOPS, 110 / 82 МБ/с | 75000 / 40000 IOPS, 1 ГБ/с |

ВМ: hdd и `network-ssd` — 3500 IOPS и 45 МБ/с на vCPU, nonreplicated/io-m3 —
10000 IOPS и 100 МБ/с на vCPU. AWS gp3 — плоские 3000 IOPS / 125 МБ/с.

### 1.3 Хост и контейнер

Под `auto` после рецепта (`compile/host.go`):

- каждый контейнер БД получает `nofile=1048576`;
- каждая машина с контейнером БД — drop-in `cfg.host.sysctl@1` (swappiness 1,
  `fs.aio-max-nr`, somaxconn, keepalive…) и скрипт THP. Рецепт уточняет через
  `tuneHost`: YDB — THP `madvise` и `vm.overcommit_memory=1` (TCMalloc), PG —
  `vm.nr_hugepages` под shared memory.

Контейнер умеет `shm_size_mb` и `security_opt`: PG/OrioleDB — shm
`min(RAM/4, 8 ГБ)`; MariaDB — `memlock=-1` и `seccomp=unconfined` (без них
io_uring не поднимается и InnoDB молча уходит с native AIO; узкий профиль не
влезает в 256 символов опции). Диск данных монтируется ext4
`defaults,noatime,nodiratime`.

### 1.4 Клиент (драйвер stroppy)

Под `auto` компилятор (`compile/tune_driver.go`) задаёт пул соединений
stroppy: по соединению на каждого виртуального пользователя и поток загрузки
(`peakConnections` = max(VU, `load_workers`) по сегментам), но не больше
8 на vCPU машин базы (`poolSize`). CockroachDB считает 4 активных соединения
на vCPU, вики PostgreSQL начинает с 2 на ядро, но транзакция TPC-C здесь —
~18 запросов с сетевым обменом на каждый, и соединение простаивает между
ними: PostgreSQL 17 на 8 vCPU дал 1049 tps при 32 соединениях и 1280 при 64.
Виртуальные пользователи сверх пула ждут соединения на клиенте, как
приложение за пулом; отчёт это ожидание показывает. Без машин базы (managed, external) пул = числу VU.

Без автоподстройки pgx держит max(4, CPU runner'а) соединений, а
`database/sql` — два простаивающих: пул зависел от машины генератора, и 256
VU на 16 соединениях runner'а L давали потолок 16 / латентность транзакции.

- PostgreSQL, OrioleDB, Picodata, pg-noop: `pool.maxConns = minConns`;
- MySQL, MariaDB, YDB: `pool.maxOpenConns = maxIdleConns`;
- CockroachDB: `pool_max_conns` и `pool_min_conns` в URL — stroppy 6.2.1
  отбрасывает блоки `pool`/`postgres` для `driverType: crdb`
  (stroppy-io/stroppy#234).

Ключ пула, заданный в workload, побеждает. `stock` оставляет дефолты драйвера.
Серверные лимиты соединений (`max_connections` PG, MySQL/MariaDB,
`max_client_conn` PgBouncer) считаются от размера пула. В отчёт попадают
только несекретные опции драйвера (пул, `bulkSize`, способ вставки).

### 1.5 Отчёт прогона

Снапшот (`run.Snapshot.Tuning`, `compile/summary.go`): режим тюнинга и
надёжность коммита — `fsync_each_commit` или `relaxed` (контракт:
`TuningMode`, `CommitDurability`). У каждой машины — кэш движка (МБ) и
лимиты диска данных. UI показывает это блоком «Настройки базы» в сайдбаре
прогона; `relaxed` подсвечен.

---

## 2. Профиль `auto` по движкам

Числа даны для эталонной машины 8/32/200. Полные таблицы с источниками — в
файле движка.

### 2.1 PostgreSQL (15–18, single / replica / Patroni / PgBouncer)

| Параметр | auto | Сейчас |
|---|---|---|
| `shared_buffers` | 25% RAM (A/B против 40% — на перезапуске матрицы) | 128MB |
| `effective_cache_size` | 75% RAM = 24 ГБ | 4 ГБ |
| `work_mem` / `maintenance_work_mem` / `autovacuum_work_mem` | 16 МБ / min(2 ГБ, RAM/16) / 1 ГБ | 4 МБ / 64 МБ / — |
| `huge_pages` | `try` + хостовые `vm.nr_hugepages` | молча выключено |
| `max_wal_size` / `min_wal_size` | от диска, clamp(4% диска, 2, 32 ГБ) → 8 ГБ / 2 ГБ | 1 ГБ |
| `checkpoint_timeout` / `completion_target` | 15 мин / 0.9 | 5 мин |
| `wal_compression` | `lz4` (официальные образы и PGDG собраны с lz4) | off |
| `random_page_cost` | 1.1 | 4 |
| `effective_io_concurrency` / `maintenance_io_concurrency` | 200 на SSD (PG18: 64 — предел `io_max_concurrency`) | 1 |
| `jit` | off | on |
| `max_parallel_workers_per_gather` | 0 для OLTP | 2 |
| `max_worker_processes` / `max_parallel_workers` | max(8, vCPU) | 8 |
| autovacuum | naptime 10s, scale_factor 0.05, analyze 0.02, workers 5, cost_limit 2000 | дефолты |
| `max_connections` | max(100, ceil((VU+20)/50)·50) → 150 | 100 (< 128 VU) |

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
- `wal_archive` не реализован и убран из формы (поле оставлено скрытым,
  чтобы старые сохранённые параметры читались).

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
- В каталоге `beta17`, `beta18`, `beta19`; по умолчанию `beta19-pg17`.

Общий код PG и OrioleDB: `compile/tune_postgres.go`; Oriole добавляет
только свои ключи сверху.

### 2.3 MySQL (8.0 / 8.4; single, async, semi-sync, Group Replication, ProxySQL)

| Параметр | auto | Сейчас |
|---|---|---|
| `innodb_buffer_pool_size` | 70% RAM, округление **вниз** до chunk·instances | 70%, округление вверх |
| `innodb_buffer_pool_instances` | clamp(ceil(vCPU/2), 2, 16), не меньше 1 ГБ на instance → 4 | 1 (схема перебивает сервер) |
| `innodb_redo_log_capacity` | clamp(pool/4, 2 ГБ, 32 ГБ) и ≤10% диска ≈ 5.6 ГБ | **100 МБ** — главный тормоз |
| `innodb_log_buffer_size` | 64 МБ | 16 МБ на 8.0 |
| `innodb_io_capacity` / `_max` | от диска: 2600 / 4700 | 200/2000 (8.0), 10000/20000 (8.4) |
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

Semi-sync: `wait_point=AFTER_SYNC`, на репликах `SET PERSIST super_read_only = ON`
из init SQL (в my.cnf он сломал бы первый старт образа). Флаг прогона по
`Rpl_semi_sync_source_status` (ушёл в async) — ещё не сделан.

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
| `innodb_snapshot_isolation` | дефолт версии (на 11.8 ON): это поведение версии, его и меряем | расходится |
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
| `--store size` + `--max-disk-temp-storage` | 80% + min(32 GiB, 15% диска) | 90% + 32 GiB не влезает в 200 ГБ (stock так и остался) |
| кластерные настройки (из официального TPC-C гайда) | `kv.snapshot_rebalance.max_rate=128MiB`, `kv.range_merge.queue_enabled=false`, `diagnostics.reporting.enabled=false`, `gc.ttlseconds=600` на default range; `rocksdb.min_wal_sync_interval=500us` только как A/B | пусто |
| изоляция | `sql.txn.repeatable_read_isolation.enabled=true` с 24.3 (на 24.1 настройка только для разработчиков и под лицензией — не ставим) | false → SERIALIZABLE |
| HAProxy | `httpchk GET /health?ready=1` на 8080 | голый TCP |
| healthcheck контейнера | single: `/health?ready=1`; кластер: `/health` (узел готов только после `cockroach init`), sidecar ждёт `?ready=1` на каждом узле | `/health` |
| репликация | sidecar ждёт нуля under-replicated диапазонов до нагрузки; ожидания между загрузкой и замером нет (загрузка внутри сегмента stroppy) | не ждёт |
| `insecure` | только `true`: сертификаты не выдаются, правило схемы отказывает в `false` | захардкожен `true` |

Схема флагов: `--wal-failover`, `--locality-advertise-addr`,
`--max-disk-temp-storage` есть уже в 24.1. Разделение «только с @25» неверно.

### 2.6 YDB (25.4–26.3; single, mirror-3-dc)

YDB отстаёт больше всех, и причин несколько сразу.

| Параметр | auto | Сейчас |
|---|---|---|
| `actor_system_config.cpu_count` | storage: vCPU−1 (≤8) / vCPU−2; compute: vCPU−1 при ≥8, при 4 — 4 | **4 на всех узлах** (хуже, чем не задавать) |
| минимум compute | 8 vCPU / 32 ГБ под auto (stock: 4/8) | ядра распределены наоборот: storage 8, compute 4 |
| pdisk | `auto_size_pdisks`: файл `max(80, 0.8·(disk−10)/pdisks)` ГБ, иначе 80 ГБ (и в stock) | 40 ГБ, ниже минимума доки (80) |
| `storage_groups` | пусто = auto: single 4; mirror-3-dc clamp(2·узлов·vCPU/4, 6, 24) в пределах 8 VDisk на pdisk | 1 |
| `memory_controller_config.hard_limit_bytes` | RAM − 2 ГБ | не задан |
| shared cache (compute) | min 25% / max 60%, target 60% | 20/50/50 |
| `resource_broker` `queue_build_index` | cpu max(1, vCPU/4) на время загрузки | 10 потоков |
| хост | THP `madvise`, overcommit 1, aio-max-nr, `nofile`/`memlock` в контейнере | THP `never` (против рекомендации) |
| сеть контейнера | уже host network у всех контейнеров (`deploy.go`) | — |

`disk_type` задаёт тип drive, пул и `database create <kind>:N`;
`auto_size_pdisks` работает. `grpcs` и `failure_domain` скрыты и не читаются
(сертификатов нет; fail domain всегда один storage-узел).

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
| `memtx.memory` | ((RAM − max(2 ГБ, 8%)) / инстансов − system_memory − 0.25 ГБ) / 1.15 ≈ 25 ГБ | 2 ГБ (в матрице 20 вручную) |
| `wal_mode` тира (26.2) | `fsync` при strict-надёжности | `write`, из параметров недоступен |
| `sql_runtime_concurrency_max` | max(50, ceil(1.5·VU / pg-эндпоинтов)) → 192 на single (на 25.3 настройки нет) | **50 < 128 VU** |
| `sql_storage_cache_count_max` / `size_max` | 200 / 32 МБ (A/B) | 50 / 5 МБ |
| `default_bucket_count` | пробрасывать в YAML | мёртвый параметр |
| `sql_motion_row_max` | 50000 под auto | 5000 |

memtx однопоточный: один инстанс на 8 vCPU загружает 1–2 ядра, а Picodata
закладывает на memtx-инстанс одно ядро (docs.picodata.io/…/admin/sizing).
`instances_per_machine` = 0 (по умолчанию) под `auto` ставит инстанс на каждый
vCPU машины базы: каждый инстанс тира становится машиной таких инстансов
(single на 32 vCPU — 32 инстанса RF1; три машины RF3 — 32 репликасета по три),
`memtx.memory` делится на их число. Под `stock` — один инстанс. Явное N (1..128)
ставит N инстансов на ВМ. Слот k слушает 3301/4327/8081 + k, каталог
`/data/picodata/<k>`, `failure_domain` по ВМ. Клиенты доходят до всех
инстансов только через HAProxy (`haproxy` ≥ 1, `pgproto`): без него весь
SQL идёт через первый инстанс. 25.3/26.1 пишут WAL без
fsync — отчёт показывает это как «не ждёт диска». `ALTER SYSTEM` в init
проверяется: отказ настройки роняет init-контейнер.

`three` с RF3 — это один шард из трёх реплик, а не масштабирование. Для
сравнения масштабирования нужен вариант RF1 × 3.

---

## 3. Связанные исправления

- Строка лога stroppy длиннее 1 МБ обрезается с пометкой, а не роняет сегмент
  (`pipelines/internal/activities/stroppy.go`).
- У noop-драйвера сегменты пропускают `validate_population` и
  `validate_answers`, если шаги не перечислены явно.
- RunSpec детерминирован: один вход компилируется в одну спеку
  (`TestAutoTuningIsDeterministic`).
- PgBouncer рядом с primary (без proxy-машины) берёт конфиг роли proxy.

## 4. Как проверяется на прогоне

- значения из `effective_configs` = то, что реально стоит в БД (`SHOW ALL`,
  `SHOW GLOBAL VARIABLES`, `SHOW CLUSTER SETTINGS`, `/actors` YDB,
  `ALTER SYSTEM` Picodata);
- PG: `num_requested` у чекпоинтов ≈ 0; MySQL: checkpoint age < 75% redo;
  YDB: загрузка пулов (System/User/IC) и число партиций индексов; Picodata:
  загрузка ядер (`top -H`); Cockroach: `SHOW transaction_isolation`;
  MariaDB: native AIO/io_uring поднялся (лог сервера).

## 5. Открыто

- Флаги недостоверности по факту прогона (semi-sync ушёл в async, фактическая
  изоляция Cockroach, чекпоинты PG по размеру) — нужен сбор статусов после
  замера.
- Ожидание репликации Cockroach между загрузкой и замером: загрузка идёт
  внутри сегмента stroppy, отдельного шага нет.
- Схема stroppy: YDB (индексы одной партицией, монотонный `history`, `item`),
  Cockroach split/scatter — в репо stroppy, после перезапуска матрицы.
- PG `shared_buffers` 25% против 40% — A/B на перезапуске.
- Managed YDB (dedicated `PermissionDenied`, serverless `Unavailable`) — см.
  `ydb.md` §6.
- pgnoop: расхождение bulk insert — сторона stroppy (`pg_noop`).
