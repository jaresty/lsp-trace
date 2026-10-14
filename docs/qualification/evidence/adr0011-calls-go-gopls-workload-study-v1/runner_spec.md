# Runner specification

## Scope

Evidence-only, private, authority 0, completeness UNKNOWN. No Manager custody, production admission, profileGO, public source edits, public CLI/MCP schema edits, delegation, commit, or full 5900 execution.

## Position mapping

For each target, the runner first verifies `source_sha256` over the private source workspace file. Only then does it locate the target symbol inside `[byte_start, byte_end)` and map that symbol byte offset to an LSP zero-based UTF-16 position:

1. Verify source bytes and target byte range.
2. Find the first UTF-8 target symbol occurrence within `[byte_start, byte_end)`.
3. Decode `source_bytes[:symbol_byte_offset]` as UTF-8.
4. `line = count("\n")`.
5. `character = UTF-16 code units since the last newline`.

For the first target, range byte offset `1117` over `acquisitionops/executor.go` maps to `{line:35, character:0}`, and the identifier `DecodeManifest` begins at byte `1122`, mapping to `{line:35, character:5}`.

## Prepared selection

A prepared call hierarchy item is selected only when all hold:

- item URI equals target URI;
- item name equals target symbol when a symbol is supplied;
- target LSP position is contained by `selectionRange` or `range`.

No match is `REFUSED_SELECTED_ITEM_TARGET_MISMATCH`; multiple matches are `REFUSED_PREPARE_AMBIGUOUS`.

## Full workload and dry plan

Full acquisition mode is explicit: `--run-full --direction incoming|outgoing`. Dry validation mode is `--dry-full-plan --direction incoming|outgoing` and performs no LSP acquisition. Both require the exact frozen 5900-target TSV. The denominator is split deterministically into 118 batches of exactly 50 targets. Each batch gets a manifest with input digest, target-id digest, and output digest; dry plans use the SHA-256 digest of empty bytes as the no-output digest. Each batch also gets a checkpoint record.

Full mode is separate from smoke mode. It does not use `smoke_max_targets_hard`; smoke continues to enforce that hard limit.

## Ledger verifier

For each direction and ordinal, accepted event language is:

- `STARTED(n), ABANDONED(n), STARTED(n+1), FINAL(n+1)`, or
- `STARTED(n), FINAL(n)`.

After a final ordinal disposition, any appended `STARTED`, `ABANDONED`, or terminal event is invalid. Attempt numbers must be strict and terminal attempts must match the open attempt. `ABANDONED` is terminal for an attempt but not a final ordinal disposition when a retry follows. Resume first verifies this ledger/checkpoint state; any open `STARTED` is appended as `ABANDONED` before the retry attempt is started.

## Semantic invariant verifier

The runner checks these constants in code, independently of schema/checksum success: `max_raw_response_bytes=33554432`, `request_timeout_seconds=30`, `initialize_timeout_seconds=60`, `smoke_max_targets_hard=5`, `batch_size=50`, `target_count=5900`, and `batch_count=118`. Drift examples such as target count `1`, batch size `999`, or smoke hard limit `5900` are rejected.

## LSP IO

The runner speaks real gopls JSON-RPC/LSP over stdio. It uses select-based bounded reads for headers and bodies. `initialize_timeout_seconds`, `request_timeout_seconds`, and `max_raw_response_bytes` are enforced. Timeout, closed stream, malformed, and over-limit paths write partial raw bytes/sidecars before failing closed.

## Document lifecycle

The runner maintains a URI cache keyed by text digest/version. It suppresses duplicate didOpen for the same URI/version and emits deterministic didClose for all open documents before shutdown/exit.
