# Data IO Module

This module owns CSV/XLSX import and export.

## Responsibilities

- Export user transactions as CSV or XLSX.
- Read CSV/XLSX imports.
- Parse import rows into typed transaction requests.
- Create imported transactions through the transactions service.
- Report per-row import success, failure, skip, and duplicate-risk counts.
- Queue file imports as persisted jobs and expose progress/history APIs.

## Maintenance Notes

- Do not update account balances directly in this module. Imported rows must go through `transactions.Service.Create` so manual and imported entries share the same ledger invariants.
- Keep supported import columns aligned with exported columns:
  `occurred_at,type,amount,category,account,note,tags,source`.
- Export `format` accepts `csv` or `xlsx` after trimming and case normalization; keep invalid
  values mapped to `invalid_request`.
- Export downloads must keep `Content-Disposition` with an ASCII-safe `filename` fallback
  and an RFC 5987 `filename*` parameter so old clients and modern browsers can recover
  stable file names without losing non-ASCII names.
- User-controlled CSV/XLSX text cells must pass through `safeCSVCell` before export to
  neutralize spreadsheet formula prefixes, including formulas hidden behind leading whitespace.
- Import totals count non-empty data rows only. Blank rows are ignored, while reported row
  numbers still point to the original source file lines.
- Imports skip duplicate rows by default; `skipDuplicates` defaults to true for both file and text imports. Duplicate checks compare user, time, type, amount, category, account, note, and tags.
- Large file imports should use `/io/import/jobs`; the synchronous `/io/import` endpoint remains for compatibility.
