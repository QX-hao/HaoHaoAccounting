# Backend Modules

Vertical backend business modules.

## Modules

- `auth`: login and current user.
- `accounts`: account CRUD.
- `categories`: category CRUD.
- `transactions`: ledger entries and account balance invariants.
- `reports`: read-only aggregations.
- `dataio`: import/export.
- `ai`: natural-language parsing endpoint.

Modules should expose handlers and services. Shared helpers belong in `internal/shared`.
