# Web App

Next.js frontend for HaoHaoAccounting.

## Structure

- `app`: route entrypoints only.
- `features`: vertical product features.
- `shared`: API client, auth helpers, types, and utilities.
- `components`: app shell and page frame components.
- `styles`: global CSS split by responsibility.

New UI work should normally start in `features/<feature>`.
