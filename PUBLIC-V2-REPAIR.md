# Public v2 repair report

## Scope

Repairs the independent-review findings for the candidate formerly ending at `3f24bb6`, rebased cleanly onto `a37ffb9`. No deployment, network access, installation, product work, live-D01, full-suite, or full-race execution was performed.

## Repairs

1. Corrected the PRD heading from erroneous `FR222` to the existing `FR22`; no requirement was added or renumbered.
2. Restored the certified analytics wire shape by removing `ClaimLevel` and `Provenance` from `LocalResult`, removing contextualization from CLI/MCP production, and removing nonexistent `--claim-level` usage. Rooted exact-byte/schema/digest/length verification remains an input-custody check and does not mutate result payloads.
3. Added v2 schema-version aliases and embedded base-schema loading so the frozen analysis/metrics/ranking schemas validate produced `local-v1` payloads without changing schema bytes.
4. Added explicit CLI verification dispatch for all three public v2 families. The focused process guard now exercises schema-get, produce, structural/semantic validation, immutable publication, selected-generation byte equality, and receipt verification for COMPLETE and LIMIT.
5. Updated only tests observing the current `NewRegistry` or built current server from 25 to 28. Historical composition constructors and 13/20/21/22/23/25 boundaries remain unchanged.
6. Added a real-built-process error matrix across analysis, metrics, and ranking covering malformed JSON, duplicate keys, unknown fields, oversized input, wrong operation/family semantics, conflicting/missing/unsafe selectors, publication collision, CLI nonzero diagnostics, MCP native/domain status/code semantics, no-overwrite behavior, and success-byte parity.
7. Renamed `PUBLIC-PARITY--REPAIRfinder.md.md` to `PUBLIC-V2-IMPLEMENTATION-NOTES.md` with history/content preserved and corrected its provenance wording.

## Verification

Passed:

- `go test ./internal/normativeanalytics ./internal/operation ./internal/publication ./internal/mcp ./internal/mcpcontract ./internal/schema -count=1` — 353 tests.
- Focused race across the same packages for analytics/schema/registry/FR22/FR23/transport assertions — 35 tests.
- Focused real-built-process race for exact parity and error matrix — 5 tests.
- Focused current-registry and process tests in `cmd/lsp-trace-mcp` — 56 tests.
- Focused `go vet` for affected packages and both commands.
- `go build ./cmd/lsp-trace ./cmd/lsp-trace-mcp`.
- `git diff --check`.

## Evidence boundary

Outputs are documented as unverified local deterministic evidence. Authenticated provenance is a separate optional receipt/verifier concern; this repair does not claim deployment, shipment, production authority, or Program B admission.
