# Auth Module

This module owns login and current-user HTTP behavior.

## Responsibilities

- Validate login input.
- Issue auth tokens for the fixed development account.
- Create the default user row on first login.
- Ensure default accounts exist for the user after login.
- Return the current user from `/me`.

The current product uses a username/password flow backed by the users table. The first admin user is bootstrapped from `ADMIN_USERNAME`, `ADMIN_PASSWORD`, and `ADMIN_NAME`. When third-party login is added later, keep provider-specific logic in this module and leave other business modules unchanged.

Login failures are rate-limited per client IP and username. Tune with `LOGIN_RATE_LIMIT_MAX_FAILURES` and `LOGIN_RATE_LIMIT_WINDOW`; setting either to `0` disables the limiter.

Login failures intentionally return the same public error for missing users and wrong passwords. The missing-user path still runs a dummy bcrypt comparison to reduce account enumeration through obvious timing differences.
