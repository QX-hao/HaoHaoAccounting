# Mobile API Client

React Native API helper layer.

## Responsibilities

- Persist bearer tokens in AsyncStorage.
- Attach bearer tokens to authenticated requests.
- Route JSON requests, uploads, downloads, and logout through the shared `fetchAPI` helper.
- Send `Accept: application/json`, attach a bounded `X-Request-ID`, and add `Content-Type: application/json` only when the body is not `FormData`.
- Bound requests with a 30 second `AbortController` timeout while still honoring a caller-provided `RequestInit.signal`.
- Parse backend JSON and structured `application/*+json` error payloads into `ApiError` values, with plain text fallback for non-JSON bodies.
- Preserve backend `code`, `requestId`, `Retry-After`, `RateLimit-*`, and `WWW-Authenticate` headers for screen-level decisions.
- Decode download filenames from `Content-Disposition` `filename*` before falling back to `filename`.

Feature folders should expose business-named API functions on top of this client.
