# Kratos — identity of Stroppy Cloud

Self-hosted [Ory Kratos](https://www.ory.sh/docs/kratos): вход, регистрация,
verification/recovery (коды по почте), TOTP, сессии. Заменил `gopherex/iam`;
сервер не имеет таблицы паролей и не вызывает Kratos на запросах — см. ниже.

## Файлы

| Файл | Что это |
|---|---|
| `kratos.yml` | конфиг dev-compose (прод переопределяет env/секреты) |
| `identity.schema.json` | traits: `email` (идентификатор логина) + `name` |
| `claims.jsonnet` | claims JWT шаблона `stroppy`: iss/aud, email, email_verified, name, aal |
| `jwks/kratos-jwks.json` | приватный ES256-ключ подписи токенов (dev, в репо) |
| `jwks/kratos-public-jwks.json` | публичная половина — её читает сервер для локальной проверки |

## Контракт со stroppy-server

1. SPA логинится через **API-флоу** Kratos (`/self-service/login/api`) →
   opaque session token.
2. SPA зовёт `GET /sessions/whoami?tokenize_as=stroppy`
   (`X-Session-Token`) → получает JWT (TTL 10m, claims из `claims.jsonnet`).
3. Дальше весь API — `Authorization: Bearer <jwt>`; сервер проверяет
   подпись **локально** по `kratos-public-jwks.json` (без хопа в Kratos).
   `sub` = identity id = ключ профиля, `sid` = сессия, `email`+`email_verified`
   заменяют прежний `users/me`.
4. Инвайты в тенант матчатся **только по verified email**.

WS (`/api/v1/ws?token=`) принимает тот же JWT — путь через верификатор общий.

## Dev (make dev)

- Kratos public: `http://localhost:4433` (CORS на :18347 и :5173), admin: `:4434`.
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
