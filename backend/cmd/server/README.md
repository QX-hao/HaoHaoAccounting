# Server Entrypoint

`main.go` initializes database, Redis, Gin middleware, CORS, and route registration.

## Runtime Contracts

Startup loads `.env` first, then runs `LoadStrict` and `validateStartupConfig` before opening database, Redis, or HTTP listeners. `CORS_ALLOW_ORIGINS` must contain explicit `http` or `https` origins; wildcards, paths, queries, fragments, custom schemes, and empty origin lists are rejected at startup. The CORS layer uses `gin-contrib/cors` with credentials disabled, explicit methods and headers, and browser-exposed response headers for request IDs, pagination, downloads, auth challenges, rate limits, and method negotiation.

`TRUSTED_PROXIES` is applied to Gin before global middleware. Global middleware is ordered as `RequestID` -> `RequestTimeout` -> logger -> `Recovery` -> `SecurityHeaders` -> CORS -> `NoStoreAPI` -> `BodyLimit` -> `ContentType` -> `Accept`, so early rejections still keep `X-Request-ID`, browser security headers, no-store API cache headers, and allowed-origin CORS headers. Server read, header-read, write, idle, body-size, and graceful-shutdown budgets come from `HTTP_*` configuration values.

The access log records `time`, `status`, `latency`, `client_ip`, `method`, sanitized `path`, `proto`, `user_agent`, `request_id`, response `bytes`, and `error`. Query strings are omitted from `path` so tokens and filters do not enter logs by default.

Keep business logic out of this package. Add business routes through `internal/app` and module handlers.
