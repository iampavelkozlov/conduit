---
name: verify-change
description: Select and run the required Conduit verification gates for the current change. Use before declaring implementation complete, when asked to run checks, or when diagnosing which backend, frontend, generated, contract, or integration checks apply.
---

# Verify a Conduit change

Verification must preserve unrelated work. Inspect `git status --short` and the
diff first, and do not clean, reset, or overwrite user changes.

## Select gates

- Go, configuration, backend tests, or migrations: `make verify-backend`.
- Frontend source or configuration: `make verify-frontend`.
- OpenAPI, sqlc SQL, Wire providers, mocks, wrapper templates, or architecture
  dependencies: `make verify-generated`.
- API contract or canonical test inputs: `make verify-contract`.
- Cross-stack, CI, release, or broad changes: `make verify`.
- HTTP, authentication, service, model, SQL, or persistence behavior: after the
  static gates pass, start the server on `:8000` and run
  `make test-integration`; stop the server afterward.

Run shared-output and database-writing gates sequentially. Do not run multiple
coverage, generation, or Hurl processes against the same checkout concurrently.

## Interpret failures

- Fix handwritten source or generator inputs, never generated output.
- Do not weaken lint, architecture, coverage, tests, or upstream comparisons.
- Distinguish an implementation failure from a missing local dependency or an
  unavailable database, and report the exact blocked gate.
- After a fix, rerun the failed focused gate, then the applicable aggregate
  gate.

## Report

List the commands run and their results. For Hurl, report the suite's succeeded
and failed file counts plus its total request count. Mention any gate not run
and the concrete reason.
