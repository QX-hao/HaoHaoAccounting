# Backend Middleware

Gin middleware and auth token helpers.

`RequestID` preserves a valid caller-provided `X-Request-ID` or generates one, returns it in the response header, and stores the same value in both Gin context and the standard request `context.Context` for downstream services. Caller IDs are trimmed and accepted only when they are bounded to 128 visible ASCII characters; invalid values are replaced everywhere to avoid unsafe log correlation values.

`RequestTimeout` adds a deadline to the standard request `context.Context` so downstream database, cache, and external-service calls can stop work when a request exceeds the configured budget. If the handler returns without writing after the middleware deadline, it writes the structured `504` timeout response; if the parent request context is canceled first, it writes the documented `499 client_closed_request` response. A zero or negative timeout is treated as disabled.

`Recovery` returns the structured internal-error response while logging the recovered panic with method, path, client IP, and request ID so production incidents can be correlated without exposing panic details to clients. Broken pipe, connection reset, aborted handler, and already-written response cases are not overwritten with another error body.

`SecurityHeaders` sets conservative browser security headers on API responses, including `Content-Security-Policy`, `Cross-Origin-Opener-Policy`, `Cross-Origin-Resource-Policy`, `Origin-Agent-Cluster`, `Referrer-Policy`, `Permissions-Policy`, `X-Content-Type-Options`, `X-DNS-Prefetch-Control`, `X-Download-Options`, `X-Frame-Options`, `X-Permitted-Cross-Domain-Policies`, and `X-XSS-Protection`. `Cross-Origin-Embedder-Policy` is opt-in through HTTP config because it can break deployments whose frontend or third-party resources are not fully marked for cross-origin isolation; the middleware only writes recognized COEP values. `Strict-Transport-Security` is also opt-in through HTTP config so local HTTP development and partial HTTPS rollouts do not accidentally persist HSTS in browsers.

`BodyLimit` rejects oversized requests before handlers run when `Content-Length` is known, and wraps streaming request bodies with `http.MaxBytesReader` so JSON and multipart handlers can map over-limit reads to the same structured payload-too-large response.

`ContentType` enforces declared request media types on routes with bodies, including structured `application/*+json` variants for JSON endpoints and multipart form data for import endpoints. Configured media types must be bare concrete types without parameters or wildcards; they are normalized, deduplicated, and invalid values are ignored before request checks run.

`Accept` enforces declared response media types for API routes, supports compatible media ranges and structured syntax suffixes, treats `q=0` media ranges as explicit exclusions, and appends `Vary: Accept` on negotiated routes. Offered media types use the same normalization as request media types so duplicate or invalid rule entries do not leak into negotiation errors.

`HTTPMetrics` records low-cardinality Prometheus counters and duration histograms after route handling. Labels use only HTTP method, Gin route pattern, and response status; unmatched routes are grouped as `unmatched` so raw URLs and query strings do not become metric labels.

`NoStoreAPI` applies `Cache-Control: no-store`, `Pragma: no-cache`, and `Expires: 0` to requests under the API prefix before early global rejections can write an error. `NoStore` applies the same headers inside API route groups. `SetNoCache` is available for non-sensitive operational endpoints such as health probes that may be stored but must be revalidated before reuse.

`RequireAuth` validates the bearer JWT signature, expiration, issued-at time, issuer, and audience before putting the user ID into request context for module handlers. `Authorization: Bearer` values must use the RFC 6750 token68 character set, valid trailing padding, and at most 4096 bytes. Token revocation checks fail closed: if the revocation backend cannot be queried, the request is rejected instead of being passed to business handlers.
