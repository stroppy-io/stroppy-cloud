# Перепроверка после раскатки Graphene 0.2.25

Статус: **блокер снят после переключения системного worker**.
Повторный resync в 15:54 UTC завершился `synced`; свежие квоты YC получены.
[Resync](resync-after-worker.json), [status](status-after-worker.json),
[квоты](quotas-refresh.json). Проверки ниже в 15:36–15:38 UTC предшествовали
переключению worker на 0.2.25, поэтому не подтверждали отказ этой версии.

По диагностике владельца, worker забрал очередь около 15:44 UTC; entity
восстановилась автоматически, без миграции. В историях кластера lookup без
version marker не возникал. Старый `stroppy-quotas` до continue-as-new использует
DefaultVersion (count → start); REJECT_DUPLICATE в startRunCore сохраняет одно
исполнение, хотя ответ арбитра может называться started вместо exists.
Проверка: 2026-09-27, 15:36–15:38 UTC (18:36–18:38 Москва).

В теге `v0.2.25` (`cc6e3f1706da8e103f08b5ccfaabffa0d6b6131b`) действительно
добавлены `workflow.GetVersion("fire-lookup", DefaultVersion, 1)` и replay-тест
истории entity от 0.2.23. [Разбор исходников](source-review.json).
Локальный checkout Graphene чистый; код не менялся. Версия раскатки сообщена
владельцем, версия реально обслуживающего системного worker через серверный API
независимо не установлена.

## Новое воспроизведение

Дважды отправлен `POST /api/v1/admin/pipelines:resync` с телом
`{"tenant_slug":"server-live-20260924"}`. В обоих случаях HTTP 202, затем
`GET /api/v1/admin/status` сообщает `failed` для namespace
`t-server-live-20260924`:

```text
stroppy-quotas: publish manifest: rpc error: code = FailedPrecondition desc =
failed workflow update: Unable to perform workflow execution update due to
Workflow Task in failed state.
```

В повторной попытке непосредственно наблюдался переход `pending → failed`.
Новые строки в журнале сервера в 15:36:45 и 15:38:20 UTC подтверждают, что это
свежие ошибки RPC, а не только старый status из БД.

- [Первая команда](resync.json), [результат](status-after.json).
- [Повторная команда](resync-retry.json), [результат](status-retry.json).
- [Отфильтрованный журнал публикации](publish-errors.json).
- [Прямое TLS-соединение, без прокси](runtime-config.json).

Health PostgreSQL и Graphene — `ok`. Наличие рабочего health endpoint не
подтверждает успешное исполнение task конкретной pipeline entity.
Клиент Stroppy Cloud остаётся 0.2.24: исправление 0.2.25 относится к workflow
системного worker Graphene, а не к изменению API публикации на клиенте.

## Дополнение к брифу для Graphene

1. Установить, какая версия **системного server worker**, обслуживающего эту
   pipeline entity, фактически активна. Проверить все её реплики; версия
   ingress/API или run-worker сама по себе недостаточна.
2. Получить последнюю ошибку Workflow Task с её временем, stack trace,
   workflow ID/run ID и историю **именно этой** entity. Ответ management API
   не раскрывает первопричину; сейчас ни panic, ни nondeterminism, ни
   незавершённая раскатка не доказаны.
3. Если worker уже 0.2.25, проверить replay этой истории на нём. Новый тест
   истории 0.2.23 полезен, но не доказывает совместимость каждой существующей
   истории. Дополнительно проверить истории, записанные на 0.2.24: lookup там
   уже мог быть выполнен без нового version marker.
4. Исправить выявленную причину и обеспечить восстановление существующей
   entity с сохранением spec/revisions/state и связей с run. Удаление entity,
   очистка истории, новый namespace и обход в Stroppy Cloud не выполнялись.

Критерии закрытия прежние: server resync всех пяти pipelines → `synced` с
ожидаемой revision; свежие квоты YC через сервер; отсутствие новых Workflow Task
failures. Затем живые outage/restart/keep проверки и TPC-H/TPC-DS.
[Исходный полный бриф](../graphene-0.2.24-brief.md).

[Активных run нет](active-runs.json), [сохранённых стендов нет](kept-runs.json),
оба ответа `meta.has_more=false`. Новые workload-запуски не создавались.
После успешной перепроверки в 15:54 UTC живая приёмка возобновлена.
