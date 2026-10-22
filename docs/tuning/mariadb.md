# MariaDB 10.11 / 11.4 / 11.8: тюнинг профиля `auto`

Дата: 2026-10-22. Только чтение репо. Метки: **[офиц]** = написано в документации MariaDB / MaxScale / ProxySQL / Galera;
**[практика]** = блоги, форумы, опыт; **[гипотеза]** = моё предположение, нужна проверка; **[проверить]** = факт, который я не смог подтвердить.
Пути относительно `/home/yaroher/devel/github/stroppy-io/stroppy-cloud`.

---

## 1. Текущее состояние

### 1.1 Как сейчас формируется mariadb.cnf

- `internal/domain/compile/innodb.go:12-39` `innodbMemory` при `buffer_pool=auto` ставит только `innodb_buffer_pool_size = 70% RAM`
  (`:17`, константа `autoBufferPoolShare=0.7` на `:9`) и, только для MariaDB, `innodb_io_capacity=10000`, `innodb_io_capacity_max=20000` (`:31-38`).
  Больше `auto` ничего не трогает. Вызывается из `recipe_mysql.go:72` (single/async/semi_sync) и `recipe_mysql_cluster.go:78` (Galera).
- Всё остальное берётся из дефолтов схемы `pipelines/schemas/cfg/mariadb.go`, а дефолты там сознательно «как у upstream» (правило из `pipelines/schemas/AGENTS.md`). Отсюда картина матрицы.
- Рендер идёт одним шаблоном `mariadbTemplate` (`cfg/mariadb.go:499-613`). Конфиг кладётся в `/etc/mysql/conf.d/zz-stroppy.cnf` (`recipe_mysql.go:22`).
- Автоподбор знает только `MemoryGB` машины (`innodb.go:56-62`). У `spec.Machine` есть и `CPU` (`pipelines/spec/spec.go:108`), но формулы на CPU сейчас нет. Числа VU и размера датасета компилятор в этом месте не видит.

### 1.2 Баги и подозрительные места

| # | Где | Что | Вес |
|---|---|---|---|
| B1 | `cfg/mariadb.go:121-124` (`sync_binlog` default 0), `:104-106` (`log_bin` default `"binlog"`), шаблон `:531-535` | **Откуда `sync_binlog=0`**: дефолт схемы скопирован с MariaDB upstream (в MariaDB он 0 **[офиц]**; в MySQL 1). В `cfg/mysql.go:147-149` для MySQL стоит 1. При этом `log_bin` включён всегда (в шаблоне строка без условия), включая single и Galera. Итог: binlog пишется, но не fsync-ится, а `innodb_flush_log_at_trx_commit=1` создаёт ложное чувство «полной» durability. Подробно в п.3. | высокий (честность сравнения) |
| B2 | `compile/recipe_mysql.go:49-61` | Для MariaDB режим `semi_sync` **не включает полусинхронность**: оверлеи `rpl_semi_sync_*` выставляются только в ветке `!maria`. Нигде в `internal`/`pipelines` нет `rpl_semi_sync_master_enabled`/`slave_enabled` вне схемы (grep пуст). Если пользователь не проставил их в `configs` руками, «semi_sync» на MariaDB равен async. | высокий |
| B3 | `library/service.go:143-145` | Для Galera сидится глобальный `wsrep_sync_wait=1`. Это session-переменная, но глобальное значение наследует **каждая** новая сессия, включая мониторы ProxySQL/MaxScale/exporter. Документация прямо советует: health-check соединения должны иметь `wsrep_sync_wait=0` **[офиц]**. Главный кандидат на `invalid connection` (п.2.6). | высокий |
| B4 | `cfg/maxscale.go` (`causal_reads` default `global`) | По документации MaxScale 24.02 режимы `local`/`global`/`universal`, опирающиеся на `MASTER_GTID_WAIT`, **с Galera не работают**, с Galera работают только `fast*` и только с 24.02.5 **[офиц]**. Для Galera дефолт `global` бесполезен, а в худшем случае шумит ошибками. | средний |
| B5 | `compile/recipe_mysql.go:250`, `cfg/proxysql.go` (`threads` default 4), `dbkind/mysql.go:~120` | ProxySQL получает 2 vCPU (`Because(2, 2, 10, …)`), а 128 VU идут через него; `threads=4` на 2 vCPU и `max_connections` per-server 1000. Прокси может быть узким местом **[гипотеза]**. `server_version` по умолчанию `8.0.36`, хотя бэкенд MariaDB. | средний |
| B6 | `pipelines/spec/spec.go:140-162`, `pipelines/internal/run/deploy.go:240-270` | Контейнер БД запускается с `NetworkMode: host`, без `SecurityOpt`, без `memlock`. Дефолтный seccomp-профиль Docker и лимит memlock ломают `io_uring_queue_init()` в mariadbd, и InnoDB молча падает в `innodb_use_native_aio=OFF` (п.2.3, **[практика]**). | высокий для 10.11/11.4/11.8 |
| B7 | `pipelines/spec/mount-data.sh` / `diskprep.go:28-30` | Диск данных: ext4, опции `defaults` (без `noatime`). Схема `cfg.host.disks` умеет xfs/noatime, но для mariadb-роли не подключена (`dbkind/mysql.go:49-51`: `ConfigSchemas` только `cfg.mariadb.cnf` + exporter). `cfg.host.sysctl` для роли тоже не подключён, sysctl только через `runtime.host_prep` пользователя. | средний |
| B8 | `cfg/mariadb.go:216-219` (`max_connections` 151) | 128 VU + exporter + health-check + миграции почти упираются в 151. Через ProxySQL нагрузка мультиплексируется, но на `single` без прокси риск `Too many connections` **[гипотеза]**. | средний |
| B9 | `cfg/mariadb.go:283-286` (`performance_schema` OFF), шаблон | OFF оправдан для бенча, но нет и других инструментов диагностики (slow log выключен). Для разбора 520 tps придётся полагаться на `SHOW GLOBAL STATUS`. | низкий |
| B10 | `cfg/mariadb.go:126-128` | `binlog_expire_logs_seconds=0` (не чистить) плюс `max_binlog_size` 1G по умолчанию. На 40-минутной загрузке binlog может занять десятки ГБ на 200-гигабайтном диске **[гипотеза, объём нужно измерить]**. | низкий |
| B11 | `cfg/mariadb.go:~375-385` (11.8) | `innodb_snapshot_isolation` в 11.8 по умолчанию ON **[офиц]**, в 11.4 OFF (с 11.4.2 переменная есть, ON с 11.8). Это даёт ошибку 1020 на write-write конфликтах под REPEATABLE READ, что для TPC-C с конкурентными апдейтами `district`/`warehouse` может заметно менять tps и количество ретраев. Сравнение 11.4 vs 11.8 при этом перестаёт быть чистым. | средний |
| B12 | `cfg/mariadb.go:94-96,203-207` | Для 10.11 в схеме переменная `innodb_flush_method`, для 11.x `innodb_log_file_buffering`/`innodb_data_file_buffering`. Но `innodb_log_file_buffering` существует уже с 10.8.4/10.9.2 **[офиц]**, значит в 10.11 она тоже доступна. Не баг, просто неполнота. | низкий |

### 1.3 Чего нет вообще (нет в схеме `cfg.mariadb.cnf` и/или в автоподборе)

- `thread_handling`, `thread_pool_*`, `extra_port`, `extra_max_connections`, `back_log`.
- `innodb_use_native_aio`, `innodb_max_dirty_pages_pct(_lwm)`, `innodb_purge_threads`, `innodb_buffer_pool_size_max`, `innodb_flush_neighbors`.
- `wsrep_provider_options` целиком (`gcache.size`, `gcs.fc_limit`, `gcs.fc_factor`, `gcs.fc_single_primary`, `evs.*`, `pc.*`), `wsrep_OSU_method`, `wsrep_retry_autocommit`, `wsrep_max_ws_size`, `wsrep_mode`, `wsrep_certification_rules`, `wsrep_sst_donor`.
  В `mariadbGaleraLines()` (`cfg/mariadb.go:~543`) эмитятся только `provider, cluster_name, cluster_address, node_address, node_name, sst_method, sst_auth, slave_threads, sync_wait`.
- `skip_name_resolve`, `binlog_commit_wait_*`, `slave_parallel_max_queued`, `max_binlog_size`, `binlog_cache_size`.
- В ProxySQL: `mysql-monitor_galera_healthcheck_interval/timeout/max_timeout_count`, `mysql-init_connect`, `mysql-max_transaction_time`, `mysql-connection_max_age_ms`, `mysql-free_connections_pct`, `mysql-connection_delay_multiplex_ms`, `mysql-multiplexing` (поля нет в `cfg/proxysql.go`).
- В MaxScale: `connection_keepalive`, `connection_init_sql_file`, `disable_master_failback`, `root_node_as_master`, `available_when_donor`, `lazy_connect` и т.п.
- Профилей по классам нагрузки нет: `buffer_pool` — единственный режим автоподбора.

---

## 2. План профиля `auto`

Предложение: расширить `innodbMemory` в общий `mariadbAuto(machine, plan)`, который знает `CPU`, `MemoryGB`, топологию, и (желательно) ожидаемое число VU. Явные значения пользователя в `configs` по-прежнему побеждают (`explicitConfig`). Обозначения: `R` = RAM, ГБ; `C` = vCPU; `BP` = размер buffer pool в МБ; `VU` = число виртуальных пользователей (если неизвестно, 128).
Для эталона (8 vCPU / 32 ГБ): `R=32`, `C=8`, `BP=22937`.

### 2.1 Память

| Параметр | Формула | Эталон | Обоснование | Метка / версии |
|---|---|---|---|---|
| `innodb_buffer_pool_size` | `floor(R*1024*0.70)` МБ (оставить как есть); при `R>=64` можно 0.75 | 22937M | Оставить. «До ~80% RAM на выделенном сервере» (MariaDB). Остаток нужен под соединения, tmp-таблицы, page cache бинлога, Docker. `innodb_buffer_pool_chunk_size`/instances не нужны: `innodb_buffer_pool_instances` убран в 10.6 (см. комментарий в `cfg/mariadb.go:25`). | [офиц] default 128 МБ. Для 100 складов (~10 ГБ данных + индексы) пул 22 ГБ достаточен; смысл «auto» в том, чтобы не оставаться на 128 МБ |
| `innodb_buffer_pool_size_max` | не задавать | — | В свежих релизах (10.11.17, 11.4.11, 11.8.7) по умолчанию 8 ТиБ (раньше = стартовый размер). Влияет на резервирование виртуальной памяти, не на RSS. | [офиц], страница InnoDB system variables |
| `tmp_table_size` / `max_heap_table_size` | оставить 16M | — | TPC-C не создаёт больших tmp-таблиц. | [практика] |
| `table_open_cache`, `table_definition_cache` | оставить 2000/400 | — | 9 таблиц TPC-C. Менять только для множества схем. | [практика] |
| `performance_schema` | `OFF` (как сейчас) | OFF | Дефолт MariaDB OFF, на бенче нужен минимум накладных расходов. Для диагностики включать отдельным флагом. | [офиц]/[практика] |
| `innodb_adaptive_hash_index` | не трогать (OFF default) | OFF | В MariaDB он OFF по умолчанию; документация: «помогает в почти read-only нагрузках». На TPC-C смысла включать нет. | [офиц] |
| Docker `memory` | без лимита (как сейчас) | — | Лимит cgroup без учёта `BP`+`gcache`+соединений даст OOM-kill вместо внятной ошибки. | [практика] |

### 2.2 Redo / binlog

| Параметр | Формула | Эталон | Обоснование | Метка / версии |
|---|---|---|---|---|
| `innodb_log_file_size` | `clamp(roundup(BP/4, 1G), 2G, 16G)` | **6G** вместо 96M | 96 МБ при 128 VU TPC-C — ключевая подозрительная точка: пул 22 ГБ, а журнал рассчитан на сотни МБ, значит checkpoint (flush грязных страниц) идёт по давлению журнала, а не по давлению пула. MariaDB описывает именно этот режим: «workload can be bound by innodb_log_file_size (must advance the checkpoint before the tail overwrites)», и в MDEV-35155 исправляли избыточную запись при малом журнале. Документация разрешает ставить журнал даже больше пула; ценой становится только дольше crash recovery. Полезный объём = (size − 12288) × 0.9. Динамически меняется с 10.9, на 10.11/11.x ставим в cnf до первого старта (в контейнере datadir пустой, журнал создаётся сразу нужного размера). | [офиц] default 96M, динамика с 10.9; формула размера [практика]. Проверка: `Innodb_checkpoint_age` / `Log sequence number − Last checkpoint at` в `SHOW ENGINE INNODB STATUS` должна стабильно быть ниже ~75% полезного объёма |
| `innodb_log_buffer_size` | `64M` при `VU>=64`, иначе 16M | 64M | 16M хватает почти всегда; 64M безвреден и убирает `Innodb_log_waits` при всплесках. Максимум ограничен 2 ГБ−4К с 10.11.8. Не динамический. | [практика]; ограничение [офиц] |
| `innodb_flush_log_at_trx_commit` | `1` на всех топологиях | 1 | Держим fsync на коммит ради честного сравнения с MySQL/Postgres (п.3). Вариант «relaxed=2» оформлять отдельным явным пресетом с пометкой в UI. Документация Galera допускает 0/2 ради скорости ценой потери подтверждённых транзакций при одновременной потере питания всех узлов. | [офиц] |
| `sync_binlog` | `1`, если `log_bin` включён и топология не Galera | 1 | Паритет с MySQL (default 1) и с Postgres `synchronous_commit=on`. Групповой коммит амортизирует fsync. Если пользователь хочет «быстро», он ставит 0 явно. | [офиц] default 0 в MariaDB; выбор 1 [решение продукта] |
| `log_bin` | single без реплик: оставить включённым (паритет с MySQL, где binlog включён). Galera: выключить (нужно разрешить «выключено» в схеме: сейчас `Str` `MinLen(1)`, `Nullable()`+условный рендер). async/semi_sync: включён обязательно | — | Для Galera репликация идёт через write-set, binlog не нужен и стоит записи на каждый коммит. Но если Galera нужна источником для async-реплик, включают `log_bin`+`log_slave_updates` (в документации Galera именно это). Платформа таких потребителей не создаёт. Параметр `wsrep_gtid_mode` оставить OFF. | [офиц] Galera-страница про `log_slave_updates`; решение [практика] |
| `binlog_format` | `ROW` | ROW | Для Galera обязателен. Для single/async: у MariaDB дефолт `MIXED` **[офиц]**, у MySQL 8 — `ROW`. Для паритета ставить `ROW` везде. | [офиц]/[решение] |
| `binlog_row_image` | `FULL` | FULL | Как в MySQL. | [офиц] |
| `binlog_expire_logs_seconds` | `86400` (1 сут.) | 86400 | Не копить десятки ГБ при длинных прогонах. По документации 0 = не удалять. | [офиц] |
| `max_binlog_size` | не трогать (1G) | — | — | [офиц] |
| `binlog_commit_wait_count/usec` | не трогать (0) | — | Нужно только для conservative parallel replication. | [офиц] |
| `innodb_doublewrite` | оставить `ON` | ON | Документация Galera: «should not be changed». В 11.x есть `fast`, но это не наш режим по умолчанию. На сетевом SSD YC torn-page защита нужна для паритета с Postgres `full_page_writes=on` и MySQL doublewrite. | [офиц] |

### 2.3 IO и потоки

| Параметр | Формула | Эталон | Обоснование | Метка / версии |
|---|---|---|---|---|
| `innodb_io_capacity` / `_max` | сейчас 10000/20000. Предложение: `io_capacity = min(0.5 × disk_iops, 10000)`, `io_capacity_max = 2 × io_capacity`, где `disk_iops` берём из каталога провайдера по `DiskType` и размеру | 10000/20000 (если у диска ≥20k IOPS) | Дефолт MariaDB 200 / max 2000 **[офиц]** рассчитан на HDD. Для network-ssd YC лимит IOPS зависит от размера диска (число аллокационных блоков), поэтому цифра 10000 может превышать реальный лимит диска. Тогда фоновый flush просто упрётся в квоту, а не ускорится. В каталоге (`internal/domain/catalog/providers.go:30-32`) IOPS не хранятся. Точные квоты YC **[проверить]** по документации Yandex Cloud (я не использовал `yc`). Нужно добавить поле `iops` по типу диска. | [офиц] defaults; формула [практика] |
| `innodb_read_io_threads`, `innodb_write_io_threads` | `clamp(C/2, 4, 16)` | 4/4 | Дефолт 4/4. Для 8 vCPU совпадает. Масштабировать для ≥16 vCPU. Динамические с 10.11 **[офиц]** для read. | [практика] |
| `innodb_use_native_aio` / io_uring | Не задавать в cnf; лечить причину: дать контейнеру право на `io_uring_*` и достаточный `memlock` | — | Если `io_uring_queue_init()` не удался, InnoDB пишет в лог «falling back to innodb_use_native_aio=OFF» и работает на симулированном AIO (в потоках). Причины: seccomp Docker (ENOSYS/EPERM), малый `ulimit -l` (ENOMEM, нужно 262144 байт и больше), ядро <5.16/5.15.3 (запись через io_uring отключается). Это реальный источник деградации на 10.11/11.x в контейнере. Проверка: `grep -i 'io_uring\|native_aio' docker logs`, `SHOW VARIABLES LIKE 'innodb_use_native_aio'`. | [практика]: mariadb-docker #454, MDEV-36234, podman #16796; ядро [офиц-коммит]. Нужно **[проверить]** на стенде: версия Docker и ядра ВМ |
| `innodb_log_file_buffering` / `innodb_flush_method` | оставить O_DIRECT-эквивалент: 10.11 `O_DIRECT`; 11.x `log_file_buffering=OFF`, `data_file_buffering=OFF` | OFF | Это и есть дефолт; на лог-файле O_DIRECT нормален. Автоматически станет ON при `flush_log_at_trx_commit=2`. | [офиц] |
| `innodb_flush_neighbors` | не трогать | — | Для SSD игнорируется («tablespaces detected as SSD behave as 0»). | [офиц] |
| `innodb_purge_threads`, `innodb_max_dirty_pages_pct(_lwm)` | не трогать | — | Дефолты MariaDB приемлемы. Менять только по данным (`Innodb_buffer_pool_pages_dirty`, history list length). | [практика] |
| ОС: файловая система | XFS (или ext4) с `noatime` | — | Схема `cfg.host.disks` уже умеет, но не подключена к MariaDB (B7). | [практика] |

### 2.4 Thread pool и соединения

- **`max_connections`**: `clamp(ceil(1.5 × VU) + 64, 300, 2000)`; при неизвестном `VU` брать 1000. Для эталона 128 VU: 256, поднимаем до 300. Дефолт 151 не оставляет запаса (B8). Память на соединение невелика при thread-per-connection (стек + лениво выделяемые буферы). **[практика]**
- **`thread_cache_size`**: 256 уже. Под pool-of-threads игнорируется, `Threads_cached=0` **[офиц]**.
- **`skip_name_resolve = ON`**: добавить. Соединения идут по IP (host-network), обратные DNS-запросы бессмысленны, а `root@%` не завязан на имена. **[практика]**
- **`back_log`**: auto-значение `min(900, 50 + max_connections/5)` **[офиц]** устраивает.
- **`thread_handling = pool-of-threads` (оценка для 128 VU на 8 vCPU)**:
  - **[офиц]** Пул лучше всего на коротких запросах и CPU-bound нагрузке (OLTP), уменьшает переключения контекста и contention. Хуже на «всплесках после простоя», на множестве длинных непрерывных запросов (DWH) и на latency простейших запросов под нагрузкой (`SELECT 1` может ждать в очереди). `thread_pool_size` по умолчанию = число CPU, `stall_limit` 500 мс, `oversubscribe` 3 («не менять»), `max_threads` 65536, `idle_timeout` 60 с, `priority=auto`: запросы внутри транзакции получают приоритет, что сокращает число одновременных транзакций.
  - **[практика, из общей литературы]** Выигрыш обычно проявляется при числе активных соединений в несколько сотен и больше. 128 VU на 8 vCPU (16 на ядро) — пограничный случай. Для TPC-C важны ожидания на row-lock: InnoDB сообщает пулу о блокировке (stall detection), так что пул не «зависает», но каждое такое ожидание может вызвать создание дополнительного потока через stall_limit (до 500 мс).
  - **Рекомендация**: не включать по умолчанию. Добавить в схему `thread_handling`, `thread_pool_size`, `thread_pool_max_threads`, `thread_pool_stall_limit`, `extra_port`, `extra_max_connections`. В матрице устроить A/B `one-thread-per-connection` vs `pool-of-threads` (size = C). Если включено, обязательно задавать `extra_port=3307`, `extra_max_connections=8`, и направлять туда `mysqld-exporter` и MaxScale-монитор (документация именно так объясняет назначение extra_port). **[гипотеза]**, что на 128 VU разница ≤10%; решение только по данным.
  - Совместимость с Galera: документальных запретов не нашёл **[проверить]** (аппликаторы wsrep — системные потоки вне пула).
  - Бенчмарк-гигиена из документации: драйвер нагрузки должен быть на отдельных CPU. Уже так (runner — отдельная ВМ). **[офиц]**

### 2.5 Репликация (async / semi_sync)

| Параметр | Формула | Обоснование | Метка |
|---|---|---|---|
| Включение semi-sync на MariaDB | `rpl_semi_sync_master_enabled=ON` на primary, `rpl_semi_sync_slave_enabled=ON` на репликах, когда `replication=semi_sync` | Сейчас не включается вообще (B2). Встроено в сервер с 10.3.3, `plugin_load` не нужен. | [офиц] |
| `rpl_semi_sync_master_wait_point` | `AFTER_SYNC` | Подтверждение реплики до коммита в движке, исключает «фантомное чтение» и потерю закоммиченного при падении primary; ближе к Postgres `synchronous_commit=on`+синхронная реплика. Сейчас в схеме дефолт `AFTER_COMMIT`. | [практика]/[офиц, опция] |
| `rpl_semi_sync_master_timeout` | оставить 10000 мс, но выставлять предупреждение в отчёте | При таймауте сервер молча переходит в async; результат прогона перестаёт означать «semi-sync». Мониторить `Rpl_semi_sync_master_status` / `Rpl_semi_sync_master_no_tx`. | [офиц] default 10000 |
| `slave_parallel_threads` | `C` (но ≤16) на репликах | Дефолт 0 (один applier); при 128 VU на primary реплика отстанет. `slave_parallel_mode` default `optimistic` с 10.5.1; потоки менять только при остановленных slave. `slave_parallel_max_queued` default 131072 на worker. | [офиц] defaults; значение [практика] |
| `gtid_strict_mode` | `ON` | Ловит расхождения репликации (в MariaDB GTID всегда существуют). | [практика] |
| `read_only` на репликах | ON (уже) | — | |

### 2.6 Galera

Принцип: профиль делится на «коррекцию» (убрать то, что ломает мониторинг и корректность) и «пропускную способность» (потоки, flow control). В `cfg.mariadb.cnf` нужно добавить поле `wsrep_provider_options` либо набор типизированных полей, которые собираются в одну строку `wsrep_provider_options = "gcache.size=…; gcs.fc_limit=…; …"` (опции провайдера нельзя задавать как отдельные переменные в cnf; в блогах частая ошибка `unknown variable 'gcs.fc_limit=80'`).

| Параметр | Формула | Эталон | Обоснование | Метка / версии |
|---|---|---|---|---|
| `wsrep_slave_threads` | старт `C` (8); верхняя граница `wsrep_cert_deps_distance` (смотреть после прогона); не более 4×C | 8 | Дефолт 1. Документация MariaDB: «начните с 4 потоков на ядро, при быстром IO можно больше, но не выше `wsrep_cert_deps_distance`». Другие источники советуют C или 2×C. Один писатель через ProxySQL (`max_writers=1`) даёт стабильный поток write-set'ов, и именно на остальных двух узлах применение однопоточно становится узким местом. Имя переменной в MariaDB — `wsrep_slave_threads`; `wsrep_applier_threads` — переименование MySQL-wsrep/Percona, для MariaDB подтверждений не нашёл. | [офиц] 4/ядро; C [практика]. `wsrep_applier_threads` для MariaDB **[проверить]** |
| `innodb_autoinc_lock_mode` | `2` (уже) | 2 | Требование Galera. | [офиц] |
| `innodb_flush_log_at_trx_commit` | 1 (п.2.2) | 1 | Паритет. | |
| `gcache.size` | `min(2G, 5% диска)`; для длинной загрузки 1-2G | 2G | Дефолт 128M (предвыделяется, не динамический). Это кольцо write-set'ов для IST. Нагрузка 100 складов быстро переполнит 128M; узел, отставший на секунды, получит SST вместо IST, что на бенче превращается в провал. Размер = скорость записи write-set'ов × допустимый простой × 1.5. | [офиц] default; формула [практика] |
| `gcs.fc_limit` | `max(100, 16 × wsrep_slave_threads)` | 128 | Дефолт по документации MariaDB 16 **[офиц]**; Percona 5.7 поднял до 100; для Galera 4 у Codership значение **[проверить]** `SHOW VARIABLES LIKE 'wsrep_provider_options'`. Для одного писателя разумно увеличить. Но поднятие limit лечит симптом: если очередь растёт из-за большой DDL или отсутствия индекса, лечить надо причину. | [офиц] default; значение [практика] |
| `gcs.fc_factor` | `0.8` | 0.8 | Возобновление при очереди < fc_limit × factor. Дефолт 1.0 в таблице MariaDB (раньше 0.5). | [практика] |
| `gcs.fc_single_primary` (`fc_master_slave`) | `yes`, если `max_writers=1` / один узел принимает записи | yes | Отключает масштабирование лимита по числу узлов. `fc_master_slave` deprecated с Galera 4.10 (MariaDB 10.6.6+), заменён `fc_single_primary`. | [офиц] |
| `evs.suspect_timeout` / `evs.inactive_timeout` / `evs.install_timeout` | `PT10S` / `PT30S` / `PT15S` | — | Дефолты PT5S/PT15S/PT7.5S. Сетевой диск YC под нагрузкой и fsync-столлы на ВМ с ограниченным CPU могут давать ложные исключения узла из кластера (`NON-PRIMARY`, кластер в «Primary Component» потерян). Инвариант: `suspect ≤ inactive`, `install ≤ inactive`. | дефолты [офиц]; значения [практика/гипотеза] |
| `wsrep_sync_wait` | **0 глобально**; 1 только на клиентских сессиях, где нужен read-your-writes | 0 | См. B3. Документация: «health-check соединения должны использовать 0»; при потере Primary Component чтения блокируются. При TOI-DDL запрос монитора `SELECT @@…` с sync_wait=1 может ждать применения очереди. Реализация «1 только для клиентов»: ProxySQL `mysql-init_connect="SET wsrep_sync_wait=1"` (выполняется на клиентских backend-соединениях, но не в мониторе) **[проверить]** точное поведение для вашей версии ProxySQL; для MaxScale — `connection_init_sql_file` **[проверить]**. Либо вообще отказаться от чтения с других узлов (ниже). | [офиц] про мониторы |
| Маршрутизация чтений в Galera | **Вариант A (по умолчанию для бенча)**: все запросы на writer: `read_write_split=false`, `writer_is_also_reader=0`. **Вариант B**: чтения на остальные узлы + `wsrep_sync_wait=1` для клиентов | A | TPC-C — write-heavy, чтения в основном внутри транзакций (`transaction_persistent=1`). Вариант B даёт ожидание применения на каждом SELECT вне транзакции и усложняет корректность; для первого цикла выгоднее A, B — отдельный эксперимент. Сейчас: `writer_is_also_reader=2` (читают только backup writers, `library/service.go:146-149`) + `wsrep_sync_wait=1`. | [практика] |
| `wsrep_OSU_method` | `TOI` (дефолт) | TOI | При TOI DDL блокирует запись на всём кластере, а незафиксированные транзакции получают откат и `ERROR 1213` **[офиц]**. RSU (на каждом узле по очереди) разумен только для заранее известной миграции; в этой платформе автоматика не нужна. NBO только Enterprise. | [офиц] |
| `wsrep_max_ws_size` | 2G (дефолт) | — | Границу не менять; крупные загрузки данных разбить на батчи. | [офиц] |
| `wsrep_retry_autocommit` | 1 (дефолт); поднять до 3 | 3 | Для autocommit-запросов, конфликтующих при сертификации. TPC-C с явными транзакциями не затрагивает. | [офиц] |
| `wsrep_certification_rules` | `strict` (дефолт) | — | `optimized` меньше сертификационных отказов с FK, но менее проверено. Только по данным. | [офиц] |
| `wsrep_sst_method` | `mariabackup` (уже) | — | Неблокирующий для донора в отличие от rsync. | [офиц] |
| `pc.recovery` | true (дефолт) | — | Автовосстановление после падения всего кластера. Платформа сознательно не делает авто-bootstrap (`mariadb-galera-start.sh:27-31`). | [офиц] |

#### Вероятные причины `invalid connection` на `create_indexes` (Galera 11.4 + ProxySQL)

`invalid connection` — сообщение `go-sql-driver/mysql` (`ErrInvalidConn`): сокет закрыт или протокол рассинхронизирован. Значит, кто-то разорвал TCP, либо драйвер сам закрыл соединение по таймауту чтения. На `CREATE INDEX` по большим таблицам (в TPC-C `order_line`, `stock`, `customer`) работают сразу несколько факторов Galera.

Что известно **[офиц]**:
- DDL в режиме TOI блокирует запись на всех узлах; пока идёт DDL, узлы не применяют другие write-set'ы; активные незафиксированные транзакции откатываются (`1213`).
- Монитор ProxySQL для Galera проверяет `wsrep_local_state`, `wsrep_local_recv_queue`, `read_only`, `wsrep_desync`, `wsrep_reject_queries`, `wsrep_cluster_status`; узел с очередью выше `max_transactions_behind` становится SHUNNED. Дефолты: `mysql-monitor_galera_healthcheck_interval=1000`, `_timeout=600`, `_max_timeout_count=3`.
- SHUNNED сохраняет текущие соединения, а перенос в offline/hard (и смена writer) их убивает; смена writer при failover произойдёт тоже.
- `wsrep_sync_wait` биты: 1 = чтения, 2 = UPDATE/DELETE, 4 = INSERT/REPLACE, 8 = SHOW.

Гипотезы по убыванию вероятности (все **[гипотеза]**, источники фактов выше):

1. **Монитор заблокирован `wsrep_sync_wait=1` во время TOI (B3).** Пока узел применяет/ждёт DDL, запросы монитора с глобальным sync_wait=1 стоят и упираются в 600 мс таймаут; три таймаута подряд (1.8 с) и узел выводится из писательской/читательской группы, соединения убиваются/переносятся. Предыдущее «исправление» (`max_transactions_behind=0 → 100`) закрывало лишь другой вариант той же болезни (очередь >0 при любом отставании).
   Проверка: на ProxySQL admin (6032): `SELECT * FROM monitor.mysql_server_galera_log ORDER BY time_start_us DESC LIMIT 50;`, `SELECT * FROM runtime_mysql_servers;`, `stats.stats_mysql_connection_pool`; в errorlog ProxySQL искать `Shunning server` / `Galera` / `timeout`. На узле: `SHOW PROCESSLIST` (состояния «Waiting for TOI DDL», «wsrep: ...»), `SHOW GLOBAL STATUS LIKE 'wsrep_local_recv_queue%'`, `wsrep_flow_control_paused`. Эксперимент: поставить `SET GLOBAL wsrep_sync_wait=0` до `create_indexes` и повторить; если проходит, причина найдена.
2. **Клиентский таймаут чтения у драйвера.** Если DSN stroppy задаёт `readTimeout`/`timeout`, то один `CREATE INDEX` на несколько минут (тем более в TOI под блокировкой кластера) вызовет `invalid connection` на стороне клиента, хотя сервер цел. Проверить DSN в рендере воркфлоу (`compile.go:294` `workload`) и параметры драйвера stroppy. Сравнить: тот же шаг напрямую на узел 1, минуя ProxySQL, и тот же шаг через MaxScale.
3. **Ложное исключение узла из кластера** (evs.suspect/inactive 5/15 с), когда DDL и build индекса нагружают диск/CPU. Признаки: в `error.log` `Received NON-PRIMARY`, `suspecting node`, `wsrep_cluster_status != Primary`, падение `wsrep_ready`. Проверка и меры в таблице выше (evs.*).
4. **Flow control и отставание**: во время TOI-DDL очередь применения растёт, `wsrep_flow_control_paused` близок к 1, а `max_transactions_behind=100` для узла-читателя недостаточен. Если все читатели шунтированы, SELECT без доступных серверов (`Hostgroup 20 has no servers available`; на это указывает комментарий в `cfg/proxysql.go`). Мера: A-вариант маршрутизации, `writer_is_also_reader=1`, повышение `max_transactions_behind` и таймаутов монитора (`healthcheck_timeout` 3000, `max_timeout_count` 10). Поля нужно добавить в схему ProxySQL.
5. **Параллельные `CREATE INDEX` из нескольких соединений stroppy**: TOI сериализует их кластерно; незавершённые транзакции соседей откатываются с 1213, а ожидающие в очереди TOI соединения висят до завершения предыдущих. Проверить, как stroppy запускает шаг (последовательно или параллельно).
6. **ProxySQL на 2 vCPU** (B5): `threads=4`, 128 соединений, мониторинг; под CPU-голоданием `ping_timeout_server=500ms` (`cfg/proxysql.go`) может срабатывать на пулёных соединениях. Менее вероятно во время DDL, когда клиентская активность низкая.
7. **Несоответствие `mysql-server_version=8.0.36` и MariaDB** (драйвер выбирает протокольные возможности по версии; в схеме это поле явно предлагают ставить в реальную версию, `cfg/proxysql.go`). Для MariaDB 11.4 поставить `11.4.x-MariaDB`. Низкая вероятность, но дёшево исправить.

Меры, которые снимают почти все гипотезы разом: `wsrep_sync_wait=0` глобально, `init_connect` для клиентов, повышенные таймауты монитора, A-вариант маршрутизации на время подготовки данных, `evs.*` расширенные, `gcache.size` 2G.

### 2.7 MaxScale 24.02 и ProxySQL

**MaxScale (`cfg/maxscale.go`, `compile/recipe_maxscale.go`)**

| Параметр | Предложение | Обоснование | Метка |
|---|---|---|---|
| `causal_reads` | Galera: `none` (дефолт MaxScale). async: `none` или `fast` | `global` с Galera не работает (B4); `fast*` с Galera только с 24.02.5, но «fast» отправляет почти все чтения на primary при записи-heavy нагрузке. Для консистентности использовать `wsrep_sync_wait` на бэкенде (см. 2.6). Нужно убедиться, что образ `mariadb/maxscale:24.02.x` ≥ 24.02.5. | [офиц] |
| `max_slave_connections` | Galera вариант A: `0` | 0 направляет все чтения на primary, сохраняя `master_reconnection`/replay. Дефолт 255 (диапазон 0-255). | [офиц] |
| `master_accept_reads` | `true` для варианта A | — | [практика] |
| `transaction_replay` | `false` для бенча (как сейчас) | Replay нужен только для HA-экспериментов; включение форсирует `delayed_retry`, `master_reconnection`, `fail_on_write`. | [офиц] |
| `monitor_interval` | 2000 мс (дефолт) | `galeramon`: `disable_master_failback`, `root_node_as_master` не заданы; мастер выбирается по `wsrep_local_index` / priority. | [офиц] |
| `threads` | `auto` | Прокси на 2 vCPU. Поднимать размер прокси-ВМ, если станет узким местом. | |
| Монитор и sync_wait | `connection_init_sql_file` на сервисе для клиентов; монитор остаётся с 0 | **[проверить]** применимость параметра в 24.02. | |
| Лицензия | BSL, бесплатно <3 серверов | Уже описано в схеме. | [офиц] |

**ProxySQL (`cfg/proxysql.go`)**

| Параметр | Предложение | Обоснование | Метка |
|---|---|---|---|
| `mysql-threads` | `= прокси.vCPU` (сейчас 4) | При 2 vCPU 4 потока излишни; при 8 нужно 8. | [практика] |
| `mysql_galera_hostgroups.max_transactions_behind` | `100` (сейчас). Для бенча-варианта A не критично; для варианта B поднять до 1000+ | Очередь `wsrep_local_recv_queue` сравнивается с порогом; 0 = шунтить при любом отставании. | [офиц]/[практика] |
| `writer_is_also_reader` | A: 0 (+ `read_write_split=false`); B: 1 (а не 2) | При 2 читают только backup writers; если все шунтированы, чтения не имеют сервера. Дефолт в документации 0. | [офиц] |
| `mysql-monitor_galera_healthcheck_timeout` / `_max_timeout_count` | 3000 / 10 | Дефолт 600 мс и 3 слишком жёстко для узла под DDL. Добавить в схему. | [офиц] defaults; значение [практика] |
| `mysql-init_connect` | `SET wsrep_sync_wait=1` (если нужна гарантия чтения) | См. 2.6. | [проверить] |
| `mysql_servers.max_connections` | 1000 (дефолт в схеме) достаточно | — | |
| Мультиплексирование | оставить включённым, `transaction_persistent=1` | — | [офиц] |
| `mysql-server_version` | реальная версия MariaDB | B5, гипотеза 7. | |
| `mysql-default_query_timeout` | 36000000 мс = 10 ч (уже) | Достаточно для долгих DDL. | |

### 2.8 Хост ОС и Docker

| Параметр | Значение | Обоснование | Метка |
|---|---|---|---|
| Docker `security_opt` | профиль seccomp с `io_uring_setup/enter/register` или `seccomp=unconfined` только для контейнера БД | Нужно новое поле контейнера (п.4). Без него io_uring недоступен (B6). | [практика] |
| Docker `ulimit memlock` | `-1` (или ≥ 8 МиБ; сообщение требует ≥262144 байт) | `io_uring` блокирует память. | [практика] |
| Docker `ulimit nofile` | ≥ 1048576 | Должно покрывать `open_files_limit=32768` и сокеты Galera. Проверить `docker info`/`ulimit -n` в контейнере. | [практика] |
| `vm.swappiness` | 1 | Уже в `cfg.host.sysctl` по умолчанию, но не применяется для mariadb-роли (B7). | [практика] |
| THP | `never` | Уже в `cfg.host.sysctl`, не применяется. | [практика] |
| `fs.aio-max-nr` | 1048576 | Дефолт ядра 65536; нужно при native AIO (libaio). | [офиц, kernel docs] |
| `vm.dirty_background_ratio`/`vm.dirty_ratio` | 5 / 15 (или меньше) | С O_DIRECT для данных влияет мало; влияет на бинлог (через page cache) и gcache. | [практика] |
| Монтирование | `noatime`, XFS либо ext4 | B7. | [практика] |
| Планировщик / readahead | `none` / 128 КБ для virtio-blk | Мелочь. | [практика] |
| Сеть | `net.core.somaxconn` 4096+, keepalive 300 с | В `cfg.host.sysctl` есть; не применяется. | [офиц, kernel docs] |
| Порты Galera | 3306, 4567, 4568, 4444 | Уже в `recipe_mysql_cluster.go:~126`. Для IST нужен 4568 (`ist.recv_addr`). | [офиц] |
| `--cpuset`/NUMA | не задавать | — | |

---

## 3. Надёжность: паритет fsync-на-коммит с MySQL / Postgres

Цель: «подтверждённая транзакция переживает потерю питания/ядра одного узла». Сравнение трёх профилей:

| Движок | Что делает коммит | Как сейчас (матрица) |
|---|---|---|
| Postgres (`cfg/postgresql_conf.go:190-198`) | `fsync=on`, `synchronous_commit=on`: WAL fsync на коммит (с групповым коммитом) | паритет-эталон |
| MySQL 8 (`cfg/mysql.go`) | `innodb_flush_log_at_trx_commit=1` + `sync_binlog=1` (default 1) | durable |
| MariaDB single (сейчас) | `innodb_flush_log_at_trx_commit=1`, `log_bin=on`, **`sync_binlog=0`** | redo fsync есть, binlog без fsync |

Что происходит при потере питания у MariaDB с `log_bin=on`, `flush=1`, `sync_binlog=0` **[гипотеза по механизму 2PC binlog/InnoDB; документация не описывает этот случай напрямую]**: коммит идёт через внутренний XA между binlog и InnoDB. InnoDB-prepare fsync-ится, binlog — нет. При восстановлении транзакции, подготовленные в InnoDB, но не найденные в binlog, откатываются. То есть подтверждённый клиенту коммит может пропасть, хотя `innodb_flush_log_at_trx_commit=1`. Из этого же следует, что MariaDB в матрице платит меньше, чем MySQL и Postgres, и сравнение tps нечестное в пользу MariaDB. Проверить прямым опытом (kill -9 контейнера не покажет проблему, нужен сброс ВМ).

Вывод и варианты:
1. **Паритет-профиль (по умолчанию в `auto`)**: single/async/semi_sync: `flush=1`, `sync_binlog=1`, `log_bin` включён. Дороже на 1 fsync на группу коммитов, групповой коммит смягчает. Это эквивалент MySQL по умолчанию.
2. **Galera**: `flush=1`; `log_bin` выключить (нечему потреблять), либо оставить и `sync_binlog=1`. Durability кластера опирается на то, что write-set сертифицирован на всех узлах до коммита; локальный fsync нужен для рестарта одного узла. Для честного сравнения с «Patroni + sync-реплика» или GR отметить в отчёте, что у Galera другая модель. Если хотят строго «fsync на каждом узле», это как раз `flush=1`.
3. **Relaxed-профиль (явный opt-in, не в `auto`)**: `flush=2`, `sync_binlog=0` или `N`; в UI/отчёте пометка «durability понижена».
4. Всегда выводить в отчёт run'а эффективные `flush_log_at_trx_commit`, `sync_binlog`, `log_bin`, `innodb_doublewrite`, `rpl_semi_sync_*`, чтобы сравнение между движками показывало разницу durability.
5. MariaDB 12.3+ имеет InnoDB-based binlog, при котором `sync_binlog` игнорируется и durability задаёт только `innodb_flush_log_at_trx_commit` **[офиц]**; в LTS 10.11/11.4/11.8 этого нет.

Дополнительно: semi-sync на MariaDB (B2) нужно сначала включить, иначе «semi_sync» равен async по гарантиям.

---

## 4. Изменения платформы (с путями)

1. `internal/domain/compile/innodb.go`: превратить `innodbMemory` в `mariadbAuto`. Новые входы: `CPU`, `MemoryGB`, топология (single/async/semi_sync/galera), ожидаемые VU. Выдаёт оверлей: `innodb_log_file_size`, `innodb_log_buffer_size`, `read/write_io_threads`, `max_connections`, `skip_name_resolve`, `sync_binlog`, `binlog_format`, `binlog_expire_logs_seconds`, Galera-блок (п.2.6). Сохранять правило «явные значения побеждают» (`explicitConfig`). Добавить режим `buffer_pool: auto` → отдельное поле профиля `tuning: auto|stock` (в `pipelines/schemas/dbparams/mariadb.go`), чтобы профиль было можно выключить независимо от размера пула.
2. `pipelines/schemas/cfg/mariadb.go`:
   - новые поля: `thread_handling`, `thread_pool_size`, `thread_pool_max_threads`, `thread_pool_stall_limit`, `extra_port`, `extra_max_connections`, `skip_name_resolve`, `innodb_use_native_aio`, `max_binlog_size`, `wsrep_provider_options` (структурно: `gcache_size`, `fc_limit`, `fc_factor`, `fc_single_primary`, `evs_suspect_timeout`, `evs_inactive_timeout`, `evs_install_timeout`), `wsrep_retry_autocommit`, `wsrep_OSU_method`;
   - `log_bin` — разрешить «выключено» (`Nullable` + условный рендер в шаблоне, `:531`);
   - `sync_binlog` — оставить поле, но менять дефолт в профиле, а не в схеме (по правилу «дефолты как upstream»); в `Desc` зафиксировать, что MariaDB default 0, а профиль ставит 1;
   - `innodb_log_file_buffering` показать и для 10.11;
   - правила: `evs.suspect ≤ inactive`, `fc_limit ≥ 1`, `thread_pool_size ≥ 1`;
   - `mariadbGaleraLines()` (`:~543`) расширить эмиссией `wsrep_provider_options`.
3. `internal/domain/compile/recipe_mysql.go:49-61`: для MariaDB `semi_sync` проставлять `rpl_semi_sync_master_enabled/slave_enabled` и `wait_point=AFTER_SYNC` (B2); добавить тест рядом с `mysql_test.go`.
4. `internal/domain/library/service.go:143-149`: убрать глобальный `wsrep_sync_wait=1` из сида Galera (или перенести в клиентский init_connect), `writer_is_also_reader` по варианту A/B.
5. `pipelines/schemas/cfg/proxysql.go`: поля `monitor_galera_healthcheck_interval/timeout/max_timeout_count`, `init_connect`, `connection_max_age_ms`, `free_connections_pct`; `threads` по умолчанию из vCPU прокси (задавать в компиляторе в `recipe_mysql.go:~230`); `server_version` из версии MariaDB.
6. `pipelines/schemas/cfg/maxscale.go`: `causal_reads` default `none` для `galeramon`; поля `connection_keepalive`, `connection_init_sql_file`, `disable_master_failback`, `root_node_as_master`; правило «causal_reads ∈ {local,global,universal} несовместимо с galeramon».
7. `pipelines/spec/spec.go:140-162` (`Container`) + `pipelines/internal/run/deploy.go:240-270`: добавить `SecurityOpt []string` (и/или `Seccomp`), пробросить в `container.HostConfig.SecurityOpt`; для БД-контейнера MariaDB выставлять `Ulimits{"memlock": -1, "nofile": 1048576}` в `recipe_mysql*.go`. Это закрывает B6.
8. `internal/domain/dbkind/mysql.go:49-51`: добавить в `ConfigSchemas` MariaDB-ролей `cfg.host.sysctl@1` и `cfg.host.disks@1` (как у YDB, `dbkind/ydb.go:25`), с xfs/`noatime`. Плюс `pipelines/spec/mount-data.sh`/`diskprep.go`: поддержать `noatime` для YC.
9. `internal/domain/catalog/providers.go:30-32`: добавить `IOPS`/пропускную способность по `DiskType` и размеру, использовать для `innodb_io_capacity`.
10. `internal/domain/compile/mariadb-galera-start.sh`: без изменений по тюнингу; опционально печатать эффективные `wsrep_provider_options` в лог при старте.
11. Диагностика прогона (чтобы разбирать «520 tps» и `invalid connection`): собирать после прогона `SHOW GLOBAL STATUS` (`Innodb_log_waits`, `Innodb_os_log_fsyncs`, `Innodb_row_lock_time_avg`, `Threads_running`, `Innodb_buffer_pool_wait_free`, `wsrep_flow_control_paused`, `wsrep_local_recv_queue_avg`, `wsrep_cert_deps_distance`, `Rpl_semi_sync_master_status`), `SHOW ENGINE INNODB STATUS` (checkpoint age), `innodb_use_native_aio`, фрагмент `mysql_server_galera_log` ProxySQL.
12. Тесты: обновить `internal/domain/compile/mysql_test.go` (проверка формул на 8 vCPU/32 ГБ), `internal/domain/compile/mysql_cluster_test.go` (Galera: нет глобального sync_wait, есть `wsrep_provider_options`), golden-файлы схем (`make schemas-export`), словари: если появятся новые enum'ы (`tuning`), синхронно контракт и локали по правилу «Словари» из `AGENTS.md`.

---

## 5. Риски и открытые вопросы

1. **Ни одна цифра профиля не проверена прогоном.** Особенно `innodb_log_file_size` и `wsrep_slave_threads`: влияние нужно мерить A/B на той же матрице (одна переменная за раз).
2. **Что реально ограничивает 520 tps на single, не установлено.** Кандидаты: маленький redo (96M), отключённый native AIO в контейнере, конкуренция за строки на 100 складах и 128 VU (в TPC-C часто главный предел), `sync_binlog`/fsync на сетевом диске, потолок IOPS диска YC. Нужны `SHOW GLOBAL STATUS` и график checkpoint age до выбора формул.
3. **Лимит IOPS диска YC** не проверен (без `yc`; нужна ссылка на официальные квоты Yandex Cloud). От него зависит `innodb_io_capacity`.
4. **`gcs.fc_limit`**: дефолт 16 по документации MariaDB, возможно 100 в билде Galera 4; проверить `SHOW VARIABLES LIKE 'wsrep_provider_options'` на стенде.
5. **`wsrep_applier_threads` в MariaDB** подтверждения не нашёл; использовать `wsrep_slave_threads`.
6. **Поведение ProxySQL `mysql-init_connect` и MaxScale `connection_init_sql_file`** для Galera требует проверки на версиях, которые ставит платформа (ProxySQL 2.6/2.7, MaxScale 24.02.x).
7. **Thread pool на 128 VU**: решение только после A/B; возможен выигрыш только при большем числе соединений. Риск: stall-эффекты на lock-waits TPC-C.
8. **`sync_binlog=1`** снижает tps single относительно сегодняшних чисел; это ожидаемая цена честности, но результаты старой матрицы с ним несопоставимы (нужна пометка).
9. **Отключение `log_bin` на Galera** ломает будущую возможность подцепить async-реплику; в профиле фиксировать как осознанный выбор.
10. **11.8 `innodb_snapshot_isolation=ON`**: изменение поведения конфликтов; для сравнения версий либо выставить OFF в профиле, либо считать частью различий версий и показывать в отчёте.
11. **Seccomp `unconfined` / memlock -1** уменьшают изоляцию контейнера БД. Для бенч-стенда допустимо, но стоит делать это минимальным профилем (только `io_uring_*`), а не `unconfined`.
12. **MaxScale 24.02.5+**: образ должен быть не ниже, иначе `fast*` режимы с Galera недоступны. Образ также BSL: ограничение на ≥3 сервера в production.
13. **Ложное исключение узла Galera** при расширенных `evs.*`: платой становится более долгая реакция на реальное падение узла. Для отказоустойчивых экспериментов профиль следует переключать.
14. Не изучено: влияние `innodb_flush_neighbors`, `innodb_lru_scan_depth`, `innodb_adaptive_flushing`, `binlog_cache_size`, NUMA; их менять без измерений не предлагается.

---

## Источники

Официальные (MariaDB / MaxScale / ProxySQL / Galera):
- InnoDB system variables: https://mariadb.com/docs/server/server-usage/storage-engines/innodb/innodb-system-variables
- InnoDB redo log: https://mariadb.com/docs/server/server-usage/storage-engines/innodb/innodb-redo-log
- Server system variables: https://mariadb.com/docs/server/server-management/variables-and-modes/server-system-variables
- Replication & binary log variables (sync_binlog, binlog_commit_wait_*, slave_parallel_*): https://mariadb.com/docs/server/ha-and-performance/standard-replication/replication-and-binary-log-system-variables
- Thread pool: https://mariadb.com/docs/server/ha-and-performance/optimization-and-tuning/buffers-caches-and-threads/thread-pool/thread-pool-in-mariadb
- Thread pool variables: https://mariadb.com/docs/server/ha-and-performance/optimization-and-tuning/buffers-caches-and-threads/thread-pool/thread-pool-system-status-variables
- Galera system variables: https://mariadb.com/docs/galera-cluster/reference/galera-cluster-system-variables
- wsrep_provider_options: https://mariadb.com/docs/galera-cluster/reference/wsrep-variable-details/wsrep_provider_options
- Galera configuration: https://mariadb.com/docs/galera-cluster/galera-management/configuration/configuring-mariadb-galera-cluster
- Schema upgrades (TOI/RSU/NBO): https://mariadb.com/docs/galera-cluster/galera-management/general-operations/performing-schema-upgrades-in-galera-cluster/
- MaxScale readwritesplit: https://mariadb.com/docs/maxscale/reference/maxscale-routers/maxscale-readwritesplit
- ProxySQL Galera: https://proxysql.com/documentation/galera-configuration/

Практика / сообщество:
- Percona, flow control: https://www.percona.com/blog/galera-flow-control-in-percona-xtradb-cluster-for-mysql/
- Percona, schema upgrades PXC: https://www.percona.com/blog/various-ways-to-perform-schema-upgrades-with-percona-xtradb-cluster/
- Percona docs, wsrep provider index (fc_limit 16→100): https://docs.percona.com/percona-xtradb-cluster/5.7/wsrep-provider-index.html
- ProxySQL native Galera support: https://proxysql.com/blog/proxysql-native-galera-support/
- Severalnines, Galera replication performance: https://severalnines.com/blog/how-improve-replication-performance-mysql-or-mariadb-galera-cluster/
- io_uring в Docker: https://github.com/MariaDB/mariadb-docker/issues/454, https://github.com/containers/podman/issues/16796, MDEV-36234
- Проблема `wsrep_slave_threads` (MDEV-30418): https://jira.mariadb.org/browse/MDEV-30418
