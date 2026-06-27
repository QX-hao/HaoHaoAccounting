# Web API Client

Thin browser-side HTTP helpers for the Next.js frontend.

## Runtime Contracts

- `request`, `upload`, `download`, and `logout` route network calls through the shared `fetchAPI` helper.
- `requestWithResponse` and `uploadWithResponse` keep the same data parsing path while exposing successful response headers such as `Location` and `Link`.
- JSON requests send `Accept: application/json`, attach a bounded `X-Request-ID`, and add `Content-Type: application/json` only when the body is not `FormData`.
- Authenticated calls attach the bearer token from shared auth storage; `401` responses clear the token and redirect browser users back to `/login`.
- `logout` clears the local token before the best-effort revocation request, so failed network calls cannot leave the browser using a stale token.
- `fetchAPI` uses `credentials: omit` so cross-origin API calls rely only on explicit bearer headers instead of ambient cookies.
- Requests are bounded by a 30 second `AbortController` timeout while still honoring a caller-provided `RequestInit.signal`; local timeouts surface with `ApiError.code` set to `request_timeout`.
- Error responses are parsed from `application/json` and structured `application/*+json` media types, with plain text fallback for non-JSON bodies.
- `ApiError` preserves backend `status`, `code`, `requestId`, `Retry-After`, `RateLimit-*`, and `WWW-Authenticate` headers for UI decisions.
- Downloads use the shared error parser and decode `Content-Disposition` `filename*` before falling back to `filename`.

Feature API files should wrap these helpers with business names such as `listTransactions` or `createAccount`.
