# Graphene: публикация существующего pipeline после обновления

Статус: **воспроизведённый блокер живой приёмки**, 2026-09-27 12:35 UTC.
Новые тестовые VM и workload-run в этой попытке не запускались.

## Что наблюдается

Локальный Stroppy Cloud пересобран и запущен как
`0.0.0-dev-f658fd8b-resilience1-20260927`, клиент Graphene `v0.2.24`.
Graphene в кластере обновлён до 0.2.24 по сообщению владельца; независимого
version endpoint в этих доказательствах нет. Namespace: `t-server-live-20260924`.

Сервер отвечает HTTP 200, PostgreSQL и Graphene health — `ok`. При синхронизации
пяти встроенных пайплайнов публикация `stroppy-quotas` завершается ошибкой:

```text
stroppy-quotas: publish manifest: rpc error: code = FailedPrecondition desc =
failed workflow update: Unable to perform workflow execution update due to
Workflow Task in failed state.
```

Воспроизведение через сервер:

1. `POST /api/v1/admin/pipelines:resync`, тело
   `{"tenant_slug":"server-live-20260924"}` — HTTP 202.
2. Дождаться завершения фоновой синхронизации.
3. `GET /api/v1/admin/status` — HTTP 200, `pipelines.namespaces[].status=failed`
   и приведённая ошибка для `stroppy-quotas`.

Доказательства: [resync](direct-resync-0.2.24.json),
[итоговый status](direct-status-0.2.24.json),
[безопасная выборка runtime-конфигурации](direct-runtime-config.json).
HTTP 202 подтверждает принятие команды синхронизации, а не её успех.

## Исключённые причины

При подготовке fault injection сначала использовался адрес reverse proxy в
качестве Graphene endpoint. Это также меняло адрес registry и не подходило для
синхронизации. Конфигурация восстановлена. Повтор выше выполнен **без прокси**:
`graphene.stroppy.io:443`, TLS включён, `HTTPS_PROXY`/`HTTP_PROXY` отсутствуют.
Процесс тестового прокси остановлен; искусственные сбои не включались.

Поэтому подтверждённая ошибка Workflow Task не является результатом имитации
недоступности или ограничения локальных Docker/socket permissions. Они сейчас
доступны, сборка и интеграционные тесты выполнены.

## Что проверить и исправить в Graphene

1. Найти entity workflow существующего pipeline `stroppy-quotas` в указанном
   namespace. Сохранить его историю и точную ошибку последнего Workflow Task
   вместе со stack trace и версией обслуживающего worker.
2. Проверить replay этой истории на 0.2.24. При подтверждённой
   nondeterminism/panic исправить совместимость и проверить остальные четыре
   существующих pipeline entity, не ограничиваясь новым пустым namespace.
3. Обеспечить восстановление уже существующей entity без потери её spec,
   revision, trigger/concurrency state и связей с прежними run. Если нужна
   операционная миграция, описать отдельно её предварительные условия,
   сохранение состояния, действия и проверку результата. Удаление entity или
   очистка истории здесь не выполнялись и не предлагаются как автоматический
   обход.

**Гипотеза, а не установленная причина:** в diff `v0.2.23..v0.2.24`
`internal/pipelineflow/pipelineflow.go`, обработчик `FireCmd`, появился
`server.run.lookup` перед прежним `server.run.count`; также изменилось
назначение ID. В этом изменении нет ветки `workflow.GetVersion`. Для
долгоживущего workflow с ранее обработанными fire-командами это место требует
проверки replay. Точную причину без истории/ошибки worker утверждать нельзя.
Локальный checkout Graphene — чистый `31dda7b8577e8b5d634056f952ff8cabfdda58f2`;
его код, кластер и данные не изменялись.

## Приёмка исправления

- Автоматический replay-тест реальной или минимально воспроизведённой истории
  entity 0.2.23, которая уже принимала FireCmd, под новым кодом.
- Upgrade-тест: существующий pipeline → обновление worker → publish новой
  revision → новый запуск. Проверить также повтор того же run ID и params.
- Серверный resync всех пяти пайплайнов указанного namespace заканчивается
  `synced`, revision совпадает с `expected_revision`; нет повторяющегося
  Workflow Task failure.
- `POST /api/v1/t/server-live-20260924/providers/b413b185-61ca-4d81-abf2-0b760aa59ed5/quotas:refresh`
  через сервер получает свежие квоты YC. Старый cached response не считается
  подтверждением.

## Состояние Stroppy Cloud

Выборочная интеграционная проверка восстановления: **19 passed**, реальные HTTP
и PostgreSQL, имитированный Graphene с настоящими симуляциями workflow.
[Отчёт](../acceptance/resilience-integration.json).
Три новых fault-теста отдельно выполнены с race detector: **7 тестов/подтестов
passed**, [отчёт](../acceptance/resilience-race.json).
Имитация StartRun/CancelRun удерживается на всех немедленных RPC-ретраях;
проверяется дальнейшая доставка сохранённой команды после восстановления.

Локальный сервер оставлен на новой сборке с прямым TLS-соединением к Graphene.
[Активных run нет](active-runs-current.json),
[сохранённых стендов нет](kept-runs-current.json), оба списка полностью прочитаны.
Новая живая проверка outage/restart/keep, обновления YC-токенов и TPC-H/TPC-DS
остаётся невыполненной. По правилу владельца при блокере Graphene прогоны
остановлены до исправления; server-side обход не добавлен.
