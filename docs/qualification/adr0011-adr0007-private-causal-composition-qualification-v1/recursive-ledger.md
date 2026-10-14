# Recursive ledger

## Correction against rereview blockers

1. Selected ordinals
   - Inspected committed incoming/outgoing provider/generic profile manifests/results and private bidirectional artifact.
   - Found no accepted selected ordinal/item set; only `one_final_disposition_ordinals=5900` plus direction result/profile identity is exposed.
   - Removed caller-chosen selected-ordinal authority by requiring empty envelope `SelectedOrdinals`, nonzero mark selected ordinals to fail, and mark binding to hard-pinned direction ordinal identity hash/count.
   - Added adversaries for self-consistent selected ordinal mutation, self-consistent ordinal identity mutation, and mark-only ordinal identity mutation.
2. MaxReferences
   - Kept the field and enforced it through overflow-safe `addReferences` before envelope/mark reference admission.
   - Added exact boundary coverage: two envelopes + two marks with `MaxReferences=4` passes; `MaxReferences=3` fails pre-effect with deterministic accounting.
   - Cancellation remains pre-ADR0007 and takes precedence over resource accounting.
3. Exact frozen write-set
   - Removed/renamed `composition.go` and `composition_test.go`.
   - Final implementation files are `compose.go` and `compose_test.go`.
   - ADR0007 source-text-search diff remains empty.

## Remaining reason for AUTHORED_NOT_GO

The implementation is intentionally private and unintegrated. It awaits independent review and explicit authorization before any public, CLI/MCP/schema, Stage8/main, or production composition work.
