# Backend

Go/Gin backend for HaoHaoAccounting.

## Structure

- `cmd/server`: process entrypoint and infrastructure setup.
- `internal/app`: route composition.
- `internal/modules`: vertical business modules.
- `internal/shared`: small cross-module helpers.
- `internal/store`: database initialization and shared store helpers.
- `internal/models`: GORM models.

Business behavior should be added to the relevant module under `internal/modules/*`.
