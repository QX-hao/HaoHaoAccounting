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
- Imports skip duplicate rows by default. Duplicate checks compare user, time, type, amount, category, account, note, and tags.
- Large file imports should use `/io/import/jobs`; the synchronous `/io/import` endpoint remains for compatibility.
