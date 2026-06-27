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

## Database Migrations

SQL migrations live in `migrations/<driver>`, currently split by `postgres` and `mysql`.

Run migrations explicitly with:

```bash
go run ./cmd/dbmigrate
```

The HTTP server still keeps GORM `AutoMigrate` for local compatibility, but production deployment should run the migration command before starting the API.

## API Contract

The business API contract lives in `api/openapi.yaml`. Regenerate web/mobile TypeScript types from the repository root:

```bash
npm run generate:api-types
npm run verify:api-contract
```

The root health probe contract lives separately in `api/health-openapi.yaml` because `/livez`, `/readyz`, and `/health` are operational endpoints, not generated business-client endpoints.
