# Server Entrypoint

`main.go` initializes database, Redis, Gin middleware, CORS, and route registration.

Keep business logic out of this package. Add business routes through `internal/app` and module handlers.
