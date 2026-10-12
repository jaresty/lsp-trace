# ADR 0011 Call Hierarchy D/R recorder reuse review v2

- **Repository:** `/Users/schwa/dev/lsp-trace-adr0011-c17`
- **Reviewed HEAD:** `8210109f3f71fd969fe3201935014d4226c90c13`
- **Reviewed assessment:** `docs/qualification/adr0011-callhierarchy-dr-recorder-reuse-assessment.md`
- **Assessment SHA-256:** `c209c3deb79b32206fa1a898bdfa1bfe381371f7d9a5cf554756d5b2bd98e1bd`
- **Preserved prior review:** `docs/qualification/adr0011-callhierarchy-dr-recorder-reuse-review.md`
- **Prior review SHA-256:** `a7901df848bf702628bbdb398cc079cd78c536717bbbd0b43ed98440d1fd7dd3`
- **Review authority:** corrected-assessment acceptance only; no implementation, evidence acquisition, source/test/design/evidence mutation, qualification, production admission, or commit authority
- **Verdict:** `ADR0011_CALLHIERARCHY_DR_RECORDER_REUSE_GO`

## Decision

The corrected assessment closes all four defects from the preserved blocked review. Its smallest-core conclusion is accepted: extend the existing private `sessionruntime` manager/owner/lease/transaction custody machinery rather than create another recorder package, while keeping incoming and outgoing semantic ownership disjoint.

This GO accepts only the assessment and its bounded candidate for a later implementation proposal. It does not authorize implementation or any later ADR0011 stage.

## Closure of the four blocking defects

### B1 — workspace provenance classification: closed

The assessment now classifies workspace identity explicitly as `REUSE_WITH_EXTENSION`. It correctly observes that `OwnedMethodPair`, `OwnedMethodPairView`, `RequestWriteObservation`, `ResponseReadObservation`, `RoundTripRequest`, and `RoundTripResult` do not carry workspace identity.

It permits workspace provenance only through an exact manager-owned join to the managed workspace, session, and generation that owns the transaction. Session identity, caller assertion, payload content, profile labels, and adapter inference are explicitly insufficient. Missing, stale, foreign, or ambiguous workspace custody refuses before semantic effects.

This classification is consistent with the reviewed source and preserves the distinction between manager-local transaction identity and workspace identity.

### B2 — private B4 assembly seam in the write-set: closed

The candidate write-set now includes `sessionruntime/b4_definition_private.go` and bounds its role to minimal method-neutral generalization.

That inclusion matches the source-visible lifecycle owner. The file contains:

- atomic request-frame capture, retained, and caller reservations in `reservePrivateB4RequestFrame`;
- preflight, accounting construction, params custody, persistent reservation, request-frame reservation, and private transaction installation in `RoundTripPrivateB4`;
- exact shrinking and caller-charge transfer before constructing `privateB4RequestFrameOwner`;
- caller response-frame hold acquisition;
- zero-effect invalid-path release of owned-pair, notification, server-error, response, and reservation owners;
- terminal cleanup in `releasePrivateB4Locked`; and
- lifecycle joining and stopped-generation retirement in `privateB4RetirementJoin` and `retirePrivateB4StoppedLocked`.

The assessment also forbids importing definition or Call Hierarchy semantic interpretation into that shared seam. No duplicate owner abstraction or new recorder package is authorized.

### B3 — exact directional three-method bindings and arbitrary-method refusal RED: closed

The assessment now defines two separate closed bindings:

```text
incoming: textDocument/documentSymbol
       -> textDocument/prepareCallHierarchy
       -> callHierarchy/incomingCalls

outgoing: textDocument/documentSymbol
       -> textDocument/prepareCallHierarchy
       -> callHierarchy/outgoingCalls
```

The shared admission mechanism may be generic in a directional binding, but each binding admits exactly its own three methods. The opposite directional method, an arbitrary fourth method, and every other disallowed method must refuse at the package-private entry or eligibility seam before reservation, cumulative charge, protocol ownership, request-ID consumption, write attempt, callback, diagnostic, retention, or lease publication.

`DRR-R10` now requires every listed effect to remain exactly zero. `DRR-R11` independently requires each exact three-transaction chain. This is adequate RED-first closure of the prior under-specification.

Current source remains narrower and method-specific: `eligibleOwnedMethodRequest`, request/response frame capture, `VerifyMethodFrameCorrespondence`, and `RoundTripPrivateB4` do not yet admit these Call Hierarchy bindings. The assessment therefore correctly classifies this work as prospective `REUSE_WITH_EXTENSION`, not existing behavior.

### B4 — cross-transaction joins and semantic boundary: closed

For each direction, the assessment now requires a pre-effect join across exact:

- workspace;
- session;
- generation;
- directional profile;
- directional binding;
- documentSymbol predecessor identity, key, and ordinal;
- prepare predecessor identity, key, and ordinal;
- selected original subspan; and
- unchanged request-item handoff.

It explicitly refuses stale generation, cross-workspace substitution, same-session foreign prepare, swapped directional predecessor or binding, profile mismatch, and selected-span substitution before semantic effects.

The shared core may retain neutral transaction and predecessor/successor identities and spans, but it may not interpret them as Call Hierarchy semantics. Incoming and outgoing adapters alone own selection, caller/callee orientation, `fromRanges`, occurrences, profiles, evidence roles, review, and later GO authority. `DRR-R11` and `DRR-R13` preserve this boundary without treating generic identity linkage as semantic proof.

## Classification rereview

The corrected requirement table is accepted within its stated evidence ceiling:

- exact-generation exclusive transport ownership and conflict behavior: `REUSE_EXACT`;
- session/generation/key/method/write/read linkage: `REUSE_EXACT`, without silently including workspace;
- workspace identity and exact workspace/session/generation custody: `REUSE_WITH_EXTENSION`;
- defensive params/result ownership: `REUSE_EXACT` as payload custody only;
- exact original request and response frames for Call Hierarchy: `REUSE_WITH_EXTENSION`;
- frame-relative params/result spans: `REUSE_WITH_EXTENSION`;
- frame observations and digest links: `REUSE_EXACT`, while retained original bytes require extension;
- owner/lease closing, borrow exclusion, zeroing, exact-once release, and retirement joining: `REUSE_EXACT` lifecycle algebra;
- cancellation, timeout, interruption, worker join, and poisoned retirement: `REUSE_EXACT`, observational only;
- independent recorder accounting, transparency, and private default-off reachability: `REUSE_WITH_EXTENSION`;
- directional selection, handoff, orientation, `fromRanges`, occurrence construction, and profile authority: `INAPPLICABLE` to the shared core and `MISSING` until separately implemented and reviewed;
- CALLS truth, admission, publication, graph/grouping/Leiden/feature identity, Stage 3–7 roles, and production admission: nontransferable and not authorized; and
- complete method and lifecycle-branch coverage: `UNKNOWN` under the assessment's disclosed truncated structural census.

The original-wire distinction remains correct. `VerifyMethodFrameCorrespondence` reconstructs the request using `json.Marshal` and framing, so it cannot establish original outbound serialization. Copied `json.RawMessage`, decoded values, hashes, and normalized records cannot substitute for retained exact frames and frame-relative spans.

## Source and write-set rereview

The candidate source write-set is now internally consistent with the reuse claim:

- `sessionruntime/owned_method_pair.go`;
- `sessionruntime/b4_owned_method_pair_lease.go` when additional neutral custody fields require it;
- `sessionruntime/b4_request_frame_lease.go`;
- `sessionruntime/b4_response_frame_lease.go`;
- `sessionruntime/method_frame_correspondence.go`;
- `sessionruntime/sessionruntime.go`;
- `sessionruntime/b4_definition_private.go`; and
- only the named focused tests and at most one small scripted fixture per direction under existing `sessionruntime/testdata/`.

This set reaches the existing eligibility, transaction, owner construction, transfer, lease, correspondence, cleanup, and retirement seams without requiring a parallel recorder package. Directional adapter placement remains deliberately `UNKNOWN` and outside the named implementation write-set until a later review establishes a zero-overlap test-only composition seam.

Any need for CLI, MCP, public API, config/environment/build-tag enablement, production registry, capability catalog, schema, manifest, evidence copy, acquisition, publication/admission, graph normalization, semantic-vector, design mutation, or an unnamed source/test/fixture file invalidates this candidate and requires renewed assessment.

## RED rereview

`DRR-R01` through `DRR-R16` now cover the assessment-level RED boundary. In particular:

- original frame identity is distinguished from equal decoded/remarshaled meaning;
- raw spans must reproduce exact retained bytes;
- workspace must come from exact managed workspace/session/generation custody;
- independent exact-limit/plus-one accounting and no borrowing are required;
- active-borrow closing, clearing, and exact-once release remain mandatory;
- cancellation and protocol/transport terminals cannot be changed by recorder state;
- enabled/disabled production behavior must remain identical;
- panic, callback, allocation, hash, and exhaustion failures remain contained;
- production reachability remains statically closed;
- each directional binding is exactly three methods and all disallowed methods refuse with every enumerated pre-effect counter at zero;
- cross-transaction predecessor, profile, binding, workspace/session/generation, selected-span, and unchanged-handoff mismatches refuse before semantic effects;
- directional types and assemblers cannot cross-consume or construct the opposite direction;
- successful empty, null, missing, malformed, partial, transport, protocol, and cancellation states remain distinct; and
- the final census must prove no changed source, test, or fixture outside the approved set.

These are prospective RED requirements. This review does not claim they exist or pass, and it does not authorize writing them.

## Evidence ceiling

This rereview inspected the corrected assessment, preserved blocked review, exact HEAD, focused current source, and existing directional design/review material. A managed LSP session was derived for the exact workspace at generation 1, but no live structural result is promoted to source completeness, provider authentication, runtime proof, or implementation authority.

No tests were run because no implementation behavior is being accepted. No source completeness, CALLS truth, exact gopls profile authority, evidence-role satisfaction, production reachability proof, directional qualification, Stage 3–7 progression, or production admission is inferred.

## Gate

The corrected assessment is accepted under the exact disposition:

```text
ADR0011_CALLHIERARCHY_DR_RECORDER_REUSE_GO
```

This token authorizes only treating the corrected assessment as the bounded basis for a separately reviewed implementation proposal within its candidate write-set and exclusions. It grants no implementation authority, no evidence or design mutation authority, no test-writing authority, no qualification or production authority, and no commit authority.
