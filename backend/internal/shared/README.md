# Backend Shared

Cross-module helpers that do not belong to one business module.

## Packages

- `queryutil`: query string parsing.
- `stringutil`: small string normalization helpers.
- `timeutil`: accepted date/time parsing and range resolution.

Keep this area intentionally small. If a helper is only used by one module, keep it inside that module.
