# Тюнинг YDB (self-hosted 25.4 / 26.1 / 26.2 / 26.3 + managed) для Stroppy Cloud

Дата исследования: 2026-10-22. Режим: только чтение репо и документации; ничего не запускалось на стенде.

Обозначения источников в тексте:
- [OFF] — официальная документация YDB / Yandex Cloud (URL в конце раздела или в списке источников);
- [SRC] — исходный код YDB (auto_config_initializer.cpp, raw GitHub main) или код stroppy/stroppy-cloud;
- [PRAC] — практика / вывод автора, в документации не подтверждено: нужна A/B-проверка;
- [?] — не удалось проверить, помечено как гипотеза.

Принятые сокращения: V — vCPU машины, M — RAM машины (ГБ), S — storage-узел (роль `db`), C — database-узел (роль `db-compute`).

---

## 0. Короткий вывод

Главные подозреваемые в «45–53 tps, p99 20–30 с», в порядке убывания уверенности:

1. `cpu_count` не заполняется сервером (всегда дефолт 4). Compute-узел 4 vCPU получает по авто-таблице пулы System 1 / User 1 / Batch 1 / IC 1 (User может «занять» до 4 только при загрузке пула >90% и деградации SelfPing). Это пол, ниже которого ydbd не имеет смысла; storage 8 vCPU использует только 4 ядра. [SRC]
2. Дисковая подсистема: pdisk — файл 40 ГБ на `network-ssd` ~60 ГБ диска. По лимитам YC это 2 аллокационные единицы по 32 ГБ = ~2000 IOPS записи и ~30 МБ/с записи на ВЕСЬ pdisk. Плюс сам YDB называет такую конфигурацию непригодной для замеров производительности (виртуализированное хранилище, диск <800 ГБ, файл вместо блочного устройства). [OFF]
3. Одна группа хранения (`create ssd:1`) на один pdisk: все ~700+ табл-тайлетов TPC-C (3 канала у каждого) пишут журнал в один VDisk. [PRAC, нужна проверка экспериментом groups ∈ {1,4,8,16}]
4. Схема stroppy `tpcc/ydb.sql`: вторичные индексы GLOBAL SYNC (`idx_order`, `idx_customer_name`) создаются без `PARTITION_AT_KEYS`; по документации партиционные параметры к индексам не применяются, т.е. индексы стартуют одной партицией и делятся только по нагрузке. Каждый New-Order пишет в `idx_order` → распределённая транзакция через одну горячую партицию. `history` — монотонный `h_id` (append-only, хвостовая партиция). [OFF + SRC; проверить `.sys/partition_stats`]
5. Нагрузка: 128 VU без pacing на 100 складах, `retry-attempts=3`, `database/sql` без лимита сессий (документация YDB: 5–10 сессий на ядро dynnode, то есть 20–40 на 4 vCPU). Много конфликтов serializable (TLI/ABORTED), три попытки — и транзакция превращается в ошибку. [OFF + SRC]
6. Одно наблюдение для проверки до любых изменений: при включённом `pacing` TPC-C потолок на 100 складах = 12.86 × 100 tpmC = 1286 tpmC ≈ 47.6 tps (всех транзакций при mix 45% New-Order). Результат 45–53 tps ровно в этот потолок. Если в матрице `pacing=true`, то 45–53 tps — это «эффективность ~100%», а проблема в p99 20–30 с, а не в throughput. Нужно посмотреть segment-параметры матричных прогонов (`pipelines/schemas/workload/segment.go:136`).

---

## 1. Текущее состояние (файл:строка)

### 1.1. Что выставлено

| Что | Где | Значение |
|---|---|---|
| Образы | `internal/domain/dbkind/ydb.go:20-23` | `ydbplatform/local-ydb:26.3.1.14`, `26.2.1.14` (default), `26.1.1.22`, `25.4.1.15`. В changelog 26.3 числится как RC 26.3.1.16 — «latest» может быть пререлизом. |
| Топологии | `ydb.go:31-32` | `single` (1 S + 1 C, 1 pdisk) и `mirror-3-dc` (9 S × 3 pdisk, 3 C, `failure_domain=disk`). |
| Плановые минимумы | `ydb.go:73-74` | S: 4 vCPU / 8 ГБ / `40*pdisks+20` ГБ; C: 4 vCPU / 8 ГБ / 20 ГБ. Это нижняя граница компилятора, а не рекомендация. |
| Клиент | `ydb.go:70` | `ydb_grpc`, порт 2136 первого `db-compute`; без haproxy (`haproxy` по умолчанию 0). |
| Размер pdisk | `recipe_distributed.go:260` `ydbPdiskGB = 40`, `:315` `truncate -s 40G` | Жёстко 40 ГБ на pdisk, разреженный файл `/var/lib/stroppy*/ydb/pdisk-N.data` (`dataMount`), монтируется в контейнер как `/ydb_data` (`:336`). |
| Диск S | `compile.go:169-176` | `max(default, EstimateDataset().RecommendedGB = 3×данные+2)`, не меньше `40*pdisks+20`. |
| Группы хранения | `recipe_distributed.go:346-349` | `admin database <path> create ssd:<storage_groups>`, дефолт `storage_groups=1` (`dbparams/ydb.go:84`). |
| Static group | `recipe_distributed.go:405-424` | один ring (или 3 для mirror-3-dc), на каждый узел один `vdisk_location` на `pdisk-1.data`. |
| state_storage | `:292-303` | ring по всем storage-узлам, `nto_select`: 1 / 3 / 5 / 9 по числу узлов и erasure. |
| Конфиг | `pipelines/schemas/cfg/ydb_config_yaml.go:326-364` | шаблон из вашего сообщения; одинаков для 25.x и 26.x (flat V1, `:25-37`). |
| Сайдкар init | `recipe_distributed.go:347-354` | `blobstorage config init` + `database create` в цикле `until`. |
| Compute | `:361-385` | `--tenant`, `--node-broker`, `--data-center`, `--rack <имя машины>`; healthcheck `ydb scheme ls`. |

### 1.2. Что объявлено в схеме, но НЕ используется рецептом (мёртвые параметры)

Параметры есть в `dbparams/ydb.go`, UI их показывает, но `recipe_distributed.go` их не читает (поиск `auto_size_pdisks|disk_type|failure_domain` по `internal/` даёт 0 попаданий вне сгенерированного кода):

- `auto_size_pdisks` (`ydb.go:88`) — размер pdisk всегда 40 ГБ.
- `disk_type` (`:70`) — всегда `ssd` (`recipe_distributed.go:283`, `storage_pool_types` `:307`).
- `failure_domain` (`:50`) — не влияет на рендер; static group всегда «один fail domain на узел».
- `grpcs` (`:105`) — рецепт всегда plain gRPC (комментарий в `ydb.go:41-42`).
- `fail_domains` (Computed) и правила `block42-min-domains` и т.п. проверяют число, которое не влияет на то, что рисуется.
- Поля схемы конфига `cpu_count`, `memory_*`, `feature_flags`, `custom`, `log_*`, `spilling_*`: рецепт передаёт только `static_erasure, drives, host_config_id, hosts, state_storage_*, storage_pool_types, blob_storage_service_set, grpc_port, interconnect_port, monitoring_port, spilling_root, node_type` (`recipe_distributed.go:306-311`). Остальное — дефолты схемы.

### 1.3. Баги и противоречия рекомендациям

1. **`cpu_count` всегда 4.** Описание поля (`ydb_config_yaml.go:151-153`) обещает «the server fills it from the machine size», но рецепт его не заполняет (`grep cpu_count internal/` пусто). Итог: S (8 vCPU) использует 4 ядра, C (4 vCPU) на пределе. Если ключ вообще не указан, ydbd берёт число CPU из affinity mask процесса (в Docker без cpuset — все vCPU ВМ) [SRC: auto_config_initializer.cpp]. Т.е. текущее значение 4 ХУЖЕ, чем отсутствие ключа.
2. **Распределение ядер инвертировано.** В `single` S получает 8 vCPU (db-1), C — 4 (db-compute-1). Вся пользовательская нагрузка (SQL, транзакции, gRPC) идёт на C; S в TPC-C 100 складов почти не нагружен по CPU. В авто-таблице COMPUTE@4 = System 1 / User 1 / Batch 1 / IC 1. [SRC]
3. **pdisk 40 ГБ.** Минимум по документации 80 ГБ (меньше — «узел не сможет использовать устройство», только для ознакомления); плюс 25.6 ГБ журнала на pdisk и ≥0.2 ГБ системной области, т.е. при 40 ГБ полезного места ~14 ГБ при 100 складах × ~100 МиБ = ~10 ГБ + индексы. На пределе (риск «жёлтой/красной» зоны места). [OFF: system-requirements]
4. **pdisk — файл, а не блочное устройство.** `cfg.host.disks` умеет `raw: true` для YDB (`host_disks.go:10-14, 36-38`), но рецепт им не пользуется. Документация: «YDB uses raw devices without a filesystem». Файл на ext4/xfs поверх сетевого диска — dev-режим local-ydb. [OFF]
5. **Тип диска.** Документация: диск <800 ГБ или любая виртуализация хранилища «cannot be used for production services or system performance testing». `network-ssd` ~60 ГБ нарушает оба условия. [OFF]
6. **Хостовые настройки не применяются к YDB.** `cfg.host.sysctl` (`host_sysctl.go`) в рецепте не вызывается (только `HostPrepScript` с mkdir/truncate, `:313-317, :356`), а её дефолт `transparent_hugepage=never` противоречит рекомендации YDB «Enable THP and memory overcommit» (TCMalloc). У контейнера нет `Ulimits` (поле есть в `spec.go:161`, рецепт не задаёт). [OFF]
7. **Контейнер без ограничений памяти.** `memory_controller_config.hard_limit_bytes` не задан и cgroup-лимита нет → лимит = вся RAM ВМ, включая то, что занимают node_exporter, docker и агент. Документация требует cgroup-лимит для строгого контроля. [OFF]
8. **Проброс портов Docker** (`Ports` для 2135/2136/19001/8765) → interconnect и gRPC идут через docker-proxy/NAT (в памяти проекта: «userland-proxy breaks h2»). У `spec.Container` нет `network_mode: host`. [PRAC]
9. **Один storage-group.** `storage_groups=1` по умолчанию (см. 0.3).
10. **Клиентский URL — один адрес первого compute** (`compile.go` clientURL → `${ip:role:db-compute}`); дальнейший балансировщик — discovery в ydb-go-sdk. Для 3 compute-узлов в mirror-3-dc это нормально, если discovery отдаёт достижимые имена/адреса (в проде работало, но гарантий нет). Haproxy выключен по умолчанию.
11. `static_erasure: none` при `mirror-3-dc` генерируется как `mirror-3-dc` (`ydbErasure`), но `state_storage` nto_select для 9 узлов = 9 (все 9 реплик) — валидно по доке для mirror-3-dc (нечётное число, ≤ ring), но `Rule nto-select-odd` совпадает; ок.
12. `ydbServiceSet` игнорирует `pdisks` (`_ = pdisks`, `:418`): статическая группа живёт на `pdisk-1.data`; остальные pdisk-и (2, 3) присутствуют в `host_configs.drive`, но используются только динамическими группами. Это нормально (рекомендация документации — одна статическая группа), но тогда `pdisk-1` держит и статику, и свою долю динамики.

---

## 2. Полный план профиля `auto`

Принцип: профиль `auto` — набор формул, из размера машины/роли/топологии выдающий значения параметров; пользователь может переопределить любое. Ниже каждый параметр: формула → обоснование → источник. Значения 25.4–26.3 различаются только там, где отдельно отмечено.

### 2.1. actor_system_config (пулы CPU)

Документация: система акторов использует большую часть CPU узла; PDisk-и и gRPC-API работают ВНЕ actor system и требуют отдельного запаса ядер. Два режима: auto (рекомендуется) и manual. [OFF: reference/configuration/actor_system_config]

| Параметр | Формула профиля | Обоснование |
|---|---|---|
| `use_auto_config` | `true` всегда | Рекомендация документации. [OFF] |
| `node_type` | роль `db` → `STORAGE`; `db-compute` → `COMPUTE`; совмещённый узел (если появится) → `HYBRID` | Определяет соотношение пулов. Уже сделано (`recipe_distributed.go:268, :360`). |
| `cpu_count` (S) | `V - reserve_S`, `reserve_S = 1` при V≤8, иначе 2; минимум 2 | Запас под PDisk-потоки, gRPC и ядро ОС (документация прямо говорит, что они вне actor system). [OFF] числа reserve — [PRAC] |
| `cpu_count` (C) | `V - 1` при V≥8; при V=4 — `4` (не урезать) | gRPC на C нагружен сильнее; на 4 vCPU резерв съест User-пул. [PRAC] |
| (альтернатива) | не задавать `cpu_count` | ydbd возьмёт affinity mask = V. Проще, но не оставляет запас. [SRC] |
| ручные пулы | не включать в `auto`; `custom: actor_system_config` как escape hatch | Документация советует менять только `threads`, пока один пул не перегружен (смотреть монитор → вкладка Pools). [OFF] |
| Предупреждение валидации | C с V<8 → warn «ниже практического минимума»; слот в доке: 10 ядер / 50 ГБ | glossary: «a typical slot size is 10 CPU cores and 50 GB RAM»; system-requirements: dynamic node 10–32 ядер. [OFF] |

Что реально получится по авто-таблице [SRC auto_config_initializer.cpp] (System/User/Batch/IC; число = потоков в базе, «макс.» заимствуется у простаивающих пулов при загрузке >90% и SelfPing хуже 10 мс):

| CPU | STORAGE | COMPUTE | HYBRID |
|---|---|---|---|
| 4 | 1 / 1 / 1 / 1 | 1 / 1 / 1 / 1 (макс. User=4, System=3) | 1 / 1 / 1 / 1 |
| 8 | 5 / 1 / 1 / 1 | 2 / 4 / 1 / 1 | 2 / 3 / 1 / 2 |
| 16 | 10 / 1 / 2 / 3 | 3 / 8 / 2 / 3 | 5 / 5 / 1 / 5 |
| 30 | 19 / 1 / 4 / 6 | 5 / 15 / 4 / 6 | 9 / 10 / 1 / 10 |

Выводы: (а) на C 4 vCPU User=1 в базе; (б) на C 8 vCPU User=4, System=2; (в) на S User всегда 1 (там пользовательских запросов нет). IO-пул вне бюджета: 1 поток на 60 CPU. При V≤4 профиль LOW_CPU_CONSUMPTION, при >4 LOW_LATENCY. gRPC: рабочие потоки и число прокси = max(2, V/7), 33×V обработчиков на completion queue. Scheduler resolution 1024 при V≤4, иначе 64. [SRC]

Версии 25.4→26.3: механизм auto-config тот же (появился в 23.1, [OFF changelog]), изменений в таблицах между версиями документация не называет.

### 2.2. memory_controller_config и shared cache

[OFF: reference/configuration/memory_controller_config] Все параметры — на узел базы (database node). Дефолты: `soft_limit_percent 75`, `target_utilization_percent 50`, `shared_cache_min/max_percent 20/50`, `mem_table_min/max_percent 1/3`, `activities_limit_percent 30`, `query_execution_limit_percent 20`, `compaction_limit_percent 10`. Ограничение: `shared_cache_min + mem_table_min + activities_limit < soft_limit` (20+1+30 = 51 < 75). `hard_limit_bytes` по умолчанию — cgroup-лимит либо вся RAM; «process may still exceed this limit». Рекомендуют cgroup-лимит.

| Параметр | Формула профиля | Обоснование |
|---|---|---|
| `hard_limit_bytes` | `(M - reserve_M) ГиБ`, `reserve_M = 2` при M≤32, иначе 3 | У нас нет cgroup-лимита; задаём явно, оставив место под ОС/docker/агент/node_exporter. [PRAC] |
| (в идеале) | добавить `mem_limit` в `spec.Container` и задать cgroup = `hard_limit` | Документация: «setting hard_limit_bytes alone does not guarantee…». [OFF] |
| `soft_limit_percent` | 75 (дефолт) | [OFF] |
| `target_utilization_percent` | C: 60; S: 50 | Кэш подстраивается под target; у C (RAM 16 ГБ при данных ~10–13 ГБ) выгодно держать больше. [PRAC, A/B] |
| `shared_cache_min_percent` | C: 25; S: 20 | Нужны: min+mem_table_min+activities < soft: 25+1+30=56<75. [PRAC] |
| `shared_cache_max_percent` | C: 60; S: 50 | Чтение таблиц идёт из shared cache (blob cache). Датасет 100 складов ≈ M C; больший кэш сокращает чтения с `network-ssd`. [PRAC] |
| `mem_table_*` | дефолт 1/3; кандидат на A/B: max 5 для write-heavy | У TPC-C сотни датаршардов, много мелких мемтаблиц. [PRAC] |
| `activities/query_execution/compaction` | дефолт | OLTP-запросы короткие; spilling не должен срабатывать. [OFF] |
| RAM C | `M ≥ 4–5 ГБ × V` (слот 10 ядер / 50 ГБ) | [OFF glossary] |

Версии: параметры одинаковы в 25.4–26.3.

### 2.3. table_service_config, query service, resource manager, resource broker

- `table_service_config.enable_query_service_spilling: true` и `spilling_service_config.local_file_config` (`enable: true`, `root`, `max_total_size`): по умолчанию `enable=true`, `root=""` (tmp), `max_total_size=20 ГиБ`; для OLTP не важны, но каталог `/ydb_data/spilling` лежит на том же диске, что pdisk-файл. Формула: `max_total_size = min(20 ГиБ, 10% диска)`. [OFF]
- `table_service_config.resource_manager.spilling_percent` (дефолт 80) — измеряется относительно пула `query_execution_limit`. Не трогать. [OFF]
- `resource_broker_config` (`!inherit` + `!append`): очереди по умолчанию `queue_ttl` cpu 2, `queue_backup` 2, `queue_restore` 10, `queue_build_index` 10, `queue_cdc_initial_scan` 2. Практика для стенда: `create_indexes` (2 индекса на customer/orders) строит индекс «в онлайне» с лимитом 10 потоков — на C с 4–8 vCPU это отбирает ядра у остальной системы; профиль ограничивает `queue_build_index.cpu = max(1, V/4)` на этапе загрузки. [PRAC]
- `query_service_config`: относится к федеративным запросам (коннекторы) — к OLTP не относится. [OFF]
- `feature_flags`: страница документации (21 флаг) не содержит ни одного флага, влияющего на OLTP-транзакции строковых таблиц (все связаны с индексами JSON/fulltext/Bloom, Kafka, бэкапами). Для 25.4–26.3 профиль `auto` флагов не выставляет. Volatile transactions по умолчанию с 24.4, config V2 с 25.1 [OFF changelog]. Исключение: на 26.2 исправлена ошибка парсинга tri-state feature flags в YAML («subsequent settings lost») — не использовать tri-state флаги на ≤26.1. [OFF changelog]
- `tli_config` (26.1+): диагностика Transaction Lock Invalidation. Включить в профиле `diagnostic`, потому что в TPC-C на serializable основной источник ошибок/повторов — `LOCKS_BROKEN`/TLI. [OFF changelog 26.1; точные ключи YAML — по странице tli_config, не проверено]
- `immediate_controls_config` существует (runtime-настройки DataShard и др.), но в документации нет перечня значений «для OLTP» — не трогаем. [OFF]

### 2.4. Blob storage / pdisk / vdisk / группы

| Параметр | Формула профиля | Обоснование |
|---|---|---|
| тип носителя | S-диск `network-ssd-io-m3` или `network-ssd-nonreplicated`, размер ≥ 186 ГБ (2 единицы по 93 ГБ); `network-ssd` — только при размере ≥ ceil(target_write_IOPS/1000)×32 ГБ | По Compute limits: `network-ssd` — 1000 write IOPS и 15 МБ/с на каждые 32 ГБ; `nonreplicated/io-m3` — 5600 IOPS и 82 МБ/с на каждые 93 ГБ. 60 ГБ network-ssd = ~2000 IOPS и 30 МБ/с. [OFF yandex.cloud compute limits] |
| `drive.type` | `ssd` для network-ssd/nonreplicated; `nvme` для локальных NVMe (на YC — только bare metal/Storage-оптимизированные; см. 7) | Параметр `disk_type` уже есть в схеме — подключить. [OFF host_configs] |
| размер pdisk-файла | `pdisk_gb = max(80, floor(0.8 × (disk_gb - 10) / pdisks_per_node))` при `auto_size_pdisks=true` | Минимум документации 80 ГБ; журнал 25.6 ГБ/pdisk; запас под файловую систему. Реализует уже существующий параметр. [OFF + PRAC] |
| данные на pdisk | `pdisk_gb ≥ 25.6 + 0.2 + 1.5 × DataGB × replication / N_pdisks_total_per_group_ring` | replication: none 1×, block-4-2 1.5×, mirror-3-dc 3×. 100 складов: DataGB ≈ 10–13 с индексами. [OFF topology + PRAC] |
| `raw` pdisk | режим «блочное устройство» вместо файла: `cfg.host.disks` raw + `/dev/disk/by-partlabel/ydb_disk_ssd_NN` | Рекомендация документации. Требует доп. неразмеченного диска на ВМ — отдельная задача провижна. [OFF] |
| `pdisks_per_node` | single: 1; mirror-3-dc: 3 (как сейчас), но все 3 файла на одном сетевом диске делят его IOPS | Если файлы на одном диске, третий pdisk не даёт параллелизма I/O — только больше слотов. Для настоящего параллелизма нужны отдельные диски. [PRAC] |
| `storage_groups` (single, erasure none) | кандидат `min(8, slots_pdisk − 1)`; эксперимент {1,4,8,16} | Больше групп — больше независимых журнальных потоков; в YC для dedicated одна группа ssd ≈ до 100 ГБ. Число слотов на pdisk задаётся `PDiskConfig/ExpectedSlotCount` (виден во view `ds_pdisks`) [OFF], значение по умолчанию — проверить на стенде [?]. |
| `storage_groups` (mirror-3-dc) | кандидат `clamp(2 × database_nodes × V_C / 4, 6, 24)`; ограничение `groups × 9 ≤ pdisks_total × slots` | Ширина группы mirror-3-dc = 9 VDisk (3 DC × 3 домена). 27 pdisk-ов. [OFF topology + PRAC] |
| Static group | одна (рекомендация документации «не более одной») | Уже так. [OFF blob_storage_config] |
| `pdisk_config.expected_slot_count` | поднять, если `groups × width` не помещается; профиль: `ceil((groups × width_per_pdisk)+2)` | [OFF glossary упоминает поле; точное YAML-место в host_configs — по sample-конфигам репозитория, не проверено] [?] |
| fail domain | single: узел; mirror-3-dc: `data_center`=зона YC, `rack`=узел (как сейчас) | [OFF topology: mirror-3-dc ≥9 узлов, 3 стойки на DC]; для 25.4–26.3 без изменений. |
| erasure | `none` (single), `mirror-3-dc` (9+ узлов); `block-4-2` — только если 8+ доменов в одной зоне | Автоматическое выравнивание нагрузки между storage-группами появилось в 26.2. [OFF changelog] |
| pool kind | оставить `ssd`; для nvme — отдельный pool kind | текущее. |

### 2.5. Сеть / interconnect / gRPC

- `grpc_config.port` 2136 (database), статический 2135 — как сейчас; `interconnect_config.start_tcp: true`, порт 19001 — как сейчас. Для 25.x–26.x ключи те же (комментарий `TODO verify` в `ydb_config_yaml.go:141-144`: страницы по grpc/interconnect в индексе документации нет; сверяться с `ydb/deploy/yaml_config_examples`). [OFF]
- Сетевая модель контейнера: добавить в `spec.Container` `network_mode: host` для ydbd (убирает docker-proxy на пути IC и gRPC; для S↔S на mirror-3-dc интерконнект идёт при каждой записи в группу из 9 VDisk). [PRAC; подтверждается проектной заметкой о userland-proxy]
- Базовая сеть YC: нет данных о требованиях YDB к сети в документации (страница system-requirements «Network: not covered»). Для mirror-3-dc нужна пропускная способность и задержка между зонами; RTT между зонами YC порядка 1 мс увеличивает каждый commit. [PRAC]
- `grpcs`: пока нет сертификатов — только plain gRPC; у managed — только grpcs 2135.
- Отказоустойчивость нагрузки: `haproxy` перед compute не нужен для ydb-go-sdk (client-side balancing по discovery); нужен только если discovery возвращает недостижимые адреса. [PRAC]

### 2.6. Размеры узлов и соотношение storage/compute для TPC-C 100 складов

Официально [OFF: ydb workload tpcc]: ~100 МиБ на склад (≈10 ГБ на 100); «начните с ~50 складов на compute-ядро»; сессии — «число ядер dynnode × 5–10, начать с 5»; максимум 12.86 tpmC на склад. Для 100 складов это ≥2 ядер dynnode для paced TPC-C. [OFF] Для unpaced 128 VU нужно больше (оценки [PRAC]: 8–16 ядер).

| Сценарий | S | C | Комментарий |
|---|---|---|---|
| single, paced (официальный) | 4 vCPU / 16 ГБ / диск io-m3 186 ГБ | 8 vCPU / 32 ГБ | cpu_count по формулам; groups 4–8. |
| single, unpaced stress 128 VU | 8 vCPU / 32 ГБ / io-m3 | 16 vCPU / 64 ГБ (слот 10 ядер/50 ГБ) | Базовая цель для оценки. |
| mirror-3-dc | 9 × (8 vCPU / 32 ГБ / ≥186 ГБ nonrepl/io-m3) | 3 × (8–16 vCPU / 32–64 ГБ) | Документация: в mirror-3-dc database-узлы должны иметь «at least 35% extra spare CPU and RAM» при штатной работе (чтобы пережить потерю DC); I/O-запас ×2 на storage. [OFF topology] |

Правила профиля: C-ядер на 100 складов ≥ `W/50` для paced и ≥ `VU/16` для unpaced [PRAC]; не более 32 ядер на dynnode, больше — несколько dynnode с taskset на NUMA-узел [OFF system-requirements]. На ВМ YC это неприменимо (ядра vCPU; NUMA скрыт).

### 2.7. Хост ОС в Docker (ВМ YC)

| Параметр | Значение | Обоснование |
|---|---|---|
| ядро ОС | ≥ 6.6 рекомендовано; минимум 4.19, libc 2.30 | [OFF system-requirements]. Проверить образ ВМ (Ubuntu 22.04 = 5.15 < 6.6). [?] |
| Transparent Huge Pages | включить (`madvise` или `always`) | Документация: «Enable THP and memory overcommit» (TCMalloc). Дефолт `cfg.host.sysctl` = `never` → для YDB переопределить. [OFF] |
| `vm.overcommit_memory` | 1 | «memory overcommit» включить. [OFF формулировка; значение 1 — PRAC] |
| `vm.swappiness` | 1 (дефолт схемы) | [PRAC] |
| `fs.aio-max-nr` | 1048576 (дефолт схемы) | PDisk использует асинхронный I/O. [PRAC] |
| `vm.max_map_count` | 262144 (дефолт схемы) | [PRAC] |
| `nofile` | ≥ 65536 в `Container.Ulimits` | Документация требует ≥10000 для spilling, systemd-юнит из доков ставит больше (по памяти 65536; не проверено) [OFF/?]. |
| `memlock` | unlimited | Юнит YDB (по памяти `LimitMEMLOCK`) [?]. |
| cpufreq governor | performance | На ВМ YC недоступно. [OFF] |
| IOMMU | n/a (виртуалка) | [OFF] |
| часы | chrony/ntp | Требуется документацией; на YC есть. [OFF] |
| logrotate/log_config | `default_level 5` ок; логи в stderr собирает агент | [OFF] |

`HostPrepSysctl` уже поддерживается в spec (`pipelines/spec/spec.go:213`) — достаточно добавить в рецепт.

### 2.8. Различия 25.4 → 26.3 (то, что влияет на тюнинг)

| Версия | Изменения | Влияние |
|---|---|---|
| 25.4 | Стриминг записи в DataShard (кроме таблиц с вторичными индексами), стриминговый Lookup Join, `in_memory` cache mode, `BATCH UPDATE/DELETE` | TPC-C с GLOBAL SYNC индексами идёт по старому пути записи. |
| 26.1 | Load-based автопартиционирование учитывает CPU лидера и всех реплик; TLI-диагностика (`tli_config`); Shuffle Elimination | Автосплит индексных партиций чувствительнее. |
| 26.2 | Быстрее split/merge для таблиц с множеством партиций; `ALTER TABLE ... COMPACT`; автобалансировка между storage-группами; Arrow формат результата и `DISCARD SELECT` по умолчанию | Полезно после загрузки: форсированная компактация; больше групп безопаснее. |
| 26.3 | В changelog помечена RC: bulk AccessService авторизация по умолчанию; локальная очистка SyncLog | Образ `26.3.1.14` ≠ RC `26.3.1.16`; считать пререлизом. |

Конфигурация (flat V1) принимается всеми четырьмя. Config V2 (с 25.1) остаётся экспериментальной; наш рендер V1 корректен (`ydb_config_yaml.go:25-37`). [OFF]

---

## 3. Сторона нагрузки (stroppy) — что определяет результат

Источник: `/home/yaroher/devel/github/stroppy-io/stroppy` (`workloads/tpcc/ydb.sql`, `tpcc.go`, `pkg/driver/ydb/driver.go`, `pkg/driver/sqldriver`).

### 3.1. Схема таблиц `ydb.sql`

- 6 таблиц, ключуемых складом (`district, customer, new_order, orders, order_line, stock`) создаются с `PARTITION_AT_KEYS = ((2),(3),…,(100))`, `AUTO_PARTITIONING_BY_LOAD/SIZE = ENABLED` и `MIN = MAX = <число складов>` (`renderDDL`, `tpcc.go:257-281`). Итого 100 партиций на таблицу → ~600 датаршардов на таблицы данных. Это нормальная раскладка «один тайлет на склад». Но: `MAX = числу складов` запрещает дальнейшие сплиты; `MIN` лишь не даёт сливать (по доке MIN **не создаёт** партиции, а только пол для merge). [OFF: concepts/datamodel/table]
- `warehouse`, `item` — без ключей; `item` имеет `MIN=500, MAX=550`: начально ОДНА партиция (MIN не создаёт партиции), делится только по нагрузке (>50% ядра в течение десятков секунд) до 550. Все 128 VU читают `item` (по 5–15 строк на New-Order) → горячая точка, пока не сработает автосплит. [OFF + SRC]
- `history`: `PRIMARY KEY (h_id)`, без ключей разбиения; комментарий в файле говорит «UNIFORM_PARTITIONS suffices», но директивы нет, а по документации `UNIFORM_PARTITIONS` поддерживает только `Uint32/Uint64` (не `Int64`). Монотонный `h_id` → вставки всегда в последнюю партицию («горячий хвост»), классический антипаттерн YDB. [OFF + SRC]
- Вторичные индексы: `idx_customer_name` и `idx_order` — `GLOBAL SYNC`, строятся после загрузки, затем `ALTER INDEX … SET (…MIN=1000, MAX=1000, PARTITION_SIZE_MB=500)`. Страница таблиц говорит: партиционные параметры к вторичным индексам не применяются, индексы делятся независимо по умолчаниям. Если так, обе индексные таблицы — одна партиция (≤1 ядро на запись), каждая New-Order (45% mix) и каждый Payment по имени пишет в неё распределённой транзакцией. Это главный кандидат на «p99 20–30 с». [OFF; требует проверки `.sys/partition_stats` на живом стенде]
- Транзакции: изоляция `serializable` (по умолчанию для YDB, `config.go:47-57`). У YDB serializable — оптимистичные блокировки; конфликты (TLI/ABORTED) ожидаемы, повторяются. Уровень менять нельзя.
- Запросы `UPSERT`, batch `IN :item_ids` — штатно.

### 3.2. Драйвер и пул сессий

- `ydbsdk.Connector(..., WithQueryService(true), WithAutoDeclare(), WithPositionalArgs())` + `WithLazyTx(true)` (`driver.go:130-143, 200-207`) — корректно для query service.
- `database/sql` без `maxOpenConns`: `GetMaxOpenConns()` возвращает 0 = без ограничения (`config.go:557-562`). В облаке опция не передаётся (`grep maxOpenConns` в stroppy-cloud — 0 совпадений). Документация YDB: нативный SDK ограничивает 50 сессий, `database/sql` — нет; лимит должен быть ≈ числу одновременных запросов; на сервере каждая сессия — актор. [OFF: recipes/ydb-sdk/session-pool-limit]
- Официальное руководство TPC-C: сессий = ядер dynnode × 5–10. Для C 4 vCPU это 20–40, а VU = 128. [OFF]
- Ретраи: классификация ошибок через `ydbretry.Check` с backoff (`pkg/driver/ydb/errors.go`), но число попыток `retry-attempts=3` по умолчанию (`tpcc.go:135`). Для serializable с высокой конкуренцией на 100 складах 3 попыток недостаточно: транзакция превращается в ошибку и попадает в латентность/процент отказов. Документация YDB для SDK: оборачивать в retry-функции (`retry.Do`/`DoTx`) с экспоненциальным backoff и без жёсткого малого предела. [OFF/PRAC]
- Старт: `primaryConnectTimeout = 3s` и откат на YC metadata-аутентификацию (`driver.go:26, 95-117`): при ошибке без явных учётных данных ловит ошибку; с `serviceAccountKeyFile` откат не выполняется.

### 3.3. Что нужно сделать для честного TPC-C

1. Определить режим: **paced** (ключевые и «думательные» времена, VU ≈ 10 × складов; метрика — tpmC и эффективность `tpmC / (12.86 × W)`), либо **unpaced stress** (макс. throughput, без verdict). Нельзя смешивать: 128 VU unpaced на 100 складах — стресс, а не TPC-C-результат; 45–53 tps в paced режиме = потолок (см. 0.6).
2. Передавать в драйвер `sql.maxOpenConns = maxIdleConns = clamp(C_cores_total × 10, 32, VU)` и `connMaxIdleTime≈1s` (как в доке про `database/sql`); отдельный прогон с `maxOpenConns=VU`.
3. `retry-attempts` ≥ 10 (для serializable) и показывать в отчёте число ретраев (`tpcc_retry_attempts` уже есть).
4. Перед измерением: `warmup` ≥ 3–5 мин (`--warmup`, constant-vus), потому что автосплит индексов и `item` происходит под нагрузкой; после загрузки — `ALTER TABLE … COMPACT` (26.2+) или `ydb workload tpcc import --compact`.
5. Схема: либо предразбить индексы (проверить, принимает ли `ALTER INDEX … SET` партиционные параметры на 25.4–26.3 — сверить `.sys/partition_stats`), либо построить индексы до загрузки и дождаться сплитов; `history`: хэш-префикс или `Uint64` ключ с `UNIFORM_PARTITIONS`; `item` — явный `PARTITION_AT_KEYS` по `i_id` (ключ `Int64`, значения 1..100000 — разбиение допустимо через AT_KEYS).
6. Контрольный эксперимент: на том же кластере запустить `ydb workload tpcc` (`init/import/run`, `--max-sessions` и `--threads` по формулам из доки). Если он даёт кратно лучший результат, проблема в схеме/драйвере stroppy; если тот же уровень — в кластере (CPU/диск/группы).
7. Сравнимость с Postgres/MySQL: у YDB всегда serializable, у pg/mysql в stroppy repeatable_read; писать это в отчёт.

---

## 4. Надёжность: что гарантирует YDB

| Режим | Гарантии | Паритет |
|---|---|---|
| `erasure: none`, `single` | Одна копия данных на одном pdisk (файл!). Потеря узла или диска = потеря данных. Документация: «only for development and testing». [OFF topology] Запись подтверждается после записи в журнал pdisk (подтверждение в рамках одного узла; без cross-node копий). | Эквивалент PostgreSQL/MySQL single без репликации. Серьёзная оговорка: pdisk — файл на ФС; fsync-семантика зависит от ФС и сетевого диска [?]. |
| `block-4-2` (≥8 доменов, одна зона) | 4+2: переживает потерю двух доменов, оверхед 1.5×. | Аналог pg с 2 синхронными репликами по надёжности, не по задержке. |
| `mirror-3-dc` (≥9 узлов, 3 зоны) | 3 реплики по 3 зонам, переживает потерю зоны + 1 домена в другой зоне; оверхед 3×; запись подтверждается кворумом. Для сохранения работы при потере зоны нужны ~2× I/O и ёмкость storage и +35% CPU/RAM на database-узлах. [OFF topology] | Ближе всего к синхронному 3-узловому кластеру (Patroni sync / Galera / GR): запись ждёт кворума между зонами (RTT зон YC). |

Для сравнимых прогонов: `single` ↔ PG/MySQL одиночный; `mirror-3-dc` ↔ 3-узловые синхронные топологии. В mirror-3-dc у нас 3 «pdisk» на узел лежат на одном сетевом диске; отказоустойчивость на уровне узла есть, на уровне дисков узла — нет. Лимит по честности: в `fail_domain=disk` схема позволила бы собрать 9 «доменов» из 3 узлов — рецепт этого не делает (комментарий `ydb.go:41`), по узлам нормально.

---

## 5. Изменения платформы (пути файлов)

| # | Изменение | Файлы |
|---|---|---|
| 1 | Заполнять `cpu_count` из `spec.Machine.CPU` по роли (формулы 2.1) через оверлей рендера; добавить параметры `actor_cpu_reserve`, `cpu_count` override | `internal/domain/compile/recipe_distributed.go` (оверлей около `:306`, `:360`), `pipelines/schemas/dbparams/ydb.go`, `pipelines/schemas/cfg/ydb_config_yaml.go` |
| 2 | Передавать `memory_hard_limit_bytes` и профильные `memory_*` из `M` и роли | то же + `ydb_config_yaml.go:222-239` |
| 3 | Реализовать `auto_size_pdisks` и размер pdisk из диска; убрать константу `ydbPdiskGB=40` (мин. 80) и проверку `compile.go:169-176` пересчитать | `recipe_distributed.go:260, :313-317`, `compile.go:169-176`, `dbkind/ydb.go:73` |
| 4 | Подключить `disk_type`, `failure_domain`, `storage_groups` как профильные формулы; удалить или реализовать мёртвые параметры (в т.ч. `fail_domains`, правила) | `dbparams/ydb.go`, `recipe_distributed.go`, `dbkind/ydb.go` |
| 5 | Профиль `auto`/`diagnostic`/`custom`: ввести поле `tuning` и порядок: профиль → пользовательские переопределения | `dbparams/ydb.go`, рецепт, UI `web/` + локали (Новое значение = домен + контракт + локали по `AGENTS.md`) |
| 6 | Хост: `HostPrepSysctl` (THP, overcommit, aio-max-nr, swappiness) и `Container.Ulimits` (`nofile`, `memlock`) для ydb; `network_mode: host` | `recipe_distributed.go`, `pipelines/spec/spec.go:161, 213`, провижн контейнеров в `pipelines/` |
| 7 | Размеры по умолчанию: рекомендации по ядрам C ≥ 8 (warn при <8), диск `io-m3`/`nonreplicated` для S | `dbkind/ydb.go:73-74` (`Requirements`), таблицы размеров провайдера, правила `dbparams/ydb.go` |
| 8 | Поддержка `raw`-pdisk: отдельный диск ВМ и `cfg.host.disks` raw | `cfg/host_disks.go` (уже умеет), провижн дисков, `recipe_distributed.go` |
| 9 | Клиентские параметры: `sql.maxOpenConns/maxIdleConns/connMaxIdleTime`, `retry-attempts`, `pacing`, `warmup` в сегменте workload по формулам 3.3 | `pipelines/schemas/workload/segment.go`, `internal/domain/compile/compile.go` (workload driver options) |
| 10 | Диагностика: вывести в отчёт прогона число партиций таблиц/индексов (`.sys/partition_stats`) и счётчики TLI | сайдкар/сбор метрик (`Scrape: /counters/prometheus` уже есть) |
| 11 | Обновить комментарий-TODO про grpc_config и сверить ключи по sample-конфигам | `ydb_config_yaml.go:141-144` |

Порядок внедрения (дешёвое и самое вероятное — первым): (1) cpu_count; (2) размер/тип диска + pdisk; (3) groups; (4) maxOpenConns/retry/pacing; (5) схема индексов/`history`/`item`; (6) THP/ulimits/host network; (7) память.

---

## 6. Managed YDB

### 6.1. Что настраивается

Параметры `dbparams/ydb_managed.go` и `provision/yandex_ydb.go`:

- Dedicated: `resource_preset_id` (medium 8/32, medium-m64, medium-m96, large 12/48, xlarge 16/64, oltp-c16-m128, olap-c16-m128), `node_count` (fixed), `storage_groups` (ssd: до ~100 ГБ на группу), `storage_type_id`, подсети в 3 зонах, `assign_public_ips`, `location_id`. Автоскейл — не поддерживается провайдером Crossplane (`recipe_ydb_managed.go:30-31`).
- Serverless: `throttling_rcu_limit`, `provisioned_rcu_limit`, `storage_size_limit_gb`.
- Нельзя: `config.yaml`, пулы акторов, память, feature flags, pdisk/VDisk-геометрию, erasure (мы не управляем). Единственные рычаги — размер пресета, число узлов, число групп, и настройки на уровне таблиц (партиционирование) и клиента.
- Квоты dedicated на облако [OFF yandex.cloud limits]: 64 ядра, 256 ГБ, 8 хостов, 4 БД, **8 групп хранения**; serverless: 1000 RU/s на облако (`ydb.serverlessRequestUnitsPerSecond`), 10 БД, 300 ГБ. Throttling по умолчанию 10 RU/s (в платформе `EnableThrottlingRcuLimit=false` при лимите 0 — без троттлинга, но облачная квота остаётся). TPC-C на serverless упрётся в квоту RU/s — результаты несравнимы с self-hosted; управляемый dedicated — единственный осмысленный вариант для замера. [OFF]
- Метрики managed YDB — отдельная задача (в STROPPY.MD §424-426).

### 6.2. `PermissionDenied: Storage group creation ... not allowed in location ru-central1` (dedicated)

Точной строки нет ни в публичной документации, ни в поиске. Гипотезы (от вероятной к менее вероятной) и проверки без `yc`:

1. Для облака/каталога не разрешено создание dedicated-групп хранения в этой локации (фича/ACL на стороне YDB, а не IAM каталога) — тип ошибки `PermissionDenied`, а не квота (`ResourceExhausted`). Проверка: создать dedicated БД вручную в консоли в том же каталоге с теми же параметрами (storage type ssd, 1 группа); если консоль тоже отказывает — обращение в поддержку с request id. Проверить страницу квот облака `ydb.dedicatedStorageGroups.count` (по умолчанию 8; ноль даёт другую ошибку, но проверить).
2. `location_id` передаётся как `ru-central1` (`recipe_ydb_managed.go:25`), а ресурс dedicated принимает конкретное значение локации/зоны или вообще не должен получать `location_id` (в Terraform-провайдере поле опционально). Проверка: сравнить с примером `yandex_ydb_database_dedicated` (location_id необязателен); повторить без `LocationID`.
3. `storage_type_id=ssd` недоступен в данной локации/для облака — список типов смотреть в консоли (`yc ydb storage-type list` на стороне пользователя, нам нельзя). Проверка: перебрать допустимые типы в консоли.
4. Права сервисного аккаунта Crossplane: нужны `ydb.editor`/`ydb.admin`, `vpc.user`, `compute.*` для nodes group — но в таком случае формулировка обычно про роль. Проверка: роли SA на каталоге.
5. Зоны: подсети создаются в 3 зонах (`threeYandexZones`); если одна из зон (например `ru-central1-d`) недоступна облаку/dedicated YDB, ошибка может приходить как PermissionDenied. Проверка: принудительно a/b/d.

### 6.3. Serverless: `Service unavailable` на `ydb.serverless.yandexcloud.net:2135`

В `provision/yandex.go` runner получает либо публичный IP, либо общий egress NAT (`NeedsPrivateEgress`, `:112-114`), SG egress «all» (`:96-97`) — сеть вероятно ни при чём, но проверить. Гипотезы:

1. **TLS/CA.** `isYandexDedicatedEndpoint` (`pkg/driver/ydb/driver.go:243-251`) добавляет `yc.WithInternalCA()` только для хостов `*.ydb.mdb.yandexcloud.net`; для `ydb.serverless.yandexcloud.net` внутренний CA не добавляется. Если системный пул контейнера пуст/не содержит нужный корень, рукопожатие падает, а SDK сообщает `Unavailable` («connection error», discovery не дождался). Проверка с раннера: `openssl s_client -connect ydb.serverless.yandexcloud.net:2135 -showcerts`; запуск ydb CLI/`grpcurl` с и без `--ca-file`.
2. **Готовность БД.** Ресурс Crossplane `Ready` и `YdbFullEndpoint` бывают заполнены раньше, чем БД реально принимает discovery (`yandexServerlessReady`, `yandex_ydb.go:68-70`). Проверка: добавить ожидание успешного `discovery` в ретрае (например, 2–5 мин) вместо одного подключения.
3. **Таймаут.** `primaryConnectTimeout=3s` на первое подключение (`driver.go:26`); холодный serverless может отвечать дольше → `Unavailable`. Проверка: увеличить таймаут/ретраить `WaitForDB`.
4. **Права/IAM.** Служебный аккаунт ключа без роли `ydb.editor` на БД возвращает Unauthenticated/PermissionDenied, но может маскироваться откатом на metadata-креды. Проверка: логи драйвера (Debug) и роль SA.
5. **Egress/DNS/прокси.** Проверка `nc -vz ydb.serverless.yandexcloud.net 2135` и `getent hosts` из ВМ раннера; отсутствие IPv6; переменные `HTTP(S)_PROXY`.
6. **Квота RU.** На connect не влияет (проявляется как `Throughput limit exceeded`).

Рекомендация: добавить в пайплайн предварительную проверку (`ydb discovery list`-аналог / Go-проба) с различением ошибок TLS / DNS / auth / unavailable и логированием первого реального gRPC-кода.

---

## 7. Риски и открытые вопросы

Риски:
1. Часть выводов (группы, THP-значение, `memlock`, ключ `expected_slot_count`) основана на практике и памяти; помечена [PRAC]/[?]. Нужен A/B на стенде (по одному фактору за раз, ≥3 повтора).
2. Увеличение дисков/ядер меняет цену прогонов: C 16 vCPU + S io-m3 заметно дороже 4+8. Профиль должен показывать стоимость и предупреждать.
3. `network-ssd-nonreplicated` — без репликации диска: при erasure none потеря диска = потеря данных; допустимо только для одноразовых стендов; non-replicated ограничены 8 дисками на placement group.
4. Файл-pdisk на сетевом диске — недокументированный режим производительности; настоящая проверка — отдельный блочный диск (raw).
5. YDB 26.3 в changelog — RC; цифры 26.3 не сравнивать напрямую с 26.2 без проверки версии образа.
6. Сравнимость с PG/MySQL: serializable vs repeatable_read, оптимистичные блокировки (повторы), разный режим pacing — фиксировать в метаданных прогона.
7. `AUTO_PARTITIONING_MAX_PARTITIONS_COUNT = число складов` в схеме stroppy запрещает сплиты таблиц сверх склада; для горячих складов это ограничение throughput.

Открытые вопросы:
- Включён ли `pacing` в матричных прогонах? (объясняет 45–53 tps; см. 0.6)
- Какой фактический размер диска db-1 (`RecommendedGB` vs `40*pdisks+20`) и его IOPS? Какой образ ВМ (ядро)?
- Сколько партиций у `idx_order`/`idx_customer_name`/`item`/`history` через 1 и 10 минут нагрузки (`.sys/partition_stats`) и принимает ли ALTER INDEX партиционные параметры?
- Значение `ExpectedSlotCount` по умолчанию для SSD-pdisk-файла и YAML-ключ в `host_configs` для этой версии.
- Достижимость имён узлов в discovery при Docker bridge: возвращает ли discovery адреса контейнеров или хостов?
- Что именно разрешает/запрещает YDB dedicated в `ru-central1` для данного облака — вопрос в поддержку YC.
- Как ведут себя метрики `GetRunMetrics` по ydb (счётчики `/counters/prometheus`) для оценки загрузки пулов (System/User/IC): добавить в дашборд мониторинга pool utilization — это главный индикатор для ручного тюнинга пулов.

---

## Источники

- YDB: actor_system_config — https://ydb.tech/docs/en/reference/configuration/actor_system_config
- YDB: memory_controller_config — https://ydb.tech/docs/en/reference/configuration/memory_controller_config
- YDB: table_service_config — https://ydb.tech/docs/en/reference/configuration/table_service_config
- YDB: resource_broker_config — https://ydb.tech/docs/en/reference/configuration/resource_broker_config
- YDB: feature_flags — https://ydb.tech/docs/en/reference/configuration/feature_flags
- YDB: blob_storage_config / host_configs — https://ydb.tech/docs/en/reference/configuration/blob_storage_config , https://ydb.tech/docs/en/reference/configuration/host_configs
- YDB: system requirements — https://ydb.tech/docs/en/devops/concepts/system-requirements , https://ydb.tech/docs/en/devops/system-requirements
- YDB: deployment preparation — https://ydb.tech/docs/en/devops/deployment-options/manual/initial-deployment/deployment-preparation
- YDB: topology — https://ydb.tech/docs/en/concepts/topology ; glossary — https://ydb.tech/docs/en/concepts/glossary
- YDB: row table partitioning — https://ydb.tech/docs/en/concepts/datamodel/table
- YDB: workload tpcc — https://ydb.tech/docs/en/reference/ydb-cli/workload-tpcc
- YDB: session pool limit — https://ydb.tech/docs/en/recipes/ydb-sdk/session-pool-limit
- YDB: changelog — https://ydb.tech/docs/en/changelog-server
- YDB source: auto actor-system table — https://raw.githubusercontent.com/ydb-platform/ydb/main/ydb/core/driver_lib/run/auto_config_initializer.cpp
- YDB blog TPC-C (403 на загрузку, только по поиску) — https://blog.ydb.tech/ydb-meets-tpc-c-distributed-transactions-performance-now-revealed-42f1ed44bd73
- Yandex Cloud: disk limits — https://yandex.cloud/en/docs/compute/concepts/limits
- Yandex Cloud: YDB serverless vs dedicated — https://yandex.cloud/en/docs/ydb/concepts/serverless-and-dedicated ; YDB quotas — https://yandex.cloud/en/docs/ydb/concepts/limits
- Код: `/home/yaroher/devel/github/stroppy-io/stroppy-cloud/internal/domain/compile/recipe_distributed.go`, `.../recipe_ydb_managed.go`, `.../compile.go`, `.../internal/domain/dbkind/ydb.go`, `.../pipelines/schemas/dbparams/ydb.go`, `.../pipelines/schemas/cfg/ydb_config_yaml.go`, `.../host_sysctl.go`, `.../host_disks.go`, `.../pipelines/internal/provision/yandex_ydb.go`; stroppy: `/home/yaroher/devel/github/stroppy-io/stroppy/workloads/tpcc/{ydb.sql,tpcc.go,config.go}`, `pkg/driver/ydb/{driver.go,errors.go}`.
