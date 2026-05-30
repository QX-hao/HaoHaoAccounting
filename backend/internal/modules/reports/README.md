# Reports Module

This module owns report aggregation.

## Responsibilities

- Build summary totals for a date range.
- Aggregate spending by category and account.
- Build monthly trend data.
- Compare the current date range with the previous equivalent range.
- Cache report payloads per user and date range when Redis is enabled.

Reports are read-only. Mutating modules invalidate report cache through the shared transaction cache invalidator.
