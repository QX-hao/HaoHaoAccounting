# Backend Middleware

Gin middleware and auth token helpers.

`RequestID` preserves a valid caller-provided `X-Request-ID` or generates one, returns it in the response header, and stores the same value in both Gin context and the standard request `context.Context` for downstream services.

`RequestTimeout` adds an optional deadline to the standard request `context.Context` so downstream database, cache, and external-service calls can stop work when a request exceeds the configured budget. A zero or negative timeout is treated as disabled.

`Recovery` returns the structured internal-error response while logging the recovered panic with method, path, client IP, and request ID so production incidents can be correlated without exposing panic details to clients.

`RequireAuth` validates the bearer token and puts the user ID into request context for module handlers. Token revocation checks fail closed: if the revocation backend cannot be queried, the request is rejected instead of being passed to business handlers.
