# Production splice interface

- `Manager.directionalRawV1` is private and owns one real `privateB4ByteAccountV2`, an attempt map, and a monotonic private sequence.
- `sessionruntime.New` constructs the account and ledger; constructor failure aborts Manager construction.
- `newDirectionalTerminalV1Locked` runs under `Manager.mu`, removes prior terminal attempts, creates a fresh attempt backed by the Manager account, and returns its terminal binding.
- Both production generic-owner creation paths receive that binding.
- Stage A exact20 V1 functions are mechanically derived from the committed immutable V3 bridge and preserve `PLANNED_*` ceilings.
- No exported symbol, dependency, schema, profile, registry, configuration, gopls behavior, D/R behavior, or `CALLS_ONLY` behavior changes.

Opaque account/lease internals are used only through existing constructors and methods; no `ID`, `Capacity`, or `Active` fields are assumed.

Evidence authority: `PRODUCTION_SPLICE_COMPILE_ONLY`.

Disposition: `AUTHORED_NOT_GO`.
