# HTTP Utilities

Shared response helpers for Gin handlers.

`Error` writes the stable JSON error envelope with `error`, `code`, `status`, and `requestId`. It refuses non-error HTTP statuses by falling back to `500 internal_error`, and it does not overwrite an already-started response, so recovery and late handler failures cannot corrupt successful bodies.

`Error` also records a Gin private error summary with only the HTTP status and stable error code. Access logs can aggregate failures without leaking request payloads, validation messages, or internal exception details.

`InternalError` hides implementation details in release mode, maps `context.DeadlineExceeded` to the structured request-timeout response, and maps `context.Canceled` to the client-closed-request response. The latter uses the documented non-standard `499` status with `client_closed_request` so handlers and OpenAPI describe canceled client work consistently.

`Unauthorized` and `InvalidToken` set `WWW-Authenticate` bearer challenges before returning the shared unauthorized error body.

`RateLimitedWithPolicy` emits `Retry-After`, `RateLimit-Limit`, `RateLimit-Remaining`, and `RateLimit-Reset` headers alongside the shared rate-limited error body. Retry delays are serialized as non-negative integer seconds.

`BindJSONBody` uses a JSON decoder with `DisallowUnknownFields`, rejects multiple JSON values, and runs Gin `binding` tag validation so request handlers accept only one closed request object that matches the documented request schema. Handlers must pass bind errors through `middleware.HandleBodyReadError` before returning `invalid_request`, so streamed oversized request bodies keep the documented `413 payload_too_large` response.

`BindQuery` is the shared entrypoint for Gin query binding. Handlers use it before returning the documented `invalid_request` response for malformed or validation-failing query parameters, so list/export filters follow the same error contract as OpenAPI.

`SetPaginationHeaders` emits `X-Total-Count` and RFC 8288 `Link` headers for paginated list responses when additional pages exist.

`SetCreatedLocation` emits a relative `Location` header for newly created or queued resources so browser and mobile clients can follow the resource URL without learning deployment hosts.
