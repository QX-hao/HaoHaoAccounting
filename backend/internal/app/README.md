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

`/readyz` and `/health` check the database and optional Redis cache with a 2 second dependency budget. Database or Redis failures return `503` with `status: unavailable` and do not expose raw dependency error details; Redis is reported as `disabled` when no enabled cache is configured.

Health probe responses use `Cache-Control: no-cache`, `Pragma: no-cache`, and `Expires: 0` so load balancers and proxies must revalidate process and dependency state before reusing a stored result.

Health probe `GET` responses use `Content-Type: application/json; charset=utf-8`. Health probes support both `GET` and `HEAD`; `HEAD` keeps the same status codes and cache headers without returning a JSON body.

All `/api/v1` routes use `NoStore` cache headers. API paths are matched exactly as documented in OpenAPI; trailing slash variants are not redirected, extra slash variants are not normalized, and both flow through the API fallback contract. API fallback errors for missing routes and unsupported methods return the shared structured error body with request IDs; API fallback errors also keep `Cache-Control: no-store`, `Pragma: no-cache`, and `Expires: 0`. The multipart parsing memory budget is aligned with the import file size limit. Non-API health probe fallbacks remain cache-neutral.

Business rules, database mutations, and file parsing should stay in `internal/modules/*` or `internal/shared/*`, not in this package.
