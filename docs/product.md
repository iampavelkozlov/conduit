# Product direction

## Purpose

Conduit is a production-oriented implementation of the RealWorld application.
It demonstrates how a small publishing product can be built with explicit API
contracts, layered Go services, PostgreSQL, generated persistence code, and a
modern Vue client without hiding important behavior behind a large framework.

## Users and core journeys

The product serves readers and authors who need to:

- register, sign in, refresh sessions, and manage a profile;
- browse global, personalized, author, tag, and favorited article feeds;
- read, create, edit, and delete articles;
- write and delete comments;
- follow authors and favorite articles;
- write readable rich-text content and use the interface across supported
  themes and common viewport sizes.

## Product boundaries

The upstream RealWorld OpenAPI document is the source of truth for externally
observable API behavior. Backend changes may improve correctness, security,
performance, operations, and maintainability, but must not create a divergent
local API.

The frontend may improve presentation and authoring while remaining compatible
with the canonical API. Current product extensions include theming, responsive
navigation, sanitized rich-text authoring, and syntax-highlighted code blocks.
These are client experiences, not API extensions.

## Quality principles

- Contract fidelity over local convenience.
- Predictable validation and authorization errors.
- Accessible keyboard and screen-reader behavior.
- Safe rendering of user-authored content.
- Bounded database and metrics cardinality.
- No N+1 data access on list views.
- Deterministic generation, tests, and releases.
- Operationally useful logs and metrics without exposing credentials or user
  content.

## Non-goals

- Replacing or locally extending the canonical RealWorld API.
- Building social-network features not supported by that contract.
- Introducing infrastructure whose maintenance cost exceeds its demonstrated
  value for this application.
- Treating generated code as a manual customization surface.

## Sources of truth

When documents disagree, use this order:

1. Upstream RealWorld OpenAPI and Hurl suite for API behavior.
2. Database migrations for persisted schema.
3. Enforced architecture, lint, test, coverage, and CI configuration.
4. Accepted architecture decision records.
5. This product direction and the repository README.
