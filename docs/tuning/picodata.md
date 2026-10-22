# Picodata 25.3 / 26.1 / 26.2 — тюнинг и профиль `auto`

Метки: [оф] — официальная документация Picodata/Tarantool; [практика] — общеизвестная практика/вывод, в доке не подтверждено; [гипотеза] — нужно проверить прогоном.
Базовые ссылки (Picodata): config https://docs.picodata.io/picodata/26.2/reference/config/ ; settings (ALTER SYSTEM) https://docs.picodata.io/picodata/26.2/reference/settings/ ; db_config https://docs.picodata.io/picodata/26.2/reference/db_config/ ; sizing https://docs.picodata.io/picodata/stable/admin/sizing/ ; Tarantool box.cfg https://www.tarantool.io/en/doc/latest/reference/configuration/configuration_reference/ ; Picodata-форк Tarantool, MR про wal_mode https://git.picodata.io/core/tarantool/-/merge_requests/467 .

---------------------------------------------------------------------------
## 1. Текущее состояние

### Что есть
- `pipelines/schemas/dbparams/picodata.go` (db.picodata.params@1): version (26.2/26.1/25.3, дефолт 26.1, :25-30), tiers[] (name, instances, replication_factor, can_vote, bucket_count, replication_mode; :35-64), `default_bucket_count` 3000 (:67), `memtx_memory_mb` дефолт 2048 (:76-78), `sql_vdbe_opcode_max` дефолт 0 (:84-86), `haproxy` 0..2 (:89), `pgproto` (:94). Правила: sync только на 26.2 (:107).
- `pipelines/schemas/cfg/picodata_yaml.go`: три схемы — `@25`, `@26` (26.2), `@26.1` (`picodataYaml(26,1)`, без replication_mode/wal_mode в tier, :63). Выбор схемы по версии: `internal/domain/catalog/config_version.go:24-31` (26.1 -> `@26.1`, 26.2 -> `@26`, 25.3 -> `@25`).
  Дефолты cfg: memtx 2G (:159), max_tuple_size 1M, system_memory 256M (:210, только 26.x), vinyl memory/cache 128M, read 1, write 4 (:163-174), log info/plain, boot_timeout 7200, shredding false, bucket_count тира 3000 (:58-60), `wal_mode` тира `write` (:72-76, только `@26`).
- `internal/domain/compile/recipe_distributed.go:138-248` (`picodataRecipe`): контейнер на VM, bind `/data/picodata`, peer = все инстансы, `PICODATA_ADMIN_PASSWORD`, healthcheck через `picodata admin` (:224), sidecar-init один раз на `nodes[0]` делает `ALTER SYSTEM SET sql_vdbe_opcode_max = N` (:231-235), HAProxy при `haproxy>0` (:238-245).
- `internal/domain/dbkind/picodata.go`: версии/образы (:19-21), `planPicodata` (:48-78): 1 инстанс = 1 VM; требования роли db: 2 vCPU, RAM = ceil(memtx_GB*1.5)+1, диск 20 ГБ (:76-77); HAProxy 2/2/10.
- Матрица: memtx 20480M выставлена в базе матрицы руками, остальное — дефолты схемы.

### Чего нет / что не доходит до YAML
1. **`wal_mode` недоступен из параметров БД.** Поле есть только в cfg `@26` (:72), но в dbparams его нет, а рецепт копирует из тира лишь `replication_factor, can_vote, bucket_count, replication_mode` (`recipe_distributed.go:166`). Итог: на 26.2 всегда `wal_mode: write`; на 25.3/26.1 ключа в схеме нет вовсе (в этих версиях его нет и в доке: `wal_mode` — с 26.2.1 [оф]).
2. **`default_bucket_count` из dbparams не передаётся в overlay** (`recipe_distributed.go:197-206` не кладёт его): в YAML уходит дефолт cfg 3000, параметр формы фактически мёртвый. Тир с `bucket_count` не задан берёт `t.bucket_count` = 3000 из дефолта поля (tierFields, :58), а не `default_bucket_count`.
3. Из dbparams не управляются: `memtx.system_memory`, `max_tuple_size`, `vinyl.*`, `log.level`, `wal_dir`, а также все динамические настройки кроме `sql_vdbe_opcode_max` (см. §2: `sql_runtime_concurrency_max`, `sql_motion_row_max`, `sql_storage_cache_*`, `iproto_net_msg_max`, `memtx_checkpoint_*`, `pg_*`).
4. `memtx_memory_mb`: если параметр 0/не задан — в YAML остаётся `2G` из cfg (:159); авто-размера нет. `Requirements` RAM = memtx*1.5+1 — это эвристика «сколько нужно ВМ под заданный memtx», а не наоборот.
5. `wal_dir` в cfg `@26` помечен как «26.1.3 не принимает ключ» (:213-218). Док 26.1 говорит «с 26.1.5», док 26.2 — «с 26.2.1» [оф, противоречие]; согласуется с наблюдением, что 26.1.3 ключ отвергает. Образ 26.1 привязан к 26.1.3 (`dbkind/picodata.go:20`) — при подъёме до 26.1.5+ `wal_dir` станет доступен.
6. В cfg нет `memtx.dir` и `vinyl.dir` (26.2.1+ [оф]) — только если понадобится разнос снапшотов/vinyl.
7. Sidecar применяет ALTER SYSTEM без `FOR TIER` — для cluster-wide настроек это нормально; per-tier (`iproto_net_msg_max`, `memtx_checkpoint_*`) потребуют `FOR TIER <name>` на каждый тир.
8. Репо-комментарий «0 = без лимита» (`picodata.go:81-83`): в докe смысл 0 для `sql_vdbe_opcode_max` не описан [оф: пробел], поведение подтверждено только прогоном на 26.2.1. На 25.3/26.1 не проверено.

---------------------------------------------------------------------------
## 2. План профиля `auto`

Входные величины: `R` = RAM машины ГБ, `C` = vCPU, `D` = диск, `n` = число инстансов на машине (сейчас 1), `N` = инстансов в тире, `V` = число VU нагрузки (если известно, иначе 128), `W` = число складов TPC-C (если известно).

### 2.1 memtx память
Факты: `memtx.memory` дефолт 64M, минимум 32M; при исчерпании INSERT/UPDATE падают ER_MEMORY_ISSUE [оф config]. `system_memory` 256M, отдельная квота, с 25.4.1 [оф]; её нужно прибавлять к RAM инстанса [оф sizing]. Реальный размер в памяти на 5–20% больше оценки (bucket_id, msgpack) [оф sizing]. Ориентир: 32 ГБ на узел, минимум 8 ГБ [оф sizing]. Диск под memtx = 3×RAM [оф sizing].

Формула [практика, на базе оф]:
```
reserve_os = max(2 ГБ, 0.08*R)            # ОС + docker + exporters + vector/agent
per_inst   = (R - reserve_os) / n
arena_ram  = (per_inst - system_memory(0.25) - vinyl_resident(0.0..0.25)) / 1.15   # 15% — Rust-куча sbroad, fibers, portals, read-view/снапшот
memtx_memory = arena_ram                       # округлить вниз до 256M
если известна нагрузка: memtx_memory = min(arena_ram, max(4 ГБ, 1.6 * data_estimate))
```
Для R=32, n=1: (32-2.56-0.25-0.25)/1.15 ≈ 25 ГБ (сейчас 20 в матрице — консервативно, но допустимо). data_estimate TPC-C ≈ 0.12–0.15 ГБ × W с индексами [гипотеза: измерить `pico.sql` размер/`box.slab.info()` после загрузки 100 складов] -> ~12–15 ГБ, значит 20–25 ГБ арены хватает с запасом на рост order/order_line/history за 10 минут.
- Арена выделяется лениво (slab arena), завышение не стоит RSS до использования [практика]; опасность только в том, что при росте данных ОС убьёт процесс (OOM) раньше, чем Picodata вернёт ER_MEMORY_ISSUE — поэтому потолок по RAM обязателен.
- Запас под снапшот: Picodata строится на Tarantool 2.11 (ветка `2.11.8-picodata` в MR выше). Снапшот memtx делается из read view в отдельном потоке, не fork [практика, не подтверждено доками Picodata]; расходуется память на копии изменённых за время снапшота кортежей. 15% закладываем; для write-heavy TPC-C проверить пик RSS во время снапшота.
- `memtx_checkpoint_interval` (tier, дефолт 3600 с) и `memtx_checkpoint_count` (дефолт 1) [оф settings/db_config]. За 10-минутный прогон — максимум один снапшот; оставить дефолты (для паритета с чекпойнтами PG). Если нужен шумоподавленный прогон — `interval = 0.0` отключает демона [оф], но WAL будет расти (диск 200 ГБ достаточно).
- `memtx.max_tuple_size` 1M: TPC-C строки малы (customer c_data 500 B). Оставить [оф дефолт]. Увеличивать только для больших BLOB-нагрузок.
- Tarantool `memtx_use_mvcc_engine`, `slab_alloc_*`, `memtx_min_tuple_size`: Picodata не выводит их в конфиг/ALTER SYSTEM [оф: в config и settings отсутствуют] — не трогаем. Если нужно — только через `custom` (ключи вне схемы Picodata отвергаются, смысла нет).

### 2.2 vinyl
Все таблицы Picodata по умолчанию memtx; vinyl для TPC-C не нужен. Офиц. сайзинг: vinyl — 3 ядра на инстанс, memtx — 1 [оф sizing]. Профиль: не менять `vinyl.*` (128M/128M — ленивая квота, не отнимает RAM заметно). `write_threads` 4 / `read_threads` 1 создают потоки, но без vinyl-таблиц простаивают [практика]. Если захочется сэкономить RAM — `vinyl.memory` можно не уменьшать ниже дефолта без пользы. Вывод: нужен ли vinyl — нет; уменьшать резерв `vinyl_resident` в формуле до 0.25 ГБ.

### 2.3 WAL и чекпоинты
- `wal_mode` тира: `write` (дефолт) / `fsync`, с 26.2.1 [оф config]; страница не описывает разницу. По Tarantool [оф]: `write` — fiber ждёт записи в WAL без fsync(2); `fsync` — fsync после каждого write; `none` Picodata не предлагает. Подробности и паритет — §3.
- `wal.max_size` 256 МБ, `wal.queue_max_size` 16 МБ, `wal.cleanup_delay` 4 ч [оф Tarantool] — Picodata не экспонирует [оф: нет в config]; не трогаем.
- `wal_dir` (26.1.5+/26.2.1+): отдельный диск под WAL. В стенде одна data-диска (`network-ssd`, 200 ГБ); разнос даст эффект только на второй сетевой SSD — вне рамок `auto`.
- Raft-журнал Picodata: `raft_wal_count_max` 16384, `raft_wal_size_max` 64 МБ [оф settings] — влияет на DDL/метаданные, не на пользовательские записи; дефолт.
- Диск: ≥3×RAM инстансов [оф sizing] -> 32 ГБ → 96 ГБ; 200 ГБ покрывает.

### 2.4 Сеть / iproto / pgproto
- `iproto_net_msg_max` (tier ALTER SYSTEM, дефолт 768) — «максимум одновременно обрабатываемых сообщений»; на мощных системах можно увеличить, на слабых уменьшить [оф db_config]. pgproto-запросы клиента не идут через этот лимит, он ограничивает iproto между инстансами (router -> storage). Профиль: `max(768, 6*V)`; для V=128 хватает 768 -> **оставить дефолт**, поднимать до 2048–4096 только если V > ~300 или кластер из многих тиров [практика].
- `readahead`, `networking.worker_pool_threads`: Picodata не выводит [оф: нет].
- `pg_portal_max` и `pg_statement_max` — по 100000 (кластер) [оф]; для 128 VU избыточно, дефолт.
- `sql_storage_cache_count_max` 50 и `sql_storage_cache_size_max` 5 МБ [оф settings]: LRU подготовленных планов на storage. В TPC-C десятки различных запросов, а stroppy ходит в режиме `defaultQueryExecMode: exec` (`compile.go:329-331`). Если уникальных плановых запросов > 50 — кэш будет вытесняться [гипотеза]. Предложение: `sql_storage_cache_count_max = 200`, `size_max = 32 МБ` — проверить A/B (метрика — рост tps/латентность на single).
- **`sql_runtime_concurrency_max` дефолт 50** (cluster-wide, на инстанс; целое ≥1; 0 недопустим) [оф db_config]. При 128 VU на одном инстансе запросы будут упираться в 50. Что именно происходит при превышении (очередь/ошибка) док не говорит [пробел]. Предлагаем `ceil(1.5 * V / n_pg_endpoints)` ≥ 50: single, V=128 -> 192; three-node через HAProxy -> 64–70. Влияние на память — лимит как раз сдерживает её рост [оф]. **Самая вероятная точка быстрого выигрыша**, [гипотеза] — проверить сначала.
- TLS на pgproto: off (дефолт), не включать.
- Если у VM несколько NIC — iproto и pgproto на разные интерфейсы, до +30% [оф sizing]; у стенда один NIC.

### 2.5 SQL-лимиты под TPC-C
- `sql_vdbe_opcode_max` дефолт 45000 [оф]; ломает full-scan (валидация GROUP BY по orders на 100 складов). Репо ставит 0. Доки смысл 0 не раскрывают; в `OPTION(...)` тип `unsigned` [оф dql]. Профиль: оставить 0, но (а) прогонять smoke «ALTER ... =0, затем SELECT count(*) по большой таблице» на каждой версии (25.3, 26.1, 26.2); (б) запасной вариант — конечное 1_000_000_000 (верхняя граница схемы, `picodata.go:86`). Лимит также можно задавать per-query `OPTION (SQL_VDBE_OPCODE_MAX = n)` [оф], но stroppy-скрипты этого не пишут.
- `sql_motion_row_max` дефолт 5000 [оф], не меняем в базе. Риск: пакетные `INSERT ... VALUES` на загрузке и выборки >5000 строк в одной виртуальной таблице (валидация, stock-level на больших W) -> ошибка `Exceeded maximum ...` [оф docs non_block]. Профиль: оставить 5000 пока загрузка/валидация проходят; добавить параметр `sql_motion_row_max` (дефолт null = не трогать), в `auto` при W>=100 ставить 50000 как страховку [практика] — не доказано необходимым.
- `sql_log` false — не включать (шум/нагрузка на лог).
- `log.level`: `info` в Picodata по умолчанию; для бенчмарка `warn` снижает шум и объём журнала контейнера [практика]; но стенд собирает логи агентом — оставить `info`, если нужно для разбора. Мелочь, на tps не влияет заметно.

### 2.6 Шардирование
- `bucket_count` дефолт 3000, применяется только при создании кластера; в файле поменять нельзя после [оф]. Правило vshard: ≥100 бакетов на реплика-сет с запасом на рост; для ≤ 30 реплика-сетов 3000 достаточно [практика, vshard]. Формула: `max(3000, 100 * max_replicasets_planned)`; для матрицы 3000 везде. Починить проброс `default_bucket_count` (§1.2).
- `replication_factor`: RF = 1 для single/для чистой шардированной проверки; RF = 3 для «3 инстанса = 1 реплика-сет с мастером» — тогда все записи идут на лидера реплика-сета, 2 реплики лишь копии (async по умолчанию). Результат «three 351 tps» в матрице — это один шард, а не масштабирование по узлам. Для сравнения «HA-паритет» (Patroni 3 узла) корректно, для оценки горизонтального масштабирования — нет. Вариант для матрицы: `3 инстанса RF1` (3 шарда, без избыточности) и `6 инстансов RF3` (2 шарда).
- **Сколько инстансов на 8 vCPU.** Офиц.: memtx — 1 ядро на инстанс [оф sizing]; Picodata/Tarantool: транзакционный поток один (tx), плюс iproto/wal-потоки [практика]. Один инстанс на ВМ 8 vCPU использует ~1–2 ядра; остальное простаивает. Это главный структурный рычаг (и вопрос честности сравнения с PG/MySQL, использующими все 8 vCPU). Проверка: `top -H` на ВМ во время прогона — если один поток ~100%, предел tps — tx-поток.
  Рекомендуемый `n`: `n = clamp(floor(C / 2), 1, 4)` для 8 vCPU -> 4 инстанса (vCPU в YC = гипертред, оставляем по ~2 vCPU на инстанс + ОС/docker); при RF>1 failure_domain на уровне ВМ (`instance.failure_domain: {host: vm}`) чтобы реплики одного реплика-сета не оказались на одной ВМ [оф tutorial/deploy]. Память: per_inst = (32-2.56)/4 ≈ 7.3 ГБ -> memtx ≈ 5.8 ГБ на инстанс = 23 ГБ суммарно (RF1) — данные TPC-C 100W (~12–15 ГБ) влезают с запасом ~1.6x.
  Это требует изменения платформы (§4): `planPicodata` сейчас = 1 инстанс на ВМ; порты 3301/4327/8081 захардкожены (`recipe_distributed.go:132-134`); нужны разные порты и `instance_dir` на инстанс (в доке: свои ports/instance_dir для каждого инстанса на хосте [оф tutorial]). HAProxy backend — список host:port всех pgproto.
  По умолчанию `auto` не должен менять число инстансов без явного параметра: топологию пользователь задал сам. Предлагаем явный параметр `instances_per_machine` (дефолт 1; `auto` -> предложение из формулы) и честную метку в отчёте «Picodata использует N ядер из 8».

### 2.7 Хост ОС в Docker
Что уже есть в `host_sysctl.go`: swappiness 1, THP never, max_map_count 262144, overcommit 0, file-max 2M, somaxconn 4096. Для Picodata/Tarantool [практика]:
- THP `never` — верно (Tarantool arena).
- `vm.overcommit_memory=0` ок, но арена > RAM+swap может быть отвергнута эвристикой — потолок арены по RAM в §2.1 это исключает.
- Лимит памяти Docker на контейнер не задаётся (рецепт не ставит) — оставлять так; иначе cgroup-OOM при арена+RSS.
- nofile: 128 соединений + файлы WAL/снапшотов — дефолт Docker достаточен; следить за `ulimit -n` при V > 10k.
- `vm.dirty_*` — для режима `wal_mode=write` WAL сидит в page cache; дефолт (20/10) ок.
- Файловая система data-диска xfs, `noatime` — уже `host_disks.go` дефолты. Диск `network-ssd`: латентность fsync миллисекунды -> для `wal_mode=fsync` это основной предел (§3).
- Часы/NTP: не оф. требование найдено; raft чувствителен к рассинхронизации [практика].

---------------------------------------------------------------------------
## 3. Надёжность: wal_mode, fsync, паритет

- Подозрение подтверждено. По умолчанию `wal_mode = write`: транзакция считается завершённой после `write(2)` в xlog, без fsync — хвост журнала в page cache теряется при падении хоста/ВМ [оф Tarantool: «fibers wait for their data to be written to the WAL (no fsync(2))»; оф Picodata: дефолт `write`, значения `write|fsync`, с 26.2.1]. Picodata-MR !467 в их форке Tarantool прямо описывает это: единственный текущий способ — `fsync`, который открывает xlog с O_SYNC и делает синхронной каждую запись инстанса; сейчас в работе per-transaction fdatasync (TXN_FORCE_SYNC_WAL) [источник: git.picodata.io MR 467, draft от 2026-09-02, не релиз].
- **Версии**: 25.3 и 26.1 `wal_mode` в конфиге не имеют; fsync-режим недоступен [оф: параметр «с 26.2.1»]; фактически у них поведение `write` [практика/вывод — докa 25.3/26.1 про WAL не говорит]. Паритет «fsync на каждый коммит» достижим только на 26.2.
- **Паритет по движкам.** PostgreSQL: `synchronous_commit=on`, `fsync=on` — коммит durable (после fsync WAL). MySQL InnoDB: `innodb_flush_log_at_trx_commit=1` — тоже. Picodata `write` ≈ `synchronous_commit=off`-подобен по окну потери данных (но без потери консистентности), т.е. быстрее нечестно, если остальные движки в durable. Решение для сравнений: профиль надёжности `durability: strict|relaxed` в `auto`: strict -> 26.2: `wal_mode: fsync`, и для остальных движков эквивалент; relaxed -> `write`. Для 25.3/26.1 помечать результат как «relaxed, не настраивается».
- Репликация: `replication_mode: async` по умолчанию [оф, 26.2.1] — потеря подтверждённых коммитов при падении лидера (аналог асинхронной стриминговой PG-реплики). `sync` — запись ждёт реплик реплика-сета (26.2 only) ≈ `synchronous_commit=on` с синхронной репликой. Для 3-узловой топологии с претензией на HA-паритет нужен `sync` (только 26.2); смысл «жертвует tps».
- Ожидание от `fsync` на YC `network-ssd`: каждый групповой fsync — миллисекунды; Tarantool WAL-поток батчит записи конкурентных транзакций, поэтому при 128 VU потеря tps может быть умеренной, но на single, вероятно, значима [гипотеза] — **измерить** `write` vs `fsync` на 26.2.
- Снапшот/чекпоинт: не влияет на durable-семантику коммита.

---------------------------------------------------------------------------
## 4. Изменения платформы (пути)

1. `pipelines/schemas/dbparams/picodata.go`: добавить в tier `wal_mode` (write|fsync; правило «fsync/ sync только 26.2» по образцу `sync-requires-26-2`, :107); верхнеуровневые параметры `profile` (auto|manual), `durability`, `instances_per_machine`, `memtx_system_memory_mb`, `sql_runtime_concurrency_max`, `sql_motion_row_max`, `sql_storage_cache_count_max`, `sql_storage_cache_size_max`, `iproto_net_msg_max`, `memtx_checkpoint_interval/count`. Каждое поле с `Title/Desc/Group/// doc:` по `pipelines/schemas/AGENTS.md`. `memtx_memory_mb`: сделать «пусто = auto».
2. `internal/domain/compile/recipe_distributed.go`:
   - :166 — добавить `wal_mode` в список копируемых ключей тира (с проверкой версии как для `replication_mode`, :168-173);
   - :197-206 — передать `default_bucket_count`, `memtx_system_memory`, `log_level` в overlay;
   - :231-235 — расширить init-скрипт: набор `ALTER SYSTEM SET ...` (cluster) и `... FOR TIER <t>` (tier) в одном `pico.sql`-цикле; проверка на каждое выражение;
   - добавить расчёт `auto` в отдельную функцию (по образцу `innodb.go:58`, где уже читается `m.MemoryGB`) — вход `Spec.Machines[i].CPU/MemoryGB`, число VU из `c.in.Workload`.
   - multi-instance на ВМ: параметризовать порты и `instance_dir`/mount (`dataMount + "/picodata-<k>"`), имя контейнера `<m>-picodata-<k>`, healthcheck на свой admin.sock, failure_domain по ВМ, HAProxy `c.servers(role, port)` -> список host:port.
3. `internal/domain/dbkind/picodata.go`: `planPicodata` — учитывать `instances_per_machine` (число ВМ = ceil(instances/ipm)); `Requirements` RAM/CPU считать от `memtx` на инстанс × ipm (:76-77), CPU = 2×ipm.
4. `pipelines/schemas/cfg/picodata_yaml.go`: при подъёме образа 26.1 до 26.1.5+ — открыть `wal_dir` для `@26.1` (:217); при необходимости `memtx.dir/vinyl.dir` для 26.2.
5. Контракт/словари: если появляется enum `durability`/`profile` — по `AGENTS.md` §Словари: `openapi/parts/15-vocabularies.yaml`, домен, `web/src/locales`, затем `make openapi generate-go generate-ts schemas-export`. Новые поля схемы -> перегенерировать golden `pipelines/schemas/testdata/db.picodata.params@1.json`.
6. Отчёты матрицы: записывать эффективные значения (`auto` -> числа), `wal_mode`, `replication_mode`, число используемых ядер.
7. Тесты: `internal/domain/compile/distributed_test.go` — кейсы на `wal_mode` в 26.2 vs ошибку на 26.1/25.3, на `default_bucket_count`, на авто-формулу (8/32, 4/16, 16/64).

---------------------------------------------------------------------------
## 5. Риски и открытые вопросы

1. Смысл `sql_vdbe_opcode_max = 0` не документирован; проверен только на 26.2.1. Нужен smoke на 25.3 и 26.1; запасной вариант — большое конечное значение.
2. Поведение при превышении `sql_runtime_concurrency_max=50` (ожидание или ошибка) неизвестно; проверить, видны ли в логах отказы при 128 VU на single.
3. Снапшот: fork или read-view и пиковая RSS — не подтверждено документацией Picodata; замерить RSS во время снапшота при write-нагрузке (можно принудительно через `picodata admin`/`box.snapshot()` в тестовом прогоне).
4. `wal_mode=fsync` на 26.2 может быть медленным; MR !467 (per-transaction sync) в draft — не рассчитывать до релиза. Паритет durability для 25.3/26.1 невозможен через конфиг; проверить, нет ли env/CLI-пути (напр. `TT_WAL_MODE` — [не проверено], Picodata сам вызывает box.cfg).
5. Один инстанс на 8 vCPU — вероятно, упор в tx-поток; без `instances_per_machine` результаты Picodata занижены относительно движков, использующих все ядра. Нужен замер загрузки по потокам и решение, считать ли это «честным» сравнением по ВМ или по ядрам.
6. Как stroppy создаёт таблицы TPC-C в Picodata (ключ распределения, глобальные таблицы vs шардированные) — не проверялось; от этого зависит выигрыш от нескольких шардов и допустимость multi-statement транзакций между шардами.
7. HAProxy перед pgproto 3-инстансного RF3 реплика-сета: балансировка на реплики добавляет hop к лидеру (запись/чтение идут через мастера реплика-сета) [практика]; health-check по роли в рецепте нет.
8. Док 26.1 и 26.2 расходятся по версии появления `wal_dir` (26.1.5 против 26.2.1); образ `26.1.3` её отвергает.
9. 26.2 single упал из-за строки лога stroppy >1 МБ — баг чтения на стороне stroppy, не тюнинг Picodata; чинить в сборщике логов (лимит строки), иначе данные 26.2 недоступны для сравнения.
10. Формулы объёма данных (0.12–0.15 ГБ/склад) — оценка, не измерение.
