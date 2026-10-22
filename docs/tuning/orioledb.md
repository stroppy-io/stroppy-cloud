# OrioleDB: профиль тюнинга `auto` для Stroppy Cloud

Дата: 2026-10-22. Режим: только чтение репо, правок нет. Метки источников:
**[офиц]** — документация/исходники OrioleDB или PostgreSQL, **[код]** — исходник
orioledb (`src/orioledb.c`, main), **[практика]** — бенчмарк-рецепты, блоги, мой вывод;
**[?]** — не подтверждено, проверять прогоном.

Целевая машина матрицы: 8 vCPU / 32 ГБ / 200 ГБ network-ssd, TPC-C 100 складов, 128 VU, 10 мин.

---

## 1. Текущее состояние

### 1.1 Как конфиг доходит до контейнера

- Рецепт: `internal/domain/compile/recipe_postgres.go:36` `postgresFamily(c, oriole=true)`. Для каждой машины ролей `db`/`db-replica`
  вызывается `c.render(role, "cfg.orioledb.postgresql.conf@N", "conf", overlay)` (стр. ~58). `overlay` содержит ТОЛЬКО
  `{"listen_addresses":"*","port":5432}`. Больше ничего в конфиг из `params` БД не подмешивается.
- Эффективный конфиг (`EffectiveConfigs`) = дефолты схемы + deviations пользователя (`DeriveDatabase`) + сид `extensions`.
  Поэтому «ничего не задано» даёт ровно дефолты схемы — это и есть строки матрицы
  (`shared_buffers=128MB`, `orioledb.main_buffers=64MB`, `free_tree_buffers=8MB`, `catalog_buffers=8MB`, `temp_buffers=64MB`,
  `bgwriter_num_workers=1`, `max_io_concurrency=0`, `use_mmap=off`, `checkpoint_completion_ratio=0.5`,
  `effective_cache_size=4096MB`, `max_wal_size=1024MB`, `random_page_cost=4`, `effective_io_concurrency=1`).
  Источники дефолтов: `pipelines/schemas/cfg/postgresql_conf.go:103-107` (shared_buffers 128), `:~230` (max_wal_size 1024),
  `:~415` (random_page_cost 4.0, eio 1 / 16 на PG18), `orioledb_postgresql_conf.go:75-190` (orioledb.*).
- Контейнер: `recipe_postgres.go:~70-90` — `--network host` (deploy.go:`containerSpec`), без `--shm-size` (docker-дефолт `/dev/shm` = 64 МБ),
  без `memlock`, без `--memory/--cpus`, без `nofile`-ulimit (поле `spec.Container.Ulimits` есть, рецепт его не ставит).
  Образ тянется из `dbkind` каталога по `Database.Version`.
- Хост: **ни одного `HostPrep` (sysctl/THP/диск-опции) для OrioleDB и для postgres вообще нет** —
  `spec.HostPrep` собирают только `recipe_distributed.go:149,317,356` (YDB). Схема `cfg.host.sysctl@1` (`host_sysctl.go`) готова и
  умеет всё нужное (swappiness, dirty_*, nr_hugepages, THP), но для БД на Docker не подключена. Диск: `cfg.host.disks@1` по дефолту
  xfs + `noatime,nodiratime` (`host_disks.go:~45`); реально ли компилятор рендерит её для `/data` — через `spec.EnsureDiskPreparation`
  (`compile/disks.go`), я детали не раскапывал.

### 1.2 Баг: `shared_buffers_mb` не доходит до конфига

`shared_buffers_mb` (`pipelines/schemas/dbparams/orioledb.go:56-60`, default 1024, Gte 128) встречается в коде ровно в двух местах:

1. `internal/domain/dbkind/postgres.go:127-130` (`planPostgres`) — оценка минимальной памяти машины: `max(4, ceil(2*sb))` ГБ,
   причина `topology.ReasonSharedBuffers`.
2. Правило схемы `buffers-vs-replicas` (`orioledb.go:79`) — warn.

Ни `compile/`, ни рецепт его не читают; нет аналога `innodbMemory` (`compile/innodb.go` вызывается из `recipe_mysql.go:72`,
`recipe_mysql_cluster.go:78`, для Postgres/OrioleDB вызова нет). Итог: параметр — **мёртвый для конфига**, влияет только на подсказку
«нужно RAM». Причина — рецепт строили по образцу plain-postgres, где вообще нет автоподбора (stock `shared_buffers=128MB` и там тоже),
а `overlay` ограничен адресами. Сам параметр, к тому же, **по смыслу неверен для OrioleDB**: описание «shared_buffers … should dominate
the machine's RAM», тогда как для таблиц OrioleDB горячие страницы кэширует `orioledb.main_buffers`, а `shared_buffers` остаётся под heap
(системные каталоги и т.п.) — официально «оставьте 128 МБ» [офиц, configuration]. Даже если «починить» пробросом
`shared_buffers = shared_buffers_mb`, OrioleDB-таблицы остались бы на 64 МБ main_buffers — производительность хуже, чем у любого другого
движка в матрице. Плюс побочный эффект: `planPostgres` требует 2× от параметра, т.е. memory-requirement считается от неверной величины.

### 1.3 Другие найденные дефекты и пробелы

- `orioledb_postgresql_conf.go:~118` (`max_io_concurrency`): Desc «0 lets OrioleDB derive it from main_buffers». По доке и коду
  `0` = выключено (`default 0 (off)`, min 0) [офиц, код]. Описание надо исправить.
- `orioledb_postgresql_conf.go:~97` (`xid_buffers`): в схеме `Gte(1)` МБ; в коде min 128 блоков (=1 МБ), «each MB accommodates 65 thousands
  of xid items» [офиц]. Ок, но стоит дописать единицу ёмкости в Desc.
- `undo_buffers` Desc: «grows with max process count» — верно по коду: default `Max(128, 16*max_procs)` блоков по 8 КБ, min `16*max_procs` [код];
  в доке default «1 MB» — расхождение документации и кода (при max_connections=100 реально ~20 МБ). Тест
  `compile/orioledb_test.go` проверяет, что `orioledb.undo_buffers =` НЕ рендерится — при авто-профиле этот инвариант придётся заменить.
- Нет в схеме (есть в доке main): `system_undo_circular_buffer_fraction` (0.1; заменил deprecated `undo_system_buffers`),
  `regular_block_undo_circular_buffer_fraction` (0.45), `default_primary_compress`, `default_toast_compress`, `use_sparse_files`,
  `strict_mode`, `logical_xid_buffers`, `rewind_*`. Нужны минимум первые два (undo-тюнинг) и `strict_mode`.
- Нет правил: (а) `max_worker_processes` > bgwriter + recovery_pool + recovery_idx_pool [офиц: иначе тихий откат на однопроцессное
  восстановление/построение индексов]; (б) нижняя граница `undo_buffers` от `max_procs`; (в) `checkpoint_completion_ratio` vs смешанные
  таблицы.
- Дефолты схемы = `max_connections=100`, при нагрузке 128 VU [?] — проверить, какой пул у stroppy (pool size); если пул ≥ 100, часть VU
  упрётся в `too many clients`. В авто-профиле `max_connections` должен считаться от `run.vus` (+ резерв на exporter/superuser).
- Каталог vs схема: `dbparams/orioledb.go:28-34` предлагает `latest-pgNN` и `beta17-pgNN`, а `dbkind/postgres.go:~52` держит в `Versions` только
  `beta17-pgNN`. На Docker Hub уже есть **beta18** (2026-09-23) и **beta19** (2026-10-01) для pg16/17/18, а также nightly `pgNN-nightly`.
- `host_sysctl`, `docker.container` (shm_size_mb default 1024, memlock -1, ulimits) в рецепте OrioleDB не применяются:
  `cfg.docker.container@1` перечислена в `ConfigSchemas` роли, но рецепт `ct := spec.Container{...}` её значения не использует
  (в `spec.Container` вообще нет поля shm-size).

---

## 2. План профиля `auto`

### 2.1 Режимы (по образцу `innodb.go`)

Новый параметр БД `memory` (Choice, группа Memory): `auto` (дефолт) | `fixed` | `stock` — тот же смысл, что `buffer_pool` у MySQL
(`compile/innodb.go:14-35`):

- `auto` — формулы ниже; явные значения пользователя в роли (deviations `explicitConfig`) побеждают.
- `fixed` — `main_buffers_mb` и `shared_buffers_mb` берутся из params (если не заданы в конфиге роли).
- `stock` — ничего не подмешивается (как сейчас: upstream-дефолты).

Входы формул: `RAM` (ГБ→МБ, `c.memoryOf(machine)`), `vCPU` (новый `c.cpuOf(machine)` — в `spec.Machine.CPU`), `disk_gb` и `disk_type` (`Machine.Disks`),
`vus` (из сегментов workload, `c.in.Workload.Segments`), роль (`db` / `db-replica`), флаг `default_table_access_method` (есть в params).
Пример: 8 vCPU / 32 ГБ / 200 ГБ network-ssd.

### 2.2 Таблица параметров

| Параметр | Формула | Для 8/32/200 | Обоснование | Источник |
|---|---|---|---|---|
| `orioledb.main_buffers` | oriole-only (`default_table_access_method=true`): `clamp(0.30*RAM, 256, 0.45*RAM)` МБ; смешанный режим: `0.125*RAM` | **9830 МБ** (~9.6 ГБ) | Официальная отправная точка — 1/4 RAM при только OrioleDB-таблицах (8 ГБ); при поровну heap/oriole — по 1/8. 30% — запас с учётом, что TPC-C 100 skl ≈ 100 МБ/склад ≈ 10 ГБ, горячая часть меньше. Остальной ОЗУ нужно под backend'ы 128 VU, page cache (OrioleDB пишет через файлы, не O_DIRECT [?]) и хост. Потолок 45% — защита от OOM (память postmaster'а нельзя вытеснить). | [офиц] 25% / 1/8; [практика] 30% и потолок. Бенчмарк-рецепт самой Oriole: 31 ГБ из 188 (16%) при data < cache — не ориентир для 32 ГБ. |
| `shared_buffers` | oriole-only: 256 МБ; смешанный: `0.125*RAM` | **256 МБ** | Heap нужен только каталогам, `pg_stat_statements`, init-таблицам. Официально «оставьте 128 МБ»; 256 — копейки, но экономит перечитывание каталога при 128 backend'ах. | [офиц] 128 МБ; [практика] 256 |
| `orioledb.undo_buffers` | `max(floor, main_buffers/16)`, `floor = ceil(16*max_procs*8КБ)` (+10%), потолок 1 ГБ | **614 МБ** | Нет официальной формулы. Ориентиры: default в коде = `16*max_procs` блоков; CloudRaft CNPG: 8 ГБ main / 2 ГБ undo (1/4); sysbench issue #941: undo_system 128 МБ. Для коротких TPC-C-транзакций нужен малый undo, но недобор → ожидание очистки/запись undo на диск и просадка p99. Следить за `orioledb_undo_size()` (метрика уже в `orioledbQueries`). Пол из `max_procs` обязателен: ниже min GUC postmaster не стартует. | [код] min/default; [практика] 1/16 |
| `max_procs` (для пола) | PG17: `max_connections + autovacuum_max_workers + 1 + max_worker_processes + max_wal_senders + special + aux`; PG18: `autovacuum_worker_slots` вместо `autovacuum_max_workers` | при 200/3/16/10 ≈ 240 → пол undo ≈ 30 МБ | Формула из `src/orioledb.c`. Считать на стороне компилятора после того, как известны остальные GUC. | [код] |
| `orioledb.system_undo_circular_buffer_fraction` | оставить 0.10 | 0.10 | DDL-нагрузка только на загрузке; поднимать только для DDL-heavy. | [офиц] |
| `orioledb.regular_block_undo_circular_buffer_fraction` | оставить 0.45 | 0.45 | остальное — row-level undo. | [офиц] |
| `orioledb.xid_buffers` | `8` МБ при RAM ≥ 16 ГБ, иначе 1 | **8 МБ** (~520k xid) | «Каждый МБ вмещает 65 тыс. xid-items»; TPC-C — тысячи TPS × 128 VU, дефолт 1 МБ может заставить писать xid-статусы на диск. Дёшево. | [офиц] ёмкость; [практика] 8 |
| `orioledb.free_tree_buffers` | 8 МБ (дефолт) | 8 | Нужен только сжатым таблицам; профиль сжатие не включает. Если `default_compress >= 0` — `max(8, main/64)`. | [офиц] |
| `orioledb.catalog_buffers` | 8 МБ; при «много таблиц» — вручную | 8 | TPC-C — 9 таблиц. | [офиц] |
| `orioledb.temp_buffers` | 8 МБ (вместо 64) | 8 | Выделяется «на backend»; при 64 МБ × 128 backend'ов = 8 ГБ, если пул не ленивый [?]. TPC-C temp-таблиц не использует. Проверить RSS backend'ов. Дефолт PG `temp_buffers` = 8 МБ. | [офиц] «per backend»; [практика] 8 |
| `orioledb.bgwriter_num_workers` | `clamp(vCPU/4, 1, 4)` | **2** | Док: «используйте >1 на машинах с многими ядрами». Потолок задаёт диск (см. 2.6): 105 МБ/с записи на network-ssd 200 ГБ — больше воркеров не ускорит, только добавят конкуренцию за IOPS. Каждый воркер занимает слот `max_worker_processes`. | [офиц] + [практика] |
| `orioledb.max_io_concurrency` | `0` (выкл.); `>0` только на локальном NVMe при упоре в ядро | 0 | Док: включать, когда «ядро ОС — узкое место при высоком конкурентном IO». На network-ssd (≤7000 IOPS) узкое место — диск. | [офиц] |
| `orioledb.checkpoint_completion_ratio` | oriole-only: `1.0`; смешанный: `0.5` | **1.0** | Прямая рекомендация доки: «1.0, если только OrioleDB-таблицы». Растягивает запись OrioleDB на всё окно чекпоинта вместо 50%+всплеск. | [офиц] |
| `checkpoint_completion_target` | 0.9 (дефолт PG) | 0.9 | Для heap-части. PG-док: 1.0 небезопасно. | [офиц PG] |
| `checkpoint_timeout` | `15min` (900 с) | 900 | Чекпоинт OrioleDB — CoW-сброс грязных страниц main_buffers; на 105 МБ/с — минуты. Дефолт 5 мин даёт 2 чекпоинта внутри 10-минутного замера; 15 мин ≈ 0–1. Платой становится длиннее recovery — для стенда неважно. Параллельно должен совпасть с профилем PG (паритет). | [офиц PG] общий принцип; [практика] 15 |
| `max_wal_size` | `clamp(disk_gb*0.04 ГБ, 2, 32)` → 8 ГБ при 200 ГБ диска; `min_wal_size = max/4` | **8192 / 2048 МБ** | Чекпоинт должен запускаться по таймеру, не по размеру: 1 ГБ (текущее) при записи TPC-C исчерпывается за минуты → `num_requested` ≫ `num_timed`. Рецепт самой Oriole: `max_wal_size=4GB` на 96-ядерном NVMe. 4% диска оставляют запас под данные+undo+WAL. Проверка: `pg_stat_checkpointer.num_requested`. | [офиц PG] checkpoint tuning; [практика] |
| `wal_buffers` | `16MB` явно | 16 | `-1` = 1/32 shared_buffers (8 МБ при 256 МБ) — мало; PG-док: предел полезного — 1 сегмент (16 МБ). Схема: Pattern требует единицу. | [офиц PG] |
| `wal_level` | `replica` (или `logical` не нужен) | replica | Нужен репликам; Oriole пишет row-level WAL в обычный WAL. | [офиц] |
| `max_wal_senders` / `max_replication_slots` | `replicas+2` (мин. 4) | 4 | `pg_basebackup -X stream` + реплики; дефолт 10 не вредит, но входит в `max_procs`. | [код] max_procs |
| `max_worker_processes` | `max(8, 2*vCPU)` | **16** | Нужно `> bgwriter(2) + recovery_pool + recovery_idx_pool + max_parallel_workers`. Иначе Oriole молча уходит в однопроцессное recovery/CREATE INDEX на репликах и при рестарте. | [офиц] |
| `orioledb.recovery_pool_size` + `recovery_idx_pool_size` | суммарно `75% vCPU`, пополам: primary 3+3, реплика 4+2 | **3+3** (дефолт) | Док: «50–100% ядер» суммарно. На реплике recovery-воркеры применяют WAL постоянно (лаг реплики!). | [офиц] |
| `orioledb.recovery_queue_size` | 8 МБ → 32 МБ при реплике | 32 (реплика) | Очереди воркеров recovery; бюджет копеечный. [практика] | [офиц] только дефолт |
| `max_connections` | `max(100, ceil((vus + 20) / 50)*50)`, если пользователь не задал | **200** | 128 VU + exporter + superuser + haproxy health. Увеличивает `max_procs` (undo-пол). | [офиц PG] |
| `work_mem` | 4 МБ (дефолт), не менять | 4 | OLTP; `hash_mem_multiplier` 2. | [офиц PG] |
| `maintenance_work_mem` | `min(2 ГБ, 3% RAM)` | 960 МБ | Для CREATE INDEX на загрузке (`create_indexes` в TPC-C). Влияет на параллельное построение индексов; на beta17-pg17 индекс-сборка — зона повышенного риска (см. раздел 5). | [офиц PG] |
| `jit` | `off` | off | OLTP; паритет с профилем PG. | [офиц PG] «usually off for OLTP» (схема Desc) |
| `random_page_cost` | `1.1` | 1.1 | network-ssd; PG-док рекомендует ~1.1 для SSD. План OrioleDB стоит на PG-стоимостях, эффект мал. | [офиц PG] |
| `effective_cache_size` | `0.65*RAM` | **21 ГБ** (вместо 4 ГБ) | Только планировщик. На 4 ГБ планы неоправданно «боятся» индексного скана. Паритет с профилем PG. | [практика] |
| `effective_io_concurrency` | не трогать | 1 (PG17) / 16 (PG18) | Только bitmap heap scan; OrioleDB-таблицы идут через собственный IO. На PG18 `io_method=worker`, `io_workers=3` — тоже для heap; не трогаем. | [офиц PG] |
| `huge_pages` + `vm.nr_hugepages` | `try`; см. 2.7 | см. 2.7 | | |
| `orioledb.use_mmap`, `device_*` | `off`, не задано | off | Только для сырых блочных устройств/NVRAM. Не использовать с Docker на облачном диске. | [офиц] |
| `orioledb.default_compress` | `-1` | -1 | Сжатие меняет профиль CPU/IO: бенчмаркать как явную опцию, не включать молча. | [офиц] |
| `orioledb.serializable` | `table_lock` (дефолт) | — | TPC-C у stroppy на READ COMMITTED/REPEATABLE READ — не затрагивается; `error` безопаснее для проверки того, что нагрузка не просит SERIALIZABLE. [?] | [офиц] |
| `orioledb.strict_mode` | `off` (дефолт) | off | `on` превращает WARNING о неподдержанном в ERROR — полезно как опция, не по умолчанию. | [офиц] |

### 2.3 Деление памяти (32 ГБ, oriole-only)

| Статья | МБ |
|---|---|
| main_buffers | ~9830 |
| shared_buffers (heap) | 256 |
| undo_buffers | ~614 |
| xid + catalog + free_tree + recovery_queue | ~56 |
| WAL buffers | 16 |
| backend'ы: 128 × (work_mem 4 + ~8 приватной) | ~1500–2000 |
| ОС + Docker + агент + node/postgres exporter | ~1500 |
| **итого резидентно** | **~14 ГБ** |
| остаток под page cache | **~18 ГБ** |

Для реплики формулы те же (`db-replica`), `hot_standby=on`. Хост ОС в Docker: cgroup-лимит памяти НЕ ставится (рецепт не эмитит
`--memory`), поэтому OOM-killer хоста выбирает жертву по oom_score — главный риск у процесса с самым большим RSS, т.е. у postgres.
Потолок 45% RAM для main_buffers — страховка.

### 2.4 Репликация

- Физическая async-реплика: `pg_basebackup -R -X stream` (уже в `pgReplicaBootstrap`). Для Oriole `pg_basebackup` копирует его файлы
  вместе с PGDATA (`orioledb_data/` внутри каталога) — отдельных шагов в доке нет [офиц getting-started].
- Паритет конфигов primary/replica: `main_buffers`, `undo_buffers` должны совпадать (иначе при promote другой профиль) — рендерится одной
  функцией по формуле, различаются только `recovery_*_pool_size` (реплика ближе к «применять WAL»).
- `hot_standby_feedback`: при ro-листенере haproxy (read-only backends на реплики) включать `on`? У OrioleDB своя MVCC/undo, как
  `hot_standby_feedback` и конфликты recovery работают на Oriole-таблицах в доке не описано [?] — открытый вопрос.
- beta19 содержит фикс: реплика, упавшая в середине replay table rewrite, могла потерять все строки до restartpoint, и фиксы
  minimum recovery point (см. релиз-ноты через NixOS PR) — довод поднять дефолт образа.
- Failover-менеджера нет (Patroni не поддерживает OrioleDB), `synchronous_standby_names` пустой — коммит ждёт только локальный flush.

### 2.5 WAL

- `synchronous_commit=on`, `fsync=on`, `full_page_writes=on` — НЕ менять (см. раздел 3).
- `wal_compression`: `off` (stock). `lz4` имеет смысл с FPW, но поддержка в сборке образа не подтверждена [?].
- `wal_keep_size`: 0 + слот не используется: `pg_basebackup -X stream` без слота при долгом простое primary может потерять сегменты;
  для стенда поставить `wal_keep_size = min(2048, max_wal_size/4)`. [практика]

### 2.6 Диск и IO (Yandex Cloud network-ssd)

Лимиты [офиц, yandex.cloud/docs/compute/concepts/limits]: блок 32 ГБ; на блок 1000 IOPS чтения и записи, 15 МБ/с чтения и записи;
на диск максимум 40 000 write IOPS / 20 000 read IOPS и 450 МБ/с; на vCPU ВМ — 3500 IOPS и 45 МБ/с.
200 ГБ → 7 блоков (224 ГБ) → **≈7000 IOPS и ≈105 МБ/с** на запись и на чтение (общий ресурс читает/пишет). Ограничение ВМ
(8×3500=28 000 IOPS) не работает — диск медленнее. Следствия:

- Диск — узкое место раньше процессора. Профиль должен избегать лишней записи: большой `max_wal_size`/`checkpoint_timeout`
  (меньше FPW и чекпоинтов), `checkpoint_completion_ratio=1.0`, 2 bgwriter'а, не 8.
- Для сопоставимости с другими движками диск-тип и размер зависят от `rs.DiskGB` (`compile.go:150-157`: диск растёт до
  `RecommendedGB` датасета). Размер диска напрямую = IOPS, так что формула `max_wal_size` от `disk_gb` сама подстраивается.
- non-replicated-ssd (5600 write IOPS/93 ГБ, 1 ГБ/с) даёт совсем иной режим: для `disk_type=network-ssd-nonreplicated` можно поднять
  `bgwriter_num_workers` до vCPU/2 и включить `max_io_concurrency` (≈ 32) — отдельная гипотеза, проверять.
- Файловая система: `host.disks` default xfs, `noatime,nodiratime` — ок. `orioledb.use_sparse_files` (off) требует `FALLOC_FL_PUNCH_HOLE`
  (xfs/ext4 поддерживают) — не включать без причины (кажущийся размер файлов расходится с `du`).

### 2.7 Хост (sysctl/THP) — новый шаг `HostPrepSysctl`

Применить `cfg.host.sysctl@1` (уже есть) ролям `db`/`db-replica`:

| Ключ | Значение | Почему | Источник |
|---|---|---|---|
| `vm.swappiness` | 1 (дефолт схемы) | своп ломает латентность | [практика] |
| `vm.dirty_background_ratio` / `vm.dirty_ratio` | 1 / 4 (при 32 ГБ ≈ 320 МБ / 1.3 ГБ) | дефолт 10/20% = 3.2/6.4 ГБ грязных страниц: на 105 МБ/с сброс такого объёма — stall'ы по 30+ с, рвут p99. Схема допускает 0..100 целыми | [практика]; kernel docs vm.dirty_* |
| `vm.overcommit_memory` | 0 (дефолт) | `2` (рекомендация PG) на хосте с Docker/агентом рискованно | [офиц PG] vs [практика] |
| `transparent_hugepage` | `never` (дефолт схемы) | PG-документация рекомендует отключать THP | [офиц PG] |
| `vm.nr_hugepages` | `0` по умолчанию; опция `ceil((main+shared+undo+10%)/2МБ)` + `huge_pages=on` | Пулы Oriole выделяются в общем сегменте (`RequestAddinShmemSpace` [?]), значит huge pages должны покрывать. Проверка: `SHOW shared_memory_size_in_huge_pages` (PG15+) после старта. Для фазы 1 не включать (риск несоответствия объёма → отказ старта при `huge_pages=on`) | [офиц PG] huge_pages; [?] Oriole |
| `fs.aio-max-nr`, `fs.file-max` | дефолт схемы | достаточно | |

Контейнер: `--shm-size` 1024 МБ (поле `shm_size_mb` уже в `cfg.docker.container@1`, default 1024), `--ulimit nofile=1048576`,
`memlock=-1` (если `huge_pages`). Сейчас эти значения не доходят до docker (нет поля в `spec.Container`).
Параллельный `CREATE INDEX`/запросы PG используют DSM в `/dev/shm`: с 64 МБ (docker-дефолт) параллельный план может упасть/деградировать,
и это нужно исключить как причину нестабильности на `create_indexes` [?].

---

## 3. Надёжность и паритет

- **Не ослаблять**: `fsync=on`, `synchronous_commit=on`, `full_page_writes=on`, `data_checksums` (уже в `POSTGRES_INITDB_ARGS`).
  OrioleDB при этом всегда проверяет собственные чек-суммы страниц независимо от `data_checksums` [офиц].
  Любое ослабление durability (`off`, `fsync=off`) допустимо только явным deviation пользователя и должно помечаться в отчёте/профиле
  прогона как «небезопасный профиль».
- Паритет с другими движками (текущее состояние в репо):
  - MySQL/MariaDB: `innodb.go` — только buffer pool (70% RAM) и io_capacity у MariaDB; `innodb_flush_log_at_trx_commit` стоковый (=1).
  - PostgreSQL (не Oriole): автоподбора НЕТ; рендерится stock 128 МБ. Значит «честный» паритет сейчас — худший случай; нужен общий
    `pgMemory` (`shared_buffers=25% RAM`, `effective_cache_size`, `max_wal_size`, `checkpoint_timeout`, `random_page_cost`, `jit=off`),
    и OrioleDB-часть просто добавляет `main_buffers`/`undo`. Это отдельное решение платформы; общие GUC лучше вынести в общий код
    (`compile/pgmemory.go`), чтобы Postgres и OrioleDB тюнились одинаково.
  - Принцип паритета по «движковому кэшу»: PG 25% RAM (общепринято), Oriole 30% (в пределах рекомендации 1/4 и запаса под undo),
    InnoDB 70% (MySQL-док). Разница у InnoDB — выброс; стоит прописать в отчёте прогона размер кэша каждого движка отдельной строкой.
- Все изменения профиля (формулы + итоговые значения) надо записывать в артефакты прогона (`effective_configs` уже есть) и показывать
  в отчёте: ниже в разделе 4 — `profile` поле.

---

## 4. Изменения платформы

1. **Схема параметров** `pipelines/schemas/dbparams/orioledb.go`
   - Добавить `memory` Choice `auto|fixed|stock` (Group Memory, default `auto`), `main_buffers_mb` (Int64, Nullable, Gte 8, Unit MB).
   - Переопределить `shared_buffers_mb`: теперь это именно heap `shared_buffers` (Nullable, default убрать); исправить Desc («heap; оставьте
     128–512 МБ при oriole-таблицах по умолчанию»). Правило `buffers-vs-replicas` перевести на `main_buffers_mb` с учётом `memory`.
   - Расширить `image_tag`: `beta19-pgNN`, `beta18-pgNN` (и убрать/объяснить расхождение со списком каталога).
   - Каждое поле с `// doc:` (правило `pipelines/schemas/AGENTS.md`): configuration, orioledb.c, PG runtime-config-resource, Yandex limits.
   - Golden: `go test ./pipelines/schemas/dbparams -update`; затем `make schemas-export` (web/src/schemas, не править руками).
2. **Схема конфига** `pipelines/schemas/cfg/orioledb_postgresql_conf.go`
   - Исправить Desc `max_io_concurrency`; добавить `system_undo_circular_buffer_fraction`, `regular_block_undo_circular_buffer_fraction`,
     `strict_mode`, `use_sparse_files`, `default_primary_compress`, `default_toast_compress`, `logical_xid_buffers`.
   - Правила: `worker-slots` (`max_worker_processes > orioledb_bgwriter_num_workers + recovery_pool_size + recovery_idx_pool_size`),
     `undo-floor` (CEL-пол от max_connections и т.п. приблизительно; точный пол считает компилятор).
   - Тест: `orioledb_postgresql_conf_test.go` (+ golden `testdata/cfg.orioledb.postgresql.conf@16/17/18.json`).
3. **Компилятор**
   - Новый файл `internal/domain/compile/orioledb_memory.go`: `func (c *compilation) orioleMemory(role, schemaID, machine string, overlay map[string]any)`;
     вызвать в `recipe_postgres.go` перед `c.render(...)` для `oriole==true` (аналог вызова `innodbMemory` в `recipe_mysql.go:72`).
     Явные ключи пользователя: `explicitConfig(role, schemaID)` (уже есть) — использовать ровно так.
   - Добавить `cpuOf(machine)` рядом с `memoryOf` (`innodb.go:55`) или вынести оба в `machine_info.go`.
   - Реплика: тот же вызов в цикле по ролям (он уже общий), с поправками `recovery_*_pool_size` и `recovery_queue_size`.
   - `max_connections` из `run.vus`: функция `c.peakConnections()` по сегментам workload.
   - Опционально общий `pgMemory` для postgres-ветки (отдельным изменением — решение о паритете).
   - Host: `spec.HostPrep{Role: db/db-replica, Kind: HostPrepSysctl, Content: render("cfg.host.sysctl@1")}`; нужен путь через Renderer
     (как для YDB — `recipe_distributed.go:149`).
4. **Контейнер**: `pipelines/spec/spec.go` — поля `ShmSizeMB`, `MemoryLockUnlimited`/`Ulimits`; `pipelines/internal/run/deploy.go`
   (`containerSpec`: `host.ShmSize`, `host.Ulimits`); `recipe_postgres.go` — ставить `shm 1024 МБ`, `nofile=1048576` для Oriole/PG.
5. **Планировщик ресурсов**: `internal/domain/dbkind/postgres.go:127` — требование памяти считать от `main_buffers_mb + shared_buffers_mb`
   (при `fixed`), в режиме `auto` не от 2× shared_buffers; ReasonSharedBuffers оставить (словарь, enums уже синхронизированы).
6. **Каталог**: `dbkind/postgres.go` `Versions` — добавить `beta19-pgNN`, `beta18-pgNN`; дефолт пока `beta17-pg17` до прогона.
7. **Тесты**
   - `internal/domain/compile/orioledb_test.go`: проверки для `auto` (значение main_buffers для 8/32, shared_buffers=256, undo ≥ пол,
     checkpoint_completion_ratio=1.0, replica получает свои пулы), для `fixed`, для `stock` (все дефолты), для явного deviation пользователя
     (`orioledb_main_buffers`), нулевой RAM (`memoryOf=0` → как stock). Заменить проверку «undo_buffers не рендерится» на «undo ≥ пол».
   - `dbparams/orioledb_test.go`: ново-поля, Invalid для `memory`.
   - Тест «значения GUC укладываются в min postmaster'а» — функция пола (`max_procs`) с таблицей PG17/PG18.
   - Живая проверка: `SELECT name, setting, unit FROM pg_settings WHERE name LIKE 'orioledb.%'` в e2e и `SHOW shared_memory_size_in_huge_pages`.
8. **Отчёт прогона / профиль**: в `effective_configs` пишется итог; в отчёте добавить блок «профиль памяти»: режим (`auto/fixed/stock`), RAM,
   доли (main/shared/undo), диск (тип, ≈IOPS/МБ/с по формуле Yandex). Реализация — через существующий `EffectiveConfigs` +
   новая запись в сводке прогона (место в API/словарях — решить отдельно: `docs/` и `openapi/parts/15-vocabularies.yaml` потребуют новой
   причины/значения → правка домена, контракта и локалей в одном изменении, по AGENTS.md «Словари»).

---

## 5. Риски, стабильность beta, открытые вопросы

- **Beta**: проект сам называет себя public beta; релизы идут примерно раз в 2–6 недель (beta17 2026-09-04, beta18 2026-09-23,
  beta19 2026-10-01); WAL-версия менялась (19→20 в beta18) — смешивать primary/replica разных тегов нельзя.
- **Segfault `pg_table_size/pg_indexes_size` при `CREATE INDEX` на beta17-pg17**: уже обходится выключением collector'а
  `stat_user_tables`. Чужой issue открыт в `STROPPY.MD:422`; проверить на beta18/beta19 и на pg16/pg18; влияние `maintenance_work_mem` и
  параллельного построения индексов на воспроизводимость — [?]. Поэтому повышать `maintenance_work_mem` до больших значений (~1 ГБ)
  стоит только после проверки (иначе можно усугубить гонку).
- **`/dev/shm` 64 МБ** при параллельных запросах/индексах — возможная причина нестабильности [?].
- **Память temp_buffers**: если Oriole выделяет 64 МБ на backend сразу — 128 backend'ов дадут 8 ГБ (лишний риск OOM при 30% main_buffers). Проверить RSS.
- **undo**: при долгой транзакции (например `CREATE INDEX` на загрузке + нагрузка) `undo_buffers` растёт/пишется на диск; пол от `max_procs`
  — иначе FATAL при старте. Любое изменение `max_connections`/`max_worker_processes` пользователем должно пересчитывать пол.
- **Поведение page cache vs main_buffers** (O_DIRECT или нет) — [?]; от этого зависит, нужны ли оставшиеся 18 ГБ. Проверка:
  `/proc/<pid>/io` и `free` в прогоне.
- **Hot-standby/recovery-конфликты на Oriole-таблицах** при чтении с реплики через haproxy ro — в доке не описаны [?].
- **Huge pages** и учёт пулов Oriole в `shared_memory_size_in_huge_pages` — [?], проверить после первого запуска.
- **Решения для пользователя**: (а) доля main_buffers 30% vs 25% (официально); (б) заводить ли общий PG-профиль для Postgres; (в) дефолт
  образа — оставить beta17 или перейти на beta19 после прогона матрицы; (г) применять ли sysctl-хост-профиль всем БД на Docker;
  (д) учитывать ли оценку датасета (`spec.EstimateDataset`) при выборе main_buffers (≥1.25× датасета в пределах 45% RAM).

## Источники

- OrioleDB configuration: https://www.orioledb.com/docs/usage/configuration (raw: https://raw.githubusercontent.com/orioledb/orioledb/main/doc/usage/configuration.mdx)
- OrioleDB getting started / limitations: https://raw.githubusercontent.com/orioledb/orioledb/main/doc/usage/getting-started.mdx
- OrioleDB `src/orioledb.c` (GUC defaults/min/max): https://raw.githubusercontent.com/orioledb/orioledb/main/src/orioledb.c
- Рецепт бенчмарка pgbench: https://www.orioledb.com/pgbench-benchmark
- Release beta18: https://github.com/orioledb/orioledb/releases/tag/beta18 ; теги Docker Hub: https://hub.docker.com/r/orioledb/orioledb/tags
- OrioleDB beta7 benchmarks (TPC-C): https://www.orioledb.com/blog/orioledb-beta7-benchmarks
- Supabase OrioleDB: https://supabase.com/docs/guides/database/orioledb
- CloudRaft CNPG: https://www.cloudraft.io/blog/postgres-optimization-with-orioledb
- Issue #941: https://github.com/orioledb/orioledb/issues/941
- PostgreSQL WAL configuration: https://www.postgresql.org/docs/current/wal-configuration.html
- Yandex Cloud disk limits: https://yandex.cloud/en/docs/compute/concepts/limits
