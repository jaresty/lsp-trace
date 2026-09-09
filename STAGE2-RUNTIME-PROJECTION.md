# Stage 2 runtime projection

## Status and scope

Stage 2 connects the runtime through `cb63480` to the Stage 1 private
`requestlifecycle` document introduced by `edbdc6b`. The integration remains
repository-internal and adds no CLI, MCP operation, schema-registry entry,
public graph bytes, deployment behavior, product behavior, or live D01 claim.
**D01 was not run and this work does not claim D01 readiness.**

## Manager-certified source

`sessionruntime.DiagnosticSnapshotSetFor` accepts the exact startup-attempt,
diagnostic-generation, and diagnostic-operation handles issued by one manager.
It rejects empty or forged handles, foreign attempts, stale/replaced
generations, duplicate operation handles, open operations, evicted operations,
and sources exceeding the declared record bound. It verifies that the supplied
generation is still the manager's current generation before certification.

The returned set owns cloned event slices and carries an unexported manager
capability. Accessors return defensive copies. Diagnostic handles and snapshot
fields remain excluded from JSON. Callers cannot mint a certified set or supply
lifecycle event arrays.

The package-private `internal/requestlifecycle.projectRuntime` adapter accepts
only this certified set. It derives opaque attempt, manager, process, document,
operation, and handle correlations from manager-owned snapshots. Operations are
ordered by their manager-assigned handle and events receive one strictly
increasing document sequence. Collector omissions, late-after-close counts, and
projection-only dropped detail are retained as explicit omission accounting.
Terminal projection remains ordered after dispatch/write/read/decode evidence.

The artifact member is deliberately a placeholder for later finalization:
`lsp-trace.graph-provenance.v3`, zero bytes, and the SHA-256 of the empty byte
sequence. Stage 2 does not bind or emit public V3 bytes.

## Runtime operation projection

The runtime now retains closed metadata on the same manager-owned operation:

- readiness records `initialize`, negotiated position encoding, call-hierarchy
  support, and document-symbol support;
- document supply records `textDocument/didOpen` and its canonical file URI;
- round trips record their exact method, protocol ID/handle, URI when present,
  write/read/correlation/decode/disposition events, and terminal event;
- process identity remains the immutable privacy-safe managed-process snapshot.

The adapter projects readiness, `didOpen`, `textDocument/documentSymbol`,
`textDocument/prepareCallHierarchy`, incoming, and outgoing operations through
the same closed Stage 1 method vocabulary. Invalid methods and missing
initialize/terminal state fail closed.

## Provider-backed documentSymbol routing

Provider-backed seed admission no longer validates semantic response bytes only
after the diagnostic collector has closed. `AdmitSeedBinding` invokes the same
private `roundTrip` implementation used by public `RoundTrip`; the only extra
input is a package-private semantic classifier. The shared path retains request
method, JSON-RPC ID, complete write, reads, correlation, decode, semantic
match/mismatch, and terminal in one collector on the same generation.

The classifier is not a public request field and callers cannot construct
collector events. A semantic mismatch remains a non-match admission result, so
the existing acquisition executor returns before
`textDocument/prepareCallHierarchy`. The focused acquisition ordering tests
continue to enforce that boundary.

## Focused guards

Added guards cover:

- matched and mismatched provider classification before collector terminal;
- certified exact-attempt/exact-generation snapshot-set admission;
- forged zero-value and stale generation rejection;
- duplicate source-handle rejection and bounded source admission;
- defensive event-slice copying;
- readiness, `didOpen`, and `documentSymbol` projection;
- retained document-symbol capability;
- placeholder public-artifact binding without public bytes.

Existing focused guards continue to cover timeout, late, unmatched response,
missing/extra event, terminal chronology, restart generation isolation, and no
prepare-call-hierarchy on seed mismatch.

Verification performed (bounded; not the full suite):

```text
go test ./sessionruntime ./internal/requestlifecycle ./acquisitionops ./incomingops ./sliceops ./lifecycleops -count=1
go test -race ./sessionruntime ./internal/requestlifecycle ./acquisitionops ./incomingops ./sliceops ./lifecycleops -count=1
go vet ./sessionruntime ./internal/requestlifecycle ./acquisitionops ./incomingops ./sliceops ./lifecycleops
go test ./sessionruntime ./internal/requestlifecycle ./acquisitionops ./incomingops ./sliceops ./lifecycleops -run '^$' -count=1
```

Normal and race runs each passed 247 tests in six packages. Vet emitted no
diagnostics, and the compile-only command completed for all six packages.
