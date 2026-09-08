# Internal managed diagnostics

FR23's first checkpoint is internal only. `internal/manageddiagnostic` owns safe diagnostic records and semantic validation; `sessionruntime.Manager` optionally retains them by exact opaque session ID and generation. This checkpoint does not register a CLI, MCP tool, public schema family, registry entry, graph-provenance field, or default file sink.

## Current capture

Closed phases are `spawn`, `initialize-write`, `initialize-response`, `request-dispatch`, `readiness-complete`, `document-supply`, and `capability-check`; `initialized-notification` is an initialize-write/readiness-complete substep. Closed terminals are `response-received`, `protocol-error`, `transport-closed`, `cancelled`, `deadline-exceeded`, `process-exited`, and `unknown`. Reason is separate from terminal.

Runtime capture currently records initialization write/read/cancellation distinctions, one composite successful-readiness record covering the initialize write, matching initialize response, and initialized notification, successful and failed document-notification writes, and actual `RoundTrip` method, framed protocol request ID, request/response message and byte accounting, caller-supplied opaque target/caller IDs, owner-scoped sequence, limits, and terminal. Ordinary `RoundTrip` records use `request-dispatch`; they are never labeled as initialize responses. `RecordCapabilityCheck` is a typed hook for the slice/incoming operation owner; operation wiring is deferred.

Every optional fact carries `observed`, `unavailable`, or `withheld`. EOF is transport closure, not process exit. Process exit is claimed only when the process owner independently observes death before cleanup; teardown-induced death is not causal. Document supply completion is a prerequisite fact, not proof the server consumed or analyzed content.

## Startup-attempt identity

Every `sessionruntime.Manager.Start` invocation receives a distinct host-owned `StartupAttemptID` before deadline, cancellation, availability, pipe, or process-start checks. Restart process starts receive distinct attempts too. The default source is a cryptographically random manager-instance prefix. A test seam may inject nonce material, but the source never selects the final ID: the manager always appends its mutex-owned monotonic sequence. Duplicate or empty source output therefore cannot alias starts, without retries or an unbounded issued-ID set. This is collision-resistant process-runtime identity, not cryptographic authentication. It is not caller asserted, is not derived from session/profile/path/command, does not equal or parse as a generation, and makes no deterministic-order claim across schedules, managers, processes, or runs. Sequence order is observable only within one Manager lifetime.

`managed-startup-attempt/v1` is a separate internal contract. Failed attempts have no session/generation admission fields. Admission is attached only after the exact runtime session generation is installed; readiness snapshots carry the same attempt ID and exact generation. Exact attempt lookup never guesses a current generation. Attempt records are immutable clones retained in a Manager-level count/byte-bounded FIFO; teardown does not specially delete them, and eviction is explicit. Startup reason/subcodes are closed static values. Raw errors, commands, arguments, environment, paths, and stderr content are never copied. Process exit remains unavailable unless independently observed before cleanup; failed spawn does not invent an exit.

## Security and ownership

Records never contain raw stderr, raw errors, JSON-RPC messages/data, request params, source bytes/URI/path, command, args, or environment. Stderr records only status, configured cap, observed retained-byte count, and truncation; there is no content, digest, reference, or release suggestion. Session/profile identities are existing opaque runtime identities. Callers must pass only opaque target/caller IDs.

Records and query results clone mutable slices. The store is in-memory, mutex-protected, count- and byte-bounded per exact `(session ID,generation)`. Evicted record count is explicit. Missing and replacement generations return unavailable rather than records from a current generation. No file is written by default.

## Compatibility and limits

Diagnostics are observational and not canonical replay claims. Existing external failure/status codes and V1 evidence bytes are unchanged; no retry, acquisition, timeout, or public operation behavior is added. Retained records support later offline projection without consulting a process.

The local validator exhaustively checks closed phases, terminals, substeps, IO states, fact statuses, nonnegative counts/durations/limits, and phase/substep/terminal/outcome/process-exit relationships. Explicit unavailable or withheld facts must carry zero values, preventing hidden secret-bearing values. A wholly zero nested fact remains the Go representation of an omitted legitimate partial fact; validation does not infer root cause.

## Exact future diagnostic extension

The current internal schema records ordinary request dispatch and actual completed read accounting, but does not claim D01 response-disposition or read-loop-state coverage. Requested limits retain raw caller values; effective limits are the exact normalized values used by execution (defaults included). Deadline remaining time is derived once from the absolute request deadline and the manager clock snapshot, then reused for requested/effective facts. A future internal contract version must separately represent request write start and completion; response read and dispatch accounting with `MATCHED`, `UNMATCHED`, and `LATE` states; observed transport read-loop stalled versus observed transport read failure; capability checks; document preparation; and process lifecycle. Each state must preserve observed message/byte accounting and explicit unavailable/withheld facts. Raw stderr content remains withheld. These records are observational reports only: timeout, ordering, and process observations do not establish cause. This checkpoint does not run D01, increase timeout, retry, probe, or access product repositories.

## Deferred integration

A later parent-owned change may map acquisition target IDs into the optional join fields, invoke capability hooks in slice/incoming, and hydrate graph provenance V2/public diagnostic output through its isolated shared writer. That later work must define public schema/versioning, CLI/MCP parity, paging, and publication policy; this package is not globally registered.
