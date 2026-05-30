# Auth Module

This module owns login and current-user HTTP behavior.

## Responsibilities

- Validate login input.
- Issue auth tokens for the fixed development account.
- Create the default user row on first login.
- Ensure default accounts exist for the user after login.
- Return the current user from `/me`.

The current product uses a fixed username/password flow. When third-party login is added later, keep provider-specific logic in this module and leave other business modules unchanged.
