# Conduit API

A production-oriented Go implementation of the
[RealWorld](https://github.com/realworld-apps/realworld) backend specification.
The service provides authentication, profiles, articles, comments, tags,
favorites, and personalized feeds through an OpenAPI-first HTTP API.

## CI status

[![Lint](https://github.com/iampavelkozlov/conduit/actions/workflows/lint.yml/badge.svg?branch=main)](https://github.com/iampavelkozlov/conduit/actions/workflows/lint.yml)
[![Tests](https://github.com/iampavelkozlov/conduit/actions/workflows/tests.yml/badge.svg?branch=main)](https://github.com/iampavelkozlov/conduit/actions/workflows/tests.yml)
[![Coverage](https://github.com/iampavelkozlov/conduit/actions/workflows/coverage.yml/badge.svg?branch=main)](https://github.com/iampavelkozlov/conduit/actions/workflows/coverage.yml)
[![Generated Code](https://github.com/iampavelkozlov/conduit/actions/workflows/generated.yml/badge.svg?branch=main)](https://github.com/iampavelkozlov/conduit/actions/workflows/generated.yml)
[![Build](https://github.com/iampavelkozlov/conduit/actions/workflows/build.yml/badge.svg?branch=main)](https://github.com/iampavelkozlov/conduit/actions/workflows/build.yml)
[![Release](https://github.com/iampavelkozlov/conduit/actions/workflows/release.yml/badge.svg)](https://github.com/iampavelkozlov/conduit/actions/workflows/release.yml)

## Code quality

[![Local Go Report Card](docs/goreportcard.svg)](https://github.com/gojp/goreportcard)
[![Handwritten Go LOC](docs/loc.svg)](docs/loc.svg)
[![Go version](https://img.shields.io/github/go-mod/go-version/iampavelkozlov/conduit)](go.mod)

The repository enforces a balanced `golangci-lint` configuration and explicit
layer boundaries with `go-arch-lint`. Generated code is reproducible and is
checked for uncommitted differences in CI. The report-card and LOC badges are
generated locally with `make badges`; generated Go sources are excluded from
the LOC count.

## Test coverage

[![Test coverage](docs/coverage.svg)](https://github.com/iampavelkozlov/conduit/actions/workflows/coverage.yml)

Coverage is measured for handwritten code. OpenAPI, `sqlc`, Wire, `mockgen`,
and repository-wrapper output is excluded through `.testcoverage.yml`. The CI
gate prevents total coverage from falling below 95.1%.

## Technology

- Go 1.26
- PostgreSQL 16 and `pgx`
- Chi HTTP router
- OpenAPI and `oapi-codegen`
- `sqlc` for type-safe database access
- Wire for dependency injection
- Prometheus metrics
- Goose migrations
- Hurl integration tests
- Vue 3 SPA with Vue Router, Pinia, and a generated OpenAPI client

## Architecture

The HTTP transport depends on application services, services depend on narrow
repository interfaces, and PostgreSQL implementations remain behind those
interfaces. Related entities are loaded in bounded batches and assembled in the
service layer to avoid both N+1 access patterns and oversized SQL queries.
Multi-step article writes and user registration use `Transactions.WithTx`; the callback receives the
same generated repository interface bound to a transaction, while the pool,
transaction handle, commit, and rollback remain in repository infrastructure.

![Architecture dependency graph](docs/architecture.svg)

The dependency rules are defined in `.go-arch-lint.yml`. Regenerate the graph
after changing package relationships:

```bash
make arch-graph
```

## Local development

Requirements: Go, Docker with Compose, and `make`.

```bash
cp .env.example .env
# Replace every change-me value in .env, then start the stack.
docker compose up -d --build
```

The Vue application and API are available through nginx at
`https://<PUBLIC_HOST>`. Port 80 redirects to HTTPS. On its first start, the
frontend container creates a local certificate authority and a server
certificate in `.data/certs`; install `.data/certs/ca.crt` as a trusted root on
client devices. PostgreSQL and the Go API are only reachable within the Compose
network. Compose waits for PostgreSQL, applies all Goose migrations, and then
starts the API and frontend. Override `CONDUIT_API_IMAGE` and
`CONDUIT_FRONTEND_IMAGE` to run published or pinned images. Their pull policies
can be controlled independently with `CONDUIT_API_PULL_POLICY` and
`CONDUIT_FRONTEND_PULL_POLICY`.

Runtime settings are loaded from `config/config.yaml` and can be overridden by
environment variables:

| Variable | Purpose |
| --- | --- |
| `DB_DSN` | PostgreSQL connection string |
| `LOG_LEVEL` | `debug`, `info`, `warn`, or `error` |
| `LOG_FORMAT` | `text` or `json` |
| `AUTH_JWT_SECRET` | JWT signing secret, at least 32 bytes |
| `AUTH_PASSWORD_PEPPER` | Password pepper, at least 16 bytes |
| `AUTH_ACCESS_TOKEN_TTL` | Access-token lifetime |
| `AUTH_REFRESH_TOKEN_TTL` | Refresh-token lifetime |
| `HTTP_ALLOWED_ORIGINS` | Comma-separated CORS allowlist |
| `PUBLIC_HOST` | IP address or hostname included in the generated TLS certificate |
| `CONDUIT_BIND_ADDRESS` | Host interface used for ports 80 and 443 |

The checked-in configuration is intended for local development. Supply unique
secrets, TLS-enabled database connectivity, and an explicit CORS allowlist in
production.

## Releases

Backend and frontend are versioned independently:

```bash
git tag -a backend-v1.0.0 -m "backend-v1.0.0"
git push origin backend-v1.0.0

git tag -a frontend-v1.0.0 -m "frontend-v1.0.0"
git push origin frontend-v1.0.0
```

Backend tags publish Go binaries, checksums, a GitHub Release, and the
multi-platform image `ghcr.io/iampavelkozlov/conduit-api`. Frontend tags publish
the static bundle, checksums, a separate GitHub Release, and the multi-platform
image `ghcr.io/iampavelkozlov/conduit-frontend`. Stable versions receive
version, major/minor, major, and `latest` image tags; prereleases do not move
`latest`.

The GHCR package must be public for anonymous Compose pulls. If it remains
private, authenticate once with `docker login ghcr.io` before starting the
stack.

## Development commands

```bash
make quality      # lint, architecture checks, tests, and coverage gate
make coverage     # coverage report and docs/coverage.svg
make badges       # local Go Report Card and handwritten Go LOC badges
make generate     # OpenAPI, sqlc, mocks, metrics wrapper, and Wire
make arch-graph   # docs/architecture.svg
```

Run the canonical RealWorld integration suite against a server listening on
port `8000`:

```bash
bash apitests/run-hurl-tests.sh
```

Run the frontend locally with hot reload (the dev server proxies API requests
to `localhost:8000`):

```bash
cd frontend
npm install
npm run dev
```

The TypeScript API types are generated from `api/open-api.yml` automatically
during `npm run build`. Run `npm run generate:api` to refresh them manually.

## Observability

HTTP request rate, status codes, latency, response size, in-flight requests,
recovered panics, repository call rate, repository errors, and repository
latency are exported in Prometheus format. Example PromQL queries are available
in [docs/metrics.md](docs/metrics.md).
