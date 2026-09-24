# Приёмка через локальный сервер

Сервер: Docker Compose, `http://localhost:18347`, текущий контракт `2.0.0`.
Ответы первой попытки сохранены с контрактом `1.0.1`.
База ревизии: `7afd0e87` плюс локальные изменения Compose, порта и публикации
pipeline-бинарников и жизненного цикла cloud-профилей. Tenant: `server-live-20260924`, Graphene namespace:
`t-server-live-20260924`. Провайдер — существующий тестовый YC-аккаунт,
ключ переиспользован; новые облачные ключи не выпускались.

## Подтверждённые проверки

- Создание tenant через API: [ответ 201](tenant-create.json).
- Публикация всех четырёх pipeline-бинарников:
  [synced](pipeline-sync-after-fix.json). Первое исполнение
  [выявило попытку go build в distroless](pipeline-sync-first.json).
  Исправление использует `selfbuild.PushBinary` для готовых бинарников и
  `ManifestAPI.PublishManifest`; тесты проверяют экспорт без Go в PATH,
  tenant/auth metadata и передачу отказа RPC. Целевой race-тест и линтер прошли.
- YC-профиль создан через API; настоящий verify завершился
  [ready](provider-verify.json).
- [Квоты](quotas-first.json) получены из YC: snapshot не stale, scope — cloud.
  Это не подтверждение provisioning или workload.
- API отклоняет неверную форму сегмента с
  [структурированными ошибками 422](invalid-segment-rejected.json).
  До выбора размера runner тест остаётся draft; после выбора XS он
  [ready](noop-test-sized.json).
- Noop запущен через `tests/{id}:launch`:
  [ответ](noop-launch-first.json),
  [состояние failed](noop-first-state.json),
  [события](noop-first-events.json), [дерево](noop-first-tree.json),
  [артефакты](noop-first-artifacts.json).

## Проверка настройки Graphene

Первый run `bc94e0ff-2a4f-4e76-a85c-faf9e013e528` завершился до VM:
`stroppy.provider.ensure-config` не нашёл `kubeconfig` в новом Graphene namespace.
Это ошибка границы ответственности: Kubernetes-доступ настраивается оператором
инсталляции Graphene; сервер и пользователи Stroppy Cloud его не предоставляют.

Контракт 2.0 использует ServiceAccount воркера и отдельный
`stroppy-provider-config` для постоянных ресурсов каждого профиля. Статус `ready`
включает проверку cloud-реквизитов и применение конфигурации. Обычный run только
проверяет её наличие. Удаление профиля и tenant обязано дождаться cleanup ресурсов,
затем удалить ProviderConfig, Kubernetes Secret и Graphene-секреты в этом порядке.

## Проверка provider lifecycle — контракт 2.0

- Graphene chart в `stroppy-io/cloud`: commit `00fda85`, ArgoCD Synced/Healthy.
  Управляемые воркеры используют ServiceAccount `graphene-crossplane`.
  Прямой patch Deployment ArgoCD откатывает; конфигурация закреплена в GitOps.
- [Все пять pipeline опубликованы](provider-config-publication.json), включая
  `stroppy-provider-config`. Сервер не получает kubeconfig или K8s identity.
- [Профиль ready](provider-config-state.json): cloud verify и постоянная
  конфигурация выполнены pipeline через SDK `library/k8s/v0.4.0`.
- [Два профиля изолированы](provider-configuration-isolation.json): разные
  ProviderConfig и Secret, привязанные к UUID профиля и Graphene namespace.
- [Ротация реквизитов](provider-lifecycle-rotation-ready.json) прошла до `ready`,
  новая версия Graphene-секрета не перезаписывает предыдущую.
- [Удаление профиля: 204](provider-lifecycle-delete.json);
  [ProviderConfig и Secret отсутствуют](provider-lifecycle-k8s-deleted.json).
- [Удаление профиля с активным run: 409](provider-active-delete-blocked.json).
- Интеграционные тесты сервера проверяют active/kept guards, повтор после
  ошибки cleanup, сохранение реквизитов, восстановление setup после разрыва
  соединения, удаление tenant с предварительным закрытием новых запусков.

## Проверка полного прогона

Run `5f04e4bc-1c83-4e7b-ad1f-88f599403e98` прошёл provisioning и deployment,
но скачивание Stroppy из GHCR закончилось сетевым timeout.
[Crossplane cleanup](noop-failed-cleanup.json) завершён;
[VM в YC не найдена](noop-failed-yc-cleanup.json).
[Повтор `731d5ee3-e248-4176-bd53-8bf0b6e5223e`](../../../noop/builtin/runner-only/execute-sql/server-api-20s-1vu/runs/731d5ee3-e248-4176-bd53-8bf0b6e5223e/run.json)
завершился `completed`: 20s workload, 3 521 195 итераций, 0 ошибок. Использована закреплённая dev-сборка
из уже испытанных live-входов через API `execution.workload.stroppy_image`.
Это проверяет существующую возможность override, не меняет каталог по умолчанию.
[Config и log скачаны через сервер после cleanup](noop-artifact-downloads.json),
размеры и SHA-256 совпали. [Удаление стенда](noop-success-cleanup.json) проверено
по Crossplane и YC. Постоянная конфигурация основного профиля остаётся. Отдельно требуется
подключить server-side чтение VictoriaMetrics/VictoriaLogs и OTLP экспорт
runner в существующую телеметрию. Сроки её хранения не меняются.

Полные матрицы workload, baseline, segment и fault остаются в
[remaining-scope.json](../../catalog/remaining-scope.json); AWS исключён.
Новый случай в единой таблице:
[noop / server-api-20s-1vu](../../../noop/builtin/runner-only/execute-sql/server-api-20s-1vu/README.md).
