# Stroppy Cloud

Database benchmarking as a service. The product server over
[Graphene CI](https://github.com/graphene-ci): tenants, benchmark model and
results UI live here; provisioning, agents, runs and observability are
Graphene's. Identity is [gopherex/iam](https://github.com/gopherex/iam).

This branch is a ground-up rewrite. The previous implementation is preserved on
`main-v0`.

The versioned server/UI/pipeline interface, examples and compatibility checks
are documented in [the integration handoff](pipelines/live/tests/platform/contracts/HANDOFF.md).

## Layout

- `cmd/stroppy-cloud/` — the server binary (API + embedded SPA).
- `internal/` — server code.
- `pipelines/` — Graphene pipelines (`stroppy-run`, `stroppy-suite`), a separate
  Go module; `pipelines/spec` is the run specification shared with the server.
- `openapi/parts/` — OpenAPI sources for the Go server and TypeScript API types.
- `web/` — SPA (React, Vite, Tailwind).
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
