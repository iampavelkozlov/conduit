# Frontend instructions

These instructions refine the repository-level `AGENTS.md` for work launched
inside `frontend/`.

## Structure and dependencies

- Use Vue 3 and TypeScript with the existing Vite, Pinia, Vue Router,
  Tailwind CSS, and shadcn-vue setup.
- Reuse components under `src/components/ui/` before introducing another UI
  primitive or dependency.
- Keep API calls in `src/api/`, shared authentication state in
  `src/stores/auth.ts`, and route definitions in `src/router/`.
- Preserve the established split-file component convention where it already
  exists. A focused single-file component is acceptable when splitting it would
  not improve reuse or readability.
- Use the `@/` alias for cross-feature imports; relative imports are acceptable
  within a component's own folder.
- Do not edit `src/api/generated/schema.d.ts` manually. Regenerate it with
  `npm run generate:api` or `make frontend-generate` from the repository root.

## Behavior and quality

- Keep TypeScript strict and do not bypass errors with `any`, unchecked casts,
  or disabled compiler rules.
- Preserve keyboard navigation, semantic labels, focus behavior, and visible
  error states for interactive UI.
- Sanitize user-authored rich text before rendering or loading it into the
  editor.
- Add or update Vitest coverage for changed behavior, including error and edge
  paths where meaningful.
- Run `make verify-frontend` from the repository root before completing a
  frontend change.
