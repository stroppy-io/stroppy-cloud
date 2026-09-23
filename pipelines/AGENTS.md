# pipelines — Graphene-пайплайны stroppy

Отдельный Go-модуль `github.com/stroppy-io/stroppy-cloud/pipelines`. Без
go.work: сервер подключает `schemas`/`spec` как обычную зависимость по версии
(тег `pipelines/vX.Y.Z`). Четыре бинарника, каждый — один `pipeline.Main`:

| Бинарник | id | Что делает |
|---|---|---|
| `cmd/run` | `stroppy-run` | RunSpec → машины (Crossplane yc/aws) → агенты → контейнеры → stroppy-сегменты → Result |
| `cmd/suite` | `stroppy-suite` | Suite → child-run `stroppy-run` на каждую ячейку (`RunAll`), сводка |
| `cmd/provider-verify` | `stroppy-provider-verify` | Проверка ключа/прав провайдера, read-only, ничего не создаёт |
| `cmd/quotas` | `stroppy-quotas` | Квоты и usage compute/vpc провайдера |

Все входы/выходы — `spec/` (Go-зеркало схем `schemas/spec/*`), протестировано
на совпадение через `Bake` (`spec/spec_test.go`).

## Раскладка

```
spec/                 Run, Suite, ProviderVerify, Quotas + Result'ы; Duration ("5m")
schemas/              все schemapb-схемы продукта (свой AGENTS.md)
workflows/            публичная дверь к телам stroppy-run/suite: id, Run/Suite,
                      wire-имена activities + алиасы payload'ов, имена milestones —
                      для симуляции настоящего workflow вне модуля (e2e сервера)
internal/run          workflow stroppy-run: run.go (фазы), deploy.go, workload.go,
                      template.go (${ip:...}), phase.go (milestones+parallel), record.go
internal/suite        fan-out child-run'ов
internal/provision    Provider{Kind,Scheme,Provision,Record}: yandex.go, aws.go → Infra
internal/activities   тела activities (воркер: creds.go; агент: hostprep/files/docker/stroppy)
internal/cloud        SDK-клиенты yc/aws для verify/quotas
internal/probe        тела пайплайнов verify/quotas
internal/events       Emit(ctx, name, payload) — milestones для проекции сервера
internal/topo         toposort/GroupBy/Slug — чистые, тестируемые
```

## Правила workflow-кода (`internal/run`, `internal/suite`, `internal/probe`)

- **Recording pass.** `record.go` объявляет ВСЕ activities, kinds (оба
  провайдера), docker/agent/artifact/stand на нулевом проходе. Новая
  activity → `activities.Register` + константа `Name*`; новый k8s kind →
  `Provider.Record`. Проверка: `GRAPHENE_MANIFEST=1 ./stroppy-run`.
- Детерминизм: итерация только по `infra.Order`, `topo.SortedKeys`,
  `topo.GroupBy`; никаких `map` range в workflow.
- `parallel()` — workflow-горутины; внутри НЕ вызывать `ToStand`
  (`pipeline.Context{Context: gctx}` теряет pipelineId).
- Фазы: `provisioning → deploying → workload → collecting → teardown`,
  каждая через `phase[T]` (milestones `phase.started/finished/failed`, panic
  от `Ready` перехватывается и перекидывается).
- Deploy после пользовательских host_prep обеспечивает Docker через
  `dockerlib.Install()` на первом runner и всех машинах с контейнерами.
  Установка идемпотентна, машины готовятся параллельно; ошибка останавливает
  deploy и запускает cleanup. Входу не нужны install-скрипты для Docker.
  Docker library 0.2.3 отправляет heartbeat сразу и каждые 15 секунд;
  pipeline задаёт минутный heartbeat timeout и 15 минут на попытку установки.
- Node-exporter читает rootfs, procfs, sysfs и udev машины через read-only
  `/host`; `--path.rootfs` сам по себе не перенаправляет остальные пути.
  Версия 1.12.1 читает `ID_SERIAL` virtio-дисков YC, включая имя `data`.
- Healthcheck принимает Docker-формы `CMD` и `CMD-SHELL`, а также прямой argv.
  `CMD-SHELL` использует shell образа (по умолчанию `/bin/sh -c`); маркеры
  не передаются как имена исполняемых файлов. Ошибка exec сохраняется в диагностике.
- Managed YDB перед workload проверяется тем же образом Stroppy: TCP, TLS/IAM
  и read-only SELECT 1. Readiness ограничена пятью минутами, не входит в нагрузку;
  её конфиг и журнал попыток сохраняются отдельными артефактами с общим retention.
- Сегменты workload — последовательно, `AtMostOnce`, таймаут
  `duration*1.5 + warmup + 30m` по умолчанию; явный segment.timeout
  задаёт deadline activity после отдельного idle wait warmup.
- `Keep` → `ToStand(infra.Root, KeepFor)` после результата; иначе cleanup
  interceptor Graphene сносит всё каскадом от root-сети.
- Загруженные артефакты перед возвратом результата передаются stand:
  конфиги без TTL (до явного удаления), логи сегментов и baseline — 30 дней.
  Они переживают cleanup run; срок не зависит от Keep инфраструктуры.

## Правила activities (`internal/activities`)

- Тело = чистая функция `(ctx, Req) (Res, error)`; `Res` обязателен
  (`activity.Fn`). Ошибка = ретрай по политике, поэтому «плохой результат»
  (нет прав, сломан профиль) возвращается В результате с `nil` err.
- Агентские activities работают в run-workspace агента
  (`machine.Workspace()`); относительный `Dir` резолвится от него, путь
  одинаков на хосте, в контейнере агента и для docker daemon (bind-mount).
- `EnsureProviderConfig` — единственная, что ходит в k8s напрямую: пишет
  Secret + ProviderConfig `t-<tenant>` в `crossplane-system`. Не
  graphene-ресурс (общий на тенанта). Kubeconfig: graphene-секрет
  `kubeconfig` в ns тенанта, тот же кластер и identity, что у `k8slib`.
  Учётка run-пода не подменяет явно заданные реквизиты.
  Live bootstrap использует постоянный token Secret `graphene/stroppy-live-api-token`
  существующей SA `stroppy-live`. Повторная настройка повторно использует этот
  токен; `--sync-token-only` (старый алиас `--refresh-token-only`) не выдаёт
  восьмичасовой токен и не меняет RBAC. Конфигурация и проверка — `live/tools/bootstrap.py`,
  `live/tools/bootstrap/bootstrap-token.yaml`, `live/tests/platform/credentials/persistent-kubeconfig-check.json`.

## Плейсхолдеры и env (контракт с сервером)

- `${ip:<machine>}`, `${ip:role:<role>}`, `${ips:role:<role>}`,
  `${public_ip:<machine>}` — в env/cmd/files контейнеров и env workload.
  Неизвестное имя/роль — ошибка до деплоя.
- Подключение к БД: `workload.url` RunSpec (с плейсхолдерами) + `driver_type`
  + `driver` (lowerCamel-ключи `drivers.0` stroppy-config.json) — пайплайн
  только пишет их в конфиг. Никакого env-моста: stroppy 6 типизирован.
- Имена облачных ресурсов: `stroppy-<tenant>-<run8>-...`; агенты
  `<run8>-<machine>`.
- Контейнеры Docker и их Graphene-записи именуются `<полный-run-UUID>-<name>`;
  логическое имя остаётся в RunSpec, depends_on, событиях и метке
  `stroppy-container`. Healthcheck и flow-ссылки используют физическое имя.
  Это разделяет одновременно работающие прогоны в одном namespace.
- Отмена suite возвращает Canceled независимо от continue_on_failure;
  обычная ошибка ячейки остаётся отдельным исходом. Cleanup Graphene
  отправляет отмену дочерним run, которые выполняют собственный teardown.

## Зависимости (не трогать без причины)

- provider-yc v0.14.0 (импорт только versioned `apis/cluster/*/v1alpha1`,
  корень `apis` тянет pre-release `common/v2`), provider-aws/v2 v2.6.0
  (типы с суффиксом `_2`), crossplane-runtime/v2 **v2.2.0** (≥2.3.3 без
  `apis/common/v1`), k8s 0.36.3, controller-runtime 0.24.0, docker
  v28.5.2+incompatible (v29 переехал в moby/moby/api).

## Симуляция прогона (pipelinetest)

`internal/run/sim_test.go` гоняет ВЕСЬ workflow `stroppy-run` на
`pipeline/pkg/pipelinetest` (Temporal testsuite + модель ресурсов/агентов/
cleanup/stand) с адаптерами `library/k8s/k8stest` (Crossplane-объекты
становятся Ready фикстурами в виртуальном времени) и
`library/docker/dockertest`. Activities машин — моки `OnAgentActivity` по
агенту; run-queue контракты (`ensure-config`, события) — `Handle1`. Ни
облака, ни docker, ни сервера: `go test ./internal/run -run TestSimulated`.
Покрыто (`sim_test.go`, `sim_more_test.go`, 16 сценариев): happy path
postgres/yandex и noop/aws; неизвестный провайдер и отказ `ensure-config`
(ничего не создано); отказ VM (квота) и агент без коннекта (таймаут) —
чистый каскад; healthcheck-fail останавливает deploy; слои `depends_on` на
двух машинах, `${ip:<m>}`/`${ips:role}`; baseline-fail и артефакт-fail
нефатальны; два сегмента, `result_expectations` → degraded; потеря
activity сегмента (AtMostOnce, без повтора); cancel в provisioning и в
workload; `keep` → stand + TTL и `keep` игнорируется после провала.
`internal/suite/sim_test.go` — fan-out через серверный контракт
start/await-child; `internal/probe/sim_test.go` — verify/quotas. Правило:
любой новый шаг workflow — сначала сюда, потом на кластер.

Ловушка Temporal: контексты выводятся по конкретному типу — в
`workflow.WithCancel/Go/NewWaitGroup/AwaitWithTimeout` отдавать
`ctx.Context` (встроенный `workflow.Context`), не `pipeline.Context`,
иначе `panic: cancelCtx not found` на первом же параллельном ожидании.

## Проверка

```
cd pipelines
go test ./... -count=1               # 113 тестов, golden: -update
golangci-lint run --config ../.golangci.yaml ./...
make -C .. build-pipelines           # bin/stroppy-*
GRAPHENE_MANIFEST=1 ../bin/stroppy-run | jq '.activities|length'   # 18
```

## stroppy 6 (без фолбеков на v5)

Пакет `stroppycfg/` — чистая половина запуска stroppy: рендер
`stroppy-config.json` и командной строки, разбор вывода. Источник правды —
чекаут `~/devel/github/stroppy-io/stroppy` (`stroppy probe -o json`,
`stroppy run <workload> --help`, `stroppy help config-file|drivers|steps`).

- Образ: `ghcr.io/stroppy-io/stroppy:v6.0.0.62` (тег = релиз + номер сборки),
  entrypoint `stroppy`, WORKDIR `/workspace`; контейнер на host-сети,
  директория сегмента примонтирована как `/workspace`.
- Команда: `run -f /workspace/stroppy-config.json --log-mode production
  --log-level <lvl> [--<extra>=<v>…]`. Конфиг: `script`, `global{runId,
  metadata, logger, exporter.otlpExport}`, `drivers.0`, `run{executor, vus,
  duration|iterations, queryTimeout}`, `params{<lowerCamel>}`, `steps/noSteps`.
  Snake_case ключи схемы → lowerCamel (`scale_factor` → `scaleFactor`);
  `sql_file`/`schema_file` с именем файла из `files` → `/workspace/<name>`.
- Итоги: JSON-отчёта у `stroppy run` НЕТ. Пайплайн парсит stderr-блоки
  `=== bench summary ===` (counters + histograms `count/avg/p50/p90/p95/p99`,
  мс) и `=== bench completed with errors ===`, плюс stdout-строку
  `{"compliance": …}` (TPC-C). Nonfatal-ошибки = exit 0 (учтены в `errors`),
  130/143 — cancel, 1 — ошибка. Пороги (`thresholds`) применяет пайплайн.
- Baseline: `stroppy baseline --json --no-save --download always [...]` до
  сегментов на runner-машине; отчёт schema 1 → `result.baseline`; fail
  вердикта не валит прогон.
- Сборки с коммита: версия `nightly-<sha>` (так печатает `stroppy version`),
  образ задаёт каталог. Параметры новых сборок, не описанные схемой, идут через
  `extra_params` как типизированные флаги.

## Native workload metrics

The workflow stamps the actual Graphene namespace/run and tenant; config rendering
adds the segment name without mutating caller labels. Explicit workload parameters
are scalars next to `script`, never a nested `params` object. Parameter-shape
validation runs before provisioning. CLI failure summaries retain the leading
`Error:` message even when Stroppy prints usage afterward.

The pipeline copies native `tps`, `iterations_per_second`, `queries_per_second`
and `measurement_seconds`, adding units without computing throughput. Workloads
without logical transactions do not report TPS. Full metrics and workload-specific
reports remain available alongside the compact headline.

Native Stroppy fixes are temporarily tested on YC using development images built
from the working tree (now preserved in Stroppy PR #166), with inputs pinned to immutable image digests.
See [the temporary build procedure](live/tests/README.md#dev-сборки-stroppy)
for provenance requirements and the source-preservation limitation. Retire this
workaround only after the upstream PR is merged, an official image is published,
and equivalent checks pass with its digest. Do not infer permission to commit or
create that PR from this temporary testing agreement.


## MySQL family live checks

The compiler selects `cfg.my.cnf@8` for MySQL 8.0, `@8.4` for 8.4 and
`cfg.mariadb.cnf@<selected-version>` for MariaDB. Semisync plugin options use
`loose-` because fresh mysqld initialization skips plugin loading; readiness
requires the plugin afterwards. Replica `read_only=ON` belongs in the permanent
config, since initdb `SET GLOBAL` does not survive the temporary-server restart.
A healthy listener alone is insufficient: verify receiver, applier, semisync
status and replicated test rows. `live/tools/mysql_probe.py` uses ordinary SQL access
and does not require sudo or Docker access in an agent shell.

Run results allow 256 metrics per segment and 64 segments, so aggregate metrics
must not reuse the per-segment cap. Artifact capacity is config + log per segment
plus the optional baseline log. Preserve every native metric instead of truncating.

SDK 0.2.5 bounds OTLP metric requests to 2 MiB by splitting complete protobuf
collections/points before gRPC export. Resource identity, labels, timestamps and
metric semantics are preserved; receiver partial rejections remain errors. Large
YDB scrapes require this SDK in the published worker. The server receive limit
and the separate database histogram/summary scrape gap are unchanged.

## Pipeline input contract

The versioned handoff is [live/tests/platform/contracts/HANDOFF.md](live/tests/platform/contracts/HANDOFF.md).
`spec.ContractVersion` and `contract.lock.json` identify the reviewed interface.
Run `make contract-check` from the repository root. Do not silently change
field meanings/defaults, merge rules, diagnostic scopes or result projections
under the same contract version; follow the handoff compatibility policy.

Inputs use the existing schemas end to end, including nested suite RunSpecs,
segments, native driver options and machine baseline. See
[live/tests/platform/contracts/README.md](live/tests/platform/contracts/README.md)
for server/UI mapping, validation evidence and explicit execution limits.
`live/tools/check_contract.py` checks against a local Stroppy binary on noop;
it does not deploy or start cloud resources.
