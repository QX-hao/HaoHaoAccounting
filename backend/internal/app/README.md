# App Module

`app` is the backend composition layer. It wires shared dependencies, registers HTTP routes, and keeps route layout separate from business modules.

## Responsibilities

- Expose `/livez`, `/readyz`, and compatible `/health` probes.
- Leave `/metrics` owned by the server entrypoint so runtime instrumentation stays outside business route contracts.
- Register public auth routes under `/api/v1`.
- Register authenticated business routes under `/api/v1` with `middleware.RequireAuth`.
- Create module handlers from shared `store.Store` and optional Redis cache.

## Route Contracts

`/livez` only reports process liveness and returns `{"status":"ok"}` without dependency checks.

`/readyz` and `/health` check the database and optional Redis cache with a 2 second dependency budget. Database failures return `503` with `status: unavailable`; Redis is reported as `disabled` when no enabled cache is configured.

All `/api/v1` routes use `NoStore` cache headers. API fallback errors for missing routes and unsupported methods return the shared structured error body with request IDs; API fallback errors also keep `Cache-Control: no-store`, `Pragma: no-cache`, and `Expires: 0`. Non-API health probe fallbacks remain cache-neutral.

Business rules, database mutations, and file parsing should stay in `internal/modules/*` or `internal/shared/*`, not in this package.
