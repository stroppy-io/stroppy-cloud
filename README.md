# Stroppy Cloud

Database benchmarking as a service. The product server over
[Graphene CI](https://github.com/graphene-ci): tenants, benchmark model and
results UI live here; provisioning, agents, runs and observability are
Graphene's. Identity is self-hosted [Ory Kratos](https://www.ory.sh/kratos/)
(`deployments/kratos/`).

The versioned server/UI/pipeline interface, examples and compatibility checks
are documented in [the integration handoff](pipelines/live/tests/platform/contracts/HANDOFF.md).

## Layout

- `cmd/stroppy-cloud/` — the server binary (API + embedded SPA).
- `internal/` — server code.
- `pipelines/` — Graphene pipelines (`stroppy-run`, `stroppy-suite`,
  provider verify/config, quotas), a separate Go module released with the
  server; `pipelines/spec` is the run specification shared with the server.
- `openapi/parts/` — OpenAPI sources for the Go server and TypeScript API types.
- `web/` — SPA (React 19, Vite, `@grafana/ui`); правила — `web/AGENTS.md`.
  `web/src/kit` — the UI kit (tokens, plain-HTML styles, charts) a release
  ships as `stroppy-kit-<version>.tar.gz`.
- `docs/` — documentation and design notes.

## Development

For the local server with embedded UI and PostgreSQL, see the
[Docker Compose instructions](deployments/README.md). Run `make dev` after
configuring `.env.compose`; this connects to an existing Graphene cluster.

```bash
make configure
make build
make test
make lint
make web-dev
```
