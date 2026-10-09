---
name: add-sqlc-query
description: Add or change a Conduit PostgreSQL repository capability through sqlc. Use for query SQL, schema migrations, repository interfaces, generated query methods, transaction use, or batch-loading changes.
---

# Add a sqlc repository capability

Read the root `AGENTS.md`, the relevant migration, query file, consuming service
interface, and neighboring generated methods before editing.

## Design the database operation

- Put query SQL in the appropriate `migrations/queries/*.sql` file.
- Add a new numbered Goose migration for schema changes; do not rewrite an
  applied migration.
- Select only columns the caller uses. Prefer an ID-only query for existence or
  boundary-key resolution.
- Keep reads table-scoped. Compose cross-table API models in services.
- For list flows, collect and deduplicate IDs before one bounded
  `ANY(uuid[])` query. Do not introduce per-row repository calls.
- Use UUIDs for all persisted and internal identifiers.
- Prefer one statement for relation replacement; otherwise use
  `Transactions.WithTx` from the service.

## Regenerate and integrate

1. Run `make sqlc`.
2. If the generated `Querier` interface changed, run `make wrap`.
3. Update the narrow interface owned by each consuming service.
4. Regenerate affected mocks with `make mocks`.
5. Update wiring only when the dependency graph actually changed; regenerate
   Wire and the architecture graph when needed.
6. Never repair generator output manually.

## Test and verify

- Cover the consumer's success path, empty input, invalid identifiers, missing
  rows, deduplication, and repository errors as applicable.
- Make mock expectations prove that the implementation stays batched and stops
  after a failed dependency.
- Run `make verify-backend` and `make verify-generated`.
- For HTTP-visible or persistence behavior, also run `make verify-contract` and
  the canonical Hurl suite against the server on `:8000`.
