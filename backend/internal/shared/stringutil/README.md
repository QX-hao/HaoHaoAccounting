# String Utilities

Shared string helpers that are intentionally small and behavior-preserving.

Do not turn this package into a dumping ground. If a helper is only used by one module, keep it inside that module.

`TruncateRunes` trims log-safe strings by Unicode character count, so middleware logs can bound user-controlled values without corrupting multibyte characters.
