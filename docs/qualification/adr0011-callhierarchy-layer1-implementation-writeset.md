# ADR-0011 Call-Hierarchy Layer-1 Implementation Write-Set

Status: **IMPLEMENTATION WRITE-SET FREEZE / GATE PENDING / NO GO**

- Repository: `/Users/schwa/dev/lsp-trace-adr0011-c17`
- Pinned HEAD: `12d9af179c49412c8203582a3fa52d7836da18b7`
- Required starting state: clean worktree
- Inputs: accepted causal RED and accepted Layer-1 oracle corpus already frozen at the pinned HEAD
- Authority: implementation write-set and verification-order freeze only
- Writer authority: **not issued**
- Commit authority: **not issued**

## Decision

The smallest Layer-1 behavior implementation write-set is exactly two existing production files:

1. `sessionruntime/sessionruntime.go`
2. `sessionruntime/b4_owned_method_pair_lease.go`

No source, test, corpus, generated artifact, or other documentation file is admitted by this freeze. No new file is admitted. Expansion requires a separate explicit amendment before any expanded mutation.

Implementation may begin only after a separate gate issues GO and assigns exactly one writer. Until then this document remains **NO GO**.

## Required production behavior by file

### `sessionruntime/sessionruntime.go`

This file owns only the `Manager.RoundTrip` integration that cannot be implemented from another file:

- admit the already-private, opt-in, valid Layer-1 attachment without changing default behavior for requests with observation disabled or absent;
- wire the admitted attachment into the existing managed transaction and its existing ownership/accounting lifecycle;
- preserve the exact original transported request and response frames and the exact externally returned result;
- publish terminal completion, the generic transport/accounting snapshot, and the exact external-result attachment only after the corresponding state is complete;
- transfer or release ownership correctly on every success, protocol-error, external-error, cancellation, deadline, shutdown, transport-unavailable, refusal, panic-contained callback, and early-return branch;
- ensure every terminal branch closes admission, drains or joins admitted callbacks as required, and performs exact-once cleanup without leaks or double release; and
- keep enabled and disabled execution externally transparent: byte-for-byte and value-for-value identical result, error, terminal, transport-message, transport-byte, and bounded external-byte behavior.

The integration must be opt-in. A missing, disabled, invalid, stale, foreign, duplicate, reordered, terminal, or otherwise non-admissible attachment must be refused before any observation effect. Refusal must not consume a reservation, acquire or publish ownership, allocate a lease, invoke a callback, emit a diagnostic, mutate accounting, rewrite transport, add a retry, change terminal state, or alter any externally visible byte or value.

### `sessionruntime/b4_owned_method_pair_lease.go`

This file owns only private, generic call-hierarchy owner/lease/snapshot behavior and the typed fields proved missing by the accepted RED:

- private generic owner creation, lifecycle, terminal closure, and snapshot operations;
- private generic lease acquisition, callback access, release, and exact-once ownership transfer/release;
- generic immutable views and snapshots for exact request/response frames, opaque commitments, terminal state, external result/error, and accounting;
- the full closed generic ledger dimensions: transport messages, transport bytes, externally visible bytes, reservations, leases, callbacks, transport diagnostics, and terminal before/after state;
- exact terminal and exact external-error representation, preserving presence, concrete class, message, data, code, serialized bytes, and independent availability wherever applicable;
- defensive copies on ingress, retained ownership, callback/view delivery, snapshot publication, and repeated egress for every admitted mutable byte slice or nested mutable value;
- bounded reserve-before-copy behavior, using the existing reservation and accounting primitives before retaining any bytes;
- refusal-before-effect for invalid lifecycle, capacity, ownership, attachment, or terminal conditions;
- callback return-error transparency, callback-panic containment, in-flight callback drain, terminal admission closure, concurrent release/terminal safety, and exact-once release; and
- generic enabled/disabled snapshots sufficient to prove external transparency without interpreting payload semantics.

Reuse the existing ownership, reservation, lease, callback, terminal, and accounting primitives. Do not introduce parallel ownership/accounting machinery, global storage, an alternate transaction path, or a second lifecycle model.

## Exact semantic ceiling

Layer 1 may retain only:

- exact original transport frames and exact externally returned result/error representations;
- generic Manager-established workspace, session, generation, request-key, transaction, and predecessor custody joins already represented by accepted private types;
- opaque commitment metadata already admitted by the accepted boundary: typed opaque identity, SHA-256 digest, exact byte length, and availability where required; and
- generic owner, lease, reservation, callback, diagnostic, terminal, and accounting observations.

Opaque commitments remain opaque. Layer 1 must not receive, retain, reconstruct, validate, hash, serialize, parse, or expose selected-item or handoff payload/preimage bytes. Commitment equality is only opaque value equality and confers no semantic validity, evidence authority, or target meaning.

There are no target semantics in this implementation. Layer 1 must not inspect or derive target identity, direction, method-family meaning, JSON token form, selected ordinal meaning, prepared-item validity, successful-empty meaning, occurrence, member, range, endpoint, grouping, relation, admission, qualification, publication, or downstream ADR-0007 meaning.

## Initially excluded `internal/lspwire`

`internal/lspwire` is excluded from the initial write-set. The implementation must reuse the existing exact frame and result/error captures.

If the exact original request frame, response frame, returned result, or exact external-error representation is inaccessible from the admitted `sessionruntime` seam, stop. Do not add an `internal/lspwire` accessor, field, callback, serialization, capture, or mutation under this freeze. A separate amendment must identify the exact missing capture and independently review the expanded write-set before implementation resumes.

## Explicit exclusions

The write-set excludes:

- all tests and testdata, including the already-frozen RED and oracle corpus;
- all corpus, fixture, manifest, digest, and generated-file changes;
- C15 accounting changes, C15 bindings, candidate classifications, or catalog changes;
- directional incoming/outgoing adapters or method allowlists;
- target selection, normalization, occurrence construction, grouping, relation admission, qualification, or publication;
- semantic-vector construction, interpretation, or output;
- CLI, MCP, command, provider, catalog, public schema, public API, configuration, feature flag, or activation changes;
- `internal/lspwire` changes unless separately amended after the mandatory stop;
- new exported identifiers, new public package paths, logging, telemetry, metrics, tracing, or diagnostic emission;
- new serialization, parsing, canonicalization, hashing, persistence, replay, or publication APIs; and
- any new production or test file.

## Forbidden imports and APIs

Neither admitted file may add an import or API whose purpose is semantic interpretation, serialization, hashing, publication, logging, or configuration. In particular, do not add:

- JSON or other codec/serialization packages;
- SHA, digest, or other hashing packages;
- logging, tracing, telemetry, metrics, or event-publication packages;
- CLI, MCP, catalog, config, environment, feature-flag, or activation packages;
- call-hierarchy direction, occurrence, normalization, grouping, relation, semantic-vector, or publication packages; or
- new exported constructors, accessors, interfaces, callbacks, request/result fields, schemas, or registration hooks.

Private helper code is permitted only inside the two admitted files when required to implement the accepted private RED and when it does not create a new candidate, API surface, serialization path, or parallel lifecycle/accounting mechanism.

## C15 preservation rule

The implementation must introduce **no new C15 candidates**.

Before proceeding past any source delta, compare the C15 v2 candidate and classification surface with the pinned baseline. If any required source change adds, removes, or changes a candidate; requires a candidate classification; requires a C15 binding or accounting change; or requires catalog regeneration, **STOP**. Do not classify, bind, regenerate, or repair it under this write-set. Resume only after an independent amendment explicitly accepts that delta.

Historical C15 v2 artifacts must remain byte-identical, including candidate rows, classification rows, manifests, and digest sidecars. Existing historical byte identity is a hard preservation requirement, not a regeneration target.

## Implementation stop boundaries

Stop immediately and request an independent amendment if any of the following is required:

1. a file other than the two admitted production files must change;
2. a new file, test, fixture, corpus row, generated artifact, public symbol, or exported API is needed;
3. exact frames, results, or external errors are not accessible through existing captures;
4. implementation requires `internal/lspwire` mutation;
5. implementation requires target, direction, method-family, token-form, selection, normalization, occurrence, grouping, relation, qualification, or publication knowledge;
6. implementation requires new serialization, parsing, hashing, logging, telemetry, configuration, activation, CLI, MCP, schema, or catalog behavior;
7. implementation requires new ownership, accounting, reservation, callback, lease, or terminal primitives rather than reuse of existing ones;
8. reserve-before-copy cannot be maintained on every retained-byte path;
9. refusal cannot occur before every observation and transport effect;
10. any terminal or early-return branch cannot prove transfer or exact-once release;
11. enabled execution cannot preserve exact disabled external behavior;
12. any C15 candidate, classification, binding, catalog, manifest, or byte pin changes; or
13. the accepted RED or corpus appears inconsistent with the implementation seam.

No stop condition may be worked around by weakening a test, updating the corpus, normalizing an error, reconstructing bytes, changing availability semantics, adding a compatibility shim, or broadening the implementation silently.

## Frozen verification order

After the separate GO and single-writer assignment, verification must run in this order. A failure stops the sequence; do not repair outside the admitted write-set.

1. **Preflight custody:** confirm exact pinned starting revision, clean starting worktree, one active writer, and a diff limited to the two admitted production files.
2. **Focused RED:** run the accepted corpus-backed causal RED in `sessionruntime`, preserving the frozen tests and corpus byte-for-byte.
3. **Package verification:** compile and test `./sessionruntime`.
4. **Wire compatibility:** compile and test `./internal/lspwire` without modifying it, proving existing captures remain sufficient.
5. **Affected downstream verification:** derive affected packages from the actual `git diff` and `go list`; compile and test only the discovered downstream package set before broader checks. Discovery does not authorize edits outside the write-set.
6. **C15 preservation:** verify zero candidate/classification delta and byte identity of the historical C15 v2 candidate, classification, manifest, and digest artifacts.
7. **Repository static analysis:** run `go vet ./...`.
8. **Final boundary check:** confirm the final diff contains only `sessionruntime/sessionruntime.go` and `sessionruntime/b4_owned_method_pair_lease.go`, with no test, corpus, generated, catalog, public-schema, config, or activation change.
9. **Independent review:** obtain a fresh read-only review of behavior, all terminal cleanup branches, enabled/disabled external transparency, defensive copying, reserve-before-copy, refusal-before-effect, concurrency, exact historical C15 v2 byte identity, and complete scope compliance.

Independent review is mandatory and cannot be performed by the writer. Passing verification does not itself authorize a commit, activation, directional semantics, publication, or production admission.

## Gate

The implementation gate is pending. This document issues no GO.

A later authorization must name the accepted gate, the exact pinned input revision, and exactly one writer before either production file is modified. No concurrent or secondary writer is permitted. The writer must stop at every boundary above and may not self-amend this write-set.

```text
ADR0011_CALLHIERARCHY_LAYER1_IMPLEMENTATION_WRITESET_GO — NOT ISSUED
```
