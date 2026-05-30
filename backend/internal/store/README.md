# Backend Store

Database initialization and shared lookup helpers.

## Responsibilities

- Open the configured database driver.
- Run GORM migrations.
- Seed system categories.
- Create default user accounts on first login.
- Provide import helpers for finding or creating categories and accounts.

Feature-specific queries should live in module services unless they are genuinely shared.
