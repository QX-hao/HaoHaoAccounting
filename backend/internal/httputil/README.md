# HTTP Utilities

Shared response helpers for Gin handlers.

`Error` writes the stable JSON error envelope with `error`, `code`, and optional `requestId`. It does not overwrite an already-started response, so recovery and late handler failures cannot corrupt successful bodies.

`InternalError` hides implementation details in release mode, maps `context.DeadlineExceeded` to the structured request-timeout response, and maps `context.Canceled` to the client-closed-request response.

`Unauthorized` and `InvalidToken` set `WWW-Authenticate` bearer challenges before returning the shared unauthorized error body.

`RateLimitedWithPolicy` emits `Retry-After`, `RateLimit-Limit`, `RateLimit-Remaining`, and `RateLimit-Reset` headers alongside the shared rate-limited error body. Retry delays are serialized as non-negative integer seconds.

`BindJSONBody` uses a JSON decoder with `DisallowUnknownFields`, rejects multiple JSON values, and runs Gin `binding` tag validation so request handlers accept only one closed request object that matches the documented request schema.

`SetPaginationHeaders` emits `X-Total-Count` and RFC 8288 `Link` headers for paginated list responses when additional pages exist.
