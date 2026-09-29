# web — SPA Stroppy Cloud

Фронтенд продукта: React 19 + Vite + TypeScript, дизайн-система
`@grafana/ui`. Собирается в `web/dist` и встраивается в бинарник сервера
(`web/embed.go`, отдаётся на все не-API пути). Это текущее состояние решений,
не лог: правило меняется — правится здесь же.

Референс по инженерным практикам — `naukograd-software/komeet`
(`AGENTS.md`, `apps/komeet/AGENTS.md`): слоистая раскладка, формы, i18n,
дисциплина токенов. Мобильную вёрстку и shadcn/tailwind оттуда НЕ берём.

## 1. Что это за продукт и как выглядит

Stroppy Cloud — SaaS для бенчмарков СУБД: библиотека (Database / Workload /
Test), прогоны (Run) с живым наблюдением, Suite-матрицы и расписания,
результаты (метрики, compare, rating, share), настройки тенанта, админка.
Объектная модель и полная карта ресурсов — `STROPPY.MD` §16 (16.0 модель,
16.12 таблица ресурс → ручки → формы); `openapi/openapi.yaml` — контракт.

Визуальный язык — **Grafana × GitHub**:

- **Grafana** даёт материал: тема (`GrafanaTheme2`), плотность, панели
  (`PanelChrome`), формы, таблицы, графики uPlot, drawer/modal, теги, бейджи.
  Тёмная тема по умолчанию, светлая поддерживается с первого дня.
- **GitHub** даёт структуру страниц: заголовок сущности с табами
  (`Overview | Runs | Settings …`), list → detail, фильтры в URL, sidebar
  метаданных справа, action-кнопки в правом верхнем углу, breadcrumbs
  `tenant / resource / item`. Плотные списки с иконкой статуса слева.
- Цвет несёт смысл, а не декор: статус рана, severity валидации, diff.
  Никаких градиентов, иллюстраций, маркетинга внутри приложения.

## 2. Стек (версии фиксируем, меняем осознанно)

| Слой | Библиотека | Примечание |
|---|---|---|
| Runtime | react 19, react-dom 19 | peer `@grafana/ui` = `react >=19` |
| Сборка | vite 7, @vitejs/plugin-react, typescript 5.8+ strict | node 22 |
| Дизайн-система | `@grafana/ui` 13.2.x, `@grafana/data` 13.2.x | **единственный** UI-кит; версии ui/data совпадают |
| Стили | emotion через `useStyles2(theme => css({...}))` | tailwind, radix, shadcn, cva — **нет** |
| Роутинг | `@tanstack/react-router` 1.170+ + `@tanstack/router-plugin` (vite, file-based) | типизированные params, `validateSearch` zod-схемой (Standard Schema, без адаптера); grafana `Link`/`LinkButton`/`PageToolbar` тянут `react-router-dom-v5-compat` — не использовать |
| Данные | `openapi-typescript` (типы) + `openapi-fetch` (клиент) + `@tanstack/react-query` 5 | кэш, инвалидация, WS → `setQueryData`; loaders роутера → `ensureQueryData` |
| Формы | `@tanstack/react-form` 1.33+ + zod 4 | Standard Schema на поле/форму, `onChangeAsync`+`asyncDebounceMs`, `setErrorMap` для серверных ошибок; контролы — grafana `Field`/`Input`/`Select`/`Combobox`/`Switch`/… через `form.Field` |
| Таблицы списков | `@tanstack/react-table` 9 (headless) + `@tanstack/react-virtual` 3 | стили — grafana-токены; grafana `InteractiveTable` (react-table v7) не используем; `TableNG` — только DataFrame-метрики |
| Тайминг/шорткаты | `@tanstack/react-pacer`, `@tanstack/react-hotkeys` | debounce/throttle/batch (валидация, WS-батчи логов); GitHub-style `g r`, `/`, `?` |
| Devtools | `@tanstack/react-devtools` + query/router плагины | dev-only |
| i18n | i18next + react-i18next, локали `ru`/`en` | как в komeet; мажоры i18next/react-i18next = те, что требует `@grafana/i18n` (один экземпляр на приложение) |
| Тосты | sonner, стилизован через `useTheme2` | в `@grafana/ui` тостов нет (они в runtime) |
| Тесты | vitest (+ существующий node contract-тест) | `yarn test` |
| Линт/формат | biome | 2 пробела, width 100, single quotes, без `;` |
| Редакторы | grafana `CodeEditor` (monaco, lazy) для YAML/JSON | `CodeMirrorEditor` из `@grafana/ui/unstable` — только после проверки |

Не тащим: `react-router`, `react-hook-form` (транзитивно внутри
`@grafana/ui` — не импортировать), `recharts` (есть uPlot-панели),
`lucide-react` (есть `Icon`), `nanostores`/`@tanstack/store` (серверное
состояние — Query, фильтры — URL роутера, локальное — React; Store допустим
только для cross-component UI-state, если появится), `@tanstack/db`,
`@tanstack/start`, `@tanstack/config` (не для приложения), `next-themes`.

Новая зависимость — только после обсуждения: зачем, что легче, что уже
есть в `@grafana/ui`.

## 3. Раскладка `src/` (слоистая, по komeet)

```
src/
├── main.tsx              # корень: ThemeProvider, GlobalStyles, QueryClient, i18n, RouterProvider
├── routeTree.gen.ts      # generated: @tanstack/router-plugin — не править руками
├── routes/               # file-based маршруты (ТОНКИЕ): validateSearch, loader, component = композиция фич
├── app/                  # каркас приложения: AppShell, TopNav, TenantSwitcher, PageHeader, Breadcrumbs
├── components/
│   ├── <Shared>.tsx      # одиночный переиспользуемый (StatusBadge.tsx, RelativeTime.tsx)
│   ├── <family>/         # семейство домен-виджетов: run/ (RunStatusIcon, RunPhaseBar), schema/ (SchemaForm …)
│   └── <feature>/        # фичевые каскады: library/, runs/, suites/, settings/, admin/, results/
├── hooks/                # shared React-хуки (useTenant, useUrlFilters, useRunLive …)
├── api/
│   ├── schema.d.ts       # generated: openapi-typescript (make generate-ts) — не править руками
│   ├── client.ts         # openapi-fetch instance + auth middleware + problem+json → ApiError
│   ├── json.ts           # bigint-safe JSON граница (уже есть)
│   ├── ws.ts             # WS-клиент /api/v1/ws: subscribe/resume/reconnect (без React)
│   └── queries/          # react-query: ключи + queryOptions + mutations per ресурс (runs.ts, tests.ts …)
├── schemas/              # generated: schemapb protoJSON + TS (make schemas-export) — не править руками
├── lib/                  # framework-agnostic инфраструктура: auth.ts, config.ts (public config), storage, env
├── helpers/              # чистые функции: format*, parse*, date, bytes, duration; без React и I/O
├── locales/              # ru/*.json, en/*.json
├── styles/               # theme.ts (createTheme + overrides), global.ts (дополнения к GlobalStyles)
└── types/                # ambient .d.ts
```

Правила слоёв:

- `helpers/` — чистые функции, ничего не импортируют сверху.
- `lib/` — синглтоны/адаптеры, без React/JSX/хуков. `api/` — то же, кроме
  `api/queries/*` (там `queryOptions`, тоже без JSX).
- `hooks/` — React поверх `api/` и `lib/`.
- `components/` — рендер. **Компонент не вызывает `fetch`/клиент напрямую** —
  только через `useQuery(queries.runs.detail(...))` / `useMutation`.
- `routes/` — только сборка: `createFileRoute` с `validateSearch` (zod),
  `loader` (`queryClient.ensureQueryData`), `component` из фич. Никакой
  логики и page-scoped компонентов внутри `routes/`.
- **Feature → feature запрещён.** Общее — в family/shared, композиция — на
  уровне page. Глубина фичи ≤ 3 (`<feature>/<sub>/File.tsx`).
- Приватный вспомогательный компонент лежит рядом с владельцем в
  `components/` его папки; понадобился второму — поднимается.
- **Нет barrel-файлов.** Импорты через алиасы (`@/`, `@components/*`,
  `@hooks/*`, `@api/*`, `@lib/*`, `@helpers/*`, `@app/*`, `@routes/*`), объявлены в
  `tsconfig.json` + `vite.config.ts` синхронно.
- Файлы компонентов PascalCase, route-файлы по конвенции роутера (`$id.tsx`, `_layout.tsx`, `index.tsx`), хуки `useX.ts`, остальное kebab/lower.
  Обработчики `handle*`, булевы `is/has/should*`. `any` — только после
  обсуждения. Мёртвый код удаляется.

## 4. Тема и стили

- Один источник токенов — `GrafanaTheme2`. `styles/theme.ts`:
  `createTheme({ colors: { mode } })`; переопределения (если понадобятся)
  только там. Режим `dark|light|system` — в `preferences.theme` профиля
  (`PATCH /me`) + localStorage до логина.
- Провайдер: `<ThemeContext.Provider value={theme}>` (из `@grafana/data`) →
  `<GlobalStyles/>` → приложение. Никакого второго reset'а.
- Стили компонентов: `const styles = useStyles2(getStyles)`,
  `getStyles = (theme: GrafanaTheme2) => ({ root: css({ ... }) })`.
  Отступы — `theme.spacing(n)`, цвета — `theme.colors.*`, шрифты —
  `theme.typography.*`, радиусы — `theme.shape.radius.*`, тени —
  `theme.shadows.*`, z-index — `theme.zIndex.*`.
- **Запрещены литералы цветов/размеров** в css: `#…`, `rgb(…)`, `white`,
  `12px` для шрифтов, `8px` для отступов. Проверка перед сдачей:
  `grep -rnE "#[0-9a-fA-F]{3,8}|rgba?\(|[0-9]+px" src --include=*.tsx` — каждое
  попадание объяснимо (1px border через `theme.colors.border.*` допустим).
- Layout — `Stack`, `Box`, `Grid`, `Space`, `Divider`, `ScrollContainer`;
  текст — `Text`; не `HorizontalGroup`/`VerticalGroup` (deprecated).
- Статусные цвета — из темы: `theme.visualization.getColorByName`,
  `theme.colors.success|warning|error|info`. Маппинг статус рана → цвет —
  один хелпер `helpers/run-status.ts`, не по компонентам.
- Ассеты `@grafana/ui`: иконки — SVG из `node_modules/@grafana/ui/dist/public/img/icons`,
  путь строится из `window.__grafana_public_path__` (`<path>build/img/icons/…`).
  Шрифты Inter/Roboto Mono в пакете **нет**, `GlobalStyles` ждёт их под
  `<path>fonts/…`. Решение: `window.__grafana_public_path__ = '/gf/'`,
  vite копирует иконки в `dist/gf/build/img/`, woff2 кладём в
  `public/gf/fonts/{inter,roboto}/`. В dev — то же через `publicDir`.
- Тестировать вид в **обеих** темах. Скриншот в PR для UI-изменений.

## 5. Компоненты: сначала `@grafana/ui`, своё — по недостатку

Порядок выбора: готовый компонент `@grafana/ui` → композиция из них →
свой на `useStyles2` в токенах. Не оборачивать grafana-компоненты «на
всякий случай»; обёртка появляется, когда есть доменное знание
(`RunStatusBadge` поверх `Badge`).

Что берём под что:

| Задача | Компонент |
|---|---|
| Кнопки, меню, тулбар | `Button`, `ButtonGroup`, `IconButton`, `ToolbarButton`, `Dropdown`+`Menu`, `ConfirmButton`, `DeleteButton`, `ClipboardButton` |
| Формы | `Field`, `FieldSet`, `InlineField`, `Input`, `TextArea`, `SecretInput`, `Select`/`Combobox`/`MultiCombobox`, `Switch`, `Checkbox`, `RadioButtonGroup`, `Slider`, `TagsInput`, `DateTimePicker`, `TimeZonePicker`, `FileDropzone` |
| Списки/таблицы сущностей | свой `DataTable` на `@tanstack/react-table` 9 + `react-virtual`, ячейки/шапка в grafana-токенах; `FilterInput`, `FilterPill`, `Pagination` |
| Данные/метрики | `TableNG` (unstable, DataFrame), `TimeSeries`, `Sparkline`, `BigValue`, `BarGauge`, `VizLegend`, `VizTooltip`, `PanelChrome` |
| Статус/метки | `Badge`, `Tag`, `TagList`, `Spinner`, `LoadingBar`, `LoadingPlaceholder`, `Counter`, `Avatar`, `UsersIndicator` |
| Оверлеи | `Modal`, `ConfirmModal`, `Drawer`, `Tooltip`, `Toggletip`, `Popover`, `ContextMenu`, `Portal` |
| Структура | `TabsBar`+`Tab`+`TabContent`, `CollapsableSection`, `Card`, `EmptyState`, `Alert`, `Divider`, `Sidebar` |
| Код | `CodeEditor` (monaco, `React.lazy`), `JsonExplorer`, `JSONFormatter` |
| Текст/ссылки | `Text`, `TextLink` (только `href` наружу), `RenderUserContentAsHTML` для markdown notes |

Не используем: `Link`, `LinkButton`, `PageToolbar` (router v5), `InteractiveTable` (react-table v7), `Form`,
`LegacyForms`, `InputControl`, `HorizontalGroup`, `VerticalGroup`,
`Segment*` (query-editor специфика), `DataSourceHttpSettings` и прочие
Grafana-плагин-ориентированные компоненты.

Своё, что точно пишем: `AppShell` (topnav + контент), `PageHeader`
(GitHub-style заголовок с табами и actions), `AppLink` (router `Link` в стилях
`TextLink`), `StatusIcon`/`StatusBadge` рана, `PhaseTimeline`, `RunProgress`,
`TopologyPreview`, `SchemaForm` (generic schemapb-рендерер), `LogView`
(виртуализированный tail на `react-virtual`), `Toast` (sonner в теме), `KeyValueList`
(sidebar метаданных), `DiffView`.

## 6. Роутинг и URL (TanStack Router)

- File-based routes в `src/routes/`, `routeTree.gen.ts` генерирует
  `@tanstack/router-plugin`. `createRouter({ routeTree, context: { queryClient },
  defaultPreload: 'intent' })`. Ошибки — `errorComponent` на layout-маршруте
  (`ErrorBoundary` из `@grafana/ui` внутри), `notFoundComponent` один.
- Навигация только типизированная: `<Link to="/t/$slug/runs/$id" params search>`
  через свой `AppLink`, `useNavigate()`; строковые URL руками — нет.
- Тенант в пути: `/t/:slug/...` — зеркало API `/api/v1/t/{slug}/...`.
  Корень `/` → редирект на `default_tenant` из `/me` или на создание тенанта.
- **URL — источник правды для фильтров, сортировки, курсора, активной
  вкладки, выбранного диапазона.** Каждый list/detail-маршрут объявляет
  `validateSearch: z.object({...}).default/.catch` — типизированный
  `Route.useSearch()`, запись через `navigate({ search: prev => ({...prev,
  page: 2}) })`. Дефолты стрипаются из URL (`stripSearchParams`), общие
  ключи (`range`) переживают переходы через `retainSearchParams`. Никаких
  фильтров в React-state, которые теряются на F5 или при копировании ссылки.
- `loader` маршрута = `context.queryClient.ensureQueryData(queries.runs.list(
  slug, search))` — данные в кэше к моменту рендера, компонент читает
  `useSuspenseQuery` тем же ключом. Loader не заменяет Query, он его греет.
- Карта маршрутов (следует §16.12; в файлах — `$slug`, `$id`):

```
/                                  → redirect
/t/:slug                           → Dashboard (GET /t/{slug}/dashboard)
/t/:slug/runs[?filters]            → RunList (+facets)
/t/:slug/runs/:id[/overview|events|logs|metrics|grafana|artifacts|spec]
/t/:slug/compare?runs=a,b,…        → Compare
/t/:slug/library/databases[/:id]   → Database list/detail (табы: Overview | Configs | Usages)
/t/:slug/library/workloads[/:id]
/t/:slug/library/tests[/:id]       → Test detail (табы: Overview | Runs | Validation) + Launch
/t/:slug/library/tests/new         → мастер: создаёт draft сразу, PATCH по шагам
/t/:slug/suites[/:id]              → Suite (Overview | Cells | Runs)
/t/:slug/suite-runs/:id            → SuiteRun (Summary | Cells)
/t/:slug/schedules[/:id]
/t/:slug/rating
/t/:slug/shares
/t/:slug/settings/(general|members|providers|quotas|webhooks|tokens|audit)
/me/(profile|tokens|invites)
/admin/(status|tenants|users|runs|settings|audit)
/examples                          → галерея (clone / quick-run в текущий тенант)
/s/:token                          → публичный lab-report (без auth, без AppShell)
```

## 7. Данные: OpenAPI + react-query

- Типы — `api/schema.d.ts` (`make generate-ts`). Клиент — `openapi-fetch`
  `createClient<paths>({ baseUrl: '' })` в `api/client.ts` с middleware:
  `Authorization: Bearer` из `lib/auth.ts`, `Idempotency-Key` на `:launch`/
  `POST runs`/`POST suite-runs`, сериализация тела через `stringifyRequest`
  (bigint-safe), парсинг `application/problem+json` в `ApiError{status,
  code, title, detail, validation?}`.
- Списки — `{ data[], meta{ next_cursor, has_more } }`; `useInfiniteQuery`
  с `getNextPageParam = meta.next_cursor`. Фасеты — отдельный запрос `:facets`.
- `api/queries/<resource>.ts`: фабрика ключей `[ 't', slug, 'runs', 'list',
  filters ]` + `queryOptions(...)` + мутации с инвалидацией по префиксу.
  Ключ **всегда** начинается с `['t', slug]` (или `['me']`, `['admin']`,
  `['catalog']`) — смена тенанта = чистый срез кэша.
- Каталог (`/catalog/*`, схемы schemapb) — `staleTime: Infinity`,
  инвалидация по `X-Stroppy-Version`.
- Ошибки: `ApiError` → `Alert` в месте запроса (list/detail) или тост для
  мутаций; 401 → сброс auth; 403 → страница «нет доступа»; `validation` →
  в форму (см. §9).
- Optimistic UI не делаем без запроса; `favorite` — единственное
  исключение (toggle с откатом).

## 8. Realtime: один WS

- `api/ws.ts` — один сокет на вкладку к `/api/v1/ws?token=…` (браузер не
  умеет заголовок). Фреймы `{type, topic, sub_id, cursor?, payload}`;
  клиент → `subscribe|unsubscribe|ping`, сервер → `ack|event|error|pong`.
  **Правда — код сервера** (`internal/transport/ws`, `internal/api/streams.go`),
  `openapi/README.md` частично устарел (`ready/dropped/closed`, auth первым
  сообщением там описаны, в коде нет).
- Топики: `run.overview/{id}`, `run/{id}`, `run.events/{id}`,
  `run.logs/{id}`, `run.metrics/{id}`, `suite_run/{id}`, `tenant.runs/{slug}`.
  `agent.pty` пока `unavailable` — вкладка Shell не строится.
- Хук `useTopic(topic, { cursor, enabled })` → пишет в react-query
  (`setQueryData` по тому же ключу, что и REST-запрос), поэтому компонент
  читает **один** `useQuery`, не зная про WS. Частые ивенты (`run.logs`,
  `run.metrics`) батчатся `useBatchedCallback` (pacer, `wait: 100`) в один
  `setQueryData`. Реконнект с backoff, resume по `cursor`, при
  недоступности WS — `refetchInterval` (polling fallback остаётся).
- Подписка живёт, пока смонтирован потребитель; терминальный статус рана
  → отписка.

## 9. Формы: TanStack Form + zod + grafana Field

Каждая форма — `useForm` из `@tanstack/react-form` + zod (Standard Schema)
+ grafana `Field` + `form.Field`. Никаких per-field `useState`.

- `defaultValues` для каждого поля (не `undefined`); тип формы выводится из
  `defaultValues`/`z.infer`; zod-схема экспортируется и лежит рядом с формой
  (в `useMemo(() => …, [t])` из-за локализованных сообщений).
- Один `form.Field name="…"` на поле; `<Field label invalid={isInvalid}
  error={errors[0]} htmlFor={field.name}>` + контрол с `id={field.name}`,
  `value={field.state.value}`, `onBlur={field.handleBlur}`,
  `onChange={e => field.handleChange(...)}`, `aria-invalid`.
- **Сервер — источник правды.** Лимиты/enum в zod зеркалят OpenAPI
  (`minLength`, `pattern`, `enum` из `schema.d.ts`), с комментарием-ссылкой
  на поле. Доменная валидация (`fits/suggested`, требования к железу,
  schemapb) — **только** сервером через `:validate`/`:preview`: поле или
  форма объявляет `validators.onChangeAsync` с `asyncDebounceMs: 400`,
  ответ мапится на поля по `path`. Клиент правила не дублирует.
- Тайминг «reward early, punish late»: синхронная схема на `onBlur`,
  после первой ошибки — `onChange`; `onChangeAsync` только после валидного
  формата (`fieldApi.parseValueWithSchema` первым шагом).
- Серверные ошибки при submit → `form.setErrorMap({ onServer: { fields:
  { [path]: message }, form: rootMessage } })` из `ApiError.validation.
  issues[]{path, code, severity, suggested}`; неатрибутируемое — в `form`,
  рендерится у кнопки. Ввод при отказе не теряется.
- Submit не дизейблится за невалидность — `form.handleSubmit()` показывает
  ошибки и фокусирует первое невалидное поле. Дизейбл — только
  `form.state.isSubmitting` (через `form.Subscribe`).
- Успех виден: переход/закрытие/тост.
- Нормализация: `.trim()` везде; name-подобные схлопывают пробелы; id-подобные
  (slug) нормализуются до валидации — в `listeners.onBlur`/`transform`.
- Массивы — `form.Field mode="array"` с `pushValue/removeValue`, ключ —
  стабильный id элемента, не индекс.
- Текст ошибки — что не так и как исправить, с примером. Не «Invalid».
- Общий валидатор переиспользуемого поля (slug, cron, cidr) — один модуль в
  `helpers/validators/`, импортируется формами.

**schemapb-формы** (`x-schema` в OpenAPI: params БД, конфиги ПО, сегменты
workload, провайдеры, sizes): generic-рендерер `components/schema/SchemaForm`
поверх protoJSON-схемы из `/catalog/schemas/{id}` (TS-типы уже в
`src/schemas/`). Виджет по типу поля → grafana-контрол; `FieldActive`/
`ChoiceOptions`/`ListCount` считаются нативным TS-движком; custom-виджеты
по `schema id/path` (топология-превью, сегменты). Значение хранится
целиком, отправляется `stringifyRequest`. Валидация — `:validate` на
сервере через `validators.onChangeAsync` формы (debounce 400 мс),
результат подсвечивает поля по `path`.

## 10. i18n

- i18next, локали `src/locales/{ru,en}/*.json`, `keySeparator: '.'`.
  Ключи по областям: `common.*`, `nav.*`, `runs.*`, `library.*`, `suites.*`,
  `settings.*`, `admin.*`, `public.*`.
- Ни одного захардкоженного пользовательского литерала. Оба языка в одном
  изменении, одинаковая структура ключей; русские plurals `_few/_many`.
- Переводимо: JSX-текст, `placeholder`, `title`, `aria-label`, тултипы,
  тосты, ошибки zod (схема в `useMemo(() => …, [t])`). Не переводимо:
  `console.*`, enum/API-имена, URL, id, имена БД/провайдеров.
- Внутренние строки `@grafana/ui`/`@grafana/data` (TimeRangePicker,
  RefreshPicker, Select/Combobox, Modal, Pagination, ConfirmModal…) идут через
  `@grafana/i18n` в **тот же** экземпляр i18next: ключи Grafana — полные пути в
  дефолтном namespace (`grafana-ui.select.placeholder`,
  `time-picker.range-content.from-input`). Английский — дефолты из кода
  Grafana; русский — `locales/grafana/ru.json`, генерируется
  `node scripts/gen-grafana-ru.mjs` из официального ru-RU `grafana.json` той же
  версии по ключам, которые реально вызывает dist, + `ru.overrides.json`
  (пробелы Crowdin и наши формулировки). После апгрейда `@grafana/ui` —
  перегенерировать; скрипт падает, если ключ остался без перевода.
  `lib/i18n.ts` вызывает `initPluginTranslations('translation')`, иначе `t`
  Grafana не привязан (в dev бросает). Смена языка ремонтирует дерево роутера
  (`App.tsx`, `key={i18n.language}`): Grafana читает строки без подписки и
  мемоизирует части пикеров. Не переводятся (захардкожены в Grafana):
  «Browser Time»/«Default» в TimeZonePicker, дефолтный `loadingMessage` Select.
  Проверка — `src/lib/i18n.test.ts`.

## 11. Авторизация: сейчас dev, шов под IAM

- Сервер в dev-режиме (`STROPPY_DEV_USERS=admin=admin@stroppy.local=Admin`,
  `STROPPY_ADMIN_EMAILS=admin@stroppy.local`) принимает статический bearer
  `admin`. `GET /api/v1/public/config` отдаёт `auth_mode: dev|iam`.
- SPA: `lib/auth.ts` — единственный модуль, знающий про токен:
  `getToken()`, `onAuthLost()`. В dev: токен из `VITE_DEV_TOKEN` (по
  умолчанию `admin`), при 401 — простое поле ввода токена. Ни логина, ни
  регистрации, ни `@gopherex/iam-sdk` сейчас.
- IAM позже подключается заменой реализации `lib/auth.ts` + FlowPage по
  образцу komeet; остальной код не меняется. Роли (`owner|admin|member|
  viewer`) читаются из `/me.tenants[].role` и уже сейчас скрывают/дизейблят
  действия (`useCan(action)`).

## 12. Живой ран (Run detail) — ядро продукта

Вкладки: `Overview` (фазы + прогресс + компоненты/машины + sidebar:
снапшот, provider, sizes, keep, rating, labels, notes), `Events` (таймлайн
`RunEvent`), `Logs` (типизированные фильтры role/machine/container/stream/
phase/search в URL, WS tail, `:raw` LogsQL в advanced), `Metrics` (каталог
по kind, окно = сегмент; `TimeSeries` на `DataFrame`), `Grafana`
(iframe через same-origin relay `/grafana`, сессия `POST grafana-session`),
`Artifacts`, `Spec` (RunSpec read-only, `CodeEditor` YAML/JSON + diff со
снапшотом). Действия в header: cancel / rerun / rerun-resume / clone /
save-as-test / share / keep extend-release / favorite / delete (по роли).

Overview — единственный экран, который обязан быть «живым» с первой
секунды: `run.overview` + `run` через WS, `source: live|persisted|synthetic`
и `degraded_reasons` показываются явно.

## 13. Порядок реализации

1. **Spike (полдня):** пустой Vite + `@grafana/ui`: `createTheme` +
   `GlobalStyles`, иконки/шрифты через `__grafana_public_path__`,
   `TimeSeries` на DataFrame, `Drawer`, `CodeEditor` lazy, `TableNG` из
   unstable; TanStack Router file-based + `validateSearch` zod 4 +
   devtools; TanStack Form с grafana `Field` + `setErrorMap`. Фиксируем
   находки здесь (§4 ассеты, warnings `@grafana/i18n`).
2. Каркас: `main.tsx`, тема, `AppShell`, `routes/__root.tsx` + layout
   `/t/$slug`, `api/client.ts`, `lib/auth.ts` (dev), `/me` + переключатель
   тенанта, i18n, тосты, hotkeys-скелет, `DataTable`. Удаление
   tailwind/radix/recharts/react-router/lucide из `package.json` и
   `components/ui`.
3. Runs: list (фильтры/фасеты/курсор) + detail Overview/Events/Logs через
   WS. Это первый экран, который показываем.
4. Library: Database/Workload/Test list+detail, `SchemaForm`, Test
   wizard (draft на сервере), Launch → Run.
5. Metrics/Grafana/Artifacts/Spec, Compare, favorites, share, публичный `/s/:token`.
6. Suites/SuiteRuns/Schedules; Settings (providers+verify, quotas, webhooks,
   members, tokens, audit); Dashboard; Rating; Examples.
7. Admin.

Каждый этап приёмка — живой прогон против локального сервера
(`make dev` / `make web-dev`), не unit-тесты сами по себе.

## 14. Команды

```bash
make web-install        # yarn install --frozen-lockfile
make web-dev            # vite на :5173, proxy /api, /v1, /grafana → :18347
make web-build          # tsc -b && vite build → web/dist (встраивается в make build)
make generate-ts        # openapi → src/api/schema.d.ts
make schemas-export     # schemapb → src/schemas
make contract-check     # tests/contract.test.mjs
cd web && yarn test     # vitest
cd web && yarn lint     # biome check
```

Перед сдачей: `yarn build` зелёный, `yarn lint` чистый, greps из §4 и §10,
обе темы, скриншот.

## Локальные промо-скриншоты

`web/.env.development.local` с `VITE_API_MODE=mock` включает мок-данные на
`http://localhost:5173` (`yarn dev`). Файл локальный и игнорируется Git;
для возврата к API удалить эту строку или файл. Production всегда использует API.
В режиме mock отображается плашка; TanStack Devtools доступны в dev-сборке.
Обычный локальный режим — `VITE_API_MODE=real`. `seed.ts` содержит согласованную
историю прогонов, QPS, результаты и участников; обновление страницы сбрасывает
мок-состояние. `UserAvatar` строит стабильный identicon по ID без внешних запросов.
Живые поля рана (`summary.expected_finish_at` по warmup+duration сегментов,
`headline` и `qps_series` из сэмплов фазы workload, прогресс сегментов) —
`api/mock/live-summary.ts`, общий для сида и симуляции; симуляция стартует с
первого запроса мока. Сортировка прогонов (`sort` enum listRuns/listTestRuns/
adminListRuns, `default` = избранные → живые → новые, пустые значения в конце) —
`api/mock/run-sort.ts`. Инварианты — `api/mock/live-summary.test.ts`.
`capacity.ts` дополняет основной тенант до 843 прогонов, включая 8 активных
строк (5 running, 1 cancelling, 2 pending). Сценарии используют до 64 машин
на прогон, 2048 VU и реальные XS–XL пресеты из каталога провайдеров;
нагрузка и результаты синтетические, не измеренная производительность продукта.
YDB: 32 storage + 31 compute + runner; PostgreSQL/MySQL: по 8 реплик.
Лимиты демонстрационного тенанта рассчитаны под этот набор; технические границы
берутся из схем и расчёта ресурсов runner, а не из прежних настроек тенанта.

## 15. Открытое

- Режим по умолчанию — real (vite proxy → `:18347`, `make dev`); мок только
  `VITE_API_MODE=mock` в dev-сборке, грузится лениво и в прод-бандл не попадает.
- Живой сервер: `Run.summary.workload_name` заполняется сервером (для inline — скрипт первого сегмента «+N»); `ListTests` ~60 мс (скомпилированные схемы кэшируются на процесс); headline `qps` + `qps_series` + `expected_finish_at` в сводке запуска.
- Prod-сборка: Monaco берётся из бандла с воркерами (`components/code/
  monaco-setup.ts`), не с CDN; чанк-ошибки после деплоя → авто-перезагрузка
  один раз (`routes/__root.tsx`).
- Логи: у каждой строки стабильный `seq` (микросекунда от начала запуска + слот), он же курсор — `?line=<seq>` открывает строку на живом сервере. Страница логов ~230 мс (перечитывание граничной микросекунды).
- `rerun {resume}` на сервере всегда деградирует до обычного rerun с пометкой в
  notes: attach к живому стенду в пайплайне не реализован (STROPPY.MD §14).

- `agent.pty` (Shell) — недоступен на сервере; вкладку не строим.
- `TableNG` — unstable API; если ломается, метрики на свой `DataTable` (react-table 9) поверх DataFrame.
- Шрифты: положить woff2 в репо или CDN — решить на spike.
- `@tanstack/react-pacer` 0.x и `react-hotkeys` 0.x — API может двигаться; пиним минор.
- `rerun:resume` в UI появляется после проверки на пайплайне (`STROPPY.MD` §14).

## 16. Как устроен код сейчас (ориентир для параллельной работы)

Фундамент готов и зелёный (`yarn typecheck`, `yarn lint`). Эталон экрана —
`components/runs/RunListPage.tsx` + `routes/t/$slug/runs/index.tsx`: копировать
паттерн, не изобретать.

- **API**: `api/client.ts` (`api`, `unwrap`), `api/errors.ts` (`ApiError`,
  `fieldErrorsFrom`), `api/types.ts` (алиасы схем), `api/queries/<area>.ts`
  (`xQueries` = `queryOptions`/`infiniteQueryOptions`, `xMutations` = функции).
  Ключи через `keys.t(slug)` / `keys.me()` / `keys.catalog()` / `keys.admin()`.
- **Mock**: `api/mock/handlers/<area>.ts` регистрирует `route(method, pattern,
  handler)`; паттерн `'/api/v1/t/:slug/runs/:id:cancel'`; ответ `{ json }`,
  `{ status: 201, json }`, `noContent()`, `problem(400, 'validation_failed',
  detail, { validation: { errors: [{ path, code, severity, message }] } })`.
  Данные — `store.tenant(slug)` (`TenantData`), сид — `api/mock/seed.ts`,
  live-прогоны — `api/mock/simulation.ts`, запуск рана — `run-factory.ts`
  (`launchFromTest`). Новый файл хендлеров добавляется в
  `api/mock/handlers/index.ts`. Списки — `paginate(items, parseListQuery(q))`.
- **Breadcrumbs — фича роутера, не страницы.** Цепочка строится из
  `useMatches()` в `app/crumbs.ts` и рендерится в `app/Breadcrumbs.tsx` внутри
  `AppShell`. Маршрут отдаёт свой crumb одним из двух способов: layout/список —
  `staticData: { crumb: 'nav.runs' }` (i18n-ключ); detail — `loader` возвращает
  `{ crumb: entity.name }`. Каждая секция имеет layout-роут
  `routes/t/$slug/<section>/route.tsx` со `staticData.crumb`, табы detail —
  `staticData: { crumb: 'runs.tabs.logs' }`. `PageHeader.breadcrumbs` ничего не
  рисует (устарел, не использовать). Публичные страницы: `staticData.bare: true`
  прячут sidebar и crumbs.
- **Каркас (`app/AppShell.tsx`)**: шелл владеет вьюпортом — `root` = `100vh`,
  `overflow: hidden`, `body { overflow: hidden }` (`App.tsx`). Шапка 48px
  (`HEADER_HEIGHT`): бренд · `GlobalSearch` по центру · `TenantSwitcher` ·
  `UserMenu`. Ниже `body` = sidebar + `main`; в `main` — crumb-бар 36px
  (`CRUMB_BAR_HEIGHT`) и `content` (`flex:1; overflow:auto`,
  `data-scroll-restoration-id="app-content"`) — **скроллится только он**.
  Страница, которой нужна высота панели «до низа», считает
  `100vh - HEADER_HEIGHT - CRUMB_BAR_HEIGHT - …` или `useOffsetTop`.
  Никаких page-level скроллбаров ни по одной оси.
- **Sidebar (`app/Sidebar.tsx`)** — рельса как в Supabase console: 56px
  иконки (`SIDEBAR_RAIL`), при hover или фокусе клавиатурой раскрывается до
  232px (`SIDEBAR_EXPANDED`, 150 мс) **поверх** контента; кнопка «Закрепить»
  (`IconButton`, низ) держит её раскрытой и **толкает** контент. В localStorage
  `stroppy.sidebar` хранится только `{ pinned }`; ресайза нет. Секции —
  uppercase-заголовок + тонкий divider над каждой (кроме первой), пункты с
  отступом; в рельсе — только divider'ы и `Tooltip placement="right"`. Низ
  (Примеры / Настройки / Админ / pin) — `margin-top: auto`, всегда у нижнего
  края. Пункт добавляется в `SECTIONS`/`BOTTOM` там же, ключ — `nav.*`.
  Тенанта в sidebar нет.
- **TenantSwitcher (`app/TenantSwitcher.tsx`)** — в шапке слева от аватара:
  `Dropdown` + `Menu` (группа «Тенанты» с check на текущем, `slug · роль`
  в description, divider, «Создать тенант»), триггер — `ToolbarButton
  icon="building" isOpen` (шеврон рисует сам ToolbarButton).
- **Самодельных контролов в шелле нет**: поиск — grafana `Input`
  (`prefix`/`suffix`/`loading`) + `Menu`/`Menu.Item` в позиционированном
  контейнере; аватар — `ToolbarButton` с `UserAvatar` (URL пользователя или стабильный
  GitHub-подобный identicon по ID; общий для профиля, участников и авторов); подсказки клавиш — `Text variant="bodySmall"
  color="secondary"`, без своих `kbd`. Интерактивное в sidebar — `Icon`,
  `Tooltip`, `IconButton`; сам пункт — `AppLink` + emotion (nav-компонента
  standalone в `@grafana/ui` нет).
- **Route**: `validateSearch` zod-схемой (`.optional().catch(undefined)` для
  фильтров, `.default(x).catch(x)` для sort/order) + `stripSearchParams` для
  дефолтов; `loaderDeps` + `loader: ensureQueryData`; компонент читает
  `Route.useSearch()` и пишет `navigate({ search: prev => ({...prev, ...next}),
  replace: true })`.
- **Live**: `useTopic({ topic, queryKey, merge, batchMs })` кладёт WS-ивенты в
  тот же ключ, что REST. Топики: `run/{id}`, `run.overview/{id}`,
  `run.events/{id}`, `run.logs/{id}`, `run.metrics/{id}`, `suite_run/{id}`,
  `tenant.runs/{slug}`.
- **UI-кит**: `app/Page`, `app/PageHeader` (crumbs/title/badge/actions/tabs),
  `app/AppLink`, `app/ErrorState`, `app/Toaster` (`toast.success/error`),
  `components/DataTable` (+ `Toolbar`), `StatusBadge`, `RelativeTime`,
  `KeyValueList`, `CopyText`, `ConfirmAction`, `TagsEditor`, `FavoriteButton`,
  `components/schema/SchemaForm` (+ `engine.ts`, `hooks/useSchemaValidate`).
  Хуки: `useTenant()` → `{ slug, tenant, role, can(action), me }`, `useMe()`.
- **Локали**: `locales/{en,ru}/<area>.json`, ключ `t('<area>.x.y')`; `common.*`
  и `nav.*` — общие. Оба языка в одном изменении.

## 17. Таблицы списков: сортировки, фильтры, экшены (эталон — main-v0 Runs)

**Что и почему — `web/docs/tables-guide.md`** (задачи таблицы, типы ячеек,
плотность, состояния, чек-лист приёмки). Здесь — как это устроено в коде.

- **Колонки только через фабрики `col.*`** (`DataTable/columns.tsx`):
  `identity` (имя + подстрока + ссылка, закреплена слева, не скрывается),
  `status` (иконка, закреплена слева), `statusText`, `text`, `pair` («postgres
  17»), `number` (справа, моно, `unit` через `formatMetric`, `bar` — полоска
  против максимума страницы), `duration`, `time`, `tags`, `link`, `bool`,
  `favorite`, `actions`, `custom` (редкий выход — тренд, размеры). Фабрика
  фиксирует ширину (`WIDTH`), выравнивание и ячейку; руками `cell:` в
  страницах не пишем.
- **Ячейки** (`DataTable/cells.tsx`): `IdentityCell`, `PairCell`,
  `NumberCell`, `TagsCell`, `LinkCell`, `BoolCell`, `TextCell`, `Dash` («—»
  для отсутствующего значения, никогда пусто/`0`/«n/a»).
- **Вид таблицы per-viewer**: `const prefs = useTablePrefs('<tableId>',
  columns)` → `<DataTable columns={prefs.visible} density={prefs.density}>` и
  `<DataTableToolbar settings={<TableSettings prefs={prefs} />}>`. Колонка
  задаёт `defaultHidden` (выключена по умолчанию) и `hideable: false`
  (идентификатор, действия). Хранится в localStorage `stroppy.table.<id>`
  дельтой к дефолтам колонок.
- **Состояния** — пропсы `DataTable`, не разметка страницы: `loading` (скелет
  строк), `error` + `onRetry`, `empty={{ message, button }}` (данных нет —
  призыв к действию), `filtered` + `onClearFilters` («ничего не найдено» +
  сброс). `bulkActions={(ids) => …}` — панель «Выбрано N» над таблицей при
  выборе строк.
- **Закрепление**: `sticky: 'left'` — ведущий префикс колонок (статус, имя),
  `sticky: 'right'` — избранное и действия; фабрики ставят это сами.
- **Клавиатура**: кликабельная строка фокусируется Tab, Enter открывает её.


Все списки сущностей (Runs, Tests, Databases, Workloads, Suites, Schedules,
Shares) строятся на `components/DataTable` и обязаны давать сортировку и
фильтр по каждому полю, у которого есть серверный ключ, плюс меню действий в
строке. Эталон кода — `components/runs/RunListPage.tsx`.

- **Шапка колонки** — `DataTable/ColumnHeader`: три слота `[⇅ сорт] Метка
  [фильтр]`, ширина слотов резервируется всегда, чтобы метки выравнивались.
  Сортировка тристабильная: нейтрально → asc → desc → нейтрально (страница
  восстанавливает дефолт). Колонка объявляет `sortKey` (серверный ключ из
  `sort` enum OpenAPI); нет ключа — нет слота сортировки. `clientSort` — только
  для ограниченных in-memory списков (рейтинг).
- **Фильтр колонки** — `filter: { kind: 'checklist' | 'text' | 'date' |
  'number', … }` (`DataTable/types.ts`, тела в `DataTable/filters.tsx`).
  Checklist берёт варианты с count'ами из фасетов (`:facets`) или из
  загруженных строк; `single: true`, если серверный параметр одиночный
  (author). Text/number применяются по Enter/blur, date — два `DateTimePicker`.
  Popover — grafana `Toggletip` с контролируемым `show`; открытая колонка
  хранится в `DataTable` (по id колонки), поэтому refetch его не закрывает.
- **Закреплённые колонки** — `sticky: 'right'` на колонке (`fav`, `actions`
  во всех списках): `position: sticky` с непрозрачным фоном, всегда видны при
  горизонтальном скролле широкой таблицы. Экшены строки никогда не должны
  уезжать за край — новая таблица обязана помечать их `sticky`.
- **Экшены строки** — `DataTable/RowActionsMenu` (кебаб `ellipsis-v`, grafana
  `Dropdown`+`Menu`). Рендерятся **все** действия сущности; недоступные —
  `disabled` с причиной в `disabledReason` (никогда не скрываются),
  деструктивные — `destructive` + `group: true` перед ними. Клики не
  пробрасываются в навигацию строки (`data-no-row-click`). Подтверждение
  удаления — один `ConfirmModal` на страницу. Звезда избранного —
  `FavoriteButton` в своей колонке, где сущность это поддерживает.
- **Состояние — только URL** (`validateSearch` zod в route, общие куски в
  `DataTable/list-search.ts`): `q` (поиск), `name` и прочие фильтры по
  колонкам, `sort` + `order`, `size` (25/50/100), `refresh`
  (`off|5s|15s|30s|1m`; дефолт `5s` у Runs и Schedules, `off` у остальных).
  Дефолты стрипаются `stripSearchParams(<PAGE>_DEFAULTS)`. Курсорная
  пагинация не меняется (`useInfiniteQuery`, «Показать ещё» в футере).
- **Панель над таблицей** — `DataTable/Toolbar`: поиск (debounce 300 мс),
  булевы `Checkbox`-переключатели («Только избранные», «Живые стенды»),
  чипы активных фильтров + «Сбросить фильтры», счётчик строк, размер
  страницы, интервал автообновления и кнопка «Обновить». Пилюль-пресетов
  («Все / Выполняются / Мои») нет — это фильтры колонок.
- **Автообновление** — `DataTable/useAutoRefresh(refresh)` → `refetchInterval`;
  `DataTable.onOverlayChange` ставит его на паузу, пока открыт фильтр или
  меню строки (иначе refetch выдёргивал бы UI из-под пользователя).
- Сервер без параметра (поиск у schedules/shares, `target`) — фильтр
  применяется client-side к загруженной странице с комментарием в `to*Query`;
  кастов `as never` под несуществующие параметры не делаем.
- **Иконки**: только имена из `IconName`, для которых есть SVG в
  `public/gf/build/img/icons/unicons` (`flask` в типе есть, файла нет → `vial`).

## 18. Один скролл на экран

На экране ровно **один** прокручиваемый контейнер — тот, где данные. Шапка
страницы, табы, тулбар и фильтры стоят на месте; страница целиком не
скроллится никогда. Второй скролл страницы поверх скролла таблицы/лога — баг.

- **Список** (таблица сущностей): `<Page fill>` + `<DataTable fill>` — строки
  прокручиваются внутри таблицы под sticky-шапкой, «Показать ещё» — в футере
  таблицы.
- **Detail с вкладками**: `<Page fill>` → `PageHeader` (с табами) →
  `<PageFill scroll>{вкладка}</PageFill>`. Вкладка с обычным контентом
  (обзор, артефакты) прокручивается в `PageFill`; вкладка с «окном» данных
  (логи, метрики, события, топология, спека) заполняет его сама
  (`TelemetryLayout`, `flex: 1; min-height: 0`) и скроллит только своё окно.
- **Никаких `calc(100vh - Npx)` и `useOffsetTop` для высоты** — только
  flex-колонка «заполнить остаток» (`flex: 1 1 auto; min-height: 0`) от
  `Page fill` вниз. Ручная математика ломается от любого отступа и даёт
  второй скролл.
- Monaco в заполняющем блоке — `CodeEditorLazy height="fill"` (Grafana
  `CodeEditor` сам по себе не растягивается).
- Проверка: на 1920×1080 и на ноутбуке у страницы нет вертикального
  скроллбара; в DOM ровно один элемент с `overflow: auto`, у которого
  `scrollHeight > clientHeight`.

## 19. Ошибки и тосты

- **Тост — только `toast` из `@app/Toaster`** (одна Grafana-карточка: иконка
  по severity, заголовок, пояснение, список проблем валидации, ID запроса с
  копированием, закрытие). `sonner` напрямую не импортировать.
- **Ошибку передавать целиком**: `onError: (e) => toast.error(e)` или
  `toast.error(e, { title: t('…failed') })`, когда нужно назвать действие.
  `describeError` (`app/describe-error.ts`) строит заголовок по стабильному
  `code` сервера (`common.errors.codes.*`), пояснение — из `detail`,
  проблемы — из `validation`. Заменять ошибку общей строкой («Что-то пошло не
  так»), `e.title` или пустой строкой — баг: пользователь не узнаёт, что
  случилось.
- Формы: ошибки полей — в `setErrorMap` у полей; тост — для того, что к полю
  не привязано (и только если оно есть).
- Предотвращать лучше, чем сообщать: действие, недоступное в текущем
  состоянии, заранее `disabled` с причиной (см. §17), а не 409 в тосте.
