---
name: implement-api-operation
description: Implement or change a Conduit HTTP operation that already exists in the immutable RealWorld OpenAPI contract. Use for handler, service, authentication, DTO, model, or endpoint behavior work; do not use to invent a local API extension.
---

# Implement a Conduit API operation

Treat `api/open-api.yml` and `apitests/` as immutable upstream inputs. Read the
root `AGENTS.md` before making changes.

## Establish the path

1. Locate the operation in `api/open-api.yml` and the corresponding generated
   server interface and types in `internal/gen/http/server.go`.
2. Trace the nearest existing transport, service, model, and repository
   implementation before deciding which layers need changes.
3. Identify authentication, validation, authorization, transaction, error, and
   response-shape requirements from the contract and neighboring operations.
4. If the requested operation is absent from the upstream contract, stop and
   explain the mismatch instead of editing the schema.

## Implement

- Keep request decoding and response encoding in transport.
- Keep business rules and composition of related records in services.
- Define narrow dependency interfaces in the consuming service package.
- Use typed service errors when the HTTP response needs a specific status or
  resource key.
- Resolve external identifiers to UUIDs once and batch-load related records.
- Use `Transactions.WithTx` for atomic multi-step writes; never expose pool or
  transaction lifecycle operations to services.
- Preserve omitted versus explicit `null` on updates.
- Never hand-edit generated files.

## Cover and verify

- Add table-driven service tests for success, validation, authorization, and
  every propagated or mapped dependency error.
- Add transport coverage when decoding, status mapping, authentication, or the
  response shape changes.
- Regenerate only affected outputs, then run `make verify-backend` and
  `make verify-generated`.
- Run `make verify-contract`, start the server on `:8000`, and run
  `make test-integration` for HTTP-visible behavior.
- Report the Hurl runner summary rather than claiming success from isolated
  requests.
