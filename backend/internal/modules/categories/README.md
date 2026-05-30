# Categories Module

This module owns category management.

## Responsibilities

- List system and user categories.
- Create custom user categories.
- Update custom user categories.
- Prevent modifying or deleting system categories.
- Prevent deleting categories that are referenced by transactions.
- Invalidate report caches after category changes.

System category seeding currently lives in `store` because it runs during database startup. If seeding grows more complex, move the seed data into this module and call it from store initialization.
