# Бриф Graphene: cleanup после удаления и метрики остановленных exporters

Проверка 2026-09-27, время UTC. Новые cloud-run остановлены до разбора.
Graphene не изменялся. Все живые наблюдения получены через Stroppy Cloud API;
прямых обращений к Kubernetes, YC или хранилищам телеметрии не было.

## Контекст

- Graphene 0.2.26 раскатан пользователем; проверенный локальный исходник:
  `e6399a2e645ca7599fa844b891f36a67225641d0`.
- Namespace `t-server-live-20260924`, tenant `server-live-20260924`.
- Server `http://localhost:18347`, dev bearer `dev`.
- Текущая сборка сервера `0.0.0-dev-f658fd8b-pull-heartbeat-20260927`;
  resync synced, Graphene/PostgreSQL health ok.
- Pipeline SDK `github.com/graphene-ci/pipeline v0.2.12`,
  Docker library `github.com/graphene-ci/library/docker v0.3.3`.
- Старый severity-дефект закрыт: одинаковые 10 ERROR-записей с числовым
  severity доступны с фильтром и без него, [доказательство](../graphene-0.2.26/verification.json).

## 1. Отмена не завершается после удаления всех ресурсов

Run `22a00ddf-a390-4e89-b04a-a2eac80894bc`, Temporal workflow
`run/22a00ddf-a390-4e89-b04a-a2eac80894bc`. Execution UUID из журнала агента:
`01a0e3cd-172b-71fd-9bae-b1ae946f647d`.

Это MySQL Group Replication, три DB + ProxySQL + runner. Run отменён через
сервер в 17:09 во время deploy после повторных таймаутов публичного registry.
Workload не исполнялся. Сервер был обновлён в ходе удаления; другие run после
того же обновления корректно завершились и сохранили результаты.

К 17:18:50 дерево уже показывало удалённую инфраструктуру. В сохранённом
снимке 17:27:33 все **16** дочерних ресурсов имеют `phase=deleted`, но run
остаётся `cancelling`, `phase=deploying`, без terminal event.
Graphene пишет повторные `server.run.cleanup`: attempt 2 в 17:19:17,
3 — 17:20:19, 4 — 17:21:23, 5 — 17:22:31, 6 — 17:23:47,
7 — 17:25:19, 8 — 17:27:19. Это текущие повторы на уже раскатанной версии,
а не сохранённый admin/status от предыдущего worker.

Доказательства: [сводка](cleanup-verification.json), [run](cleanup-run.json),
[всё дерево](cleanup-tree.json), [повторы cleanup](cleanup-warnings.json).

Воспроизведение чтения через сервер:

```text
GET /api/v1/t/server-live-20260924/runs/22a00ddf-a390-4e89-b04a-a2eac80894bc
GET /api/v1/t/server-live-20260924/runs/22a00ddf-a390-4e89-b04a-a2eac80894bc/tree
GET /api/v1/t/server-live-20260924/runs/22a00ddf-a390-4e89-b04a-a2eac80894bc/logs?level=warn,error&limit=100
```

Нужно в Graphene:

1. Посмотреть PendingActivities/history этого workflow: тип timeout/error,
   heartbeat details и этап, на котором застревает `server.run.cleanup`.
2. Проверить `internal/worker/worker.go:runCleanup`, `cascade`, `awaitClosed`,
   затем `Registry.StopRunContainers` / `queueHasLiveEntities` и `fireDownstream`.
   `awaitClosed` отправляет heartbeat, но последующие ожидания команд/опросы
   требуют отдельной проверки. Точная зависшая операция через текущий API
   Stroppy не видна; её нельзя честно объявить найденной по одному retry-log.
3. Повтор cleanup после уже удалённых сущностей должен безопасно завершаться.
   Исчезнувший executor/agent и побочный downstream trigger не должны
   удерживать отменённый run бесконечно. Не помечать run завершённым раньше
   реального окончания teardown.
4. Добавить regression: отмена multi-machine deploy, удаление VM до остановки
   executor, повтор cleanup после timeout и восстановление после worker restart.
   Приёмка: terminal cancelled, все ресурсы deleted, нет дубликатов и оставшегося
   занятого слота. Сначала восстановить этот run, затем повторить сценарий.

Инфраструктуры этого run по дереву уже нет; успешное завершение lifecycle пока
**не подтверждено**. Нельзя вручную заменить его серверный статус на cancelled.

## 2. Scraped CPU counter даёт около −100% после остановки exporter

Три независимых run, все уже completed и очищены:

- Galera `5fc7ddd6-5c13-4be7-b51b-21bf19277540`: db-2 минимум
  −100.036664%, db-3 −95.416667%.
- PostgreSQL generated TPC-DS `7b3d8ae2-1b43-4b41-81ca-64f9e522ea86`:
  runner-1 минимум −100.316667%.
- PostgreSQL baked TPC-H/TPC-DS `09e86d6e-3eab-4e5d-ac47-621d19accff3`:
  runner-1 минимум −75.166667%. Выявлено повторной проверкой полного интервала
  после завершения teardown; проверка во время удаления не захватывала этот хвост.

Формула каталога сервера:

```promql
100 * (1 - avg by ("graphene.agent") (rate(node_cpu_seconds_total{mode="idle"}[1m])))
```

Scope namespace/run добавляется сервером и Graphene. Отрицательные значения
воспроизводятся через `/metrics:raw` в `rate(node_cpu_seconds_total{cpu="0",mode="idle"}[1m])`:
idle rate местами близок к 2 seconds/second уже после прекращения samples.
Это не округление готового графика в UI; UI в проверке не участвует.

Для Galera db-2, интервал 17:13–17:15 UTC:

- значимые значения counter: 632.30 → 662.30 → 692.28, примерно раз в 30 s;
- `tlast_over_time(...[1m])` показывает также промежуточные timestamps через 15 s;
- после последнего sample `count_over_time(...[1m])` уменьшается 4 → 3 → 2 → 1;
- `rate`/`irate` дают около 2 при одном оставшемся sample, учитывая предыдущий
  вне окна; `rate_prometheus` не даёт большого скачка. Это диагностическое
  сравнение, не внесённый в сервер workaround. Различие функций документировано
  [VictoriaMetrics](https://docs.victoriametrics.com/metricsql/#rate_prometheus).

Доказательства лежат в [Galera evidence](../ha-20260927/mariadb):
`telemetry.json`, `cpu-idle-raw.json`, `cpu-mode-rates.json`,
`cpu-last_over_time.json`, `cpu-tlast_over_time.json`, `cpu-count_over_time.json`,
`cpu-rate_prometheus.json`, `cpu-irate.json`. Второй run:
[telemetry](../analytical-generated-20260927/telemetry.json).

В точной используемой Docker library v0.3.3 `observe.go:shipScrape` передаёт
Prometheus COUNTER в `obs.Gauge`. Pipeline SDK v0.2.12 реализует его через
`Float64Gauge.Record`, а не через импорт исходных samples с их временем и типом.
Связь повторяемых timestamps с периодическим OTLP export нужно проверить
регрессией; одних range-ответов недостаточно, чтобы назвать это единственной
причиной. Готовность component telemetry сейчас не подтверждена.

Нужно в Graphene/library:

1. Проверить точность counter samples и timestamps на пути scrape → OTLP →
   storage; отличить измерение от повторного экспорта предыдущего значения.
2. Сохранить counter semantics, время измерений и корректное прекращение
   серии при остановке exporter. Проверить вычисление rate у коротких серий
   и на правой границе. Не обрезать отрицательный CPU в Stroppy Cloud.
3. Regression с известным линейным idle counter, scrape 30 s/export 15 s,
   остановкой exporter, несколькими CPU и машинами; сравнить с исходными
   samples, отдельно проверить реальные counter reset/restart.
4. Если требуется изменение library/SDK — выпустить соответствующие модули;
   одной смены версии сервера Graphene будет недостаточно. После этого обновим
   зависимость pipeline и повторим приёмку через Stroppy Cloud.

Sanity-check в нашем проверочном инструменте теперь отклоняет CPU за пределами
−1…101% (малый допуск только на точность счётчиков). Native-метрики Stroppy и
логи проверяются отдельно и не объявляются сломанными из-за host CPU.

## Граница остановки

MySQL mirror-тест подготовлен, но **не запущен**. Новых запусков после выявления
блокера нет. PostgreSQL analytical/baked, PostgreSQL generated и Galera завершены,
их инфраструктура удалена. Kept stands отсутствуют. В active остаётся только
зависшая отмена MySQL; [active list](active-runs.json), [kept list](kept-runs.json),
[свежие квоты](quotas.json).

Отдельный дефект Stroppy, не задача Graphene: generated TPC-DS пропустил Q88,
поскольку `dsqgen.Generate` возвращает его в `Skipped`, а `generateStream`
обрабатывает только `Queries`. Полнота generated-режима **не принята**;
[доказательство](../analytical-generated-20260927/query-coverage.json).
