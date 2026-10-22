# CockroachDB: профиль тюнинга `auto` (версия после сверки с документацией)

Источники (все открыты в этой сессии; docs.cockroachlabs.com — новый домен, www.cockroachlabs.com/docs редиректит 301 туда):
- PC = https://docs.cockroachlabs.com/docs/stable/recommended-production-settings (production checklist)
- START = https://docs.cockroachlabs.com/docs/stable/cockroach-start (+ v24.1, v24.3, v25.4 той же страницы)
- CS = https://docs.cockroachlabs.com/docs/stable/cluster-settings
- TX = https://docs.cockroachlabs.com/docs/stable/transactions
- RC = https://docs.cockroachlabs.com/docs/stable/read-committed
- AC = https://docs.cockroachlabs.com/docs/stable/admission-control
- PERF = https://docs.cockroachlabs.com/docs/v25.2/performance
- TPCC-L = https://docs.cockroachlabs.com/docs/v24.3/performance-benchmarking-with-tpcc-large
- WL = https://docs.cockroachlabs.com/docs/stable/cockroach-workload
- GEN = https://docs.cockroachlabs.com/docs/stable/cockroach-gen
- LIC = https://docs.cockroachlabs.com/docs/stable/licensing-faqs
- REL = https://docs.cockroachlabs.com/docs/releases/v24.3, .../v26.2
- YC = https://yandex.cloud/en/docs/compute/concepts/limits
- исходники cockroach (GitHub, как подтверждение существования настройки): https://github.com/cockroachdb/cockroach
- stroppy: локальный чекаут /home/yaroher/devel/github/stroppy-io/stroppy, коммит 83342f99 (main = origin/main, 2026-10-14), репо https://github.com/stroppy-io/stroppy

Метки: [офиц] — прямо в документации по ссылке; [практика] — вывод, нужен A/B; [код] — проверено по исходникам. Где официального источника нет, это сказано словами «не нашёл официального источника».

## 1. Текущее состояние (в репо stroppy-cloud)

### Что рисуется
- `pipelines/schemas/cfg/cockroach_flags.go`: фабрика `cockroachFlags(major)` (@24, @25, @26). Дефолты: `store_size=90%`, `cache=25%`, `max_sql_memory=25%`, `max_tsdb_memory=64MiB`, `max_offset=500ms`, `insecure=true`, `log` только stderr/WARNING, `max_disk_temp_storage=32GiB` (только @25+), `wal_failover=disabled` (только @25+); шаблоны `conf` и `settings`; `cluster_settings` — пустая map.
- `internal/domain/compile/recipe_distributed.go:26-97` — `cockroachRecipe`: `start-single-node` для 1 узла, `start --join=<все>` иначе; init-sidecar `<node1>-init`: `cockroach init`, ожидание `SELECT 1`, `settings.sql`, `init_sql`.
- `internal/domain/dbkind/cockroach.go`: роль db -> `cfg.cockroach.flags@25`, Requirements db 4 vCPU/8 ГБ/40 ГБ, proxy 2/2/10.
- HAProxy: `internal/domain/compile/recipe.go:86-124` (общий): `mode tcp`, `balance leastconn`, проверка `kind: tcp`; для cockroach `Check` не переопределяется (`recipe_distributed.go:95`).
- Хост: `cfg.host.sysctl@1`, `cfg.host.disks@1` — универсальные, cockroach-специфики нет.

### Чего нет
1. Профиля `auto` нет: все значения статические, не зависят от RAM/vCPU/диска/числа узлов.
2. `settings.sql` пуст: ни одной кластерной настройки (а официальный TPC-C гайд ставит несколько, см. 2.4).
3. Нет `Ulimits` у контейнера (поле есть: `pipelines/spec/spec.go:161`).
4. HAProxy проверяет голый TCP. Официально — `option httpchk GET /health?ready=1` на порту 8080 ([офиц] GEN; сгенерированный конфиг: `balance roundrobin`, `timeout connect 10s`, `timeout client/server 1m`, `option clitcpka`, `maxconn 4096`). Healthcheck контейнера БД — `/health` без `?ready=1` (`recipe_distributed.go:69`).
5. После загрузки данных рецепт не ждёт, пока реплики выровняются.
6. Параметр `db.cockroach.params.insecure` игнорируется: в рецепте `"insecure": true` (`recipe_distributed.go:41`) и `--insecure` в sidecar (:81-88).

### Расхождения схемы с документацией (найдены при сверке)
- **`--wal-failover` и `--locality-advertise-addr` есть уже в 24.1** ([офиц] https://docs.cockroachlabs.com/docs/v24.1/cockroach-start: `--wal-failover` добавлен в v24.1, preview; `--locality-advertise-addr` присутствует), `--max-disk-temp-storage` с дефолтом 32GiB есть и в 24.1/24.3. Схема @24 (`cockroach_flags.go:28-31, 214-224`) считает их «только с @25» — это неверно, `modern_flags` на 24.x пуст зря. Исправлять: общий набор флагов для всех major, различие — только в `--cache` по умолчанию.
- Дефолт `--cache`: 128 MiB на 24.1 и 24.3, **256 MiB на 25.4 и stable** ([офиц] https://docs.cockroachlabs.com/docs/v24.3/cockroach-start, .../v25.4/cockroach-start). Комментарий схемы (`cockroach_flags.go:27-31`) верен.
- Статус `--wal-failover`: на странице v24.1 — preview; на v25.4/stable статус не указан. Нужен второй store, у нас диск один — не применим.
- Комментарий в `cockroach_flags.go` о том, что `--max-go-memory` не моделируется: флаг существует во всех проверенных версиях (см. 2.1).

## 2. План профиля `auto`

Входы: `vcpu`, `ram`, `data_disk`, `nodes`. Числа — для 8 vCPU / 32 ГБ / 200 ГБ.

### 2.1 Память (флаги)
| Флаг | Значение | Обоснование |
|---|---|---|
| `--cache` | `35%` при RAM >= 32 ГБ, `30%` при меньшей RAM | [офиц] PC: для production `25%` или больше; можно пробовать увеличивать каждое значение до `35%`; формула «(2 * --max-sql-memory) + --cache <= 80% RAM»; пример `--cache=.35 --max-sql-memory=.35`. START: сумма `--cache`, `--max-sql-memory`, `--max-tsdb-memory` не должна превышать 75%. [практика] Размер данных: workload tpcc ~200 МБ на склад ([офиц] WL), т.е. 100 WH ~ 20 ГБ несжатых; кэш 11 ГБ (35%) не вмещает всё, 8 ГБ (25%) — тем более; отсюда выбор в пользу кэша. На 3 узлах RF3 каждый узел держит полную копию — потребность в кэше та же, что у single. |
| `--max-sql-memory` | `20%` | [офиц] дефолт 25% и «подходит для production» (START). [практика] TPC-C запросы короткие; 20% удерживает формулу PC: 2x20+35 = 75% <= 80% и сумму START: 35+20+~1 = ~56%. |
| `--max-tsdb-memory` | оставить дефолт | [офиц] START: «0.01 (1%) физической памяти или 64 MiB, что больше». Схема ставит 64MiB — совпадает с дефолтом для узлов < 6.4 ГБ и на 32 ГБ даёт меньше дефолта (~320 МиБ); влияния на SQL нет (START), можно оставить 64MiB. |
| `--max-go-memory` | не задавать | [офиц] START (24.1, 24.3, 25.4, stable): мягкий лимит Go; дефолт «`--max-sql-memory x 2.25`, но не больше 90% доступной RAM»; `0` отключает (не рекомендуется). Для sql=20% дефолт = 45% RAM; PC флаг не упоминает. |
| Лимит памяти Docker | не ставить | [практика] без лимита проценты считаются от RAM хоста; не нашёл официального источника про поведение с cgroup-лимитом. |

Малые ВМ (4 vCPU/16 ГБ из `dbkind`): cache 30%, sql 20%.
[офиц] PC: RAM не менее 4 GiB на vCPU (8 vCPU/32 ГБ = ровно рекомендация); 2 GiB/vCPU — минимум только для тестов.

### 2.2 Хранилище / Pebble / IO
| Что | Значение | Обоснование |
|---|---|---|
| `--store size` | `90%` | [офиц] START: дефолт `100%`, это мягкий лимит. 90% — наш выбор [практика]. |
| `ballast-size` | `0` | [офиц] START: отключение `ballast-size=0`. |
| `--max-disk-temp-storage` | `min(32GiB, 15% диска)` | [офиц] START: дефолт `32GiB`, процент считается от первого store, temp не входит в учёт store -> temp + store должны помещаться на устройство. На 200 ГБ: 30 ГБ; со `store size=90%` (180 ГБ) сумма 210 ГБ > 200 ГБ — **нужно либо снизить store size до 80%, либо temp до 10%**. Это баг текущих дефолтов (90% + 32GiB на диске 200 ГБ). |
| ФС | ext4 или xfs, `noatime` | [офиц] PC: «рекомендуются ext4 и XFS»; не LVM в IO-пути, не распределённые ФС/NAS. host_disks по умолчанию xfs — ок. |
| `rocksdb.min_wal_sync_interval` | `500us` в бенчмарк-профиле | [офиц] TPCC-L (шаг настройки кластера): `rocksdb.min_wal_sync_interval = '500us'`. В публичном списке CS не найдена; настройка используется официальным гайдом, значит существует. Гайд идёт на локальных SSD; для network-ssd эффект [практика], A/B. |
| `storage.max_sync_duration` | оставить | [офиц] CS: `20s`; `storage.max_sync_duration.fatal.enabled` = `true`: при зависании fsync > 20 с процесс падает. Следить за стальностью fsync на network-ssd; не ослаблять заранее. |
| `--wal-failover` | `disabled` | нужен второй store (START). |
| Блочное устройство | scheduler `none`, `read_ahead_kb` дефолт | не нашёл официального источника; [практика]. |

### 2.3 Admission control
- [офиц] AC: «Admission control is enabled by default»; `admission.kv.enabled` = `true` (CS). Не выключать. При смене настроек одного слоя — менять для всех слоёв.
- `kvadmission.store.provisioned_bandwidth`: [офиц] CS дефолт `0 B`; AC: Preview; задание включает disk bandwidth admission control («pacing background disk writes to keep disk bandwidth within provisioned bandwidth»). Применимо, если известна полка диска (см. 2.7): для network-ssd 200 ГБ ~ 95-105 МБ/с на запись [практика, расчёт из YC].
- `admission.l0_sub_level_count_overload_threshold`: **не нашёл официального источника** (нет в CS и AC; AC лишь ссылается на график IO Overload). В REL v24.3.14 упомянут `admission.l0_file_count_overload_threshold` (дефолт 4000). Не трогать.
- Метрики для отчёта (теперь, когда Graphene >= v0.3.6 отдаёт histogram): `admission_wait_durations_*`, `sql_service_latency`, `ranges_underreplicated` — имена метрик по /_status/vars сверять с живым узлом, в документации сверка не делалась.

### 2.4 SQL и транзакции
Настройки кластера, которые **сам CockroachLabs ставит в официальном TPC-C гайде** ([офиц] TPCC-L):
| Настройка | В гайде | Дефолт | Для нашего профиля |
|---|---|---|---|
| `kv.snapshot_rebalance.max_rate` | `256 MiB` | `32 MiB` (CS) | `128MiB` на время загрузки (сеть/диск ВМ слабее), затем вернуть. [практика] |
| `sql.stats.automatic_collection.enabled` | `false` | `true` (CS) | `false` после явного `CREATE STATISTICS` на загруженных таблицах; без статистики после загрузки планировщик слеп. [практика] Гайд не говорит, когда собирать статистику. |
| `kv.range_merge.queue_enabled` | `false` | не в публичном списке CS | `false`: слияние диапазонов в окне замера не нужно. [офиц как практика гайда] |
| `rocksdb.min_wal_sync_interval` | `500us` | не нашёл | см. 2.2 |
| `kv.dist_sender.concurrency_limit` | `2016` | — | для 140k WH; на 100 WH не нужен. |
| `ALTER RANGE default CONFIGURE ZONE USING gc.ttlseconds = 600` | да | 4 ч (по умолчанию в версиях; не проверял) | снижает MVCC-мусор в коротком замере. [практика] |

Прочее:
| Настройка | Значение | Источник/статус |
|---|---|---|
| `diagnostics.reporting.enabled` | `false` | [офиц] CS дефолт `true`. Но LIC: Enterprise Free/Trial требуют телеметрию, без неё throttling через 7 дней. Для стенда без лицензии, живущего < 7 дней, безопасно. |
| `sql.metrics.statement_details.enabled` | `false` | [офиц] CS дефолт `true` («collect per-statement query statistics»). Что отключение заметно ускоряет, **не нашёл официального источника**; [практика], A/B. |
| `kv.range_split.by_load.enabled` | `true` (дефолт) | [офиц] CS (alias `kv.range_split.by_load_enabled`); порог `kv.range_split.load_qps_threshold` = `2500`. |
| `kv.transaction.write_buffering.enabled` | см. ниже | [офиц] CS (stable): `true`. REL: введено в 25.2 как public preview, off by default; убрано из публичного списка в 25.2.12; off by default в 26.1; GA в 26.2. Заметка в REL v26.2 одновременно говорит «off by default» в bug-fix — противоречие, **не нашёл однозначного источника по дефолту в 26.2/26.3**; текущая страница stable показывает `true`. Профиль: не трогать, но проверять `SHOW CLUSTER SETTING` на каждой версии матрицы. |
| `kv.raft_log.disable_synchronization_unsafe` | никогда не включать | существует в исходниках ([код] https://github.com/cockroachdb/cockroach/blob/master/pkg/kv/kvserver/logstore/logstore.go), в публичных CS отсутствует. |
| `kv.snapshot_recovery.max_rate` | не использовать | не нашёл ни в CS, ни в исходниках `store_snapshot.go`. В профиль не включать (в прошлой версии отчёта была ошибочно). |
| `kv.allocator.load_based_lease_rebalancing.enabled` | дефолт | существует в [код] pkg/kv/kvserver/allocator/allocatorimpl/allocator.go; в CS публичный `kv.allocator.load_based_rebalancing` (enum, дефолт `auto`). |

**Изоляция (проверено по документации и коду).**
- [офиц] TX/RC: по умолчанию SERIALIZABLE. `sql.txn.read_committed_isolation.enabled` = `true` по умолчанию (CS; RC); если `false`, «каждая транзакция апгрейдится до SERIALIZABLE независимо от запрошенного уровня» (TX).
- [офиц] CS: `sql.txn.repeatable_read_isolation.enabled` = **`false`** (alias `sql.txn.snapshot_isolation.enabled`) — «permits REPEATABLE READ if specified by BEGIN/SET». Появилась как false в 24.3 (alpha.1 и 24.3.3 «settings added», по результатам поиска по REL). Страницы TX и RC REPEATABLE READ как поддерживаемый уровень не упоминают («два уровня: SERIALIZABLE и READ COMMITTED»). Версию GA для REPEATABLE READ **не нашёл официального источника** (в v26.3 docs уровень упомянут как обычный). Версию GA для READ COMMITTED на страницах REL v24.3 тоже не нашёл (страница обрезана); RC-страница stable говорит, что уровень включён по умолчанию.
- Следствие: что именно происходит с запросом RR при `repeatable_read_isolation.enabled=false`, страницы явно не говорят; по общему правилу TX это апгрейд до SERIALIZABLE. **Проверка на стенде обязательна**: `SHOW transaction_isolation` внутри транзакции pgx, либо график «Upgrades of SQL Transaction Isolation Level» в DB Console (RC).
- [код] stroppy (коммит 83342f99): `workloads/tpcc/config.go:47-61` — `resolveIsolation`: picodata -> none, ydb -> serializable, иначе (`default: // postgres, mysql`) -> `repeatable_read`. Драйвера cockroach в stroppy нет: `pkg/bench/enums.go:20-27` содержит только postgres/mysql/picodata/ydb/noop/csv/recording, cockroach идёт как `postgres` (pgx). `pkg/driver/postgres/tx.go:28-39` (`toTxIsoLevel`) отдаёт `pgx.RepeatableRead`. Переопределение: параметр `tx-isolation` (`workloads/tpcc/tpcc.go:137`).
  GitHub: https://github.com/stroppy-io/stroppy/blob/main/workloads/tpcc/config.go и /pkg/driver/postgres/tx.go (пути из локального чекаута; на удалённый GitHub не ходил — проверено локальным `origin/main`).

**Клиентская сторона (честный TPC-C):**
- Ретраи 40001: [офиц] TX: 40001 «restart transaction» ожидаем при SERIALIZABLE, при READ COMMITTED редки и клиентских ретраев не требуют; автоматически сервер повторяет только одиночные операторы и пакеты, пока результат < буфера 16KiB (`sql.defaults.results_buffer.size`); многооператорные транзакции за несколько round trip повторять должен клиент. [код] stroppy: `pkg/driver/postgres/errors.go:32-33` — SQLSTATE 40001 -> `ErrorKindSerialization`; `pkg/bench/retry.go:26-33` — `DefaultErrorActions`: serialization = retry (+deadlock, lock timeout, transient); `retry.go:81-105` `Retry0`; backoff `retry.go:202-226` (база 0.05 с, максимум 1 с, джиттер 20%); число попыток `retry-attempts` по умолчанию **3** (`workloads/tpcc/tpcc.go:135`). То есть ретраи есть, но 3 попытки при 128 VU могут давать неудавшиеся транзакции; нужен счётчик `tpcc_retry_attempts` (`tpcc.go:315`) и доля ошибок. Вывод `retry-attempts >= 10` — [практика].
- Пейсинг: [код] `pacing` по умолчанию `false` (`tpcc.go:134`). [офиц] WL: у `cockroach workload tpcc` `--wait` по умолчанию `true` (think/keying), `--workers` = warehouses x 10 (10 воркеров на склад), ~200 МБ на склад. То есть 100 WH официально = 1000 воркеров с паузами; у нас 128 VU без пауз — другая нагрузка.
- Максимум tpmC на склад: страница PERF говорит, что у TPC-C есть потолок на склад, но **значения не приводит** (12.86 на docs не нашёл; из 1 684 437 tpmC / 140 000 WH получается ~12.0, расчёт мой). Эффективность 95.45% на 81 узле, SERIALIZABLE (PERF). Так что вывод «100 WH ~ 1286 tpmC максимум» из первой версии отчёта оставляю только как оценку по общему правилу TPC-C, не из docs.
- Размер пула: [офиц] PC: активные соединения «не должны сильно превышать 4 x число vCPU» кластера (это ориентир, не формула). Для 8 vCPU — 32; для 3x8 — 96. Текущие 128 VU больше. [код] stroppy поддерживает `MaxConns` для драйвера postgres (`pkg/driver/postgres/pool/pool.go:162-164`); как экспонируется в RunSpec stroppy-cloud — в `internal/domain/compile` и `pipelines/schemas` по `max_conns` не нашёл (grep пуст) -> нужна проверка/добавление.
- Партиционирование: [офиц] TPCC-L использует `--partitions=81 --partition-strategy=leases` и Enterprise-лицензию (`enterprise.license`, `cluster.organization`) для 140k WH на 81 узле; на странице PERF вопрос не поднимается. Для 100 WH и одной зоны нет требования партиционировать [практика].
- Split/scatter: [офиц] WL: у `workload tpcc` есть флаги `--split` («ALTER TABLE ... SPLIT AT») и `--scatter` («Scatter ranges»); дефолты на странице не указаны (поэтому моё прежнее утверждение «по умолчанию делает split» **не подтверждено**). В `crdb.sql` stroppy нет ни SPLIT, ни SCATTER (grep по `workloads/tpcc/crdb.sql` пуст). Гипотеза про лишний хоп и горячие диапазоны остаётся [практика], а не официальным фактом.

### 2.5 Кластер (3 узла)
| Что | Значение | Источник |
|---|---|---|
| RF | дефолт 3 | [практика]; на 3 узлах каждый узел хранит полную копию. |
| Выравнивание после загрузки | перед замером ждать `ranges_underreplicated = 0` | [практика]; метрика — проверить имя на живом узле. |
| `--max-offset` | 500ms | [офиц] PC/START: дефолт 500ms; для multi-region предлагается 250ms; на узле не должно быть > 1 службы синхронизации часов. |
| `--locality` | не нужен | [практика], один регион/зона. |
| Лицензия | `enterprise.license` + `cluster.organization` | [офиц] LIC: без ключа — 7 дней grace; «single-node clusters для разработчиков ключа не требуют»; Enterprise Free/Trial — throttling через 7 дней без телеметрии; изменение лицензионной модели с 24.3 ([офиц] LIC, REL v24.3). Для эфемерных стендов не критично. |
| HAProxy | httpchk `GET /health?ready=1` на `port 8080`, `balance` roundrobin или leastconn | [офиц] GEN: roundrobin; leastconn наш выбор [практика]. Переопределить `Check` в `recipe_distributed.go:95`. Общий `maxconn 20000` и таймауты 1800 с в `haproxy_cfg.go` отличаются от официальных (4096; 10s/1m/1m) — на результат не влияют. |

### 2.6 Хост ОС в Docker
- Swap: [офиц] PC «Disable Linux memory swapping»; в `host_sysctl` `vm.swappiness=1` — не равнозначно отключению swap (если swap есть). Лучше выключить swap в host-prep [практика].
- Дескрипторы: [офиц] PC: минимум 1956 (иначе узел не стартует), рекомендовано `unlimited` или >= 15000; системный лимит минимум в 10 раз выше (например 150000). Официальный TPC-C гайд ставит `ulimit -n 500000` на воркер-ВМ. `fs.file-max=2097152` есть; контейнеру добавить `Ulimits{"nofile": 1048576}` (`recipe_distributed.go:61`).
- Часы: [офиц] PC: узел падает, если часы расходятся с >= половиной других узлов на 80% `--max-offset`; запускать не более одной службы синхронизации.
- THP, scheduler, readahead: **не нашёл официального источника** для CockroachDB; оставить дефолты `host_sysctl`.

### 2.7 Железо
- vCPU:RAM: [офиц] PC: >= 4 GiB/vCPU; 8/32 соответствует.
- IO: [офиц] PC: «Disks must be able to achieve 500 IOPS and 30 MB/s per vCPU» -> для 8 vCPU: 4000 IOPS и 240 МБ/с. Ёмкость до 320 GiB на vCPU, максимум 10 TiB на узел.
- YC network-ssd ([офиц] YC): блок аллокации 32 ГБ; на блок 1000 IOPS чтение и запись, 15 МБ/с чтение и 15 МБ/с запись; максимум на диск 20 000 IOPS чтение / 40 000 запись, 450 МБ/с; VM ограничивает диск до 3500 IOPS и 45 МБ/с на vCPU. Для 200 ГБ: 200/32 = 6.25 блока -> ~6250 IOPS и ~94 МБ/с (7 блоков при округлении вверх: 7000 IOPS, 105 МБ/с; как округляется размер — на странице не нашёл). IOPS (>=4000) хватает, **пропускная способность записи ~95-105 МБ/с — менее половины рекомендованных 240 МБ/с**; чтобы получить 240 МБ/с, диск должен быть >= 512 ГБ (16 блоков x 15 МБ/с). Это наиболее вероятный потолок (WAL+компакции, у 3 узлов — 3 копии). Эксперимент: диск 512 ГБ, повторить 25.4 single.
- Официальные бенчмарки: [офиц] TPCC-L: c5d.9xlarge, локальные SSD, «не используйте network-attached block storage». vCPU c5d.9xlarge (36) и RAM — из спецификации AWS, не со страниц CockroachDB. Расчёт: 1 684 437 tpmC / 81 узел / 36 vCPU = ~578 tpmC на vCPU (PERF). Для 8 vCPU это ~4600 tpmC; перевод в «общие tps» при доле New Order 45% даёт ~170 tps — доля 45% из стандарта TPC-C, не с docs; и неизвестно, что именно считает stroppy в «tps». Эти оценки показывают порядок величины, не потолок.

## 3. Надёжность и паритет
- Запись идёт через Raft, коммит требует кворума и fsync Raft log (WAL) на репликах. Настройка, отключающая fsync, существует (`kv.raft_log.disable_synchronization_unsafe`, [код]), но она «unsafe» и в публичной документации не описана — не использовать. Явного описания «fsync на каждый коммит» на проверенных страницах не нашёл; утверждение опирается на архитектуру Raft/Pebble и название настройки.
- Single-node (`start-single-node`) — одна реплика, без кворума; 3 узла RF3 — кворум 2 из 3 (аналог sync-репликации). Что именно `start-single-node` ставит RF=1 в зонах, на проверенных страницах не нашёл (знаю из практики); сравнивать single и three как «масштабирование» нельзя.
- Изоляция: SERIALIZABLE по умолчанию, [офиц] официальные TPC-C результаты получены под SERIALIZABLE (PERF). В матрице cockroach, вероятно, шёл в SERIALIZABLE (RR запрошен, но `repeatable_read_isolation.enabled=false`), остальные движки — в RR. Зафиксировать явно в отчёте.
- `--insecure` допустим только в приватной сети стенда.

## 4. Изменения платформы
1. `pipelines/schemas/cfg/cockroach_flags.go`: убрать деление «modern flags только @25» (флаги есть с 24.1); `auto`-профиль — вычислять `cache`/`max_sql_memory`/`max_disk_temp_storage`/`store_size` из размера машины; по умолчанию заполнять `cluster_settings` бенчмарк-набором (2.4); поправить `// doc:` ссылки на docs.cockroachlabs.com; регенерировать golden `pipelines/schemas/testdata/cfg.cockroach.flags@*.json`.
2. `internal/domain/compile/recipe_distributed.go`: `overlay` от размера машины; `Ulimits`; healthcheck `/health?ready=1` (:69); `Check` httpchk 8080 для HAProxy (:95); sidecar ждёт отсутствия under-replicated ranges; уважать `params.insecure`; `CREATE STATISTICS` после загрузки (если выключаем автосбор).
3. `internal/domain/dbkind/cockroach.go`: рекомендуемый размер 8 vCPU/32 ГБ; минимальный диск с учётом YC (пропускная способность растёт с размером).
4. stroppy (внешний репо): в `workloads/tpcc/crdb.sql` — post-populate `SPLIT AT`/`SCATTER` (или использовать `--split/--scatter`-подобное); `retry-attempts` для cockroach >= 10; явное `tx-isolation`; параметр `max_conns` в RunSpec (в `internal/domain/compile/workload_connection.go` и схеме workload).
5. `internal/domain/compile/workload_cockroach.go`: по умолчанию подставлять `retry-attempts`, `tx-isolation`.
6. Метрики: собирать `_bucket`-гистограммы SQL latency и admission (имена сверять на живом узле).
7. `STROPPY.MD` §16: зафиксировать профиль `auto` и различие RF1/RF3.

## 5. Риски и открытые вопросы
- Что реально сделал CockroachDB с запросом `REPEATABLE READ` на 24.3/25.4/26.3 в матрице — проверить `SHOW transaction_isolation`. Версию GA для RR и RC в документации не нашёл.
- Дефолт `kv.transaction.write_buffering.enabled` на 26.2/26.3 в REL противоречив; на stable-странице `true`. Проверить на живом узле и зафиксировать, иначе 26.x сравнивается не с тем же поведением, что 25.4.
- Почему three (170) < single (230): гипотезы — RF3, диск (~95-105 МБ/с на запись), отсутствие split/scatter, лишний хоп через HAProxy, 128 соединений при ориентире ~96. Порядок экспериментов: диск 512 ГБ; pre-split/scatter; retry-attempts 10; пул <= 96.
- Метрики `_bucket` не доходили до v0.3.6, p99=1000 мс на матрице считать недостоверным.
- Лицензия: после 7 дней без лицензии/телеметрии — throttling (LIC). Касается `keep` и долгих сьютов.
- Документация CockroachDB 26.2/26.3 читалась через страницу `stable` и release notes с обрезкой; детальные изменения 26.x по Pebble/admission в этом отчёте неполные.
- 24.1 помечена Deprecated и использует `crdb24.sql`; в тестах профиль для неё не проверялся.

## 6. Что изменилось после сверки (по сравнению с первой версией)
- Отозвано: «`--wal-failover`/`--locality-advertise-addr` только с 25.x» (есть в 24.1); «`kv.snapshot_recovery.max_rate`» (не нашёл); «`admission.l0_sub_level_count_overload_threshold` по умолчанию 20» (нет источника); «official workload делает split по умолчанию» (флаги есть, дефолты не указаны); «12.86 tpmC на склад» (в docs нет); размер данных 100 WH «8-10 ГБ» (docs: ~200 МБ/склад -> ~20 ГБ).
- Подтверждено и уточнено: `--cache` 128MiB (24.1/24.3) и 256MiB (25.4+); формула памяти PC «(2*sql)+cache <= 80%», потолок 35%; `--max-go-memory` существует; 500 IOPS и 30 МБ/с на vCPU; 4 GiB/vCPU; ext4/XFS; дескрипторы; пул ~4x vCPU; HAProxy `GET /health?ready=1`; `repeatable_read_isolation.enabled=false`; `read_committed...=true`; `storage.max_sync_duration` 20s fatal; `provisioned_bandwidth` 0B preview; write buffering GA с 26.2.
- Добавлено из официального TPC-C гайда: `rocksdb.min_wal_sync_interval=500us`, `kv.snapshot_rebalance.max_rate=256MiB`, автосбор статистики off, `kv.range_merge.queue_enabled=false`, `gc.ttlseconds=600`; официальные 10 воркеров на склад с `--wait=true`.
- Новый баг: `store size=90%` + `max-disk-temp-storage=32GiB` на диске 200 ГБ не помещаются.
- Диск YC: подтверждено 15 МБ/с на блок 32 ГБ -> ~95-105 МБ/с на 200 ГБ против рекомендованных 240 МБ/с; 512 ГБ нужен для паритета с рекомендацией.
- Код stroppy проверен с точными путями: изоляция RR для cockroach (config.go:47-61, enums.go:20-27, tx.go:28-39), `retry-attempts=3` (tpcc.go:135), классификация 40001 (errors.go:32-33, retry.go:26-33).
