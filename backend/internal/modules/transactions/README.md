# Transactions Module

This module owns ledger transaction behavior.

## Responsibilities

- List, create, update, and delete transactions.
- Validate transaction payloads.
- Confirm selected categories and accounts are accessible to the current user.
- Keep account balances in sync with transaction changes.
- Invalidate user report caches after ledger mutations.

## Maintenance Notes

- Creating a transaction must create the row and apply the account balance delta in the same database transaction.
- Updating a transaction must first revert the old balance delta, then apply the new balance delta, then save the updated row in the same database transaction.
- Deleting a transaction must revert the balance delta and delete the row in the same database transaction.
- Import flows should call `Service.Create` instead of duplicating balance adjustment logic.
