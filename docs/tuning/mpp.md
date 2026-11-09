# Семейство Greenplum: профиль тюнинга `auto`

Greenplum, Greengage, WarehousePG, OpenGPDB и Apache Cloudberry — пять движков
с одной формой развёртывания и одним набором параметров
(`db.<kind>.params@1`, конфиги `cfg.mpp.gpinitsystem@1` и `cfg.mpp.gucs@1`).
Код: `internal/domain/dbkind/mpp.go` (каталог, план машин),
`internal/domain/compile/recipe_mpp.go`, `recipe_mpp_scripts.go`, `mpp_ssh.go`
(развёртывание), `tune_mpp.go` (профиль), `workload_mpp.go` (диалект stroppy).

Источники (исследование от 2026-11-09; настройки нигде не сверены прогоном на кластере в YC):

- D6 / D7 — документация Greenplum 6 / 7 на techdocs.broadcom.com:
  `best_practices-sysconfig`, `admin_guide-wlmgmt_intro`, `install_guide-prep_os`,
  `public_cloud_requirements`, `ref_guide-config_params-guc-list`,
  `utility_guide-ref-gpinitsystem`, `utility_guide-ref-gpconfig`;
- CB — документация Cloudberry (cloudberry.apache.org): `sizing/methodology`,
  `config-params-guc-list`, `best-practices/system-configuration`;
- SYNX — отчёт SynxDB о TPC-DS 1 ТБ на Greenplum 6.27.1, 7.1.0 и SynxDB
  (docs.synxdata.com/synxdb-4/product-overview/tpc-ds-benchmark-report.html), единственный
  открытый отчёт с полным списком GUC; хосты 16 vCPU / 128 ГБ, 8 primary + 8 mirror;
- KIT — открытые наборы для TPC-H/TPC-DS: cloudberry-contrib/TPC-H-Toolkit
  (`tpch_tools/set_gucs.sh`), cloudberry-contrib/TPC-DS-Toolkit, avamingli/cbdb_tpcds,
  pivotalguru/TPC-DS;
- MPP — `github.com/stroppy-io/mpp-builds` (README: контракт образов и форма развёртывания);
- GG — документация Greengage (greengagedb.org/en/docs-gg/7);
- свои проверки — прогон формы развёртывания на трёх контейнерах (раздел 6).

Метки: [док] — прямо в документации; [отчёт] — в отчёте/наборе вендора; [код] — проверено по
исходникам форка; [наше] — наше решение, вендор советов не даёт; [проверено] — проверено
запуском на реальных образах.

## 1. Что развёртывается

Одна машина — один контейнер образа движка, сеть `host`, диск данных в `/data/mpp`.
Топология: `coordinator` (1 машина) и `db` (`segment_hosts` машин, 1–32). Зеркал и standby
координатора нет и не предлагается: упавший сегмент останавливает кластер. Размеры машин
выбирает пользователь; пол: координатор 2 vCPU / 8 ГБ, сегмент-хост 4 vCPU / 16 ГБ (при заданном
`segments_per_host` — не меньше 2 vCPU и 8 ГБ на сегмент) [док D7 public_cloud_requirements:
не меньше 8 ГБ на сегмент-процесс].

- Один ключ ed25519 на весь кластер. Он выводится из id прогона (`sha256("stroppy-mpp-ssh:"+runID)`),
  а не берётся случайным: RunSpec — чистая функция входа. Ключ открывает только sshd контейнеров
  этого прогона (порт 2222, доступен из подсетей прогона), поэтому вывод из публичного id ничего
  не отнимает у случайного ключа. Файлы: `/etc/mpp/ssh/{id_ed25519,id_ed25519.pub,authorized_keys}`.
- Контейнеры сегмент-хостов только держат sshd (CMD образа). Контейнер координатора запускает
  `mpp-init.sh` от root как CMD entrypoint'а: ждёт sshd каждого хоста и свой; спрашивает у каждого
  хоста `hostname` и пишет `IP имя` в `/etc/hosts` координатора (имя ВМ при компиляции неизвестно,
  утилиты движка ходят по имени, которое хост про себя сообщает; имя, привязанное к loopback,
  вычищается); `mpp-cluster.sh` запускается от gpadmin **по ssh** (`pam_limits` и `$USER`, без которых
  `gpconfig` и `gpinitsystem` не работают): `gpinitsystem -a` (код 0 и 1 — успех), `gpconfig`, `gpstop -ar`,
  роль `stroppy` и база `stroppy`, записи `pg_hba` (`127.0.0.1`, подсеть координатора, runner), `gpstop -u`.
  Готовность — файл `/tmp/stroppy-mpp-ready`; healthcheck контейнера координатора проходит только
  при нём, так что нагрузка стартует на готовом кластере. После перезапуска контейнера
  скрипт видит кластер и делает `gpstart -a`.
- Имена ключей `gpinitsystem_config`: `MASTER_*` на основе PostgreSQL 9.4 (6.x), `COORDINATOR_*` на 12 и 14
  [MPP README]. Опция «только координатор» у `gpconfig`: `--masteronly` / `--coordinatoronly`.
- Клиент: `postgresql://stroppy:stroppy@<координатор>:5432/stroppy`, протокол `pg`.
- Экспортёры: node_exporter на всех машинах, postgres_exporter на координаторе. Коллекторы
  проверены на всех девяти образах: отключены `wal` (нужен суперпользователь), `stat_statements`
  (расширения нет), `process_idle`, `stat_checkpointer` (PostgreSQL 17+), `stat_user_tables` и
  `long_running_transactions` (падают на части форков); на основе PostgreSQL 9.4 (6.x) ещё
  `database_wraparound`, `stat_wal_receiver`, `replication`, `replication_slot`,
  `stat_progress_vacuum` (нет представлений), и там нет метрик `pg_stat_activity` (в 9.4 нет
  `backend_type`). На Cloudberry 2.1.0 запрос настроек экспортёра падает на NULL, поэтому
  `pg_exporter_last_scrape_error` там остаётся 1, остальные метрики отдаются.
- Сценарии: допустимы `tpch/tx`, `tpcds`, `simple`, `execute_sql`; `tpcc/*`, `tpcb/*`, `banking*`,
  `baseline` валидатор отказывает с причиной: движки аналитические, транзакционные смеси вне
  задачи этих kind'ов.

## 2. Профиль `benchmark`

Единого вендорского «benchmark-автотюна» нет. Вендорские формулы памяти (D6/D7) рассчитаны на
безопасность продакшена и отдают сегментам около половины памяти хоста; отчёт SynxDB отдаёт 91%.
Профиль `auto` следует отчёту: цель — измерить движок, а не его защитный режим. Это
не продакшен-настройки, и в отчёте прогона режим назван `auto`. Профиль «официальный
безопасный» (формулы D6/D7) в продукт не включён [решение].

Входы: `RAM` — память машины, `P` — сегментов на хосте (`segments_per_host`, при 0: `max(1, vCPU/2)`
сегмент-машины [ADB, отчёт: SYNX P=vCPU/2]), `Q` — запросов одновременно (VU самого
многопоточного сегмента, по умолчанию 1). Список сегментов один на роль и считается по самой
маленькой машине роли. Для координатора `P = 1`.

| Параметр | auto | Источник |
|---|---|---|
| `gp_vmem_protect_limit` | `0.9 · RAM / P` МБ (128 ГБ, P=16: 7372; SynxDB: 15000 × 8 = 91% хоста) | [отчёт SYNX]; движок по умолчанию 8192 МБ на любой хост [код], поэтому на большой машине без правки он ограничивает |
| `max_statement_mem` | `0.9 · gp_vmem_protect_limit` | [отчёт SYNX] 14000 из 15000 = 93%; [наше] 90%; пишется **перед** `statement_mem` (`statement_mem` обязан быть строго меньше, дефолт 2000 МБ) [код] |
| `statement_mem` | `max(125 МБ, 0.75 · gp_vmem_protect_limit / Q)` | [отчёт SYNX] 12000 из 15000 = 80%; официальная формула D6/D7: `0.9 · limit / Q`; [наше] 75%, деление на `Q` — чтобы параллельные запросы не обещали одну память дважды |
| `work_mem` | `clamp(0.1 · RAM / P, 32 МБ, 1 ГБ)` | [KIT avamingli] `min(1 ГБ, RAM · 10% / сегментов)`; D7: не связана с `statement_mem`; влияние на исполнитель в исходниках не проверено |
| `effective_cache_size` | `0.5 · RAM / P` | [наше]: оценка кэша ОС на один сегмент для PostgreSQL-планировщика; GPORCA не читает |
| `shared_buffers` | не задаётся (125 МБ) | [KIT] «128 МБ против 256 МБ — разницы нет», все наборы оставляют 128 МБ |
| `runaway_detector_activation_percent` | 100 | [KIT cloudberry-contrib/TPC-H-Toolkit]; по умолчанию 90, а предел уже 90% хоста |
| `gp_resource_manager` | `queue` на всех | [отчёт] образы без настройки cgroup; `group` требует cgroup v1 (6.x) и подготовленного хоста; 7.x по умолчанию `none`, явное `queue` делает движки сравнимыми |
| `gp_autostats_mode` | `none` | [KIT] все наборы; в 6.x по умолчанию `on_no_stats` [код]. stroppy сам делает `ANALYZE` после загрузки (`tpch/tx`, `tpcds`), и автостатистика только дублировала бы его во время загрузки. Поставить «на время загрузки» нельзя (GUC на уровне кластера) |
| `autovacuum` | `off` (7.x, 8.x, Cloudberry) | [отчёт SYNX], [KIT]; в 6.x `gpconfig` отказывает («cannot be changed under normal conditions») — там по умолчанию и так off [проверено] |
| `optimizer` | `on` для `orca`, `off` для `postgres` — параметр БД | задан пользователем, пишется и в `stock`. Источники расходятся (ORCA быстрее по статье 2014 и наборам Cloudberry/Pivotal; наборы avamingli: «планировщик на 77% быстрее» без масштаба): правильно прогнать оба варианта на каждом движке |
| `gp_max_packet_size` | 1420 | [док D7 public_cloud_requirements, D6 prep_os]: для MTU 1500; дефолт 8192 рассчитан на jumbo-кадры. MTU Yandex Cloud в документации: «как правило 1500», jumbo-кадры не упомянуты — **не проверено**, замерить `ip link` / `tracepath` |
| `gp_interconnect_queue_depth` / `_snd_queue_depth` | 16 / 16 | [KIT] два независимых набора (TPC-H toolkit, avamingli); SynxDB: 10 / 8; дефолты 4 / 2 |
| `gp_interconnect_type` | не задаётся (`udpifc`) | [док D7] единственное поддерживаемое значение; дефолт во всех форках |
| `max_connections`, координатор | `max(250, ceil((пулы · пул + 50) / 50) · 50)` | дефолт `gpinitsystem` 250 [код]; пул клиента — как у PostgreSQL |
| `max_connections`, сегменты | `3 ×` значения координатора | [док D7 GUC]: сегментов не меньше, чем у координатора (`gpconfig` отказывает, если не больше — [проверено]); дефолт 750 |
| `gp_fts_probe_timeout` / `gp_fts_probe_interval` / `gp_segment_connect_timeout` | 600 с / 600 с / 1800 с | [отчёт SYNX]; без зеркал «упавший» под нагрузкой сегмент останавливает кластер, а не переключается |
| `max_wal_size` (12 и 14) | 4 ГБ | [наше]: вендоры не дают советов по загрузке; дефолт 1 ГБ |
| `checkpoint_segments` (6.x) | дефолт движка | `gpconfig` отказывает менять его, а `CHECK_POINT_SEGMENTS` из `gpinitsystem_config` доходит только до координатора (сегменты остаются на 8) [проверено] |
| `log_*` | дефолты | `log_min_messages = warning`, без `log_statement` — уже тихо |
| `gp_workfile_compression`, `gp_resqueue_memory_policy` | дефолты | [KIT] расходятся; `eager_free` и так в `postgresql.conf.sample` всех форков |

Пример: сегмент-хост 32 vCPU / 128 ГБ, P = 16, Q = 1 — `gp_vmem_protect_limit` 7372, `max_statement_mem`
6634 МБ, `statement_mem` 5529 МБ, `work_mem` 819 МБ, `effective_cache_size` 4096 МБ, `max_connections` 750
(координатор 250). Координатор 2 vCPU / 8 ГБ: те же значения памяти, один экземпляр.

## 3. Ядро хоста

Применяется машине с контейнером базы (общий механизм `cfg.host.sysctl@1`, `host.go`):

| Параметр | Значение | Источник |
|---|---|---|
| `vm.overcommit_memory` | 2 | [док D6/D7 prep_os] |
| `vm.overcommit_ratio` | `floor(100 · (RAM − 0.026 · Σ gp_vmem_protect_limit) / RAM)`, при 90% хоста — 97 | формула D6/D7 `100 · (RAM − 0.026 · gp_vmem) / RAM` с реальной суммой лимитов; примеры вендора — 95 |
| `vm.dirty_background_ratio` / `vm.dirty_ratio` | 3 / 10 до 64 ГБ RAM; выше — ближайшие целые проценты к 1.5 и 4 ГиБ (128 ГБ: 1 / 3) | [док D7 prep_os]: до 64 ГБ именно эти ratio, выше — байты (1.5 / 4 ГиБ), которых в схеме нет |
| `vm.min_free_kbytes` | 3% RAM | [док D7 prep_os]: 3%, не больше 5% |
| `vm.swappiness` | 1 (дефолт схемы) | [док] |
| `transparent_hugepage` | `never` (дефолт схемы) | [док] |
| `net.core.rmem_max` / `wmem_max` | 16777216 (дефолт схемы) | [док D6/D7 prep_os] |
| `nofile` контейнера | 1048576 (общий механизм) | [док] 524288 для gpadmin; выше — запас |

Строгий overcommit при 90% памяти сегментам оставляет ядру 3% RAM сверху вместе с общей памятью
сегментов и самим хостом: на машине 16 ГБ запас около 500 МБ. Для маленьких машин это тесно.

**Не применяется** (в продукте для этого нет механизма либо это не относится к контейнеру):
`kernel.shmmax/shmall/shmmni/sem/msg*` (IPC-пространство имён контейнера, не хоста); `ipfrag_*`
(при `gp_max_packet_size = 1420` фрагментации нет); `blockdev --setra`, планировщик ввода-вывода,
опции монтирования XFS (`noatime` и прочее — общий слой дисков, не специфика движка);
`net.core.netdev_max_backlog`, `tcp_*`; `RemoveIPC=no`; `MaxStartups` sshd (sshd внутри образа,
`gpinitsystem` держит до 16 сессий — умещается в дефолт); `ip_local_port_range` (дефолт 32768+ не
пересекается с `PORT_BASE` 6000).

## 4. `stock`

Движок с собственными дефолтами. Пишутся только `optimizer` (параметр БД) и то, без чего кластер не
соберётся: `gpinitsystem_config`, роль и база `stroppy`, `pg_hba`. Ядро хоста и `nofile` не трогаются.

## 5. Диалект stroppy

Аналитические сценарии на этих движках нужны с `--sql-dialect gp` (stroppy-io/stroppy#259): иначе
таблицы создаются без `DISTRIBUTED BY`, и числа несопоставимы. Платформа выставляет `gp`, когда
сценарий в каталоге stroppy перечисляет флаг `sql-dialect` (`workload_mpp.go`). В схеме
`workload.segment@1` поля `sql_dialect` для `tpch/tx` и `tpcds` ещё нет: его добавление ломает
контракт со stroppy 6.3.3 (`TestStroppyParameterContract`), поэтому оно ждёт релиза stroppy с флагом
и обновления слепка `pipelines/stroppycfg/testdata/stroppy-probe.json`.

## 6. Что проверено запуском

Форма развёртывания (скрипты и файлы ровно такими, как их собирает компилятор для 1 координатора и
2 сегмент-хостов по 2 сегмента) запущена на реальных образах по трём контейнерам на docker-сети:
`greengage:7.5.1`, `greengage:6.32.0`, `greenplum:6.27.1`, `greenplum:7.1.0`, `warehousepg:6.27.7`,
`warehousepg:7.6.1`, `opengpdb:6.30.2`, `cloudberry:2.1.0-incubating`, `greengage:8.0.0-alpha.1`.
Кластер поднимается, все GUC применены (`gpconfig -s`), `optimizer` совпадает с параметром,
`stroppy` входит по паролю с другого хоста. На `greengage:7.5.1` дополнительно: `stroppy` из ветки с
диалектом `gp` отработал `tpch/tx` (22 запроса), `tpcds` и `simple` без ошибок, перезапуск
контейнера координатора возвращает работающий кластер.

Не проверено: ядро хоста (`sysctl`, на стендах без прав), настоящие отдельные ВМ с
`--network host` (ввод — мост docker), MTU и jumbo-кадры в Yandex Cloud, зеркала и standby (не
предлагаются), IPv6, влияние `work_mem` на исполнитель, ORCA против планировщика на SF ≥ 100.
