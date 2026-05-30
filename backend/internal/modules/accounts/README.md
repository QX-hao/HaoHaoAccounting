# Accounts Module

This module owns user account management.

## Responsibilities

- List user accounts.
- Create accounts with a default `custom` type when no type is supplied.
- Update account name, type, and opening balance.
- Prevent deleting accounts that are referenced by transactions.
- Invalidate report caches after account changes.

Balance changes caused by ledger transactions are owned by the `transactions` module, not by account CRUD handlers.
