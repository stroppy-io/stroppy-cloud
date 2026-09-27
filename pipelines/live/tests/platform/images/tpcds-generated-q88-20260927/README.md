# Generated TPC-DS: Q88 и запрет молчаливого пропуска

Временная dev-сборка из рабочего дерева Stroppy, без commit/PR/push исходников.
Образ опубликован отдельным тегом GHCR; digest и SHA-256 бинаря в image.json.
base_commit + source-changes.tar.gz восстанавливают исходники сборки. После
сборки менялась только сортировка imports в новом integration test; её хеш
указан в validation.json, код бинаря не менялся.

Удалено неиспользуемое определение STORE из шаблона Q88: оно падало на
неподдерживаемом dist(stores), хотя SQL использует фиксированное имя ese.
Генерация PostgreSQL теперь включает все 99 IDs / 103 statements.
workload отвергает любой Skipped или неверное число queries; preflight проходит
до drop/create/load, учитывая фильтр шага workload. MySQL generated, где нет
переписывания FULL OUTER JOIN для Q51/Q97, завершается явной ошибкой с причинами.
Baked MySQL и load-only режимы остаются отдельными вариантами.

Проверки: 27 сочетаний scale/seed/stream; race; обязательный интеграционный
набор; реальный PostgreSQL SF=.01 со всеми 99 запросами; MySQL preflight с
сохранением sentinel-таблицы. Обычный lint чистый. Включение integration build
tag выявляет старые замечания в существующем harness; новый test чистый.

Живая проверка: [campaign](../../server/resilience/analytical-generated-q88-20260927).
