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

## Code quality

[![Go Report Card](https://goreportcard.com/badge/github.com/iampavelkozlov/conduit)](https://goreportcard.com/report/github.com/iampavelkozlov/conduit)
[![Go version](https://img.shields.io/github/go-mod/go-version/iampavelkozlov/conduit)](go.mod)

The repository enforces a balanced `golangci-lint` configuration and explicit
layer boundaries with `go-arch-lint`. Generated code is reproducible and is
checked for uncommitted differences in CI.

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

## Architecture

The HTTP transport depends on application services, services depend on narrow
repository interfaces, and PostgreSQL implementations remain behind those
interfaces. Related entities are loaded in bounded batches and assembled in the
service layer to avoid both N+1 access patterns and oversized SQL queries.

![Architecture dependency graph](docs/architecture.svg)

The dependency rules are defined in `.go-arch-lint.yml`. Regenerate the graph
after changing package relationships:

```bash
make arch-graph
```

## Local development

Requirements: Go, Docker with Compose, and `make`.

```bash
docker-compose up -d
make migrate
go run ./cmd/server --addr :8000
```

The API is available under `http://localhost:8000/api`; Prometheus metrics are
available at `http://localhost:8000/metrics`.

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

The checked-in configuration is intended for local development. Supply unique
secrets, TLS-enabled database connectivity, and an explicit CORS allowlist in
production.

## Development commands

```bash
make quality      # lint, architecture checks, tests, and coverage gate
make coverage     # coverage report and docs/coverage.svg
make generate     # OpenAPI, sqlc, mocks, metrics wrapper, and Wire
make arch-graph   # docs/architecture.svg
```

Run the canonical RealWorld integration suite against a server listening on
port `8000`:

```bash
bash apitests/run-hurl-tests.sh
```

## Observability

HTTP request rate, status codes, latency, response size, in-flight requests,
recovered panics, repository call rate, repository errors, and repository
latency are exported in Prometheus format. Example PromQL queries are available
in [docs/metrics.md](docs/metrics.md).
