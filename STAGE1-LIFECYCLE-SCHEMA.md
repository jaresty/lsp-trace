# Stage 1 private request lifecycle schema

## Status and scope

Stage 1 defines the repository-internal document family
`lsp-trace.private-request-lifecycle-diagnostics.v1`, its closed offline
validator, and a projection boundary for an immutable certified runtime
snapshot. It does not add runtime routing, CLI process scenarios, public MCP
operations, schema-registry entries, deployment behavior, or product/live D01
qualification. **This work does not claim D01 readiness.**

The implementation is isolated in `internal/requestlifecycle`. The existing
FR23 private sink and shared hardened publisher from base commit
`b019c2eae54eec50d107ca98ffb0575310593f97` are unchanged.

## Closed model

A document contains exactly these top-level members:

- `schema_version`
- `attempt`: attempt, manager, process, and generation identities
- `initialize`: initialization operation, terminal status, and negotiated
  `documentSymbol` capability
- `documents`: URI-digest-bound document identities and their `didOpen` /
  optional `documentSymbol` operation references
- `operations`: unique operation IDs, unique nonzero handles, closed methods,
  and optional document foreign keys
- `events`: strictly increasing lifecycle observations
- `retention`: exact availability, omission, eviction, truncation, and bounds
- `artifact`: exact public graph-provenance V3 schema, byte length, and SHA-256

JSON object members are closed at every level. Duplicate decoded keys
(including escaped aliases), unknown fields, and trailing values reject.
Identifiers use the closed ASCII alphabet `[A-Za-z0-9._:-]`, start
alphanumerically, and are at most 256 bytes; this excludes path/URI adaptation
and control-character smuggling. Digests are lowercase `sha256:` plus 64 hex
characters. Methods, statuses, and event kinds are closed enums.

The hard limits are 64 aggregate document/operation/event records, 65,536
serialized bytes, and 4,096 bytes per retained string. Retention embeds these
exact constants and must account exactly for retained records.

## Validation order

`Verify(raw, publicV3)` applies these layers:

1. reject empty/oversized or syntactically duplicate JSON;
2. extract only the closed artifact binding;
3. require `lsp-trace.graph-provenance.v3`, exact public byte length, and exact
   SHA-256;
4. only after binding succeeds, decode the complete closed model and apply
   lifecycle semantics.

A corrupt or substituted public artifact therefore fails before lifecycle
semantic claims are evaluated.

## Semantic closure

Operations and handles are unique. Every event references one known operation
and its exact handle, except process-exit and cleanup events, which carry no
operation identity. Event sequences are positive and strictly increasing.
Every operation dispatches once and reaches exactly one terminal disposition.

`MATCHED` requires, in order, dispatch, complete request write, response body,
decode, and match. `LATE` requires a prior non-matched terminal for the same
operation. `UNMATCHED` requires a written request and decoded response body.
Framing failure occurs after a complete write but before a response body;
write failure occurs before write completion.

A document's `didOpen` operation must target that exact document. Declared
`didOpenComplete` equals the retained completion event. `documentSymbol`
requires completed `didOpen`, the exact document foreign key, and a negotiated
capability from a `MATCHED` initialize operation. Capability cannot be asserted
for failed or unmatched initialization.

A retained process exit is unique, precedes cleanup, and requires cleanup.
`AVAILABLE` permits omitted records only when truncation is true; `OMITTED`
retains no records, has a positive omitted count, and is truncated; `EVICTED`
retains no records, has a positive eviction count, and is not truncation.

## Projection authority

`Project` accepts only `CertifiedRuntimeSnapshot`, whose representation is
unexported and defensively copies slices. The package exposes no constructor
from caller-supplied event or operation arrays. Certification and snapshot
construction remain package-private. Go's `internal` import boundary also
prevents consumers outside the module from importing the package.

Stage 1 intentionally leaves runtime capture/wiring for a later stage.

## Focused guard matrix

Valid fixtures cover:

- matched request;
- timeout followed by late response;
- unmatched decoded response;
- framing failure;
- write failure;
- seed `didOpen` followed by `documentSymbol`.

Table mutations cover adapted and duplicate IDs, duplicate handles,
non-increasing sequence, unknown operation, wrong handle, duplicate terminals,
MATCHED without complete write, LATE without prior terminal, premature
UNMATCHED, didOpen mismatch, documentSymbol without capability, capability
without matched initialize, exit without cleanup, retention/truncation/eviction
mismatches, string and record bounds, known-field method/digest smuggling,
unknown top-level and nested fields, duplicate keys, trailing JSON, public
artifact mismatch, semantic-after-binding order, uncertified projection, and
defensive snapshot copying.

Focused verification command:

```text
go test ./internal/requestlifecycle
```
