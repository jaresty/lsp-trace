# ADR-0011 Call-Hierarchy Layer-1 Implementation Write-Set Review

## Scope

Independent strict read-only review of pinned HEAD `12d9af179c49412c8203582a3fa52d7836da18b7` and the untracked proposal `docs/qualification/adr0011-callhierarchy-layer1-implementation-writeset.md`. The worktree was otherwise clean. This review creates only this document; it performs no source, test, corpus, generated-artifact, activation, or commit change.

Authority is limited to the Layer-1 production implementation write-set. It does not accept feature identity or semantics, authorize production activation, authorize a commit, or authorize any file beyond the two named production files.

## Inputs accepted for this gate

- Accepted corpus-backed causal RED at HEAD `12d9af17`.
- Accepted immutable corpus `adr0011.callhierarchy.layer1.red-oracle-corpus/v1`: 25 ordered worlds, aggregate byte length `431974`, aggregate SHA-256 `7fddcd923a9299c52014334d5460cb1a5ce78604e4a732faaef11d02adc30e84`, source provenance `4f2b5504c1e4f4e559f48db7ab8e0c5b8ca70fa9`.
- Accepted causal review `docs/qualification/adr0011-callhierarchy-observation-api-causal-red-review.md`, which classifies the remaining failures as missing private API/propagation behavior and reports no corpus, harness, decode, digest, path, race, panic, external-execution, or unrelated-regression defect.
- Existing private scaffolding and source seams at the pinned HEAD.

The RED, tests, testdata, corpus worlds, manifest, digests, and prior qualification records are frozen inputs, not implementation write-set members.

## Exact production write-set

The proposed production write-set is both minimal and sufficient:

1. `sessionruntime/sessionruntime.go`
2. `sessionruntime/b4_owned_method_pair_lease.go`

No new file is admitted.

### Why `sessionruntime/sessionruntime.go` is necessary

`Manager.RoundTrip`, `roundTripWithPrivateTransaction`, and `finishRoundTrip` own the only existing transaction seam at which Layer-1 attachment admission can be checked before effect and then joined to the exact Manager-established session, generation, request key, transaction lifecycle, request write, response read, external result/error, terminal, diagnostics, and accounting outcome. This file must perform only the private default-off admission/integration and terminal snapshot handoff.

The implementation must preserve the existing single transport path and external return behavior. Enabled and disabled executions must remain byte-for-byte and value-for-value externally identical for result, error, failure, notifications, responses, message counts, byte counts, request counts, request bytes, terminal outcome, and bounded external bytes.

### Why `sessionruntime/b4_owned_method_pair_lease.go` is necessary

This file already owns the private Layer-1 attachment/ledger/snapshot scaffolding and the reusable owner/lease pattern. It is the minimal location for the missing private generic fields and operations required by the accepted RED: retained owner/lease, immutable observation views, exact frame and custody snapshots, opaque commitments, generic terminal/external-result/external-error representation, complete accounting deltas and known-zero state, defensive copying, callback containment, terminal admission closure, callback drain, concurrent release safety, and exact-once release.

Implementation must extend the existing machinery rather than add a second owner, lease, reservation, callback, terminal, or accounting model. No duplicate D/R machinery is admitted.

## Existing capture seam is sufficient

No `internal/lspwire` change is required for this phase.

The existing transaction path already provides:

- bounded exact request-frame retention through `framedWriteCapture.retainedFrame`;
- bounded exact response-frame retention through `lspwire.Reader.ReadWithFrameIfWithin`, with the existing private successor-reader path retaining its original response frame;
- request/response frame observations and custody joined to the Manager-generated request key;
- exact returned `RoundTripResult.Result`, `RoundTripResult.ServerError`, `RoundTripResult.Failure`, notifications, responses, transport message/byte counts, and request message/byte counts; and
- one common terminal integration funnel in `finishRoundTrip` for completion and failure branches after early pre-admission refusals.

These primitives can be reused without adding an accessor, callback, field, serializer, capture mode, or mutation in `internal/lspwire`. If implementation discovers that an exact original request frame, exact original response frame, exact external result, or exact external-error representation cannot be obtained from these existing seams, it must stop. That discovery is a blocker requiring an independently reviewed write-set amendment naming the exact missing capture; it does not authorize an `internal/lspwire` edit.

## Required behavioral boundary

The two admitted files are sufficient only if the implementation proves all of the following without expanding scope:

- Manager attachment admission is private, opt-in, exact-session/generation/request-lifecycle bound, and default-off.
- Missing, disabled, invalid, stale, foreign, duplicate, reordered, terminal, or otherwise non-admissible attachments are refused before reservation, lease, callback, diagnostic, accounting, transport, terminal, or publication effect.
- Retained owner/lease state uses reserve-before-copy and exact-once transfer/release on success, server/protocol error, external error, cancellation, deadline, shutdown, transport unavailable, refusal, and every early-return/poison branch.
- Exact original request and response frames, exact externally returned result/error, exact custody joins, and generic terminal/accounting snapshots are preserved without reconstruction or normalization.
- Every admitted mutable byte slice and nested mutable value is defensively copied on ingress, retention, callback/view delivery, snapshot publication, and repeated egress.
- Callback return errors are externally transparent; callback panics are contained; terminal closure prevents new callbacks; release waits for admitted callbacks; concurrent release has one winner; cleanup is exact once.
- Generic snapshots represent availability independently from zero/empty/null and preserve external-error presence, concrete class, message, data, code, serialized bytes, and availability wherever the accepted corpus requires them.
- Enabled and disabled executions remain externally transparent; observation contrast must not alter transport or caller-visible values.

## Semantic and surface exclusions

Layer 1 retains generic transport/custody/accounting observations and opaque commitment metadata only. It must not inspect, derive, validate, reconstruct, hash, serialize, parse, normalize, or expose selected-item or handoff payload/preimage bytes.

There is no target or directional semantics in this write-set. Explicitly excluded are:

- target identity or selection;
- incoming/outgoing adapters, direction, method-family interpretation, allowlists, or token-form meaning;
- normalization, prepared-item validity, successful-empty interpretation, occurrence/member/range/endpoint construction;
- grouping, relation admission, qualification, publication, or downstream ADR-0007 meaning;
- semantic-vector construction, interpretation, or output;
- CLI, MCP, commands, providers, catalogs, configuration, feature flags, activation, public API, public schema, or exported identifiers;
- logging, tracing, telemetry, metrics, persistence, replay, serialization, parsing, canonicalization, or hashing APIs; and
- changes to tests, corpus, fixtures, manifests, digest sidecars, generated artifacts, `internal/lspwire`, or any third production file.

No public schema or public registration surface is authorized. Production activation is explicitly not authorized.

## C15 preservation and stop/amend rule

The implementation must produce no C15 candidate delta. The C15 v2 candidate/classification surface and historical candidate rows, classification rows, manifests, and digest sidecars must remain byte-identical.

After each source delta, compare the candidate/classification surface with the pinned baseline. If a change adds, removes, or changes a candidate; needs a new classification or binding; changes accounting ownership; requires regeneration; or changes a catalog, manifest, or byte pin, stop immediately. Do not classify, bind, regenerate, repair, or weaken the RED under this authority. Resume only after an independent amendment explicitly admits the exact delta.

The same mandatory stop/amend rule applies if implementation requires:

1. any file other than the two admitted production files;
2. any new file, test, fixture, corpus row, generated artifact, exported symbol, or public API;
3. any `internal/lspwire` mutation or missing exact capture seam;
4. semantic target, direction, method-family, selection, normalization, occurrence, grouping, relation, qualification, publication, or semantic-vector knowledge;
5. new serialization, parsing, hashing, logging, telemetry, config, activation, CLI, MCP, schema, or catalog behavior;
6. parallel owner/accounting/reservation/callback/lease/terminal machinery;
7. a retained-byte path that cannot reserve before copying;
8. a refusal that cannot occur before every observation and transport effect;
9. a terminal branch that cannot prove transfer or exact-once release;
10. enabled execution that cannot preserve disabled external behavior; or
11. inconsistency between the accepted RED/corpus and the two-file seam.

No stop condition may be worked around by changing tests/corpus, normalizing errors, reconstructing bytes, changing availability semantics, or adding a compatibility shim.

## Writer and verification order

Exactly one writer may implement the authorized change, and that writer may modify only the two admitted production files. No concurrent or secondary writer is permitted. The writer may not self-amend the write-set.

Verification order is frozen:

1. Confirm pinned input revision `12d9af179c49412c8203582a3fa52d7836da18b7`, the known qualification-document inputs, exactly one active writer, and an implementation diff limited to the two admitted production files.
2. Run the accepted focused corpus-backed causal RED without changing tests or corpus.
3. Compile and test `./sessionruntime`.
4. Compile and test `./internal/lspwire` without modifying it.
5. Derive affected downstream packages from the actual diff and `go list`; compile and test that discovered set without expanding edit authority.
6. Verify zero C15 candidate/classification delta and byte-identical historical C15 v2 artifacts.
7. Run `go vet ./...`.
8. Confirm the final implementation diff contains only `sessionruntime/sessionruntime.go` and `sessionruntime/b4_owned_method_pair_lease.go`, with no test, corpus, generated, catalog, public-schema, config, or activation change.
9. Obtain a fresh independent read-only review of admission/refusal ordering, exact frames/custody, terminal and accounting snapshots, defensive copies, callback/release concurrency, exact-once cleanup, enabled/disabled transparency, C15 preservation, and scope compliance.

A failure stops the sequence. Verification success does not authorize commit, activation, directional semantics, publication, or a broader write-set.

## Verdict

`ADR0011_CALLHIERARCHY_LAYER1_IMPLEMENTATION_WRITESET_GO`

This GO authorizes assignment of exactly one implementation writer to modify exactly `sessionruntime/sessionruntime.go` and `sessionruntime/b4_owned_method_pair_lease.go`, subject to every stop/amend boundary above. It does not authorize production activation, a commit, test or corpus changes, `internal/lspwire` changes, semantic interpretation, publication, or any additional file.
