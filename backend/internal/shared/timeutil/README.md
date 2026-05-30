# Time Utilities

Shared date parsing helpers live here so reports, transactions, and import/export code use the same accepted formats.

## Accepted Input Formats

- RFC3339 timestamps, for API and browser-generated values.
- `YYYY-MM-DD`
- `YYYY-MM-DD HH:mm:ss`
- `YYYY/MM/DD`

Keep new date formats centralized in `ParseDateTime` to avoid different modules interpreting the same input differently.
