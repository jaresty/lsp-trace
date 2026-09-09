# Provider final review repair

## Completed

- Removed workspace-local `git rev-parse` seed authority. Seed validation now accepts only an injected `HostReceiptAuthority`; absent, unauthenticated, incomplete, or mismatched receipts fail closed as `SEED_BINDING_UNAVAILABLE`.
- Bound authenticated custody to repository root, source revision, target relative path, and the SHA-256 of the bytes read from the opened descriptor.
- Added prepared-copy receipt fields. Prepared receipts require a modification-manifest digest, enumerate allowed changes separately, and reject any manifest that permits changing the seed target.
- Opened seed targets through `os.OpenRoot`, which rejects escaping and symlink components. Validation binds descriptor identity before/after the bounded read and re-stats the target through the rooted descriptor.
- Added a 1 MiB manifest ceiling and recursive duplicate-key rejection.
- Added an 8 MiB DocumentSymbol response ceiling, recursive duplicate-key rejection, strict array-level shape handling, and explicit required-member validation for hierarchical and flat LSP symbols, including nested ranges, positions, and location URI.
- Replaced the cross-generation admission boolean with an exact-generation latch and explicitly clears it during restart setup. A new generation must perform retained-byte `didOpen` and `textDocument/documentSymbol` again before target resolution.
- Added bounded, duplicate-safe bootstrap JSON decoding while retaining historical trailing-value error behavior.

## Focused evidence

- RED: receipt and semantic-cap guards initially failed to compile because `HostReceiptAuthority`, `CustodyClaim`, and `MaxDocumentSymbolBytes` were absent.
- GREEN: focused seed-binding/session/runtime/bootstrap/acquisition tests: `111 passed in 5 packages`.
- Existing focused tests continue to assert semantic admission precedes prepare and mismatch prevents prepare.

## Explicitly not claimed

- No live provider or D01 readiness run was performed.
- No product, network, install, deploy, full-suite, or full-race operation was performed.
- The existing acquisition-order test remains in-memory; this repair does not claim a new real fake-LSP subprocess harness.
- Frozen V1/V2/V3 CLI/MCP/schema/tool-25 byte goldens and wrong-decoder mutation coverage were not added in this bounded repair.
- Production bootstrap currently supplies no authenticated receipt channel, so requested seed binding fails closed until the host wires an independently authenticated `HostCustodyReceipt` into the manager.
