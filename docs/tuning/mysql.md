# MySQL 8.0 / 8.4 LTS — профиль `auto` (тюнинг под OLTP/TPC-C)

Пометки источников: **[doc]** — официальная документация MySQL/ProxySQL/YC,
**[prac]** — практика (Percona, Oracle-блоги, инженерный опыт; не гарантия),
**[?]** — не проверено в этой сессии. Различия версий помечены **8.0/8.4**.
Пути — относительно репо stroppy-cloud.

---

## 1. Текущее состояние

### 1.1 Что есть
- Параметры БД: `pipelines/schemas/dbparams/mysql.go` — `version` (8.4 | 8.0),
  `replicas` 0..8, `replication` async|semi_sync|group, `single_primary`,
  `semi_sync_wait_for_slave_count`, `buffer_pool` auto|fixed|stock (стр. 63-71),
  `buffer_pool_mb`, `proxysql` 0..2, `init_sql`.
- Автоподбор: `internal/domain/compile/innodb.go` — только
  `innodb_buffer_pool_size = floor(RAM_GB*1024*0.7)` МБ (стр. 18-21) и (только
  для MariaDB!) `innodb_io_capacity=10000/20000` (стр. 29-36).
  Явное значение в роли-конфиге побеждает (`explicitConfig`).
- Рендер my.cnf: `pipelines/schemas/cfg/mysql.go` (одна функция на обе
  версии, `is80`-ветки: 339, 414, 530, 564). Шаблон InnoDB-блока — стр. ~500.
- Контейнеры: `internal/domain/compile/recipe_mysql.go` (single/async/semisync),
  `recipe_mysql_cluster.go` + `mysql-group-start.sh` (GR), ProxySQL —
  `recipe_mysql.go:194-258` + `cfg/proxysql.go`.
- Машина для компилятора знает `CPU`, `MemoryGB`, `DiskGB`, `DiskType`
  (`compile.go:117-122,185,193,206`), `workload` знает VU
  (`compile.go:367`, пока только baseline).

### 1.2 Баги и пробелы (проверено чтением кода)

| # | Что | Где | Эффект |
|---|-----|-----|--------|
| B1 | `innodb_redo_log_capacity` не трогается: схемный дефолт 100M. На матрице отрисовалось именно 100M | `cfg/mysql.go` ~стр. 78-81; `innodb.go` | **Главный подозреваемый по 615 tps.** 100 МБ redo при сотнях МБ/мин записи = постоянный синхронный checkpoint-flush (soft limit ~30/32 ёмкости, дальше пользовательские потоки блокируются [doc: dev.mysql.com/blog-archive/dynamic-innodb-redo-log-in-mysql-80]). На диске ~105 МБ/с это ещё и бьёт по пропускной способности |
| B2 | `innodb_buffer_pool_instances` схема всегда рендерит явно, дефолт `1` | `cfg/mysql.go` ~74-77, шаблон ~501 | Перебивает серверный автоподбор (8.0: 8 при пуле >1 ГБ; 8.4: min(chunks/2, CPU/4)). Один instance на 22 ГБ = один mutex на LRU/free list, контеншн на 128 VU. Он же держит `innodb_page_cleaners` (8.4: = instances) |
| B3 | `auto` поднимает `io_capacity` только для MariaDB (`if maria`) | `innodb.go:29` | На **MySQL 8.0** `auto` оставляет 200/2000 (схемный дефолт 8.0), хотя Desc в `dbparams/mysql.go:64` обещает «disk-class io_capacity». На 8.4 10000/20000 пришли из дефолта схемы, а не из `auto` |
| B4 | io_capacity 10000/20000 не соответствует диску: network-ssd 200 ГБ = 7 блоков по 32 ГБ → **7000 write IOPS, 105 МБ/с** [doc: yandex.cloud/en/docs/compute/concepts/limits]. 20000 «max» недостижим; flush по io_capacity_max упирается в полосу, а не IOPS | `cfg/mysql.go` 8.4 дефолты | Адаптивный flush даёт задержки на redo/binlog fsync, конкурирующих за те же 105 МБ/с |
| B5 | Округление пула: MySQL округляет **вверх** до кратного `chunk(128M)*instances` [doc: 8.4/innodb-buffer-pool-resize]. 22937M при instances=1 → 23040M; при instances=8 → 23552M (71.9% RAM) | `innodb.go:20` | Реальный пул чуть выше задуманных 70%; при больших instances перелёт до `chunk*instances` (до 1 ГБ+) |
| B6 | `binlog_transaction_dependency_tracking=WRITESET` и `transaction_write_set_extraction` на 8.0 рендерятся **только внутри GR-блока** (`grLines`, стр. ~414) | `cfg/mysql.go` | Для async/semi-sync на 8.0 дефолт сервера `COMMIT_ORDER` — параллельный applier на реплике почти не параллелится. 8.4 всегда WRITESET (опция удалена) → разный лаг 8.0 vs 8.4 |
| B7 | `group_replication_consistency` у схемы Nullable и не рендерится: 8.0 → EVENTUAL, 8.4 → **BEFORE_ON_PRIMARY_FAILOVER** (смена дефолта в 8.4.0 [doc: 8.4/mysql-nutshell]). Аналогично `group_replication_exit_state_action`: 8.0 READ_ONLY, 8.4 OFFLINE_MODE | `cfg/mysql.go` GR-блок | Несопоставимая семантика GR между версиями |
| B8 | Нет `innodb_adaptive_hash_index`, `innodb_change_buffering`, `innodb_log_buffer_size`, `innodb_purge_threads`, `innodb_page_cleaners`, `innodb_use_fdatasync`, `innodb_flush_neighbors`, `skip_name_resolve`, `innodb_buffer_pool_chunk_size`, `binlog_group_commit_*`, `rpl_semi_sync_source_wait_point`, `group_replication_*` (flow control, expel timeout, paxos_single_leader, message_cache_size) — всё на дефолте сервера, а дефолты **расходятся 8.0 vs 8.4** (см. п. 2.9) | `cfg/mysql.go` | Сравнение 8.0 vs 8.4 в матрице измеряет в том числе разницу дефолтов, а не только версий |
| B9 | `max_connections=151` | `cfg/mysql.go:235` | 128 VU + exporter + мониторинг ProxySQL ≈ впритык; без ProxySQL любой пул >150 даст `Too many connections` |
| B10 | `open_files_limit=5000` при `table_open_cache=4000` (+ `max_connections`) | `cfg/mysql.go:273` | mysqld при нехватке сам поднимет/ругнётся; правильнее считать от `max_connections`/`table_open_cache` |
| B11 | `binlog_expire_logs_seconds` = 30 дней, диск 200 ГБ | `cfg/mysql.go:151` | На серии прогонов в одном датадире binlog (~10-20 МБ/с при нагрузке) может занять диск; для benchmark-стенда нужен короткий срок |
| B12 | `semisync`: `rpl_semi_sync_source_timeout=10000` мс | `cfg/mysql.go`, semisync-блок | При отставании реплики >10 с мастер **молча** уходит в async (status `Rpl_semi_sync_source_status=OFF`) → результат «semi-sync» недостоверен [doc: 8.4/replication-semisync] |
| B13 | Хост ОС: рецепт MySQL не добавляет `HostPrep` sysctl/THP (схема `cfg/host_sysctl.go` есть, но mysql-рецепт её не использует; в `compile` единственный `HostPrep` — picodata), `Container.Ulimits` не задаётся, диск монтируется как `ext4` + `defaults` (`pipelines/spec/mount-data.sh`: `filesystem=ext4`, `options=defaults`, нет `noatime`) | `recipe_mysql*.go`, `pipelines/spec/diskprep.go` | THP/swappiness/readahead — по умолчанию дистрибутива |
| B14 | ProxySQL: нет `mysql-multiplexing`, `free_connections_pct`, `connection_warming`, `max_stmts_per_connection`, `connection_max_age_ms`, `threads` фиксировано 4, у серверов нет `max_replication_lag` (async), `monitor_*` для GR задан только через общий интервал | `cfg/proxysql.go` | Нечем управлять пулом; `threads=4` при 8 vCPU у прокси-ВМ недоиспользует ядра |
| B15 | `buffer_pool_instances`/`chunk` не участвуют в `explicitConfig` логике → при явном `innodb_buffer_pool_size` в роли instances всё равно принудительно 1 | `cfg/mysql.go` | см. B2 |

### 1.3 Что уже верно
- `innodb_flush_method=O_DIRECT` (8.4 дефолт тоже O_DIRECT при поддержке; 8.0 — fsync, поэтому схема правильно задаёт явно).
- `flush_log_at_trx_commit=1`, `sync_binlog=1`, `innodb_doublewrite=ON` — полная долговечность.
- ProxySQL: `writer_is_also_reader=1`, `max_transactions_behind=100`, `transaction_persistent=1`, правила `FOR UPDATE`→writer / `SELECT`→reader.
- GR: XCOM-стек, ssl REQUIRED, allowlist, bootstrap-маркер.

---

## 2. План профиля `auto`

Идея: `buffer_pool: auto` превращается в профиль **`tuning: auto`** (или остаётся именем, но
расширяется): все значения ниже вычисляются компилятором из `CPU`, `MemoryGB`,
`DiskGB/DiskType` машины роли, роли (primary/replica/GR-member/proxy), топологии и
VU. Явное значение в конфиге роли (`explicitConfig`) побеждает всегда; значение
пишется в `effective_configs` (UI показывает итог). Обозначения: `R` = RAM МБ,
`C` = vCPU, `V` = VU нагрузки (если неизвестно — 128).

### 2.1 Память

| Параметр | Формула | Обоснование / ссылка |
|---|---|---|
| `innodb_buffer_pool_size` | `share(R) * R`, затем **округлить вниз** до кратного `chunk*instances`; `share`: R≤4 ГБ → 0.50, ≤8 ГБ → 0.60, ≤64 ГБ → 0.70, >64 ГБ → 0.75 | [doc] MySQL допускает до ~80% RAM на выделенном сервере; кроме пула в процессе живут: log buffer 64M, perf schema (сотни МБ при `max_connections` в тысячах), per-connection буферы, GR message cache до 1 ГБ, temptable (8.4: 3% RAM, 1-4 ГБ), плюс mysqld_exporter и Docker на той же ВМ. Для 32 ГБ: 22.5-22.9 ГБ как сейчас. Округление вниз закрывает B5 [doc: 8.4/innodb-buffer-pool-resize] |
| `innodb_buffer_pool_instances` | pool<1 ГБ → 1; иначе `clamp(ceil(C/2), 2, 16)` и чтобы `pool/instances ≥ 1 ГБ`; 8 vCPU → 4 | [doc] default 8.0 = 8, 8.4 = min(chunks/2, C/4)=2 на 8 vCPU — версии расходятся, фиксируем одинаково. [prac] Percona/Oracle бенчмарки OLTP используют 8-16 на 16+ ядрах; для 8 vCPU 4 разумно. Низкая уверенность в точном числе: **проверять матрицей 1/2/4/8** |
| `innodb_buffer_pool_chunk_size` | оставить 128M (не трогать) | [doc] число чанков ≤1000; 22 ГБ/128M ≈ 180 |
| `innodb_log_buffer_size` | 64M (обе версии) | [doc] 8.4 дефолт 64M, 8.0 — 16M; выравнивание версий. Большой буфер избавляет от записи redo до коммита крупных транзакций (TPC-C Delivery) |
| `tmp_table_size`/`max_heap_table_size` | оставить 16M | TPC-C почти не использует временные таблицы; не раздувать |
| `temptable_max_ram` | не задавать (8.4: 3% RAM; 8.0: 1 ГБ) | различие версий, влияние на TPC-C ничтожно; записать в известные различия |
| `performance_schema` | ON, оставить | нужен exporter и healthcheck реплик (`performance_schema.replication_*`). [prac] overhead несколько %; фиксируем как константу стенда, чтобы сравнение было честным |
| `innodb_dedicated_server` | **не использовать**, оставить OFF | см. 2.10 |

### 2.2 Redo, binlog, flush

| Параметр | Формула | Обоснование |
|---|---|---|
| `innodb_redo_log_capacity` (8.0.30+; 8.0 <8.0.30 — `innodb_log_file_size*files`, образ `mysql:8.0` плавающий, ожидается ≥8.0.30) | `clamp(pool/4, 2 ГБ, 32 ГБ)` и `≤ 10% DiskGB`; для 32 ГБ/200 ГБ → ~5.6 ГБ | [doc] ёмкость задаёт место, при заполнении >~30/32 блокируются клиенты; динамическая. [prac] Percona/Oracle: ёмкость ≈ 1 час пиковой записи (замер `Innodb_redo_log_current_lsn`), dedicated_server 8.4 даёт `C/2` ГБ (=4 ГБ при 8 vCPU) [doc: 8.4/innodb-dedicated-server]; 8.0 — по RAM (≈ round(0.5625*RAM) ГБ=18 ГБ при 32 ГБ). 5-6 ГБ — середина; цена — время crash recovery (минуты) и место на диске. **Уточнить прогонами: смотреть `Innodb_redo_log_current_lsn − checkpoint_lsn` к ёмкости** |
| `innodb_flush_log_at_trx_commit` | 1 | см. п. 3 |
| `sync_binlog` | 1 | см. п. 3 |
| `innodb_flush_method` | `O_DIRECT` | [doc] O_DIRECT_NO_FSYNC допустим с 8.0.14, но выигрыш невелик и есть оговорка про разные устройства; O_DIRECT — безопасный выбор |
| `innodb_use_fdatasync` | ON (8.0.26+) | [doc] 8.4 дефолт ON, 8.0 OFF; выравнивание версий, fdatasync не флашит лишние метаданные |
| `innodb_doublewrite` | ON (DETECT_AND_RECOVER) | паритет с PG `full_page_writes` |
| `binlog_expire_logs_seconds` | 86400 (1 сутки) в профиле benchmark | B11, место на диске |
| `binlog_row_image` | full | паритет, минимизация риска (minimal ломает часть GR/WRITESET-проверок на таблицах без PK; у TPC-C PK есть, но оставляем) |
| `binlog_group_commit_sync_delay` | 0 (по умолчанию), **кандидат для эксперимента** 200-1000 мкс | [prac] при fsync network-ssd ~1-2 мс групповой коммит с задержкой повышает tps ценой латентности; не включать в auto до замеров |
| `log_bin` для single без реплик | оставить ON | production-подобно, ср. PG WAL; опция «выключить binlog для single» — отдельный флаг, не auto |

### 2.3 IO и фоновые потоки (с учётом сетевого диска)

Предел диска (YC) вычисляется из `DiskType`/`DiskGB` [doc: yandex.cloud limits]:
- `network-ssd`: блок 32 ГБ → 1000 write / 1000 read IOPS и 15 МБ/с на блок; потолок 40000 write / 20000 read IOPS, 450 МБ/с; `units = ceil(GB/32)`.
- `network-ssd-nonreplicated` (и `io-m3`): блок 93 ГБ → 5600 wIOPS/28000 rIOPS, 82/110 МБ/с на блок; потолок 75000 IOPS, 1 ГБ/с (io-m3: 40000 wIOPS).
- Лимит ВМ: network-ssd 3500 IOPS и 45 МБ/с **на vCPU**; non-repl/io-m3 10000 IOPS и 100 МБ/с на vCPU. Эффективный лимит = min(диск, ВМ).
- Читают и пишут из общего ресурса диска.

Пусть `wIOPS`, `BW` (МБ/с) — эффективные лимиты записи.

| Параметр | Формула | Обоснование |
|---|---|---|
| `innodb_io_capacity` | `min(0.5*wIOPS, 0.4*BW*1024/16)` страниц/с; округл. до сотни; 200 ГБ network-ssd → min(3500, 2625)=**2600** | [doc] io_capacity — бюджет фонового flush в IOPS, держать «как можно ниже без отставания»; [prac] Percona-бенчмарки используют 2000-2500 для NVMe. Страница 16 КБ → полоса 105 МБ/с ≈ 6700 страниц/с, а redo+binlog+doublewrite тоже едят полосу, поэтому 40% на фоновый flush |
| `innodb_io_capacity_max` | `min(wIOPS, 0.7*BW*1024/16)`; → **4500** | [doc] верхняя граница при отставании, ≥ io_capacity. Текущие 20000 физически недостижимы (B4). 8.4: дефолт = 2*io_capacity, 8.0: ≥2000 |
| `innodb_read_io_threads` / `innodb_write_io_threads` | `max(4, C/2)` каждый, потолок 16 | [doc] 8.4 дефолт read = max(4, C/2), write 4. На сетевом диске с латентностью 1-2 мс важна очередь запросов; для 8 vCPU = 4/4 (как сейчас). Низкая уверенность выигрыша |
| `innodb_page_cleaners` | = `innodb_buffer_pool_instances` | [doc] 8.4 дефолт так; 8.0 = 4 (и обрезается до instances) |
| `innodb_purge_threads` | `4` при C≥8, иначе `max(1, C/2)` | [doc] 8.4 понизил дефолт до 1 при ≤16 CPU, 8.0 = 4. TPC-C (Delivery/Payment) создаёт много undo; отстающий purge раздувает history list и замедляет. Выравниваем версии |
| `innodb_adaptive_hash_index` | OFF (обе версии) | [doc] 8.4 дефолт OFF («before 8.4 was ON»); [prac] Percona: AHI — точка контеншна при высокой конкурентности/записи. Для паритета версий — OFF и на 8.0 |
| `innodb_change_buffering` | `none` на обеих (8.0 дефолт `all`, 8.4 `none`) | [doc] **Change buffer в 8.4 НЕ удалён**, только дефолт сменён на `none` [doc: 8.4/innodb-parameters]. При полностью кэшированном наборе TPC-C (100 складов ≈ 10 ГБ ≪ пула) эффект ≈ 0; выравнивание версий. Средняя уверенность |
| `innodb_flush_neighbors` | 0 | [doc] дефолт 0 в 8.0+, SSD; зафиксировать явно |
| `innodb_log_writer_threads` | не трогать | [?] в 8.4 по общим знаниям автоматически выключается при малом числе CPU — страницу не дочитал; пометить в вопросах |
| `innodb_max_dirty_pages_pct` / `_lwm`, `innodb_adaptive_flushing` | дефолты (90 / 10 / ON) | менять только по данным замера |

### 2.4 Конкурентность и соединения (128 VU)

| Параметр | Формула | Обоснование |
|---|---|---|
| `max_connections` | `max(500, ceil(1.5*V)+100)`, кап `RAM_ГБ*40`; для 128 VU → **500** | текущие 151 впритык (B9). При ProxySQL серверная сторона = сумма `mysql_servers.max_connections` (1000/бэкенд) — достаточно держать ≥ реально открываемых пулом; не ставить тысячи без нужды (perf schema растёт) |
| `thread_cache_size` | `min(max_connections, 128)` | [doc] 8.0 авто = 8+max_connections/100 (≤100); кэш избавляет от создания потока на каждое подключение при переподключениях драйвера |
| `innodb_thread_concurrency` | 0 | [doc] 0 = без лимита (дефолт). [prac] на 8 vCPU/128 VU работает adaptive; ограничение 16-24 — только эксперимент |
| `thread_handling` | one-thread-per-connection (Community; thread pool — только Enterprise/Percona) | в образе `mysql:8.x` thread pool нет; для 128 VU штатный режим ок. Ограничить число одновременно работающих — ProxySQL multiplexing |
| `table_open_cache` | 4000 (оставить), `table_open_cache_instances` дефолт 16 | TPC-C: 9 таблиц ×instances ≪ кэша |
| `table_definition_cache` | 2000 (оставить) | |
| `open_files_limit` | `max(65535, 10 + max_connections + 2*table_open_cache)` | B10; в Docker задать `Ulimits{"nofile": 1048576}` на контейнере (поле `spec.Container.Ulimits` уже есть) |
| `skip_name_resolve` | ON | [doc] убирает обратный DNS при каждом подключении; в доках — рекомендуется, когда все гранты по IP. В схеме нет → добавить; **проверка**: гранты `root@%`, `stroppy`@`%` — по `%`, всё ок |
| `innodb_lock_wait_timeout` | 50 (оставить) | `innodb_deadlock_detect` оставить ON: при отключении deadlock превращается в 50-секундное ожидание и tps проваливается [prac] |
| `transaction_isolation` | **вопрос паритета**, см. п. 5 | REPEATABLE-READ vs PG READ COMMITTED |

### 2.5 Репликация (async / semi-sync)

| Параметр | Формула | Обоснование |
|---|---|---|
| `replica_parallel_workers` | `min(C, 8)` на реплике (сейчас 4) | [prac] при WRITESET параллелизм реально используется; потолок экономит CPU |
| `binlog_transaction_dependency_tracking` | `WRITESET` на **8.0 во всех режимах** (закрыть B6), 8.4 — внутреннее | [doc] 8.4 всегда WRITESET. Без этого на 8.0 реплика отстаёт сильнее, и ProxySQL шунит читателей |
| `replica_preserve_commit_order` | ON (оставить) | [doc] нужен для консистентности с параллельным applier; для GR обязателен |
| `replica_parallel_type` (8.0) | LOGICAL_CLOCK (оставить) | [doc] в 8.4 удалено |
| `rpl_semi_sync_source_wait_point` | `AFTER_SYNC` (явно) | [doc] дефолт, lossless |
| `rpl_semi_sync_source_wait_for_replica_count` | `semi_sync_wait_for_slave_count` (уже) | |
| `rpl_semi_sync_source_timeout` | **4294967295 мс** (не уходить в async) вместо 10000 | [doc] по таймауту мастер уходит в async; для честного замера нужно либо ловить это в метрике, либо запретить. Альтернатива: оставить 10000 и алертить `Rpl_semi_sync_source_status` — решение за продуктом (см. п. 5) |
| `replication_sender_observe_commit_only`, `replication_optimize_for_static_plugin_config` | ON на source и replicas (при semi-sync) | [doc] 8.4/replication-semisync: снижают lock contention при росте числа реплик. Доступность на 8.0 [?] (8.0.26+, проверить) |
| `relay_log_recovery`, `sync_relay_log` | дефолты | не влияют на ack semi-sync [doc: ack после записи в relay log и flush] |
| `read_only` / `super_read_only` на репликах | уже `read_only=ON`; добавить `super_read_only=ON` | защита от записи пользователями с SUPER; на perf не влияет |

### 2.6 Group Replication

| Параметр | Значение | Обоснование |
|---|---|---|
| `group_replication_consistency` | `EVENTUAL` явно на 8.0 и 8.4 | B7. [doc] 8.4 дефолт `BEFORE_ON_PRIMARY_FAILOVER`; EVENTUAL — самый быстрый и сравним с остальными движками; семантику читателей через ProxySQL оставить как есть. Если продукт захочет read-your-writes — `BEFORE` (на чтениях) за отдельную цену |
| `group_replication_exit_state_action` | `READ_ONLY` явно | [doc] 8.0 READ_ONLY, 8.4 OFFLINE_MODE; выравнивание |
| `group_replication_paxos_single_leader` | ON при `single_primary` | [doc] с 8.0.27, изменение применяется после рестарта группы, статус в `replication_group_communication_information.WRITE_CONSENSUS_SINGLE_LEADER_CAPABLE`. [prac] снижает накладные расходы консенсуса в single-primary. Включать в конфиг до `START GROUP_REPLICATION` — безопасно. Средняя уверенность выигрыша |
| `group_replication_member_expel_timeout` | 15 с | [doc] дефолт 5 с (+~5 с детекции). [prac] при насыщении сетевого диска fsync-паузы могут привести к выселению; на стенде лучше пережить. Не влияет на steady-state |
| `group_replication_flow_control_mode` / `_applier_threshold` / `_certifier_threshold` | QUOTA / 25000 / 25000 (дефолты) | [doc] throttling включается при очереди >25000; следить по `Gr_flow_control_throttle_*` (8.4). Повышать порог до 50000 — только по данным |
| `group_replication_message_cache_size` | `clamp(3% RAM, 128M, 1G)` | [doc] дефолт 1 ГБ, минимум 128 МБ; на малых ВМ дефолт съедает значимую долю RAM |
| `group_replication_compression_threshold` | дефолт (1 МБ) | |
| `group_replication_transaction_size_limit` | дефолт 150 МБ | TPC-C транзакции малы |
| `binlog_checksum`, `transaction_write_set_extraction` | дефолты (CRC32, XXHASH64 на 8.0) | GR работает с CRC32 |
| `replica_parallel_workers` для канала `group_replication_applier` | `min(C,8)` + `replica_preserve_commit_order=ON`, WRITESET | [doc] рекомендация GR docs |
| Платформа | PK на всех таблицах (TPC-C ок) | GR требует первичные ключи |

### 2.7 ProxySQL

Расчёт от машины роли `proxy`: `Cp` vCPU, `V` VU.

| Параметр | Значение | Обоснование |
|---|---|---|
| `mysql-threads` | `min(Cp, 16)` (сейчас 4) | [doc] дефолт 4, число потоков = число ядер — рекомендация проекта; `threads` не меняется на лету |
| `mysql-max_connections` | `max(2048, 2*V)` | [doc] дефолт 2048; ≥ клиентов |
| `mysql_servers.max_connections` (на бэкенд) | `max_connections бэкенда − 50` или 1000 из схемы, меньшее | не больше, чем бэкенд может принять (с учётом exporter/monitor) |
| `mysql_users.max_connections` | 10000 (оставить) | |
| `mysql-multiplexing` | true (дефолт, явно) | [doc] рекомендует держать включённым. **TPC-C исполняет явные транзакции** → мультиплексирование выключено на время транзакции (active transaction), т.е. число бэкенд-соединений ≈ число активных транзакций ≈ V; выигрыш только между транзакциями. Любые `SET`/`@var` в сессии драйвера отключают мультиплексирование навсегда на соединении → смотреть `stats_mysql_processlist.extended_info` [doc: proxysql.com/documentation/multiplexing] |
| `mysql-free_connections_pct` | 10 (дефолт) | [doc] доля `max_connections` бэкенда, держимая «тёплой» |
| `mysql-connection_warming` | true | [doc] дефолт false; без прогрева первые секунды нагрузки платят за открытие 128 бэкенд-соединений (TLS к бэкенду включён, `use_ssl=1`) — искажение начала прогона. Средняя уверенность |
| `mysql-max_stmts_per_connection` | 20 (дефолт) | |
| `mysql-connection_max_age_ms` | 0 | не рвать соединения посреди прогона |
| `mysql-commands_stats`, `query_digests` | true, оставить | [doc] digests нужны для детекции мультиплексирования; overhead мал; `eventslog` выключен |
| `mysql-ping_interval_server_msec` | 120000 (в схеме) | оставить |
| Hostgroups (async/semisync): `check_type=read_only`; `max_replication_lag` на репликах | 10 с | [doc] mysql_servers.max_replication_lag шунит отстающих читателей (сейчас у async отстающая реплика продолжает отдавать чтения); `monitor_replication_lag_interval` дефолт ок |
| Hostgroups (GR): `writer_is_also_reader=1`, `max_transactions_behind=100` | как есть | [doc: mysql_group_replication_hostgroups]; см. комментарий в `proxysql.go:197-200` |
| Query rules | `^SELECT.*FOR UPDATE` → writer, `^SELECT` → reader | `transaction_persistent=1` держит запросы внутри транзакции на одном hostgroup (стартовала на writer); автокоммит-SELECT идёт в reader — для TPC-C корректно |
| Задержка хопа | ~0.1-0.3 мс на запрос [prac, не измерено] | ProxySQL-режим сравнивать с прямым подключением отдельным вариантом матрицы |

### 2.8 Хост ОС в Docker (ВМ Yandex Cloud)

Выполнить через `spec.HostPrep` (kinds `sysctl`, `script`) в рецепте MySQL и
`spec.Container.Ulimits`; схемы `cfg/host_sysctl.go`, `cfg/host_disks.go`,
`cfg/docker_container.go` уже описывают нужные ключи.

| Что | Значение | Обоснование |
|---|---|---|
| `vm.swappiness` | 1 | [doc kernel.org] уже дефолт `cfg.host.sysctl` |
| Transparent Huge Pages | `never` | [prac] Percona/Oracle KB: THP даёт непредсказуемые паузы; schema `transparent_hugepage` default never |
| `vm.dirty_ratio`/`dirty_background_ratio` | 20/10 (дефолт) → рассмотреть 10/5 | при O_DIRECT на данных влияние мало (binlog и redo-бекграунд идут через page cache binlog), оставить |
| Файловая система данных | `xfs` + `noatime,nodiratime` | `host.disks` Desc: xfs дефолт для данных, а `mount-data.sh` форматирует ext4/`defaults`. [prac] Для MySQL с O_DIRECT XFS предпочтителен (параллельные DIO-записи в один файл), ext4 тоже ок; важнее — `noatime`. Изменение даёт `spec.Disk.Filesystem`/`MountOptions` (`pipelines/schemas/spec/run.go:118`) |
| I/O scheduler, read-ahead | `none`/`mq-deadline`, ~128 КБ [prac, ?] | virtio-blk сетевой диск; влияние не измерено |
| `nofile` ulimit контейнера | 1048576 | B13/B10 |
| `--cpus`/`--memory` | **не ограничивать** (`limit_cpu/limit_memory=false`, как сейчас) | MySQL считает пул от RAM ВМ; cgroup-лимит ниже пула привёл бы к OOM-kill |
| Сеть | `host` (уже) | без NAT-хопа |
| `tmpdir` | оставить `/tmp` контейнера [?] | временные таблицы TPC-C пренебрежимы; если появятся spill — вынести на data-диск |
| `fs.aio-max-nr` | 1048576 (`host.sysctl` дефолт) | [doc kernel.org] InnoDB использует libaio; 65536 по умолчанию мало при 8+ io-потоках на нескольких инстансах |
| Redo/binlog на отдельный диск | вне объёма auto; `spec.Machine.Disks` позволяет несколько | на одном network-ssd redo+binlog+doublewrite+данные делят 105 МБ/с — вероятнее всего **главный потолок tps** независимо от my.cnf |

### 2.9 Различия 8.0 vs 8.4, влияющие на матрицу (дефолты → что фиксируем)

| Параметр | 8.0 | 8.4 | Профиль |
|---|---|---|---|
| `innodb_adaptive_hash_index` | ON | OFF | OFF оба |
| `innodb_change_buffering` | all | none (опция жива) | none оба |
| `innodb_buffer_pool_instances` | 8 (1 если <1 ГБ) | min(chunks/2, C/4) | вычисляем, оба |
| `innodb_io_capacity`/`_max` | 200 / ≥2000 | 10000 / 2×cap | по диску, оба |
| `innodb_log_buffer_size` | 16M | 64M | 64M |
| `innodb_use_fdatasync` | OFF | ON | ON |
| `innodb_purge_threads` | 4 | 1 (если ≤16 CPU) | 4 |
| `innodb_read_io_threads` | 4 | max(4, C/2) | вычисляем |
| `innodb_page_cleaners` | 4 | = instances | = instances |
| `innodb_flush_method` | fsync | O_DIRECT | O_DIRECT |
| `innodb_numa_interleave` | OFF | ON | n/a (ВМ без NUMA) |
| `group_replication_consistency` | EVENTUAL | BEFORE_ON_PRIMARY_FAILOVER | EVENTUAL |
| `group_replication_exit_state_action` | READ_ONLY | OFFLINE_MODE | READ_ONLY |
| dependency tracking | COMMIT_ORDER (WRITESET по желанию) | всегда WRITESET | WRITESET оба |
| `temptable_max_ram` | 1 ГБ | 3% RAM (1-4 ГБ) | дефолты, зафиксировать как известное различие |
| `innodb_doublewrite_*` | files = instances*2, pages = write_io_threads | files=2, pages=128 | дефолты |
| удалено в 8.4 | — | `transaction_write_set_extraction`, `binlog_transaction_dependency_tracking`, `default_authentication_plugin`, `replica_parallel_type`, `mysql_native_password` по умолчанию off | схема уже ветвится по `is80` |
Источник колонок: [doc: dev.mysql.com/doc/refman/8.4/en/mysql-nutshell.html].

### 2.10 `innodb_dedicated_server`: использовать ли
**Нет.** Причины:
1. [doc] Алгоритмы **разные**: 8.0 — пул 0.75*RAM при >4 ГБ, redo по RAM (для 32 ГБ ≈ round(0.5625*32)=18 ГБ), flush method `O_DIRECT_NO_FSYNC`; 8.4 — пул тот же, но redo = `C/2` ГБ (4 ГБ при 8 vCPU), flush_method **не трогает** [doc: 8.0/8.4 innodb-dedicated-server]. Сравнение 8.0 vs 8.4 «с dedicated» сравнивало бы разные настройки.
2. 0.75*RAM = 24 ГБ из 32 — оставляет ≈8 ГБ на perf schema, GR cache (до 1 ГБ), соединения, exporter, Docker, page cache binlog.
3. Значения не попадают в `effective_configs` (их считает mysqld), а продукт показывает итоговый конфиг.
4. Если пул задан явно, производные величины всё равно считаются от **RAM**, а не явного пула [doc] — путаница.
Вывод: считаем явно (п. 2.1-2.2); поле `innodb_dedicated_server` оставить в схеме для ручного включения.

---

## 3. Надёжность и паритет

- **MySQL «по умолчанию = durable»**: `innodb_flush_log_at_trx_commit=1` (redo flush+fsync на коммите) и `sync_binlog=1` (fsync binlog на каждую группу коммитов) + `innodb_doublewrite=ON`. Это 2 fsync на коммит (redo + binlog) в одном групповом коммите — тяжелее, чем WAL у PostgreSQL (1 fsync). Не ослаблять в `auto`.
- Паритет с другими движками стенда: PG `synchronous_commit=on` + `fsync=on` + `full_page_writes=on` ≈ MySQL trx_commit=1 + sync_binlog=1 + doublewrite ON. Cockroach/YDB — синхронная реплицированная запись. Профили «fast/unsafe» (trx_commit=2, sync_binlog=0, doublewrite OFF) — только отдельным именованным профилем с пометкой в отчёте, не в `auto`.
- `sync_binlog=1` + `trx_commit=1` обязательны для гарантий semi-sync `AFTER_SYNC` («все видимые транзакции есть на реплике»); при `sync_binlog≠1` оговорка в доке: упавший мастер нельзя использовать [doc: 8.4/replication-semisync].
- GR: durability членов независима (каждый fsync локально) + консенсус; не ослаблять на членах группы.
- Недостоверные результаты, которые надо детектить: semi-sync ушёл в async (B12), GR throttling (`Gr_flow_control_throttle_*`), checkpoint age близок к ёмкости redo, шунт реплик ProxySQL (все читатели отстали → `writer_is_also_reader=1` уже страхует), мультиплексирование выключено.
- Рекомендуется добавить в `run result` итоговые «качественные флаги» из `SHOW GLOBAL STATUS`: `Rpl_semi_sync_source_status`, `Innodb_redo_log_*` (для checkpoint age), `Innodb_buffer_pool_wait_free`, `Innodb_log_waits`, `Gr_flow_control_throttle_*` (8.4).

---

## 4. Изменения платформы

1. `internal/domain/compile/innodb.go` — переименовать/расширить в `mysqlTuning(role, schemaID, machine, overlay)`:
   - принимать `CPU`, `MemoryGB`, `DiskGB`, `DiskType` машины (из `c.out.Spec.Machines`/`out.Machines`, см. `memoryOf`), VU (через `c.in.Workload`);
   - считать по п. 2 (пул с округлением вниз, instances, redo, io_capacity по диску, потоки, max_connections, open_files_limit, GR/semisync параметры);
   - убрать `if maria` для io_capacity (B3): MariaDB оставить отдельной веткой (`innodb_io_capacity`, `innodb_log_file_size`; не входит в этот отчёт);
   - уважать `explicitConfig` для **каждого** ключа (сейчас только пула и io_capacity);
   - ввести таблицу лимитов дисков YC (`network-ssd`, `network-ssd-nonreplicated`, `network-ssd-io-m3`, лимиты ВМ per vCPU) в `internal/domain/catalog` или рядом.
2. `pipelines/schemas/cfg/mysql.go`:
   - новые поля (все с `// doc:` и Desc с единицей): `innodb_adaptive_hash_index`, `innodb_change_buffering`, `innodb_log_buffer_size`, `innodb_purge_threads`, `innodb_page_cleaners`, `innodb_use_fdatasync`, `innodb_flush_neighbors`, `innodb_buffer_pool_chunk_size`, `skip_name_resolve`, `rpl_semi_sync_source_wait_point`, `replication_sender_observe_commit_only`, `replication_optimize_for_static_plugin_config`, `group_replication_exit_state_action`, `group_replication_paxos_single_leader`, `group_replication_member_expel_timeout`, `group_replication_message_cache_size`, `group_replication_flow_control_*`, `binlog_group_commit_sync_delay`, `log_replica_updates` уже есть;
   - **`binlog_transaction_dependency_tracking`/`transaction_write_set_extraction` вынести из `grLines` в общий блок для 8.0** (B6);
   - `innodb_buffer_pool_instances` и `innodb_redo_log_capacity`, `max_connections` сделать «пусто = выбор сервера/auto» (Nullable без схемного дефолта 1/100/151) — иначе схема перебивает автоподбор (B2);
   - `group_replication_consistency`: при рендере всегда писать значение (EVENTUAL по профилю);
   - ограничить диапазоны: `io_capacity_max ≥ io_capacity` уже есть; добавить правило `redo_log_capacity ≥ 8M`, `pool` кратность (warn).
3. `pipelines/schemas/dbparams/mysql.go` — режим `buffer_pool` → общий `tuning` (`auto|fixed|stock`; `fixed` оставляет `buffer_pool_mb`), обновить Desc (перечислить, что auto меняет), добавить опциональный `workload_vus` hint **не нужен** — VU берётся из теста.
4. `internal/domain/compile/recipe_mysql.go` и `recipe_mysql_cluster.go`:
   - вызов `mysqlTuning` для каждой роли (и db, и db_replica, и GR-участника);
   - `spec.Container.Ulimits{"nofile": 1048576}` для контейнера БД;
   - `c.out.Spec.HostPrep` kinds `sysctl` (`vm.swappiness=1`, `fs.aio-max-nr`, `net.core.somaxconn`) и `script` для THP never (рендер из `cfg.host.sysctl`);
   - semi-sync: `loose-rpl_semi_sync_source_wait_point=AFTER_SYNC`, `timeout` по решению (п. 5), `super_read_only=ON` на репликах;
   - GR: `loose-group_replication_paxos_single_leader`, `member_expel_timeout`, `exit_state_action`, `message_cache_size` в конец конфигa рядом с имеющимся блоком `loose-group_replication_*` (стр. 86);
   - ProxySQL (`proxysql()`): `threads` от CPU роли proxy, `max_connections` от VU, `max_replication_lag` на репликах, `connection_warming=true`.
5. `pipelines/schemas/cfg/proxysql.go` — поля `multiplexing`, `free_connections_pct`, `connection_warming`, `connection_max_age_ms`, `max_stmts_per_connection`, у `mysql_servers` — `max_replication_lag`; обновить `servers_block`/шаблон `mysql_variables`.
6. `pipelines/spec/mount-data.sh` / `diskprep.go` и `compile.go:193`: диск БД MySQL — `Filesystem: xfs`, `MountOptions: ["noatime","nodiratime"]` (по умолчанию для всех db-ролей — кросс-движковое решение, согласовать с остальными профилями).
7. Тесты: `internal/domain/compile/mysql_test.go`, `mysql_cluster_test.go` — добавить таблицу «машина → значения» (8/32/200, 4/16/100, 16/64/500; 8.0 и 8.4), проверку `explicitConfig`, кратности пула, инвариантов (`io_capacity_max ≥ io_capacity`, `redo ≤ 10% disk`); golden `pipelines/schemas/testdata/*` перегенерировать (`make schemas-export`, `make openapi generate-go generate-ts` если меняется контракт), `internal/api/vocab_test.go` и `web/src/locales` — если появятся новые enum (напр. режим `tuning`).
8. Отчёт прогона: добавить сбор статусов из п. 3 (см. `internal/...` observer; конкретный пакет не изучался).

---

## 5. Риски и открытые вопросы

1. **Главный потолок — диск, не my.cnf.** 200 ГБ network-ssd = 7000 IOPS/105 МБ/с на ВСЁ (данные+redo+binlog+doublewrite). Ожидание: после фикса redo/instances рост tps может упереться в полосу; сравнение движков на таком диске отражает диск. Решение продукта: размер диска как параметр профиля (>=512 ГБ даёт 16 блоков = 240 МБ/с) или `network-ssd-nonreplicated`.
2. **Уверенность по пунктам.** Высокая: redo (B1), instances (B2), io_capacity по диску, max_connections, WRITESET на 8.0, GR consistency. Средняя: AHI/change buffering/purge threads (паритет версий, эффект невелик), paxos_single_leader, connection_warming. Низкая/не проверено: `innodb_log_writer_threads` auto-off на малом CPU, I/O scheduler, io/page-cleaner thread counts, влияние `binlog_group_commit_sync_delay`. Всё это — для матричного A/B, а не для слепого включения.
3. **Размер redo vs recovery time.** 5-6 ГБ redo → крэш-рекавери минуты; в стенде допустимо, но `Restart: always` + health-check timeout в `mysql-group-start.sh` (300 с на готовность) может не хватить после аварии при большом redo. Проверить.
4. **Формула io_capacity — эвристика** (0.5×IOPS, 0.4×полоса/16 КБ): подтвердить мониторингом `Innodb_buffer_pool_pages_dirty`, `Innodb_buffer_pool_wait_free`, checkpoint age; при нехватке — поднять до 0.5-0.6 BW.
5. **Паритет изоляции.** MySQL по умолчанию REPEATABLE READ, PostgreSQL — READ COMMITTED. Если stroppy TPC-C не задаёт уровень сам, движки сравниваются при разной изоляции (gap locks в MySQL → больше блокировок/deadlock'ов на districts). Решение: либо `transaction_isolation=READ-COMMITTED` в auto, либо явно в workload; выяснить, что выставляет stroppy (не проверялось).
6. **semi-sync timeout.** Либо «никогда не уходить в async» (честная модель, но при падении реплики стенд зависает), либо оставить 10000 и помечать прогон невалидным по `Rpl_semi_sync_source_status`. Нужно решение продукта.
7. **`mysql:8.0` образ.** `innodb_redo_log_capacity` требует ≥8.0.30; `innodb_use_fdatasync` ≥8.0.26; `replication_*` оптимизации и `paxos_single_leader` (≥8.0.27) — проверять `loose-` префиксами (в GR-блоке уже используются) и зафиксировать минимальную 8.0.x в каталоге.
8. **ProxySQL.** Мультиплексирование при явных транзакциях TPC-C почти не работает; прокси не снимает нагрузку с бэкенда, только добавляет хоп. Метрики прокси (`stats_mysql_connection_pool`, `ConnUsed/ConnFree`) — собирать в отчёт. `threads` от CPU роли proxy требует, чтобы машина proxy была не меньше 4 vCPU.
9. **Docker.** Лимиты cgroup не задаём; если кто-то включит `limit_memory`, `auto` должен считать пул от лимита контейнера, а не от RAM ВМ — сейчас так не сделано.
10. **Перф-схема и exporter.** `performance_schema=ON` + collectors mysqld_exporter — постоянный overhead, одинаковый во всех прогонах; если нужен «чистый максимум» — отдельный профиль.
11. **Верификация источников не завершена**: страницы InnoDB variables (8.4) прочитаны не полностью (>100 КБ), поэтому дефолты `innodb_log_writer_threads`, `innodb_flush_neighbors`, `innodb_purge_threads` в таблице 2.9 частично взяты из nutshell-страницы (надёжно) и общих знаний (помечено [?]).

## Источники
- MySQL 8.4 «What Is New / nutshell» (смена дефолтов, удалённые опции): https://dev.mysql.com/doc/refman/8.4/en/mysql-nutshell.html
- `innodb_dedicated_server` 8.4 / 8.0: https://dev.mysql.com/doc/refman/8.4/en/innodb-dedicated-server.html , https://dev.mysql.com/doc/refman/8.0/en/innodb-dedicated-server.html
- Redo log: https://dev.mysql.com/doc/refman/8.4/en/innodb-redo-log.html ; Oracle blog: https://dev.mysql.com/blog-archive/dynamic-innodb-redo-log-in-mysql-80/
- Buffer pool chunk/instances: https://dev.mysql.com/doc/refman/8.4/en/innodb-buffer-pool-resize.html
- InnoDB параметры (AHI, change buffering): https://dev.mysql.com/doc/refman/8.4/en/innodb-parameters.html
- Semi-sync: https://dev.mysql.com/doc/refman/8.4/en/replication-semisync.html
- GR переменные: https://dev.mysql.com/doc/refman/8.4/en/group-replication-system-variables.html (дефолты — из общих знаний, страница-сводка подтверждает наличие и динамичность)
- ProxySQL multiplexing: https://proxysql.com/documentation/multiplexing/ ; переменные: https://proxysql.com/documentation/global-variables/mysql-variables/
- YC лимиты дисков: https://yandex.cloud/en/docs/compute/concepts/limits
- Percona: https://www.percona.com/blog/mysql-8-0-dynamic-redo-log-sizing/ , https://www.percona.com/blog/more-on-checkpoints-in-innodb-mysql-8/ , https://www.percona.com/blog/tuning-mysql-innodb-flushing-for-a-write-intensive-workload/ , https://docs.percona.com/percona-server/8.4/8.4-defaults-and-tuning.html
- Dimitri Kravtchuk (checkpointing): http://dimitrik.free.fr/blog/posts/mysql-80-innodb-checkpointing.html
