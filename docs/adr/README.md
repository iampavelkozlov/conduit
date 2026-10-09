# Architecture decision records

Use an ADR for a durable technical decision whose rationale would otherwise be
rediscovered: layer boundaries, persistence strategy, identifier layout,
security model, generator choice, or a significant dependency.

Do not create ADRs for routine implementation details or decisions already
fully enforced and explained by an authoritative configuration file.

Name records `NNNN-short-title.md` and use this structure:

```markdown
# NNNN: Decision title

Status: proposed | accepted | superseded

## Context

What constraint or problem requires a decision?

## Decision

What will the project do?

## Consequences

What becomes easier, harder, or intentionally constrained?
```

Link a superseding ADR from the old record instead of rewriting accepted
history.
