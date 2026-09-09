# Sessionruntime integration claim

Base: `61602f5` (`feat: derive private managed process identity`).

## Implemented scope

- The managed start and restart paths derive privacy-safe process identity from the exact `managedprocess.Spec` before invoking `Starter.Start`; retained identity contains only fixed-size digests, status, and byte count.
- Optional diagnostics add manager-attempt-bound session, generation, and operation handles. `DiagnosticSnapshotFor` authorizes with the exact startup-attempt identity, returns only closed collector snapshots, and clones event storage.
- Readiness records initialize write attempt/completion, response reads, parsed call-hierarchy capability projection, initialized notification completion, and one terminal event.
- `PrepareDocument` records cache hits and didOpen/didChange write lifecycle. The cache mutates only after a complete write; late exact-generation completion cannot mutate a replacement.
- `RoundTrip` binds owner sequence to the actual wire request ID, records requested versus effective deadline (including an earlier parent deadline), write/read transport events, unmatched/matched/late classifications, and first-terminal closure. Cancellation retires transport and joins the outstanding read before return, so a closed snapshot cannot precede read completion.
- The existing per-request observer callback and diagnostic store write now run after `Manager.mu` is released. Collector operations do not invoke external callbacks. No callback is introduced under Manager, writer, or pending locks.
- Runtime handle fields use `json:"-"`; nil diagnostics retain zero handles and historical behavior/public marshaled bytes.

## Deterministic guards and qualification

- `TestRoundTripDiagnosticCallbackRunsWithoutManagerLock` uses callback re-entry as a deterministic lock barrier. The retained RED record is `runtime-integration-red.log`; the same assertion passes after the lock-order repair.
- `TestObserveIdentityDetectsExecutableReplacementAtBarrier` deterministically replaces the executable between read and the second stat.
- Integration tests cover exact-attempt authorization, closed/immutable snapshots, pre-start identity projection, readiness capability/initialized completion, zero-handle nil mode, and JSON exclusion.
- Existing focused sessionruntime guards continue to cover response/deadline ordering, late/unmatched responses, write failure, initialize capability forms, didOpen completion/cache/stale generation, and exact-generation lifecycle behavior.
- Focused normal and race runs passed for `./sessionruntime ./internal/manageddiagnostic ./internal/managedprocess` (142 tests). Focused vet/build passed before the final report update and is rerun after it.

## Explicit gaps

- **Sink gap:** `StartupDiagnosticSink` still projects the existing startup-attempt and generation-record schema only. It does not serialize event-collector snapshots, operation handles, or process identity. This is intentionally excluded to avoid private schema/publication expansion.
- **CLI gap:** no CLI flag, command, file selector, output field, or help text exposes the new runtime snapshots. CLI publication is intentionally excluded.
- No public schema, MCP, registry, deployment, product, live-D01, installation, full-suite, or full-race changes were made or claimed.
