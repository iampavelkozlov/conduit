# AGENTS.md

## Repository overview

This repository is a Go implementation of the RealWorld (Conduit) HTTP API. It
uses an OpenAPI-first HTTP layer, PostgreSQL, `pgx`, `sqlc`, Chi, and Hurl
integration tests.

The canonical external contract is the RealWorld API specification. Application
code must conform to that contract; never adjust the contract or its tests to
fit the implementation.

## Non-negotiable upstream invariants

- `api/open-api.yml` must remain byte-for-byte identical to:
  <https://github.com/realworld-apps/realworld/blob/main/specs/api/openapi.yml>
- The contents of `apitests/` must remain byte-for-byte identical to:
  <https://github.com/realworld-apps/realworld/tree/main/specs/api/hurl>
- Do not add local `.hurl` files to `apitests/`.
- Do not edit the OpenAPI schema or Hurl tests to make a failing implementation
  pass. Fix Go code, SQL, migrations, runtime configuration, or generated code
  inputs instead.
- After relevant work, compare both the schema and Hurl files with upstream.

The canonical schema advertises the production server URL. Local routing is
configured at runtime in `cmd/server/main.go`; keep environment-specific server
configuration out of the schema.

## Repository layout

- `api/open-api.yml` — immutable canonical OpenAPI document.
- `apitests/` — immutable canonical RealWorld Hurl test suite and runner.
- `cmd/server/main.go` — dependency wiring, middleware, router, and server start.
- `internal/gen/http/` — generated `oapi-codegen` HTTP types and handlers.
- `internal/gen/postgres/` — generated `sqlc` query code. Do not edit manually.
- `internal/models/` — conversion between generated API DTOs and domain models.
- `internal/metrics/` — Prometheus registry, HTTP instrumentation, and generated
  repository decorator.
- `internal/repository/transaction/` — transaction lifecycle implementation.
- `internal/service/` — business logic, split by domain.
- `internal/service/shared/` — shared context, identifiers, and typed API errors.
- `internal/storage/postgres/` — PostgreSQL pool initialization.
- `internal/transport/http/` — HTTP handler implementations.
- `internal/transport/middleware/` — authentication and response middleware.
- `docs/architecture.svg` — generated architecture dependency graph.
- `migrations/*.sql` — Goose database migrations.
- `migrations/queries/*.sql` — source SQL for `sqlc` generation.
- `config/config.yaml` — default runtime configuration.
- `config.yaml` — `oapi-codegen` configuration.
- `sqlc.yaml` — `sqlc` configuration.

## Architecture and implementation rules

- Keep `cmd/server/main.go` limited to wiring and runtime configuration.
- Treat `.go-arch-lint.yml` as the enforceable architecture contract. Transport
  may call services but never repositories; services may depend on models,
  configuration, and narrow repository contracts; repositories must not import
  services or transport. Only the composition root may assemble across layers.
- HTTP handlers decode requests, invoke services, map errors, and encode
  responses. Business rules belong in services.
- Services depend on narrow interfaces defined in their package.
- Keep transaction handles, pool operations, begin, commit, and rollback out of
  services. Use `Transactions.WithTx`; only invoke methods on the repository
  passed to its callback for work that must be atomic.
- Keep repository metric labels bounded. Use generated Go method names and a
  fixed result label; never add SQL text, arguments, user IDs, slugs, or other
  unbounded values as Prometheus labels.
- Keep HTTP metric labels bounded. Use Chi route templates, normalized HTTP
  methods, and response status codes; never use raw paths, query strings,
  request values, user IDs, slugs, panic values, or error text as labels.
- Use typed errors from `internal/service/shared` when a response requires a
  specific status and resource key, for example `errors.article`,
  `errors.comment`, or `errors.username`.
- All SQL belongs in `migrations/queries/*.sql`. Never embed handwritten SQL in
  Go services or handlers.
- Keep read queries table-scoped and narrowly projected. Do not assemble API
  read models with multi-table JOIN/aggregation queries; load base entities and
  related IDs in bounded batch queries, then compose models in services.
- Never introduce per-row repository calls for list endpoints. Deduplicate the
  collected IDs and use `ANY(uuid[])` batch methods for users, follows, tags,
  favorites, comments, and other relations.
- Select and return only columns used by the caller. Prefer dedicated ID-only
  existence/filter lookups over loading full rows, especially rows containing
  credentials or large article bodies.
- Use UUIDs for every domain entity primary key, foreign key, relation lookup,
  and internal service identifier. Resolve boundary keys such as article slugs
  to UUIDs once, then use the UUID for subsequent repository operations.
- Generate persisted identifiers with `shared.NewUUID` or
  `shared.NewUUIDValue`; both produce UUIDv8 values. Keep the UUIDv8 database
  constraints in table-creation migrations so alternate write paths cannot
  violate this invariant.
- Comment UUIDs use the stable UUIDv8 layout defined in
  `internal/service/comment/id.go`. Their 62-bit payload is exposed as the
  integer comment ID required by the immutable RealWorld contract. Keep this
  encoding reversible and do not add a second integer ID column.
- Schema changes require a new Goose migration; do not rewrite applied
  migrations unless explicitly required for an unreleased migration.
- Keep multi-step relation replacement atomic when possible. Prefer one SQL
  statement or an explicit transaction over delete-then-insert sequences.
- Preserve the distinction between an omitted update field and an explicit
  JSON `null`. This is especially important for nullable `bio` and `image`.
- For update endpoints, validate only fields that were supplied. Empty or null
  required scalar fields must produce the contractually required `422` response.
- API responses must not contain fields absent from the canonical schema.

## Generated code

Never manually edit:

- `internal/gen/http/server.go`
- files under `internal/gen/postgres/`
- `mock_*.go` files generated by `mockgen`
- `cmd/server/wire_gen.go`
- `internal/metrics/querier_metrics_gen.go`

Regenerate HTTP code after syncing the OpenAPI document:

```bash
make gen-http
```

Regenerate database code after changing migrations or query SQL:

```bash
make sqlc
```

Regenerate mocks after changing a service interface:

```bash
go generate ./internal/service/... ./internal/transport/http ./internal/transport/middleware
```

Regenerate the repository metrics wrapper after changing the sqlc `Querier`
interface or its template:

```bash
make wrap
```

Regenerate the Wire injector after changing providers:

```bash
make wire
```

Regenerate the architecture graph after changing package dependencies or the
architecture configuration:

```bash
make arch-graph
```

Generator versions are pinned in `Makefile`. Do not switch generator versions
casually; generated output and runtime dependencies must remain compatible.

## Local development

The default database DSN is:

```text
postgres://postgres:postgres@localhost:5432/conduit?sslmode=disable
```

Start PostgreSQL using the available Compose command, then apply migrations:

```bash
docker-compose up -d
make migrate
```

The server defaults to port `8080`, but the canonical Hurl runner expects port
`8000`. Start the integration-test server explicitly:

```bash
go run ./cmd/server --addr :8000
```

Run the canonical integration suite in a separate shell:

```bash
bash apitests/run-hurl-tests.sh
```

The runner uses a unique `uid` and executes files serially. Do not replace it
with an altered local runner when reporting contract-test results.

## Testing and coverage rules

- Every change to handwritten behavior must include or update tests in the same
  change. New production code without meaningful automated coverage is not
  complete.
- Prefer table-driven tests with descriptive case names. Each operation must
  cover the successful path, validation and authorization failures, and every
  meaningful dependency failure that the operation can propagate or map.
- Use `go.uber.org/mock/gomock` for service, repository, transport, and other
  dependency boundaries. Depend on narrow interfaces owned by the consuming
  package; do not introduce concrete dependencies merely to simplify tests.
- Use `github.com/stretchr/testify/require` for test assertions. Assert returned
  values and typed errors, and rely on mock expectations to prove that later
  dependencies are not called after an earlier failure.
- Cover batch behavior explicitly: deduplication, empty input, invalid IDs,
  missing related records, and an error from each batch dependency. Tests must
  prevent regressions to N+1 repository calls.
- Unit tests must be deterministic, isolated from developer `.env` values, and
  safe to run concurrently. Use `t.Context()`, `t.Setenv`, `t.TempDir`, and
  in-memory fakes where appropriate. Do not require a live database for a unit
  test; database-backed behavior belongs in integration tests.
- Integration tests complement unit tests and never replace their error-path
  coverage.
- `make coverage` is the canonical coverage command. It uses
  `.testcoverage.yml`, excludes only generated sources, and enforces at least
  95.1% total coverage for handwritten code. Do not lower the threshold or add a
  handwritten file, package, function, or branch to coverage exclusions to make
  the gate pass. Add tests instead.
- Generated code may be excluded from the coverage calculation only through
  the narrowly scoped patterns in `.testcoverage.yml`. Do not add coverage
  annotations to generated files and do not edit generated files manually.
- Keep coverage at or above the existing percentage. New and materially changed
  handwritten code should be covered as completely as practical even when the
  repository-wide threshold is already satisfied.

## Required verification

For ordinary Go changes, run:

```bash
gofmt -w <changed-go-files>
go test ./...
go vet ./...
make quality
```

Unit tests, the coverage threshold, and both linters are mandatory gates. Fix
their findings in tests, code, or architecture. Do not silence findings with
broad exclusions, `nolint` directives, `todo`
architecture dependencies, coverage exclusions, or relaxed rules unless the
exception is narrowly scoped and technically justified. Run `make arch-graph`
whenever dependencies between components change and include the regenerated
SVG in the change.

For HTTP, authentication, service, model, SQL, or persistence changes, also:

1. Regenerate affected generated code.
2. Run `make quality`, fixing every test, coverage, lint, and architecture
   finding.
3. Regenerate `docs/architecture.svg` when component dependencies changed.
4. Start the server on `:8000`.
5. Run `bash apitests/run-hurl-tests.sh`.
6. Continue fixing application code until all 13 Hurl files pass.
7. Stop the test server when finished.
8. Verify the OpenAPI and Hurl files still match upstream exactly.

Do not report individual HTTP requests as independently successful unless Hurl
provides that breakdown. The authoritative integration result is the number of
succeeded and failed Hurl files plus the total executed requests.

## Working-tree safety

- The repository may contain pre-existing uncommitted work. Inspect
  `git status --short` before editing and preserve unrelated changes.
- Do not reset, discard, or overwrite user changes.
- Do not manually modify generated files to work around source-level errors.
- Avoid destructive database cleanup unless the user explicitly requests it;
  canonical Hurl tests use unique identifiers and should not need a clean DB.
