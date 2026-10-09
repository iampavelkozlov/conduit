# AGENTS.md

## Repository purpose

Conduit is a production-oriented implementation of the RealWorld application:
a Go HTTP API backed by PostgreSQL and a Vue 3 frontend. The backend uses an
OpenAPI-first transport, Chi, `pgx`, `sqlc`, Goose, Wire, Prometheus, and Hurl.

The external API contract is owned by the upstream RealWorld specification.
Implementation code must conform to that contract.

## Non-negotiable contract invariants

- Keep `api/open-api.yml` byte-for-byte identical to the upstream RealWorld
  OpenAPI document.
- Keep every file in `apitests/` byte-for-byte identical to the upstream Hurl
  suite. Do not add local Hurl files there.
- Never edit the schema or Hurl tests to make the implementation pass. Fix
  handwritten code, SQL, migrations, configuration, or generator inputs.
- Keep environment-specific server configuration out of the OpenAPI document.
- Run `make verify-contract` after work that can affect the API contract or
  integration behavior.

## Architecture

- Treat `.go-arch-lint.yml` as the enforceable dependency contract.
- Keep `cmd/server/main.go` limited to composition and runtime configuration.
- Transport decodes requests, invokes services, maps errors, and encodes
  responses. Business rules belong in services.
- Transport may call services but never repositories.
- Services own narrow dependency interfaces and may depend on models,
  configuration, and repository contracts.
- Repositories must not import services or transport. Only the composition root
  assembles layers.
- Keep transaction handles and pool operations out of services. Use
  `Transactions.WithTx`, and use only the repository passed to its callback for
  atomic work.
- Put all SQL in `migrations/queries/*.sql`; never embed handwritten SQL in Go
  services or handlers.
- Keep read queries table-scoped and narrowly projected. Compose related API
  models in services instead of using multi-table aggregation queries.
- Never introduce per-row repository calls for list endpoints. Deduplicate IDs
  and use bounded `ANY(uuid[])` batch queries for related data.
- Prefer ID-only existence and lookup queries over loading unused columns,
  especially credentials and large article bodies.

Regenerate `docs/architecture.svg` when component dependencies change.

## Domain and API behavior

- Use UUIDs for every domain primary key, foreign key, relation lookup, and
  internal service identifier.
- Create persisted IDs with `shared.NewUUID` or `shared.NewUUIDValue`; preserve
  the UUIDv8 constraints in table-creation migrations.
- Preserve the reversible comment UUIDv8 layout in
  `internal/service/comment/id.go`; do not add a second integer comment ID.
- Resolve boundary keys such as article slugs to UUIDs once, then use UUIDs for
  subsequent repository calls.
- Use typed errors from `internal/service/shared` when a response needs a
  specific status and resource key.
- Preserve omitted versus explicit JSON `null`, especially for nullable `bio`
  and `image` updates.
- Validate only supplied update fields. Empty or null required scalar fields
  must return the contractually required `422` response.
- Do not return response fields absent from the canonical schema.
- Keep multi-step relation replacement atomic, preferably in one SQL statement
  or an explicit transaction.
- Add a new Goose migration for schema changes. Do not rewrite an applied
  migration unless the task explicitly concerns an unreleased migration.

## Metrics

- Repository metric labels may contain only generated Go method names and a
  fixed result label. Never label with SQL, arguments, IDs, slugs, or errors.
- HTTP metric labels may contain only normalized methods, Chi route templates,
  and status codes. Never label with raw paths, queries, request values, panic
  values, or error text.

## Generated sources

Never edit these files manually:

- `internal/gen/http/server.go`
- files under `internal/gen/postgres/`
- generated `mock_*.go` files
- `cmd/server/wire_gen.go`
- `internal/metrics/querier_metrics_gen.go`
- `frontend/src/api/generated/schema.d.ts`

Change their inputs and regenerate them with the relevant target:

- OpenAPI server: `make gen-http`
- sqlc: `make sqlc`
- mocks: `make mocks`
- repository metrics wrapper: `make wrap`
- Wire: `make wire`
- all backend outputs: `make generate`
- frontend API types: `make frontend-generate`
- architecture graph: `make arch-graph`

Generator versions are pinned in `Makefile`; do not change them casually.

## Tests and verification

- Every change to handwritten behavior requires meaningful automated coverage
  in the same change.
- Prefer table-driven Go tests with descriptive case names, `gomock` at
  dependency boundaries, and `testify/require` assertions.
- Cover success, validation, authorization, and every meaningful dependency
  error. Mock expectations must prove that later dependencies are not called
  after an earlier failure.
- For batch behavior, cover deduplication, empty input, invalid IDs, missing
  relations, and errors from every batch dependency. Tests must detect N+1
  regressions.
- Unit tests must be deterministic, isolated from developer `.env` values, and
  safe to run concurrently. Prefer `t.Context()`, `t.Setenv`, `t.TempDir`, and
  in-memory fakes. Unit tests must not require a live database.
- Do not weaken lint, architecture, coverage, or generated-source checks to
  make a change pass. The handwritten Go coverage floor remains 95.1%.

Choose verification by changed area:

- Go/backend: `make verify-backend`
- Frontend: `make verify-frontend`
- Generated inputs or outputs: `make verify-generated`
- API contract or Hurl-sensitive behavior: `make verify-contract`, then start
  the server on `:8000` and run `make test-integration`
- Cross-stack or release-facing changes: `make verify`

The canonical Hurl runner summary is the authoritative integration result.
Stop the test server after the suite finishes.

## Frontend

- Follow `frontend/AGENTS.md` for frontend-specific conventions when working
  from that directory. The essential rules also apply when working from the
  repository root.
- Keep API access in `frontend/src/api/` and shared login state in
  `frontend/src/stores/auth.ts`.
- Reuse the existing shadcn-vue primitives under `frontend/src/components/ui/`
  before adding new UI foundations.
- Preserve accessibility semantics and sanitize rendered rich text.
- Do not hand-edit generated API types.

## Working-tree and agent safety

- Inspect `git status --short` before editing and preserve unrelated changes.
- Do not reset, discard, overwrite, or reformat user-owned changes outside the
  task.
- Avoid destructive database cleanup; canonical Hurl tests use unique IDs.
- The primary agent owns implementation edits. Use subagents mainly for
  independent exploration and read-only review; do not let parallel agents
  edit the same files.
- Run commands that write shared outputs, generated files, coverage profiles,
  or database state sequentially.

## Reusable workflows

Repository skills under `.agents/skills/` define the detailed workflows for
implementing API operations, adding sqlc queries, verifying changes, and
running multi-agent reviews. Invoke them explicitly with `$skill-name` when a
task benefits from a named workflow; Codex may also select them from their
descriptions.
