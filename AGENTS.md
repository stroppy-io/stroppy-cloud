# stroppy-cloud — Repository Guidelines

Stroppy Cloud: сервер-фасад над Graphene CI (тенанты, модель бенчмарков,
результаты, SPA). Провижн, агенты, исполнение и наблюдаемость — Graphene;
identity — self-hosted Ory Kratos (`deployments/kratos/`). Рабочий документ
решений — `STROPPY.MD` в корне (§16 — фасад продукта: модель, ресурсы,
ручки, формы).

## Layout

| Путь | Что | Локальные правила |
|---|---|---|
| `cmd/stroppy-cloud/` | бинарник сервера: API + встроенная SPA | — |
| `internal/` | код сервера (домен, api, transport, oas generated) | — |
| `openapi/parts/` | источник контракта; `make openapi` → `openapi.yaml` → ogen Go + TS-типы | `openapi/README.md` |
| `pipelines/` | Graphene-пайплайны, отдельный Go-модуль; `pipelines/spec` — RunSpec | `pipelines/AGENTS.md` |
| `pipelines/schemas/` | schemapb-схемы форм/конфигов (источник для UI и сервера) | `pipelines/schemas/AGENTS.md` |
| `web/` | SPA: React 19 + Vite + `@grafana/ui` | `web/AGENTS.md` |
| `deployments/` | Dockerfile, compose | `deployments/README.md` |
| `docs/` | заметки и перенос знаний из `main-v0` | — |

## Component-level instructions

Подсистема несёт свой `AGENTS.md`. Перед работой в каталоге прочитать его и
родительские; локальные правила уточняют этот файл и имеют приоритет в
своей области. Правило меняется — правится там же (документ = текущее
состояние, не лог).

## Kind базы данных

Всё, что продукт знает о kind'е, кроме развёртывания, — один файл в
`internal/domain/dbkind/<kind>.go`: запись каталога (роли, версии,
протоколы, шаблоны), план машин из параметров, логин клиента, протоколы
workload. URL подключения строит `dbkind.URL` из плана. Развёртывание —
рецепт в `internal/domain/compile/recipe_<kind>.go`, регистрация в
`compile.recipes`. Параметры — схема `pipelines/schemas/dbparams/<kind>.go`.

Новый kind = схема + файл в `dbkind` (и строка в `kinds()`) + рецепт +
значение в enum `DatabaseKind` контракта. Свитчей по kind вне этих мест нет;
тест `TestEveryDeployableKindHasRecipe` ловит kind без рецепта.

## Словари

Любой набор значений, который сервер отдаёт (kind'ы БД, роли топологии,
события прогона, причины ожидания в очереди, действия и объекты аудита,
ключи и группы метрик, квоты и их единицы, уровни логов, причины ошибок), —
enum в контракте (`openapi/parts/15-vocabularies.yaml` и рядом с полем).
Доменный список живёт в домене; `internal/api/vocab_test.go` держит его
равным enum'у. Тексты — только в `web/src/locales`: `web/src/lib/vocab.test.ts`
проверяет перевод каждого значения на обоих языках. Новое значение =
правка домена, контракта и локалей в одном изменении; сервер прозы не шлёт.

## Generated code — не править руками

`internal/oas/*` (ogen), `web/src/api/schema.d.ts` (openapi-typescript),
`web/src/api/enums.ts` (`scripts/openapi_enums.py`),
`web/src/schemas/*` (schemapb export). Меняется источник
(`openapi/parts`, `pipelines/schemas`) → `make openapi generate-go
generate-ts schemas-export`.

## Commands

```bash
make configure          # проверка тулчейна
make build              # web-build + бинарник с встроенной SPA
make test lint          # Go
make dev                # compose: сервер + Postgres, встроенная UI
make web-dev            # vite dev с proxy на :18347
```

Git: Conventional Commits, без co-author-футеров. Коммит/пуш — только по
явной просьбе.
