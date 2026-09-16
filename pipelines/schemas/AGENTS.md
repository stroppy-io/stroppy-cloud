# schemas — правила для всех схем

Пакет `github.com/stroppy-io/stroppy-cloud/pipelines/schemas` — единственный
источник всех schemapb-схем продукта (параметры БД, конфиги ПО, workload,
провайдеры, спеки пайплайнов, системные). Импортируют сервер (формы,
валидация, рендер) и пайплайны (RunSpec). TS получает их как protoJSON из
`testdata/<id>.json` (golden, генерируется тестами).

## Раскладка

```
schemas/
  all.go                 Registry(): все схемы, id → *Schema
  ids.go                 namespaces + helper ID()
  dbparams/<kind>.go     db.<kind>.params@1
  cfg/<software>_<major>.go   cfg.<software>@<major>
  workload/*.go          workload.* (stroppy 6: segment = script+typed params, run, steps)
  provider/*.go          provider.*
  spec/*.go              spec.* (RunSpec, SuiteSpec, служебные)
  system/*.go            system.*, test.*, tenant.*
  internal/schematest    общий тест-харнесс
  testdata/<id>.json     golden protoJSON схемы (коммитится)
```

Одна схема — один файл, одна функция `func X() *schemapb.Schema`, одна
запись в `all.go`. Тест рядом: `<file>_test.go`.

## Как писать схему

```go
func PostgresParams() *schemapb.Schema {
	return schemapb.NewSchema(ids.DB("postgres", 1)).
		Descr("PostgreSQL topology and options").
		Strict().Coerce().
		Fields(
			schemapb.Choice("version").Title("Version").Group("Engine").
				Opt(schemapb.StrV("17"), "17").Opt(schemapb.StrV("16"), "16").Opt(schemapb.StrV("15"), "15").
				Default(schemapb.StrV("17")).Required(),
			schemapb.Int64("replicas").Title("Replicas").Group("Topology").
				Desc("Streaming replicas").Gte(0).Lte(8).Default(0),
			// ...
		).
		Rules(
			schemapb.Rule("int(root.sync_replicas) <= int(root.replicas)", "sync_replicas ≤ replicas").ID("sync-le-replicas"),
		).
		Template("conf", "...").  // только cfg.*
		MustBuild()
}
```

Обязательно для каждого поля: `Title`, `Desc` (что делает, единица,
ограничение), `Group` (секция формы). Единицы — `Unit("MB")` и т.п.
Enum — `Choice` с человеческими лейблами (`Opt(value, label)`), не `Str().In`.
Секреты — `.Secret()`. Производные значения — `Computed` (CEL), условные
поля — `.When(cel)`, кросс-полевые инварианты — `Rules(...)` с `ID`.
Дефолты — как у upstream-софта для этой major-версии, если продукт не
требует иного (тогда `Desc` объясняет почему).

**Актуальность — обязательна.** Каждый параметр конфига ПО сверяется с
официальной документацией именно этой major-версии (context7 / docs
сайта). У каждого поля — комментарий `// doc: <url или раздел>`. Параметры,
удалённые/переименованные в этой версии, не включаются; новые — включаются.
Неизвестных ключей нет: `Strict()`. Для «остального» — поле
`custom` (`Map` строк с `Pattern` на имя ключа), рендерится в конец файла.

`cfg.*` обязаны иметь `Template("conf", ...)` (Mustache, контекст
`fields/groups/values`, см. schemapb `render.go`) — рендер в реальный файл.
Списки/вложенные блоки, которые Mustache не тянет (haproxy backends,
group_replication seeds, ydb host_configs) — собираются в `Computed` строку
(CEL `strings`), и шаблон вставляет её.

Роли/хосты/порты кластера, известные только на этапе компиляции RunSpec
(peers, seeds, primary address), — поля с `Group("Cluster")` и
`Desc("filled by the server from topology")`, обычные `Str/List`, без
дефолта; формы их скрывают (`When("false")` не использовать — сервер
подставляет перед Bake; использовать `.Nullable()` и Rules без них).

## Тест каждой схемы

```go
func TestPostgresParams(t *testing.T) {
	schematest.Run(t, PostgresParams(), schematest.Cases{
		Valid:   []map[string]any{{...минимальный...}, {...полный...}},
		Invalid: []schematest.Invalid{{Value: map[string]any{...}, Code: "GTE_VIOLATED", Path: "replicas"}},
		Render:  "conf", // для cfg.* — рендер должен быть непустым и содержать ключевые строки
		Contains: []string{"shared_buffers = "},
	})
}
```

`schematest.Run`: Build, Compile, CheckDescriptor, валидные значения
проходят Bake, невалидные дают ожидаемый code на ожидаемом path, рендер
(если задан) непустой и содержит подстроки, и **golden**: protoJSON схемы
сравнивается с `testdata/<id>.json`; `-update` перезаписывает.

## Версии ПО (major схемы)

Источник истины — версии, которые предлагает `dbparams/*.go`. Каждая
выбираемая там версия обязана иметь свою `cfg.*` схему.

| ПО | версии в `dbparams` | схемы `cfg` |
|---|---|---|
| PostgreSQL | 15 / 16 / 17 / 18 | `cfg.postgresql.conf@15/@16/@17/@18` |
| OrioleDB | образы `*-pg16` / `*-pg17` / `*-pg18` | `cfg.orioledb.postgresql.conf@16/@17/@18` |
| MySQL | 8.0 / 8.4 | `cfg.my.cnf@8/@8.4` |
| MariaDB | 10.11 / 11.4 / 11.8 | `cfg.mariadb.cnf@10.11/@11.4/@11.8` |
| Picodata | 25.3 (legacy) / 26.1 / 26.2 | `cfg.picodata.yaml@25/@26.1/@26` |
| YDB | 25.4 / 26.1 / 26.2 / 26.3 | `cfg.ydb.config.yaml@25/@26` |
| CockroachDB | 24.1 / 24.3 / 25.2 / 25.4 / 26.2 / 26.3 | `cfg.cockroach.flags@24/@25/@26` |
| MaxScale (параметр mariadb) | линия 25.10 | `cfg.maxscale.cnf@25` |

Обвязка, версия которой продуктом не выбирается: `pg_hba 1`, `patroni 3` (legacy 3.3), `patroni 4` (4.1.5), `etcd 3` (3.5), `haproxy 2` (2.9), `pgbouncer 1` (1.23), `proxysql 2`
(2.6), экспортёры: postgres_exporter 0.15, mysqld_exporter 0.19,
node_exporter 1.8.

Version-gating внутри одного билдера (что именно расходится по мажорам):

* `postgresql.conf`: `effective_io_concurrency` 1 → 16 (18), `io_method` /
  `io_workers` / `autovacuum_vacuum_max_threshold` /
  `autovacuum_worker_slots` только 18, `log_connections` из boolean в список
  аспектов (18), `reserved_connections` с 16, `transaction_timeout` с 17.
  `cfg.orioledb.postgresql.conf@N` переиспользует эту таблицу целиком; сам
  набор `orioledb.*` от мажора PostgreSQL не зависит.
* `mariadb.cnf`: 10.11 — SQL-поле `tx_isolation` рендерится как `transaction-isolation`, `innodb_flush_method`,
  `innodb_change_buffering`; 11.4/11.8 — `transaction_isolation`,
  `innodb_log_file_buffering` / `innodb_data_file_buffering`,
  `innodb_doublewrite` как enum с `fast`; 11.8 добавляет
  `innodb_snapshot_isolation` (в 11.8 дефолт стал ON),
  `max_tmp_session_space_usage` / `max_tmp_total_space_usage` (11.5),
  `log_slow_always_query_time` / `slave_abort_blocking_timeout` (11.7).
* `picodata.yaml`: 26.x вложил сеть — `iproto_listen`/`iproto_advertise` →
  `instance.iproto.{listen,advertise}`, `http_listen` →
  `instance.http.listen`, `instance.pg` → `instance.pgproto` (и `pg.ssl` →
  `pgproto.tls.enabled`); добавились `memtx.system_memory`, `wal_dir`, `backup_dir`.
  `cluster.tier.<t>.replication_mode` / `wal_mode` появились только в 26.2;
  схема @26.1 запрещает их.
* `ydb.config.yaml`: 25 и 26 рендерят один и тот же документ configuration
  V1. V2 (обёртка `metadata:`/`config:`) появилась в 25.1, но остаётся
  экспериментальной, а 26.x её не требует и V1 не убирает. Мажор 26 заведён
  ради соответствия версиям параметров; когда V2 станет обязательным —
  обёртку получит тот мажор, а @25/@26 останутся как есть.
* `cockroach.flags`: ни один флаг не удалён и не переименован в 24 → 25 →
  26; с @25 добавлены `--wal-failover`, `--locality-advertise-addr`,
  `--max-disk-temp-storage`. Дефолт `--cache` у upstream 128MiB (24.x) →
  256MiB (25.4+), схема во всех мажорах держит 25%.
* `maxscale.cnf`: только @25 (линия 25.10). 25.10.0 свернул
  `backend_connect_timeout`/`backend_read_timeout`/`backend_write_timeout` в
  один `backend_timeout`; в 24.02 дефолт `master_failure_mode` стал
  `fail_on_write`, а `transaction_replay_timeout` — 30s.

Patroni 4 uses cfg.patroni.yml@4; the @3 schema is retained unchanged.
An external HBA path renders as postgresql.parameters.hba_file in @4,
not as the inline pg_hba list. The compiler supplies topology addresses,
bootstrap hook and the selected PostgreSQL binary/config paths. Cluster-wide
DCS tuning comes from the db role for all members.
