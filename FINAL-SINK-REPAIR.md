# Final Sink Repair

Base: `0b4226cdaf196d851616e1ba0398764414ee90fa` (already contains `7c1788898a5788629dbe6d5d11c9a8708b53cdba`; no rebase required).

## Implemented

- Replaced the uncompilable cross-package `safeStartupSelector` reference with exported `internal/manageddiagnostic.PublishHardened`.
- Routed startup and request diagnostic publication through one internal hardened primitive.
- The shared primitive validates safe relative selectors, rejects root symlinks and root identity races, requires private writable root and nested directories, writes random `0600` temporary files, syncs, rereads and revalidates exact bytes, rejects replacement through link publication, cleans up, and syncs the directory.
- Updated request-sink tests to establish the required private-root precondition and fixed the no-opt-in process invocation to omit both flag pairs.
- Bounded private request record retention at callback source to 64 records, 65,536 encoded bytes, and 4,096 bytes per retained string, with exact omitted-record accounting.

## Verification performed

- Focused normal: 24 passed across `internal/manageddiagnostic`, `sessionruntime`, and `cmd/lsp-trace`.
- Focused race: 24 passed across the same packages.
- Focused `go vet`: passed.
- `go build ./cmd/lsp-trace`: passed.
- V1/V2/V3/FR22/FR23/MCP25/qualification-focused test selection: 161 passed across four packages.
- `git diff --check`: passed.
- Targeted private JSON-field privacy scan: passed.

Full suite, full race, network, installation, deployment, product/live execution, public MCP, schema-registry operations, and D01 were not run.

## Not certified by this repair

The supplied review additionally requires a substantially larger closed semantic validator, authoritative multi-scenario built-process harness, and routing of manager-internal `documentSymbol` seed validation through the same diagnostic operation collector. Those changes are not implemented or certified here. Therefore this document does not claim that every finding in the independent review is repaired, and D01 remains not ready.
