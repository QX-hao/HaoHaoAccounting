# Mobile API Client

React Native API helper layer.

## Responsibilities

- Persist bearer tokens in AsyncStorage.
- Attach bearer tokens to authenticated requests.
- Route JSON requests, uploads, downloads, and logout through the shared `fetchAPI` helper.
- Expose `requestWithResponse` and `uploadWithResponse` for screens that need successful response headers such as `Location` and `Link`.
- Clear the local token before the best-effort logout revocation request so failed network calls cannot leave the app using a stale token.
- Broadcast session invalidation after `401` responses and logout so `useSession` can immediately return the app to the logged-out state.
- Send `Accept: application/json`, attach a bounded `X-Request-ID`, and add `Content-Type: application/json` only when the body is not `FormData`.
- Use `credentials: omit` so API calls rely only on explicit bearer headers instead of ambient cookies.
- Bound requests with a 30 second `AbortController` timeout while still honoring a caller-provided `RequestInit.signal`; local timeouts surface with `ApiError.code` set to `request_timeout`.
- Parse backend JSON and structured `application/*+json` error payloads into `ApiError` values, with plain text fallback for non-JSON bodies.
- Preserve backend `status`, `code`, `requestId`, `Retry-After`, `RateLimit-*`, and `WWW-Authenticate` headers for screen-level decisions.
- Decode download filenames from `Content-Disposition` `filename*` before falling back to `filename`.

Feature folders should expose business-named API functions on top of this client.
