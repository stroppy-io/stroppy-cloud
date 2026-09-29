# stroppy-cloud — Repository Guidelines

Stroppy Cloud: сервер-фасад над Graphene CI (тенанты, модель бенчмарков,
результаты, SPA). Провижн, агенты, исполнение и наблюдаемость — Graphene;
identity — gopherex/iam. Рабочий документ решений — `STROPPY.MD` в корне
(§16 — фасад продукта: модель, ресурсы, ручки, формы).

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

## Generated code — не править руками

`internal/oas/*` (ogen), `web/src/api/schema.d.ts` (openapi-typescript),
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
