# Internal managed diagnostics

FR23's first checkpoint is internal only. `internal/manageddiagnostic` owns safe diagnostic records and semantic validation; `sessionruntime.Manager` optionally retains them by exact opaque session ID and generation. This checkpoint does not register a CLI, MCP tool, public schema family, registry entry, graph-provenance field, or default file sink.

## Current capture

Closed phases are `spawn`, `initialize-write`, `initialize-response`, `document-supply`, and `capability-check`; `initialized-notification` is an initialize-write substep. Closed terminals are `response-received`, `protocol-error`, `transport-closed`, `cancelled`, `deadline-exceeded`, `process-exited`, and `unknown`. Reason is separate from terminal.

Runtime capture currently records initialization write/read/cancellation distinctions, successful and failed document-notification writes, and actual `RoundTrip` method, framed protocol request ID, request/response accounting, caller-supplied opaque target/caller IDs, owner-scoped sequence, limits, and terminal. `RecordCapabilityCheck` is a typed hook for the slice/incoming operation owner; operation wiring is deferred.

Every optional fact carries `observed`, `unavailable`, or `withheld`. EOF is transport closure, not process exit. Process exit is claimed only when the process owner independently observes death before cleanup; teardown-induced death is not causal. Document supply completion is a prerequisite fact, not proof the server consumed or analyzed content.

## Security and ownership

Records never contain raw stderr, raw errors, JSON-RPC messages/data, request params, source bytes/URI/path, command, args, or environment. Stderr records only status, configured cap, observed retained-byte count, and truncation; there is no content, digest, reference, or release suggestion. Session/profile identities are existing opaque runtime identities. Callers must pass only opaque target/caller IDs.

Records and query results clone mutable slices. The store is in-memory, mutex-protected, count- and byte-bounded per exact `(session ID,generation)`. Evicted record count is explicit. Missing and replacement generations return unavailable rather than records from a current generation. No file is written by default.

## Compatibility and limits

Diagnostics are observational and not canonical replay claims. Existing external failure/status codes and V1 evidence bytes are unchanged; no retry, acquisition, timeout, or public operation behavior is added. Retained records support later offline projection without consulting a process.

The local validator rejects negative/mismatched elapsed time, response/timeout contradictions, claimed process exit without an observed process fact, unsupported capability with unknown negotiation, and observed exit without chronology. It intentionally accepts legitimate partial records and does not infer root cause.

## Deferred integration

A later parent-owned change may map acquisition target IDs into the optional join fields, invoke capability hooks in slice/incoming, and hydrate graph provenance V2/public diagnostic output through its isolated shared writer. That later work must define public schema/versioning, CLI/MCP parity, paging, and publication policy; this package is not globally registered.
