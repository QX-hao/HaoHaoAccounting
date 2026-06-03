# Backend Commands

Executable entrypoints live here.

`server` is the HTTP API process used by local development and Docker.

`dbmigrate` applies SQL migrations from `backend/migrations/<driver>` using `DB_DRIVER` and `DB_DSN`.
