# Conduit frontend

This directory contains the Vue 3 single-page application for the Conduit API.

```bash
npm install
npm run dev
```

The development server runs on `http://localhost:5173` and proxies `/api` and
`/internal` requests to the Go server on `http://localhost:8000`.

Useful commands:

```bash
npm run generate:api  # regenerate TypeScript types from ../api/open-api.yml
npm run typecheck     # check all TypeScript and Vue files
npm run build         # generate types and create the production bundle
```

Every Vue component keeps its template, TypeScript, and CSS in separate files.
Shared login state lives in `src/stores/auth.ts`; API calls live in
`src/api/conduit.ts`.
