# TRANSPORT-CORE repair

## Scope

Repaired blockers identified in `/tmp/lsp-trace-fr23-request-diagnostics/TRANSPORT-CORE-INDEPENDENT-REVIEW.md` for commits `760f3b8`, `84b414e`, and `61602f5` on branch `pi-agent-a9679d85-fb65-45f`. No network, install, deploy, product/live-D01, full-suite, full-race, runtime behavior, CLI behavior, or MCP behavior was exercised or changed.

## Repairs

1. **Environment identity:** environment pairs are sorted canonically before the existing domain-separated, length-prefixed digest. Duplicate environment names are explicitly rejected by returning `IdentityUnavailable`; reordered equivalent pairs retain identity, while changed values change identity.
2. **Executable identity:** the executable is opened first; the descriptor is `fstat`-validated as regular, executable, and not group/world writable; the bounded digest is read from that descriptor; then the canonical path is `lstat`/`stat` checked against the descriptor identity. Replacement yields `IdentityRaced`; unavailable validation/path operations fail closed. A deterministic canonical-stat seam test replaces the path after open.
3. **Writer ordering:** immutable event batches are enqueued while `Writer.mu` still owns serialization order. A separate queue/drainer delivers callbacks outside `Writer.mu`; reentrant writes append without waiting on the active callback and preserve queue order.
4. **Reader decode failures:** malformed JSON, trailing content, wrong JSON-RPC version, and invalid message shape each emit one `EventDecode` with `KindInvalid` and `Closed=true`.
5. **Pending history:** generation history tracks active reference counts and deterministically evicts the oldest inactive generations down to the configured tombstone capacity. Active generations are retained even above the history cap; evicted late responses classify as `ResponseUnknown`.

Public constructors and types are preserved. Default/no-observer serialized wire bytes remain unchanged.

## RED evidence

Persistent guard output: `/tmp/lsp-trace-fr23-request-diagnostics/TRANSPORT-CORE-RED.txt`.

Observed before production repair:

- `ASSERT_PROCESS_IDENTITY_ENV_ORDER_INSENSITIVE`
- `ASSERT_LSPWIRE_WRITER_EVENT_WIRE_ORDER: [39 38]`
- `ASSERT_LSPWIRE_READER_DECODE_FAILED_CLOSED: []` for wrong version and trailing content
- `ASSERT_LSPWIRE_PENDING_GENERATIONS_BOUNDED: 1000`

## GREEN and focused verification

- Managed identity guards: `Go test: 3 passed in 1 packages`.
- Repaired transport plus existing focused guards: `Go test: 20 passed in 1 packages`.
- Repeated focused normal: `Go test: 170 passed in 3 packages`.
- Repeated focused race: `Go test: 51 passed in 3 packages`.
- Complete focused package normal: `Go test: 75 passed in 3 packages`.
- Complete focused package race: `Go test: 75 passed in 3 packages`.
- `go vet ./internal/lspwire ./internal/manageddiagnostic ./internal/managedprocess`: passed.
- `go build ./cmd/lsp-trace ./cmd/fake-lsp`: passed.
- `git diff --check`: passed.
