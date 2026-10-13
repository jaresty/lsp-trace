# Causal RED plan

Status: plan only; no tests were written or run.

## Entry and current block

Reverify all pins, package hashes, predecessor-package hashes, managed session identity, default-off reachability, and source manifest. Drift is `BLOCKED_DRIFT`.

The current real path is `Manager.RoundTrip` → `roundTripWithPrivate` → `roundTripWithPrivateTransaction` → the managed transport read loop in `sessionruntime/sessionruntime.go`, then `internal/lspwire` scanning. Current private B4 admission and `lspwire.DefaultLimits().MaxBodyBytes` each impose 16 MiB constraints. `provider.MaxProviderLimits.ResponseBytes` is an independent provisioning ceiling, not claimed as a proven universal Manager call edge.

Therefore a complete body `16 MiB + 1` cannot reach the missing 32 MiB directional behavior on this revision. The current outcome is `BLOCKED_NOT_RED`. Direct scanner/owner injection, changed globals, fabricated raw results, or bytes inserted after guards are non-causal.

## Separately authorized causal sequence

1. Declaration RED for the frozen private reservation/scanner/selector/assembler symbols.
2. Real-path RED only after private transaction-scoped reachability exists: reserve before any enlarged effect, then exercise the production Manager write/read/scanner path.
3. Never expand public/global defaults. Nil/disabled remains exact current behavior.
4. Separately execute incoming and outgoing vectors; neither can satisfy the other.

## Required matrix

- exactly `33554432`: accepted only if all independent guards pass; one slot; exact component map;
- `33554433` and arithmetic overflow: pre-effect rejection with ledger unchanged;
- second simultaneous attempt: pre-effect rejection because v1 has one slot, regardless of direction;
- release then reuse: clear first, exact-once decrement, fresh identity, stale lease closed;
- duplicate release: false/no-op;
- malformed, timeout/cancellation, short EOF, and partial reads: scanner retains only bounded actually consumed bytes as `INCOMPLETE_OPAQUE_PREFIX`, `complete=false`; no reconstruction or semantics;
- zero bytes read versus present prefix remain distinct;
- prepare array with multiple members: shared bounded scanner preserves every raw span/ordinal; deterministic selector chooses nonzero ordinal;
- incoming and outgoing assemblers independently embed byte-identical selected raw item in fixed `{"item":...}` wrapper;
- one-byte mutation refuses before directional write;
- `null`, `[]`, nonempty, complete malformed, protocol error, and `-32601` remain distinct;
- disabled and `CALLS_ONLY` controls are byte/value identical and allocate no new custody;
- disabled and `CALLS_ONLY` history/lineage snapshots require exact equality for readiness-history entries/bytes/digests/directions/order/identity, acquired/outstanding bytes, receipt-owner/borrower counts, retirement/lifecycle state, predecessor commitment bytes/order, and generic attempt-lineage fields; prefix-mapping ID count is zero;
- enabled prefix mapping IDs are directionally namespaced and never equal or feed history entry IDs, history identity, predecessor IDs/digests, lineage identity, ordering, classification, or lifecycle decisions.

## Ledger/census assertions

Every retained backing has one C15 classification and at most one byte-bearing directional component per attempt. Aliases charge zero; transfers do not double-charge; directional reservation rows are not C15 backing rows. C15 delta, directional delta, and relationship mapping are independently complete and reconciled.

## Stop conditions

Return `REVISE` on public/global change, duplicate owner/terminal, reconstruction, default-on behavior, inability to reserve before effect, more than one v1 slot, unexplained backing, merged C15/directional accounting, non-causal injection, unexplained census delta, any history dependency mutation, or any history/predecessor/lineage byte-value or semantic delta. Passing would prove private mechanics only, never CALLS truth, completeness, profile GO, or production authority.
