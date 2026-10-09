---
name: review-change
description: Review a Conduit branch, diff, or implementation for contract, architecture, data-access, security, and test risks. Use for explicit code-review or parallel-review requests; do not silently turn a review into an implementation task.
---

# Review a Conduit change

Reviews are read-only unless the user separately asks for fixes. Inspect the
current diff and repository instructions before delegating.

## Parallel review

When subagents are available, delegate independent read-only passes to the
project agents in `.codex/agents/`:

- `contract_reviewer`: immutable RealWorld behavior, HTTP semantics, generated
  boundaries, and response shapes.
- `architecture_reviewer`: layer dependencies, transactions, SQL shape,
  batching, UUIDs, and bounded metrics.
- `test_reviewer`: missing success, error, authorization, batch, frontend, and
  regression coverage.

Give every reviewer the same diff scope and ask for evidence with file and line
references. Wait for all requested reviewers, verify their findings against the
code, and remove duplicates or speculative claims.

Do not assign parallel write work in the shared checkout. Do not run concurrent
commands that mutate generated files, coverage profiles, or database state.

## Output

Lead with actionable findings ordered by severity. For each finding, state the
observable risk, the triggering path, and the smallest safe correction. Then
list open questions and verification gaps. If no material findings remain, say
so and identify any checks that were not run.
