# Тюнинг PostgreSQL 15–18 для Stroppy Cloud: профиль `auto`

Дата: 2026-10-22. Только чтение репозитория. Пометки источников:
**[док]** — официальная документация (postgresql.org/docs, patroni.readthedocs.io, pgbouncer.org, yandex.cloud, docs.docker.com);
**[практика]** — PGTune-логика, EDB/Crunchy/Cybertec/опыт эксплуатации, не гарантируется документацией;
**[проверить]** — нельзя утверждать без проверки на стенде.
Ссылки в формате `// doc:` для схем указаны в конце каждой строки таблицы (короткие: `rcr` = https://www.postgresql.org/docs/18/runtime-config-resource.html, `rcw` = .../runtime-config-wal.html, `rcq` = .../runtime-config-query.html, `rca` = .../runtime-config-autovacuum.html, `rcc` = .../runtime-config-connection.html, `rcb` = .../runtime-config-replication.html).

---

## 1. Текущее состояние

### 1.1. Что и откуда выставлено

Все значения — заводские дефолты схемы `cfg.postgresql.conf@N` (`pipelines/schemas/cfg/postgresql_conf.go`), рецепт переопределяет только адреса.

| Параметр | Значение | Источник |
|---|---|---|
| shared_buffers | 128MB | postgresql_conf.go:~97 (`Default(128)`) |
| effective_cache_size | 4096MB | postgresql_conf.go:~125 |
| work_mem / maintenance_work_mem | 4MB / 64MB | там же |
| max_wal_size / min_wal_size / checkpoint_timeout | 1024MB / 80MB / 300s | postgresql_conf.go:~140–160 |
| wal_buffers | -1 | при sb=128MB даёт 4MB |
| random_page_cost / effective_io_concurrency | 4.0 / 1 (16 в 18) | postgresql_conf.go, блок Planner |
| max_connections | 100 (superuser_reserved 3) | блок Connections |
| huge_pages | try | при vm.nr_hugepages=0 тихо не работает |
| jit | on | блок Planner |
| autovacuum | все дефолты (naptime 60s, scale 0.2/0.1, cost_limit -1→200, 3 воркера) | блок Autovacuum |
| io_method / io_workers (18) | worker / 3 | только 18 |

Рецепт (`internal/domain/compile/recipe_postgres.go:60`, `recipe_patroni.go:88`) вызывает `c.render(role, schema, "conf", {"listen_addresses":"*","port":5432})`: никакого расчёта от машины. Аналога `innodbMemory` (`innodb.go`) для Postgres нет.

### 1.2. Выставлено в схеме, но не доходит до конфига (баги/дыры)

1. **Patroni игнорирует часть postgresql.conf.** [док: patroni_configuration] `max_connections` (деф. 100), `max_locks_per_transaction` (64), `max_worker_processes` (8), `max_prepared_transactions` (0), `wal_level`, `track_commit_timestamp` — «local config takes no effect», берутся только из DCS (`bootstrap.dcs.postgresql.parameters`) и передаются postgres в командной строке. `max_wal_senders` (10), `max_replication_slots` (10), `wal_keep_size` (128MB), `wal_log_hints` — тоже только динамическая конфигурация. `recipe_patroni.go:140–170` кладёт конфиг как `custom_conf` (база с самым низким приоритетом), а `postgresql_parameters` (поле схемы `patroni_yml.go`, рендер в `bootstrap.dcs.postgresql.parameters`) компилятор не заполняет. Итог: в Patroni-топологии любые max_connections/max_worker_processes/max_locks_per_transaction из схемы **не действуют** — всегда 100/8/64. Остальные параметры из custom_conf действуют (порядок: custom_conf < postgresql.conf Patroni с DCS-параметрами < auto.conf < командная строка).
2. **max_connections=100 при 128 VU.** Доступно 97 слотов (100 − 3 superuser_reserved) плюс postgres_exporter и Patroni. Если пул stroppy (`pool.maxConns`, по умолчанию, судя по `baseline.go`, = VUs) равен 128, часть соединений получит `too many clients` или ждёт. **[проверить]** по логам прогонов pg17-single и по `pg_stat_activity`; возможно, часть 171 tps — это упор в соединения, а не диск.
3. **`sync_replicas` не работает без Patroni.** В `postgresFamily` нет ни `synchronous_standby_names`, ни `application_name` в `primary_conninfo` (`pg_basebackup -R` кладёт без имени). Схема `dbparams/postgres.go:38` обещает синхронную репликацию, фактически она асинхронная. Для Patroni работает (synchronous_mode в DCS). Правило `sync-single-replica` в схеме предупреждает о том, чего нет.
4. **`wal_archive` не реализован**: в `internal/` и `pipelines/schemas/cfg` нет `archive_mode/archive_command`; параметр используется только в `dbkind/postgres.go:123` для оценки диска (40 ГБ).
5. **`cfg.docker.container@1` (shm_size, ulimit, cpus, memory, cap_add) никем не читается** в компиляторе (grep по `internal/domain/compile` пуст; упоминается лишь в `dbkind` и `observe`). Фактически: `/dev/shm` контейнера Postgres = дефолт Docker (64 МБ **[док docker: --shm-size, дефолт 64MB, подтверждён общеизвестной практикой]**); `pipelines/internal/run/deploy.go:243` строит `HostConfig` без `ShmSize`. Postgres использует `/dev/shm` для динамической разделяемой памяти (`dynamic_shared_memory_type=posix`): параллельные запросы/parallel hash на TPC-H/DS могут падать с `could not resize shared memory segment`. Для TPC-C не критично, для аналитики — критично.
6. **Хост-настройки не применяются для Postgres.** `cfg.host.sysctl@1` и THP существуют как схема, но компилятор `HostPrep` для Postgres не эмитит (для Picodata эмитит скрипт, `recipe_distributed.go:149`). vm.nr_hugepages = 0, значит `huge_pages=try` — тихий откат на обычные страницы.
7. **Расхождение схем диска.** Схема `cfg.host.disks` говорит «xfs по умолчанию, noatime,nodiratime», но реально диск монтирует `spec.EnsureDiskPreparation` (`pipelines/spec/diskprep.go:24–30`): ext4, опции `defaults` (relatime). `cfg.host.disks` рендерится нигде (нужно проверить). WAL и данные на одном диске (`/data/pg`).
8. **Дрейф версий**: `AGENTS.md` пишет haproxy 2.9, `images.go` использует `haproxy:3.0`; `locale` в рецепте дефолт "C", в схеме "C.UTF-8" (эффективно берётся схема).
9. **PgBouncer**: `max_client_conn` = 100 (деф.) < 128 VU — клиенты сверх лимита получат отказ (`no more connections allowed`); `default_pool_size`=20; `max_prepared_statements` в схеме отсутствует (в 1.24.1 деф. 200 — работает, но неявно). Порядок/пул не от нагрузки.

### 1.3. Чего нет вообще

- Параметров в схеме: `bgwriter_delay/lru_maxpages/lru_multiplier/flush_after`, `checkpoint_flush_after`, `backend_flush_after`, `wal_sync_method`, `commit_delay/commit_siblings`, `wal_init_zero/wal_recycle`, `track_io_timing`, `track_wal_io_timing`, `maintenance_io_concurrency`, `io_max_concurrency` (18), `autovacuum_work_mem`, `max_prepared_transactions`, `max_files_per_process`, `huge_page_size`, `dynamic_shared_memory_type`, `compute_query_id`, `log_min_messages`, `wal_receiver_*`, `max_standby_*_delay`, `vacuum_buffer_usage_limit` (16+), `max_pred_locks_*` (не нужны). Пока обходится полем `custom`.
- Режима `auto|stock` для Postgres (есть только у MySQL: `dbparams/mysql.go:63`, `compile/innodb.go`).
- Знания о диске (IOPS/MB/s) и числе соединений в компиляторе: `catalog.DiskType` (`catalog/types.go:118`) хранит только ID/MinGB/StepGB.
- Отображения применённого тюнинга в отчёте/снапшоте (`run.MachineSnapshot` — только размеры).

---

## 2. План профиля `auto`

### 2.0. Входы и принципы

Входы для расчёта (все доступны в `compilation`): RAM машины (`memoryOf`), vCPU (`out.Spec.Machines[].CPU`), диск `/data` (GB, тип — `Machines[].Disks`), роль (db/db-replica), major версия, число VU и класс нагрузки (`c.in.Workload.Segments`: tpcc/tpcb → OLTP; tpch/tpcds → аналитика), число реплик, pgbouncer/haproxy/patroni.

Принципы:
- Явная настройка пользователя (`explicitConfig`) выигрывает над `auto` (как у MySQL).
- Параметры, которые на реплике должны быть **не меньше** значений на primary (`max_connections`, `max_worker_processes`, `max_locks_per_transaction`, `max_prepared_transactions`, `max_wal_senders` **[док rcc, warm-standby]**), считать **от нагрузки/топологии, а не от машины** (иначе реплика другого размера не стартует или не пускает запросы). Память (shared_buffers и пр.) — от машины роли.
- Диск считается по эффективному лимиту: min(лимит диска, лимит ВМ).

### 2.1. Диск Yandex Cloud (расчёт IOPS/MB/s)

[док yandex.cloud/en/docs/compute/concepts/limits] (проверено 2026-10-22):

| Тип | Блок | Write IOPS/блок | Read IOPS/блок | Write MB/s/блок | Read MB/s/блок | Макс. на диск (w/r IOPS; MB/s) |
|---|---|---|---|---|---|---|
| network-ssd | 32 ГБ | 1000 | 1000 | 15 | 15 | 40000 / 20000; 450 |
| network-ssd-nonreplicated | 93 ГБ | 5600 | 28000 | 82 | 110 | 75000; 1000 |
| network-ssd-io-m3 | 93 ГБ | 5600 | 28000 | 82 | 110 | 40000 w / 75000 r; 1000 |

Потолок ВМ (общий на все диски): network-ssd — 3500 IOPS и 45 МБ/с на vCPU; nonrepl/io-m3 — 10000 IOPS и 100 МБ/с на vCPU (до 100000 / 1 ГБ/с). Чтение и запись делят одну ёмкость.

Наша машина БД: 200 ГБ network-ssd = 6,25 блока → **6–7 тыс. IOPS и ~90–105 МБ/с на запись/чтение** (доку не говорит, как округляется размер, не кратный 32; считать консервативно `floor` = 6 блоков: 6000 IOPS, 90 МБ/с). Потолок ВМ 8×3500=28000 IOPS/360 МБ/с — не ограничивает. **Главный вывод: диск — самое узкое место; на ~90 МБ/с и 6k IOPS WAL-fsync, чекпоинты и полностраничные образы (FPI) конкурируют.** Все решения по WAL ниже исходят из этого. (Для M при 8 vCPU у MySQL с пулом 22 ГБ те же 6k IOPS — паритет по железу.)

`diskProfile` в компиляторе: `writeIOPS, readIOPS, writeMBps, readMBps` = min(floor(GB/unit)·perUnit, diskMax, vmCapPerVCPU·vCPU). Добавить в `catalog.DiskType` поля (`UnitGB, WriteIOPSPerUnit, ReadIOPSPerUnit, WriteMBpsPerUnit, ReadMBpsPerUnit, MaxWriteIOPS, MaxReadIOPS, MaxMBps`) и в `catalog.SizeSpec`/Provider — лимиты ВМ на vCPU. Для AWS gp3/io2: gp3 = 3000 IOPS/125 МБ/с базово (настраиваемо), io2 до 256000 — отдельная таблица, при неизвестном типе → консервативный класс «network ssd».

### 2.2. Память

Пример для M (32 ГБ, 8 vCPU, 200 ГБ, 128 VU, TPC-C 100 WH ≈ 10 ГБ по `spec.EstimateDataset` = 0,1 ГБ/склад; реально с индексами ~12–13 ГБ **[практика]**).

| Параметр | Формула | Для M | Обоснование |
|---|---|---|---|
| shared_buffers | `clamp(0.40·RAM, 128MB, …)`, округл. до 128MB; для аналитики 0.25·RAM | 12 GB (12288MB) | [док rcr] «25% RAM разумное начало; >40% вряд ли лучше». Для OLTP с рабочим набором в памяти берём верхнюю границу 40%: меньше двойного кэширования, чем у MySQL (70%, O_DIRECT, без page cache), но у PG остаётся ОС-кэш для остатка. Явный `auto` ниже 25% не опускаем |
| effective_cache_size | `0.75·RAM` | 24 GB | [практика PGTune] только планировщик, ничего не выделяет |
| huge_pages | `try`; хост `vm.nr_hugepages = ceil(1.10·sb_MB/2) + 64` | 6 656 страниц | [док rcr] huge_pages рекомендованы; THP не рекомендованы. 10% запас — [практика]. Проверка: PG ≥17 `SHOW huge_pages_status`, PG 15/16 — `HugePages_Free` в /proc/meminfo [проверить] |
| work_mem | `clamp((RAM−sb)/(max_conn_active·3)/max(1,per_gather), 4MB, 64MB)` → для OLTP считать на активные соединения | 16 MB | [практика PGTune]; [док rcr] лимит на каждую операцию и каждый параллельный воркер → суммарно кратно work_mem. TPC-C это индексные точечные запросы, 16 MB — с запасом |
| hash_mem_multiplier | 2.0 (оставить) | 2.0 | [док rcr] деф. 2.0 с PG 15 |
| maintenance_work_mem | `min(2GB, RAM/16)` | 2 GB | [практика] ускоряет CREATE INDEX после загрузки (создаётся в stroppy) |
| autovacuum_work_mem | `min(1GB, maintenance_work_mem/2)` | 1 GB | [док rcr] деф. -1 = maintenance; задать явно, чтобы 3–5 воркеров × 2 ГБ не заняли 10 ГБ. В PG ≤16 vacuum не использует >1 ГБ на массив dead tuples [практика] |
| wal_buffers | оставить `-1` | авто = 16MB (sb/32 ограничено сегментом) | [док rcw] 1/32 sb, макс. один сегмент WAL (16MB); при sb=12 ГБ уже предел. Сейчас при sb=128MB получается 4 MB — после тюнинга sb само исправится |
| temp_buffers, max_prepared_transactions | оставить 8MB / 0 | | не нужны TPC-C |
| shared memory в контейнере | `/dev/shm` ≥ `min(RAM/4, 8GB)` через `Container.ShmSize` | 8 GB | см. п. 4; иначе параллельные запросы падают |

Бюджет памяти: sb 12 ГБ + пользователи ≈ (200 соединений × ~10 МБ служебной + work_mem на активные) < 5 ГБ + autovacuum ≤ 3 ГБ + exporter/agents ~1 ГБ → оставляет ≥10 ГБ ОС-кэшу. Лимитов cgroup у контейнера нет (`limit_memory=false`), OOM-риск низкий; huge pages выделяются из резерва вне кэша.

### 2.3. CPU, параллелизм, фоновые процессы

| Параметр | OLTP (tpcc/tpcb) | Аналитика (tpch/tpcds) | Обоснование |
|---|---|---|---|
| max_worker_processes | `max(8, vCPU)` = 8 | то же | [док rcr] реплика ≥ primary → от vCPU **главной** машины роли db (брать max по ролям). В Patroni — только через DCS (п. 1.2.1) |
| max_parallel_workers | `vCPU` = 8 | vCPU | [практика PGTune] |
| max_parallel_workers_per_gather | **0** | `ceil(vCPU/2)` cap 4 | TPC-C: короткие индексные запросы, 128 VU на 8 vCPU уже загружают CPU; параллелизм только отнимет ядра и вызовет нестабильность. [практика] |
| max_parallel_maintenance_workers | `ceil(vCPU/2)` cap 4 = 4 | то же | ускорение индексов после загрузки |
| jit | **off** | оставить on (дефолт) | [док rcq] JIT срабатывает по стоимости; на OLTP только риск латентности [практика] |
| io_workers (PG18, io_method=worker) | `max(3, ceil(vCPU/4))` = 3 | `ceil(vCPU/2)` | [док rcr] деф. 3; поднимать при высокой параллельности чтений |
| io_method (PG18) | `worker` | `worker` | [док] деф. `worker`. `io_uring` в Docker блокируется seccomp-профилем по умолчанию (io_uring_* syscalls закрыты в Docker ≥25) и требует сборки с liburing [практика/проверить]; для паритета и повторяемости остаёмся на worker. Опция `io_method` остаётся в схеме для ручного выбора |
| autovacuum_max_workers | 5 | 3 | см. 2.6 |
| max_files_per_process | деф. 1000 | | ulimit nofile контейнера 1048576 хватает |

### 2.4. Диск / IO под network-ssd

| Параметр | Значение | Обоснование |
|---|---|---|
| random_page_cost | 1.1 (SSD) | [практика PGTune]. Сетевой SSD медленнее локального; для TPC-C (всё индексные точечные запросы, данные в кэше) влияние ничтожно. Альтернатива 1.5 для network-ssd — не критично |
| seq_page_cost | 1.0 | |
| effective_io_concurrency | PG ≤17: 200 (SSD) ; PG18: 64 | [док rcr] ≤17 влияет только на bitmap heap scan (posix_fadvise), 18 — дефолт 16, диапазон до 1000; `io_max_concurrency` авто ограничен 64, выше смысла нет. 200 — PGTune для SSD [практика]; при лимите 6k IOPS большая глубина не ускорит. Безопасно оставить 100 |
| maintenance_io_concurrency | ≤17: 100; 18: 64 | [док rcr] деф. 10 (≤17) / 16 (18); для vacuum/создания индексов |
| bgwriter_delay | 50ms | [практика] чаще чистим грязные буферы при sb 12 ГБ, чтобы backends не писали сами; [проверить] через `pg_stat_io` (16+) `writes` бэкендов и `pg_stat_bgwriter.maxwritten_clean` |
| bgwriter_lru_maxpages | 1000 (деф. 100) | [практика]; с лимитом диска 6k IOPS потолок самоограничится |
| bgwriter_lru_multiplier | 4.0 (деф. 2.0) | [практика] |
| bgwriter_flush_after | 512kB (деф. Linux) | оставить |
| checkpoint_flush_after | 256kB (деф. Linux) | [док rcw] не трогать |
| backend_flush_after | 0 | деф. |
| track_io_timing | on | [док] небольшой оверхед на современных часах (kvm-clock/tsc); даёт реальные I/O-времена в `pg_stat_io`/`EXPLAIN`. **Вынести в отчёт как включённый**; сначала `pg_test_timing` в контейнере [проверить] |
| wal_sync_method | деф. (fdatasync) | [док rcw] «не обязательно оптимален»: проверить `pg_test_fsync` на YC [проверить]; без замеров не менять |
| wal_init_zero / wal_recycle | on / on | [док rcw] выключать только на COW-ФС (ext4/xfs не COW) |

Хост: ФС и опции. Рекомендация: ext4 (как сейчас) с `noatime` (`MountOptions: ["noatime"]` в `Disk`; `diskprep.go` принимает опции) — сокращает метаданные-записи; xfs не даёт гарантированного выигрыша [практика] — поменять ФС только если схема `cfg.host.disks` останется источником правды; сейчас привести схему и `diskprep.go` к одному значению (см. п. 4).

### 2.5. WAL и чекпоинты (размер от диска)

Формулы:
- `wal_budget_GB = disk_GB − 1.5·dataset_GB − 10% disk` (dataset по `spec.EstimateDataset().RecommendedGB`; на 200 ГБ и TPC-C 100 WH: 200 − 15 − 20 = 165).
- `max_wal_size = clamp(min(0.25·disk_GB, wal_budget_GB·0.5), 2GB, 64GB)` → **32–50 ГБ → берём 32 GB** (при 200 ГБ; запас под replication lag).
- `min_wal_size = max_wal_size/4` = 8 GB (рециркуляция вместо создания сегментов; при `wal_init_zero=on` создание нового файла = запись 16 МБ нулей, лишний IO на диске в 90 МБ/с).
- `checkpoint_timeout = 15min` (900 s).
- `checkpoint_completion_target = 0.9` (деф. 15+).
- `wal_compression = lz4` (PG ≥15; для PG15 не в кэше: нужна сборка с `--with-lz4`).
- `full_page_writes = on`, `fsync = on` (см. раздел 3).
- `wal_level = replica` (минимум для репликации; `minimal` недопустим при репликах).
- `wal_keep_size` при replicas>0 без слотов (plain streaming): `2GB` (Patroni: через DCS + `use_slots`).
- `max_wal_senders = replicas + 4`, `max_replication_slots = replicas + 4` (деф. 10 — достаточно, можно не менять).

Обоснование:
- [док rcw] `max_wal_size` — мягкий предел; запрос чекпоинта при его исчерпании; поднятие удлиняет crash recovery. [EDB, Dave Page «Tuning max_wal_size»] в HammerDB TPROC-C `max_wal_size` — второй по важности параметр после shared_buffers (~×1,5 сверху к ×3,5 от sb; на облачных дисках ×10); цель — почти все чекпоинты «по таймеру», `checkpoint_timeout` 15 мин, target 0,9. [практика]
- Сейчас `max_wal_size=1GB` при ~100 МБ/с и FPI создаёт **частые принудительные чекпоинты**: после каждого чекпоинта первое изменение каждой страницы снова пишет 8 КБ полностраничный образ в WAL, что раздувает WAL на ~порядок и забивает диск 90 МБ/с. Это, вероятно, **главная причина 171 tps**: **[проверить]** `pg_stat_checkpointer.num_requested` (PG17+; `pg_stat_bgwriter.checkpoints_req` ≤16) и `log_checkpoints` в прогоне.
- `wal_compression=lz4` уменьшает FPI в 2–3 раза (зависит от данных), нагрузка CPU невелика; на диске с лимитом пропускной способности — выигрыш [практика, EDB HammerDB-конфигурация использует wal_compression=on]. Проверка поддержки в образе: `pg_config --configure | grep lz4` либо `SET wal_compression='lz4'` при старте; официальный образ postgres и PGDG-пакеты собираются с lz4 [проверить по каждому образу, иначе старт упадёт].

Честность сравнения: 10-минутный замер при `checkpoint_timeout=15min` может вообще не содержать чекпоинта в окне — для PG это выгоднее, чем для InnoDB (непрерывный fuzzy-flush). Рекомендация: (а) `log_checkpoints=on` (уже), (б) выполнять `CHECKPOINT` после загрузки и до замера (одинаково чистое начало), (в) в отчёт выводить число чекпоинтов в окне, (г) для сравнения с MySQL использовать длинный прогон (soak) либо `checkpoint_timeout=5min` + `max_wal_size` большой (чекпоинт будет в окне, но не форсированный). Выбор по умолчанию: **15 min для `auto`, 5 min — опция «strict comparison»** **[открытый вопрос п. 5]**.

### 2.6. Автовакуум под TPC-C

TPC-C: интенсивные UPDATE по маленьким горячим таблицам (warehouse, district), крупным stock/customer, insert в order_line/history/new_order, удаление из new_order. Окно замера 10 мин, но soak 5 ч.

| Параметр | Значение | Обоснование |
|---|---|---|
| autovacuum_naptime | 10s (деф. 60s) | [док rca]; горячие таблицы раздуваются за минуты; после загрузки быстрее получим ANALYZE |
| autovacuum_vacuum_scale_factor | 0.05 (деф. 0.2) | [практика]; на stock (10 млн строк на 100 WH) 20% = 2 млн мёртвых — слишком поздно |
| autovacuum_analyze_scale_factor | 0.02 (деф. 0.1) | то же |
| autovacuum_vacuum_insert_scale_factor | 0.1 (деф. 0.2) | [док rca] для order_line/history (insert-only) — заморозка/visibility map |
| autovacuum_vacuum_threshold / analyze_threshold | 50 / 50 | деф. |
| autovacuum_max_workers | 5 | [практика]: больше таблиц одновременно; PG18 → оставить `autovacuum_worker_slots=16` (деф.), `max_workers` меняется по reload |
| autovacuum_vacuum_cost_delay | 2ms | [док rca] деф. 2ms |
| autovacuum_vacuum_cost_limit | `200·clamp(diskWriteMBps/50, 1, 10)` ≈ 400 на 90 МБ/с; при ≥300 МБ/с — 1000–2000 | [док rca] лимит делится между воркерами; деф. 200/2ms ≈ 40 МБ/с записи на воркер-группу. На 90 МБ/с диске 400 безопасно; более высокое значение отнимет у workload полосу |
| log_autovacuum_min_duration | 0 в `auto`-профиле (логируем все запуски) → 1s | [практика] для отчёта: видно, что vacuum работал |
| fillfactor горячих таблиц, per-table `autovacuum_*` | через `init_sql` (не профиль) | HOT-обновления district/warehouse [практика]; платформа — опционально, п. 4 |

Профиль для PG18: можно оставить `autovacuum_vacuum_max_threshold` дефолтом (100M) — [док rca].

### 2.7. Соединения

| Топология | max_connections | Прочее |
|---|---|---|
| Прямое, без пулера | `ceil((VU_total + 20) / 50)·50`, не меньше 100 | 128 VU → **200**. +20: exporters, Patroni, ручные подключения; `superuser_reserved_connections=5`. Считать по сумме VU всех раннеров/сегментов (`Workload.Segments`), одинаково на primary и репликах [док rcc: реплика ≥ primary] |
| PgBouncer (transaction) | `default_pool_size·pools + reserve + 30` (≈ 100) | пуллер ограничивает активные серверные соединения; PG-сторона не растёт с VU |

PgBouncer ([док pgbouncer.org/config.html]; всё сверено 2026-10-22):

| Параметр | Значение | Обоснование |
|---|---|---|
| max_client_conn | `max(1000, 4·VU)` (деф. 100!) | иначе 128 VU не подключатся. Увеличить `ulimit nofile` контейнера (fd ≈ max_client_conn + pool·db·users) |
| default_pool_size | `clamp(4·vCPU_db, 20, 100)` = 32 для 8 vCPU | [практика] активных серверных соединений 2–4×vCPU оптимально; меньше контенции на PG |
| min_pool_size | = default_pool_size | прогрев пула до замера (иначе storm подключений в окне) |
| reserve_pool_size | 5 | [док] деф. 0 |
| max_db_connections | = default_pool_size + reserve | страховка |
| pool_mode | transaction (как сейчас) | продуктовый выбор в схеме; **workload должен быть совместим**: pgx в режиме `cache_statement` (деф. stroppy, `workload/driver.go:30`) использует именованные prepared statements → нужны PgBouncer ≥1.21 и `max_prepared_statements>0` (деф. 200 в 1.24 по доке; добавить поле в схему и задать явно) |
| max_prepared_statements | 200 | [док] включает протокольные prepared statements в transaction mode; цена — память/CPU пуллера |
| server_idle_timeout / query_wait_timeout | деф. 600 / 120 | оставить |
| so_reuseport / несколько инстансов | не включать | [док] PgBouncer однопоточный, **один инстанс упрётся в 1 ядро**; при высоком tps это измеряется в отчёте (`pgbouncer-exporter` есть). Несколько инстансов — отдельная фича |
| ignore_startup_parameters | extra_float_digits (уже) | |

HAProxy (`haproxy_cfg.go`): `maxconn` 20000 достаточно; `nbthread 0` = авто; при Patroni — httpchk по REST 8008, `inter` деф.; `timeout client/server 1800s` ок для бенчмарка. Тюнинг не нужен, но проверить, что серверный `maxconn` не задан ниже max_connections **[проверить]**. Размер proxy-машины: сейчас XS (2 vCPU) — при HAProxy+PgBouncer на 128 VU может быть узким местом; учесть в `dbkind/postgres.go:` `Requirements[RoleProxy]` (сейчас `Because(2,2,10,...)`).

### 2.8. Репликация и Patroni

Streaming (ha=none):
- Добавить `application_name=<machine>` в `primary_conninfo` (через `pg_basebackup -R -d "... application_name=..."`, либо `ALTER SYSTEM`) и при `sync_replicas>0` выставлять `synchronous_standby_names = 'ANY N (name1,name2...)'` на primary [док rcb] — закрывает дыру 1.2.3.
- `hot_standby=on` (деф.), `hot_standby_feedback=off`, `wal_receiver_status_interval` деф.; `max_standby_streaming_delay` деф. 30s (читающих запросов на реплике нет).
- `recovery_prefetch` (15+) деф. `try` — оставить.
- Слот репликации или `wal_keep_size=2GB` — чтобы реплика не отвалилась при долгом чекпоинте.

Patroni (4.1.5; ttl 30/loop_wait 10/retry_timeout 10 — деф., [док patroni dynamic_configuration]; `ttl ≥ loop_wait + 2·retry_timeout` уже правило схемы):
- **Заполнить `postgresql_parameters` (DCS)**: `max_connections`, `max_worker_processes`, `max_locks_per_transaction`, `max_prepared_transactions`, `wal_level`, `max_wal_senders`, `max_replication_slots`, `wal_keep_size`, плюс **все остальные тюнинговые параметры** — для единообразия кластера и чтобы смена primary не меняла настройки (остальные параметры из custom_conf тоже работают, но у реплики-кандидата с другой RAM будут другие sb). Рекомендация: общие значения (счётчики) → DCS; память/IO per-node → custom_conf.
- `use_slots: true`, `use_pg_rewind: true` — проверить дефолты в `patroni_yml.go`.
- `wal_log_hints=on` (Patroni) при `data-checksums` избыточен, но безвреден.
- `synchronous_mode_strict=true` + `synchronous_node_count=N` (есть). Для сравнимости с semi-sync MySQL: `synchronous_commit=on` + sync standby = ожидание flush реплики (не apply). `remote_apply` не использовать [док rcw].
- etcd: отдельная ВМ (2 vCPU/2 ГБ/10 ГБ), параметры по умолчанию (heartbeat 100ms/election 1000ms) [практика]; fsync etcd на network-ssd приемлем.
- Watchdog выключен (`watchdog_mode off` в рецепте) — оставить.

### 2.9. Хост ОС (применимо в Docker на ВМ YC)

Контейнеры идут в `NetworkMode: host`, ВМ целиком наша; настройка хоста выполняется `HostPrep` (kinds `sysctl`, `disks`, `script` — `pipelines/internal/activities/hostprep.go`) до старта контейнеров. Контейнерные sysctl для net.* не нужны (host-сеть).

| Что | Значение | Применимо? | Источник |
|---|---|---|---|
| vm.nr_hugepages | `ceil(1.10·sb_MB/2)+64` (6 656 для 12 ГБ) | Да, на хосте (контейнер использует host hugetlb; MAP_HUGETLB). В Docker не требуется `--privileged`; **[проверить]** на YC: `HugePages_Free` падает после старта PG | [док rcr, kernel-resources] |
| THP | `never` (или `madvise`) через `script`: `echo never > /sys/kernel/mm/transparent_hugepage/enabled` | Да, sysfs хоста; Ubuntu 24.04 по умолчанию `madvise` — уже приемлемо | [док rcr: THP не рекомендованы] |
| vm.swappiness | 1 (схема уже по умолчанию 1) | Да | [практика] |
| vm.dirty_background_bytes / vm.dirty_bytes | 256 MB / 1 GB вместо `ratio` (10%/20% от 32 ГБ = 3,2/6,4 ГБ) | Да, но схема `host.sysctl` умеет только ratio (мин. 1% = 320 МБ). Добавить поля `vm_dirty_background_bytes`, `vm_dirty_bytes`. Причина: большие всплески writeback на диске 90 МБ/с блокируют fsync WAL | [практика: Crunchy/Cybertec/Citus] |
| vm.overcommit_memory | оставить 0 | PG-документация рекомендует 2 для выделенных серверов, но на ВМ с Docker, node-exporter, agent риск OOM-отказов выделения; отключено | [док kernel-resources; осознанный отказ] |
| vm.max_map_count, fs.file-max, kernel.sem | схемные дефолты | Достаточно | |
| kernel.shmmax/shmall | не нужны | PG ≥9.3 использует mmap | [док] |
| IO scheduler | `none`/`mq-deadline` (virtio-blk по умолчанию) | менять нечего | |
| mount | `noatime` | Да, `Disk.MountOptions` | [практика] |
| read_ahead_kb | не менять | | |
| CPU governor / NUMA / zone_reclaim | n/a в ВМ | | |
| net.core.somaxconn, keepalive | схемные дефолты | Достаточно | |
| Docker: `--shm-size` | ≥ 8 GB | Да — требует новое поле в `spec.Container` (п. 4) | [док docker run] |
| Docker: `--ulimit nofile` | 1048576 (уже по умолчанию демона) | | |
| Docker: `--ulimit memlock=-1` | не нужен для mmap hugetlb; при `shmget`-варианте — да | | [проверить] |
| Docker log driver | json-file 100m×5 (уже) | проверить, что `log_statement`/`log_min_duration_statement` не раздувают лог | |

---

## 3. Надёжность (durability) и паритет с другими движками

### 3.1. Дефолты

| Параметр | Сейчас | Что даёт |
|---|---|---|
| fsync | on | запись WAL/данных на устройство; off = крах → потеря кластера [док rcw] |
| synchronous_commit | on | COMMIT ждёт flush WAL на диск; с `synchronous_standby_names` — ждёт flush реплики [док rcw таблица] |
| full_page_writes | on | защита от «рваных» страниц (аналог InnoDB doublewrite) [док rcw] |
| data-checksums | включены (`POSTGRES_INITDB_ARGS` и Patroni initdb) | аналог InnoDB page checksum |
| wal_sync_method | fdatasync (Linux) | |
| MySQL (для сравнения) | `innodb_flush_log_at_trx_commit=1`, `innodb_flush_method=O_DIRECT`, `innodb_doublewrite=ON` (`cfg/mysql.go`); `sync_binlog=1` и бинлог по умолчанию включены в 8.x **[проверить в отрисованном my.cnf]** | то же по смыслу |

### 3.2. Паритет

- **Эквивалент «fsync на каждый коммит»**: PG `fsync=on, synchronous_commit=on` ≡ MySQL `innodb_flush_log_at_trx_commit=1` (+ `sync_binlog=1`). Заметка: у MySQL при включённом бинлоге на коммит приходится два fsync (redo + binlog, group commit) — у PG один WAL; PG в этом честно дешевле по IO. Это свойство движка, а не недочёт сравнения; отметить в отчёте.
- **`synchronous_commit=off`** — не нарушает целостность (нет порчи; возможна потеря последних ≤3×`wal_writer_delay`=600ms коммитов) [док rcw], но это **не durable-коммит**; эквивалент MySQL `flush_log_at_trx_commit=2`/`0`. 
- **`fsync=off`, `full_page_writes=off`** — небезопасны [док rcw]: использовать только как явно помеченный «unsafe» режим; в сравнения не включать.
- **Что выбирать для сравнения**: основной headline — полный durable (`fsync=on`, `synchronous_commit=on`, `full_page_writes=on`, checksums) для PG и (`flush_log=1`, `sync_binlog=1`/или без бинлога на single, doublewrite on) для MySQL, одна и та же ФС и диск. Дополнительный, явно подписанный режим `relaxed` (PG `synchronous_commit=off`; MySQL `flush_log=2`) можно запускать отдельно, но не смешивать на одной диаграмме с durable. Для реплицируемых топологий сравнивать синхронную пару: PG `sync_replicas=1` (Patroni synchronous_mode / `ANY 1`) ↔ MySQL semi-sync AFTER_SYNC, и асинхронную пару с асинхронной.
- Как менять: поле `durability: strict|relaxed` (рекомендуется, п. 4) вместо отдельных «рычагов»; `relaxed` → `synchronous_commit=off` (единственное «безопасное» ослабление); `fsync`, `full_page_writes` остаются доступными только через явный `configs` и помечаются в отчёте «UNSAFE».
- Дисклеймер для отчёта: сетевой SSD YC реплицирован (3 копии), fsync на нём стоит ~ миллисекунда; на таком диске «durable» = подтверждение записи сетевым хранилищем. Не-реплицируемый SSD быстрее, но менее надёжен — различие отметить в метаданных прогона.

---

## 4. Изменения платформы (конкретно)

Нумерация — порядок реализации. Принцип: чистая функция расчёта + тонкая обвязка (как `innodb.go`).

1. **Схема `dbparams/postgres.go`** (`pipelines/schemas/dbparams/postgres.go`)
   - `tuning`: Choice `auto|stock` (деф. `auto`), Group «Tuning». `stock` = заводские настройки (повтор текущего поведения, чтобы воспроизводить старые прогоны).
   - `workload_class`: Choice `auto|oltp|analytics` (деф. `auto`: определяется по скрипту сегментов).
   - `durability`: Choice `strict|relaxed` (деф. `strict`).
   - (опц.) `shared_buffers_pct`/`fixed` не нужны: явное значение в `configs` побеждает, как у MySQL.
   - Каждому полю `// doc:` и `Desc` (правило `pipelines/schemas/AGENTS.md`). Тесты: `dbparams/postgres_test.go`, golden `pipelines/schemas/testdata/db.postgres.params@1.json` (`-update`). Затем `make openapi generate-go generate-ts schemas-export`; значения enum в `openapi/parts/15-vocabularies.yaml`, локали `web/src/locales/{ru,en}`, `internal/api/vocab_test.go`, `web/src/lib/vocab.test.ts` (правила «Словари» корневого AGENTS.md).
   - Поправить `Desc` у `sync_replicas` (сейчас обещает работу без Patroni) и описание `wal_archive` (не реализован → либо реализовать, либо убрать параметр; см. п. 7).
2. **Схема `cfg/postgresql_conf.go`**: добавить поля (с `// doc:` и гейтингом по major) — `bgwriter_delay/lru_maxpages/lru_multiplier/flush_after`, `checkpoint_flush_after`, `backend_flush_after`, `maintenance_io_concurrency` (деф. 10 для ≤17, 16 для 18), `io_max_concurrency` (18), `autovacuum_work_mem`, `track_io_timing`, `track_wal_io_timing`, `wal_sync_method`, `commit_delay`, `commit_siblings`, `wal_init_zero`, `wal_recycle`, `max_prepared_transactions`, `max_files_per_process`, `dynamic_shared_memory_type`, `huge_page_size`; расширить пределы (`work_mem`, `max_connections`). Шаблон — `l.num/l.bare`. Golden-файлы `cfg.postgresql.conf@15..18` и OrioleDB (переиспользует таблицу) обновить. Правило `wal_compression` lz4/zstd — оставить, но компилятор выбирает `lz4` только если образ его поддерживает (см. п. 3).
3. **Каталог дисков**: `internal/domain/catalog/types.go` (`DiskType` + лимиты), `catalog/providers.go:29–32` (значения YC из таблицы 2.1; AWS gp3/io2 — свои), функция `diskLimits(type, gb, vCPU)` в `internal/domain/catalog/disk.go` с тестом (200 ГБ network-ssd, 8 vCPU → 6000 IOPS/90 МБ/с).
4. **Компилятор — новый файл** `internal/domain/compile/pgtune.go`:
   - `type pgTuneInput struct{ RAMMB, VCPU, DiskGB int; Disk diskLimits; Major int; Role string; VUs int; Class string; Replicas int; Pooler bool; Patroni bool; Dataset float64 }` и `func pgSettings(in pgTuneInput) map[string]any` — чистая, табличные тесты (`pgtune_test.go`: M 8/32/200, XS 2/8/50, XL 32/128/800, PG15 vs PG18, OLTP vs analytics, с/без PgBouncer, replica).
   - `func (c *compilation) postgresTuning(role, machine, schemaID string, overlay map[string]any)` — по аналогии с `innodbMemory`: читает `tuning` из `c.params()`, пропускает ключи из `c.explicitConfig`; вызывается в `recipe_postgres.go` (в цикле по ролям, перед `c.render`) и `recipe_patroni.go:patroniNode`.
   - `connectionSettings(c)` — одинаков для всех ролей: `max_connections`, `max_worker_processes`, `max_locks_per_transaction`, `max_wal_senders` считаются от VU/топологии/максимума vCPU по ролям БД.
   - Для Patroni: `patroniConfig` — заполнить `overlay["postgresql_parameters"]` счётчиками DCS (п. 2.8). Тест в `patroni_test.go`: в итоговом YAML `bootstrap.dcs.postgresql.parameters.max_connections: "200"`.
   - Для сырых streaming: `application_name` + `synchronous_standby_names` в `pgReplicaBootstrap`/overlay (`recipe_postgres.go`), тест в `recipe_test.go`/новый.
   - `durability=relaxed` → `synchronous_commit=off`.
   - `postgresHost(c)`: добавить `spec.HostPrep{Role, Kind: sysctl, Content: <рендер cfg.host.sysctl@1 с vm.nr_hugepages и dirty_*_bytes>}` и `Kind: script` (THP) для ролей db/db-replica (`spec.HostPrep` см. `pipelines/spec/spec.go:208–225`); Patroni — те же.
   - PgBouncer: в `c.pgbouncer` (`recipe_postgres.go:168`) добавить в overlay `max_client_conn`, `default_pool_size`, `min_pool_size`, `reserve_pool_size`, `max_db_connections`, `max_prepared_statements`; в `pgbouncer_ini.go` добавить поле `max_prepared_statements`; контейнеру — `Ulimits{"nofile": …}` (поле уже есть в `spec.Container`, `deploy.go:~266` его применяет).
5. **Контейнер**: `pipelines/spec/spec.go` — `Container.ShmSizeMB int` (`json:"shm_size_mb,omitempty"`), `schemas/spec/run.go` зеркало, `pipelines/internal/run/deploy.go:243` — `host.ShmSize = int64(c.ShmSizeMB)<<20`; контракт: `spec.ContractVersion`/`contract.lock.json` (изменение входа пайплайна — по политике `pipelines/live/tests/platform/contracts/HANDOFF.md`; пока 0.x — без бампа на каждую правку, но в релизной ноте). Рецепт ставит `ShmSizeMB = min(RAM/4, 8192)`. Либо подключить `cfg.docker.container@1` как источник (сейчас мёртв) — решение: либо читать его в рецепте, либо удалить из `ConfigSchemas`/`dbkind` (иначе «настройка в UI ничего не делает»).
6. **Диск**: привести к одному источнику: `pipelines/spec/diskprep.go` (ext4, `defaults`) vs `cfg.host.disks` (xfs, noatime). Минимум: `Disk.MountOptions = ["noatime"]` для `/data` в `compile.go:~176` (создание `spec.Disk`).
7. **`wal_archive`**: реализовать (`archive_mode=on`, `archive_command='test ! -f /data/wal_archive/%f && cp %p /data/wal_archive/%f'`, каталог на диске) либо убрать из схемы; не оставлять «обещание».
8. **Отчёт/снапшот**: добавить в `compile.Output` поле `Tuning map[string]map[string]any` (роль → применённые ключи с источником `auto|explicit|stock`), сохранить в `run.Snapshot` (`internal/domain/run/run.go`) рядом с `MachineSnapshot`, отдать через OpenAPI (`openapi/parts/61-components-runs.yaml`), показать в `web/` (Run detail → вкладка Config/Tuning: таблица «параметр → значение → почему: формула»). Это закрывает «профиль виден в отчёте» и позволяет сравнивать прогоны «с тюнингом/без». В заголовок результата — флаги `tuning=auto|stock`, `durability`.
9. **Проверки на стенде** (в рецепт или отдельный check после старта): `SHOW huge_pages_status` (PG≥17) / `HugePages_Free`; `SELECT name,setting FROM pg_settings WHERE name IN (...)` — «проверка, что применилось» (особенно Patroni-управляемые); `pg_stat_checkpointer`/`pg_stat_bgwriter` до и после — вывести число таймерных/запрошенных чекпоинтов в отчёт.
10. **Тесты**: `TestPostgresTuningAuto` (значения для M), `TestPostgresTuningStock` (равенство нынешнему рендеру), `TestPostgresExplicitWins`, `TestPatroniDCSParameters`, `TestReplicaConnectionsNotBelowPrimary`, `TestPgBouncerPoolFor128VU`; golden `testdata`, `go test ./... -count=1`, `GRAPHENE_MANIFEST=1` не затронут.

---

## 5. Риски и открытые вопросы

1. **Справедливость сравнения с MySQL (70% RAM)**: PG 40% sb + page cache — не то же самое по составу кэша. Нужно решить, сравниваем «рекомендованные настройки каждого движка» (предлагаю так) или «одинаковый бюджет памяти». Для PG возможен вариант sb=60–70% (EDB использовал ~sb 100 ГБ на больших машинах) — но [док rcr] «>40% вряд ли лучше»; **[проверить]** прогонами 25/40/60%.
2. **Окно замера и чекпоинты** (п. 2.5): 15 мин timeout может исключить чекпоинт из 10-минутного окна. Решить стратегию (CHECKPOINT перед замером; длинный прогон; `checkpoint_timeout` как параметр профиля).
3. **Реальная причина 171 tps не доказана.** Кандидаты: принудительные чекпоинты + FPI на диске 90 МБ/с, sb=128MB (нагрузка читается из ОС-кэша через syscalls), max_connections=100 при 128 VU, jit, autovacuum/ANALYZE после загрузки, `pg_stat_statements`/monitoring overhead. Проверять **по метрикам** (pg_stat_io 16+, pg_stat_checkpointer, wait events, iostat/node-exporter), а не гадать; `auto`-профиль подтвердить A/B-прогоном `tuning=stock` vs `auto`.
4. **Поддержка lz4 в образах** (`postgres:N`, `patroni:pgN-4.1.5`, orioledb): неверное значение `wal_compression` = PG не стартует. Нужна проверка/фолбэк на `pglz`/`off`. OrioleDB (`beta17-pg*`): тот же `postgresFamily` — не ломать его собственные `orioledb.*` параметры (`shared_buffers_mb` у него отдельный параметр, `dbkind/postgres.go:121`); для него профиль `auto` включить отдельным решением.
5. **io_uring**: Docker seccomp, версия liburing в образе, лимит `fs.aio-max-nr`/`memlock` — не включать в `auto` без отдельной проверки.
6. **Huge pages в контейнере на YC**: нужен прогон на стенде; при нехватке `nr_hugepages` `try` тихо откатывается — добавить явную проверку, иначе тюнинг «есть в конфиге, но не работает».
7. **Патрони и смена лидера**: DCS-параметры (max_connections и др.) и параметры custom_conf разных узлов расходятся, если машины разного размера; шеринг счётчиков через DCS решает это, а память — нет (по задумке, но надо зафиксировать).
8. **PgBouncer и транзакционный режим**: несовместимость с workload (session-level state, advisory locks, `SET`), prepared statements; `max_prepared_statements`; однопоточность пуллера → потолок. Поведение при `pool_mode=transaction` и `cache_statement` в stroppy нужно подтвердить на стенде.
9. **Размер блоков YC** (округление не кратного 32 ГБ размера диска) в документации не указано — оценка 6–7k IOPS приблизительная; можно уточнить замером `fio` на стенде и закрепить в каталоге.
10. **Хост-sysctl и общий стенд**: настройки хоста (`vm.nr_hugepages`, `dirty_bytes`) действуют на всех контейнерах ВМ (exporter, agent) — приемлемо на выделенной БД-машине; для colocated-ролей (PgBouncer на db-машине) учесть память.
11. **Мониторинговый оверхед**: `pg_stat_statements`, `track_io_timing`, postgres_exporter — симметрично ли это включено у MySQL (performance_schema)? Для честности пометить в отчёте.
12. **Размер proxy-машины** (XS: 2 vCPU/2 ГБ) может ограничивать PgBouncer/HAProxy на 128 VU.
13. **Устаревание**: дефолты и поведение сверены с докой PG 18 (18.6) и YC на 2026-10-22; для 15/16/17 гейтинг параметров (io_*, autovacuum_worker_slots, huge_pages_status — PG17+, effective_io_concurrency, maintenance_io_concurrency default 10→16) нужно ещё раз сверить по версионным докам при реализации (`// doc:` по каждой major).

## Источники

- PostgreSQL 18 docs: runtime-config-resource, -wal, -autovacuum, -connection, -query, -replication — https://www.postgresql.org/docs/18/
- Patroni: https://patroni.readthedocs.io/en/latest/patroni_configuration.html, .../dynamic_configuration.html
- PgBouncer config: https://www.pgbouncer.org/config.html
- Yandex Cloud compute limits: https://yandex.cloud/en/docs/compute/concepts/limits
- Docker run reference: https://docs.docker.com/reference/cli/docker/container/run/
- EDB (Dave Page), Tuning max_wal_size: https://www.enterprisedb.com/blog/tuning-maxwalsize-postgresql
- EDB HammerDB howto: https://www.enterprisedb.com/blog/how-to-benchmark-postgresql-using-hammerdb-open-source-tool
- pgEdge performance tuning: https://www.pgedge.com/blog/postgresql-performance-tuning
- PGTune (формулы приведены по памяти; страница с исходниками недоступна при ресёрче — [проверить] на https://pgtune.leopard.in.ua)
