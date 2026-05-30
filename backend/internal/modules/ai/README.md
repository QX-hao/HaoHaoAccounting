# AI Module

This module owns natural-language ledger parsing.

## Responsibilities

- Accept text input for `/ai/parse`.
- Use the parser service to produce a suggested transaction.
- Cache parse results per user and text when Redis is enabled.

AI parse results are suggestions only. The client still asks the user to confirm before creating a transaction.
