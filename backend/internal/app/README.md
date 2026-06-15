# App Module

`app` is the backend composition layer. It wires shared dependencies, registers HTTP routes, and keeps route layout separate from business modules.

## Responsibilities

- Expose `/livez`, `/readyz`, and compatible `/health` probes.
- Register public auth routes under `/api/v1`.
- Register authenticated business routes under `/api/v1` with `middleware.RequireAuth`.
- Create module handlers from shared `store.Store` and optional Redis cache.

Business rules, database mutations, and file parsing should stay in `internal/modules/*` or `internal/shared/*`, not in this package.
