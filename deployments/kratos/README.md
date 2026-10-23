# Kratos — identity of Stroppy Cloud

Self-hosted [Ory Kratos](https://www.ory.sh/docs/kratos): вход, регистрация,
подтверждение почты и восстановление пароля (коды по почте), смена пароля,
сессии. Заменил `gopherex/iam`;
сервер не имеет таблицы паролей и не вызывает Kratos на запросах — см. ниже.

## Файлы

| Файл | Что это |
|---|---|
| `kratos.yml` | конфиг dev-compose (прод переопределяет env/секреты) |
| `identity.schema.json` | traits: `email` (идентификатор логина) + `name`, оба обязательны |
| `claims.jsonnet` | claims JWT шаблона `stroppy`: iss/aud, email, email_verified, name, aal |
| `github.jsonnet` | вход через GitHub: claims провайдера → traits, подтверждённая GitHub почта сразу подтверждена |
| `courier.templates` в `kratos.yml` | письма Kratos: шаблоны лежат в бинаре сервера (`internal/infrastructure/kratos/courier`), Kratos берёт их по HTTP с внутреннего порта сервера (`:18348`, наружу не публикуется) |
| `jwks/kratos-jwks.json` | приватный ES256-ключ подписи токенов (dev, в репо) |
| `jwks/kratos-public-jwks.json` | публичная половина — её читает сервер для локальной проверки |

## Контракт со stroppy-server

1. Kratos живёт на том же origin, что SPA: сервер проксирует `/v1/*` в
   Kratos public (`STROPPY_AUTH_KRATOS_UPSTREAM`, префикс срезается);
   `serve.public.base_url` = `<origin>/v1`. Cookie сессии и CSRF работают
   без cross-site настроек.
2. SPA проходит **browser-флоу** с `Accept: application/json` (API-флоу
   Kratos v26 из браузера отсекает CSRF-защита): начать или продолжить flow
   по `flow`-id, отправить форму с `csrf_token` из `ui.nodes`.
3. Сессия — cookie Kratos. SPA меняет её на JWT:
   `GET /v1/sessions/whoami?tokenize_as=stroppy` (TTL 10m, claims из
   `claims.jsonnet`), тихо обновляет до протухания.
4. Весь API — `Authorization: Bearer <jwt>`; сервер проверяет подпись
   **локально** по `kratos-public-jwks.json`. `sub` = identity id = ключ
   профиля, `email`+`email_verified` — адрес и его подтверждение.
5. Инвайты в тенант и админство матчатся **только по подтверждённой почте**.

WS (`/api/v1/ws?token=`) принимает тот же JWT.

## Экраны flow'ов (`ui_url`)

Каждый flow ведёт в свой экран SPA, ссылки из писем и редиректы Kratos
приходят туда с `?flow=<id>`:

| Flow | Экран | Что происходит |
|---|---|---|
| login | `/login` | почта + пароль; «Забыли пароль?» → `/recovery` |
| registration | `/register` | почта, имя, пароль; хуки `session` + `show_verification_ui` — сразу вход и переход на `/verify` |
| verification | `/verify` | код из письма; ссылка из письма приходит с кодом в flow — экран отправляет его сам; повторная отправка |
| recovery | `/recovery` | почта → код → Kratos выдаёт привилегированную сессию и settings-flow → `/me/security?flow=` |
| settings | `/me/security` | статус почты, смена пароля (старой сессии — сначала повторный вход, `session_refresh_required`) |
| error | `/auth/error?id=` | текст ошибки Kratos |

Пока почта не подтверждена, в каркасе приложения висит баннер с переходом
на `/verify`. Сообщения Kratos SPA показывает по их числовым `id`
(`auth.kratos.<id>` в локалях, значения из `context`) под полем, к которому
они относятся; неизвестный `id` — текстом Kratos.

## Dev (make dev)

- Kratos public: через сервер `http://localhost:18347/v1` (напрямую `:4433`), admin: `:4434`.
- Courier → **Mailpit**: письма видны на `http://localhost:8025`, наружу не уходят.
- СУБД — общий postgres-контейнер, база `kratos` (создаётся init-скриптом
  `deployments/postgres/init/`); миграции Kratos применяет сам.

## Прод-инсталляция

- `dsn` и `secrets.default` — через env (`DSN`), не из файла.
- **Закрытая платформа**: оверлей конфига ставит
  `selfservice.flows.registration.enabled: false` — публичной регистрации
  нет; приглашённые тоже не смогут сами завести аккаунт (инвайт-кодов в
  Kratos нет), identity создаёт админ через admin API (`POST :4434/admin/
  identities`) или тумблер временно включают. SPA это понимает сам:
  `/register` пробит готовность флоу и показывает «регистрация закрыта».
- Подписи токенов: **свой** ES256-ключ на инсталляцию (файл не из репо!),
  публичная половина — в `STROPPY_AUTH_JWKS_FILE` сервера. Ротация: новый `kid`
  вперёд, старый живёт пока валидны 10-минутные токены.
- `serve.public.base_url` → `https://iam.stroppy.io`, CORS = origin SPA,
  courier → реальный SMTP (Postbox).
- Dev-ключи из репо использовать нельзя — они публичны.
