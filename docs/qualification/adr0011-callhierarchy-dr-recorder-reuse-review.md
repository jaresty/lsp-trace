# ADR 0011 Call Hierarchy D/R recorder reuse review

- **Repository:** `/Users/schwa/dev/lsp-trace-adr0011-c17`
- **Reviewed HEAD:** `8210109f3f71fd969fe3201935014d4226c90c13`
- **Reviewed assessment:** `docs/qualification/adr0011-callhierarchy-dr-recorder-reuse-assessment.md`
- **Assessment SHA-256:** `c80445ceeb0d190e0393e8096a97ecfa70472a43ee1ee6ea07a507f6fb94b6bf`
- **Review authority:** assessment acceptance only; no implementation, evidence acquisition, design mutation, qualification, production admission, or commit authority
- **Verdict:** `ADR0011_CALLHIERARCHY_DR_RECORDER_REUSE_BLOCKED`

## Decision

The architectural direction is correct: the smallest shared core is an extension of the existing private `sessionruntime` manager/owner/lease/transaction machinery, not a duplicate recorder package. Incoming and outgoing adapters must retain separate semantic authority, selection policy, occurrence construction, limits, evidence roles, review chains, and GO tokens.

The exact assessment cannot be accepted yet because its candidate write-set and provenance classification omit required existing seams and identity facts. These are assessment defects, not implementation defects.

## Exact blocking defects

### B1 — workspace identity is missing from the classified shared provenance

The requested shared identity is method/key/session/generation/workspace. The assessment classifies “Session, generation, request key, method, write/read linkage” as `REUSE_EXACT` and lists transferable session/generation/key/method facts, but it never classifies workspace identity.

`OwnedMethodPair`, `OwnedMethodPairView`, `RequestWriteObservation`, `ResponseReadObservation`, `RoundTripRequest`, and `RoundTripResult` carry no workspace identifier. Session routing may be available elsewhere in manager state, but no inspected recorder/lease value establishes a workspace join, and session identity alone must not be silently promoted to workspace identity.

Required correction: add an explicit workspace requirement row. On this source it is `MISSING` for the shared retained view unless a separately exact, generation-bound manager join is named and tested; if workspace binding belongs to each directional target envelope instead, classify it `INAPPLICABLE` to the core and state the mandatory adapter join. Do not leave it implicit.

### B2 — the candidate write-set omits the existing assembly seam required by its own lifecycle claim

The assessment allows edits to `owned_method_pair.go`, the three lease/correspondence files, `sessionruntime.go`, and focused tests. It omits `sessionruntime/b4_definition_private.go`.

That omitted file currently owns essential parts of the claimed reusable lifecycle:

- `reservePrivateB4RequestFrame` reserves capture, retained, and caller request-frame charges atomically;
- `RoundTripPrivateB4` performs preflight, account creation, params copying, persistent reservations, request-frame reservation, transaction installation, and post-return validation;
- after exact shrinking, `requestFrameCaller.transfer()` constructs `privateB4RequestFrameOwner` and suppresses the defensive non-lease copy;
- it acquires the caller response-frame hold;
- invalid paths release pair/notification/error/response owners and the reservation;
- `releasePrivateB4Locked`, `privateB4RetirementJoin`, and `retirePrivateB4StoppedLocked` provide terminal release and lifecycle joining.

Therefore the proposed write-set cannot both reuse the existing B4 lease/accounting algebra and create Call Hierarchy transferable frame custody without either editing this file, duplicating its assembly elsewhere, or weakening the lifecycle. The latter two contradict the assessment.

Required correction: include the minimum existing private transaction assembly file(s), especially `sessionruntime/b4_definition_private.go`, or replace the proposal with a precisely named existing method-neutral entry seam that demonstrably constructs the same owners, reservations, transfers, refusal ordering, and retirement joins. Any broader inclusion must remain private and must not import definition semantics into the method-neutral core.

### B3 — arbitrary-method admission and the three-transaction private entry contract are under-specified

Current source is closed by method:

- `eligibleOwnedMethodRequest` admits definition/references/documentSymbol only;
- request-frame capture in `roundTripWithPrivateTransaction` admits definition/references plus a separate documentSymbol path;
- response capture switches over definition/references/documentSymbol;
- `VerifyMethodFrameCorrespondence` admits definition/references only;
- `RoundTripPrivateB4` admits definition only;
- ordinary `RoundTrip` has no Call Hierarchy private recorder entry.

The assessment correctly marks Call Hierarchy frame custody `REUSE_WITH_EXTENSION`, but it does not name the private method-neutral enablement/entry contract that permits exactly prepare/incoming/outgoing while rejecting arbitrary methods before any reservation, write, callback, diagnostic, or lease effect. DRR-R09 checks public reachability, but no RED explicitly proves the closed three-method allowlist, rejection of a fourth arbitrary method, and zero-effect refusal at the actual private entry seam.

Required correction: add a RED that enables the private recorder for exactly the three Call Hierarchy methods, rejects an arbitrary fourth method before effect, and proves no reservation/cumulative charge, protocol ownership, request ID consumption, write attempt, callback, diagnostic, or lease publication. Name the package-private entry or eligibility owner in the candidate write-set.

### B4 — cross-transaction sequence and workspace joins are not fully covered by DRR-R03

DRR-R03 tests substitutions within session/generation/key/method/write/read/owner sequence and request/response ordinals. The target behavior is a prepare transaction followed by a separately keyed incoming or outgoing transaction. Existing manager keys identify individual transactions; they do not by themselves prove that a directional second transaction consumed the selected prepare item from the intended predecessor in the same workspace and generation.

Required correction: keep this out of shared CALLS semantics, but add adapter REDs that independently bind prepare key/ordinal to directional key/ordinal, workspace, session, generation, selected original subspan, and unchanged request-item handoff. Reject swapped incoming/outgoing predecessors, same-session foreign prepare results, and cross-workspace substitutions. The shared core may expose neutral identities/spans; the directional adapter retains authority for the join.

## Classification review

### Accepted classifications

- Exclusive generation transport ownership and conflict behavior: `REUSE_EXACT`.
- Defensive params/result ownership: `REUSE_EXACT` as payload custody only.
- Original request and response frames: `REUSE_WITH_EXTENSION`; current eligibility and assembly are method-specific.
- Raw params/result spans: `REUSE_WITH_EXTENSION`; copied `json.RawMessage` is not a frame-relative span.
- Frame lengths/digests: `REUSE_EXACT` for observations and `REUSE_WITH_EXTENSION` for retained bytes.
- Active-borrow exclusion, closing, zeroing, exact-once charge release, and retirement joins: `REUSE_EXACT` as lifecycle algebra.
- Cancellation, timeout, short-write/read interruption, worker join, and poisoned retirement: `REUSE_EXACT`; recorder behavior must remain observational.
- Recorder accounting, semantic transparency, and private default-off enablement: `REUSE_WITH_EXTENSION`.
- Prepare selection, selected ordinal/span, and unchanged handoff: `INAPPLICABLE` to the core and currently `MISSING` until directional adapters exist.
- Incoming/outgoing meaning, `fromRanges`, occurrence multiplicity, CALLS authority, admission, graph, grouping, Leiden, feature identity, Stage 3–7 roles, and production admission: correctly nontransferable (`INAPPLICABLE` or `MISSING` as stated).
- Complete method/branch coverage: `UNKNOWN`; truncated structural output cannot establish absence or completeness.

### Required classification correction

The session/generation/key/method row is acceptable only for the identities it names. It must not be read as covering workspace. Workspace requires the explicit B1 classification.

## Original-wire versus decoded review

The distinction is correct and must remain normative:

- `framedWriteCapture.retainedFrame` can preserve bytes actually reported written, including framing, only after complete bounded capture;
- private successor original-frame custody can preserve the accepted response frame;
- `OwnedMethodPair.Params` and `.Result` are copied payload values, not complete frame custody;
- `VerifyMethodFrameCorrespondence` reconstructs the request through `json.Marshal` and `lspwire.Writer`, so it proves local correspondence to canonical reconstruction, not original outbound serialization;
- decoded `lspwire.Message` and typed Call Hierarchy values cannot prove whitespace, member order, duplicate-key history, or framing;
- hashes link bytes but never substitute for retained bytes.

Missing or partial original-frame custody must remain explicit and must never be reconstructed and relabeled as original.

## Terminals, limits, refusal, and accounting

The existing tests support reuse of:

- stale-generation and lifecycle-conflict refusal;
- short-write withholding;
- timeout/cancellation retirement and read join;
- active-callback drain and post-close borrow refusal;
- exact-once release under copied handles and races;
- byte clearing and charge release;
- atomic multi-reservation refusal and exact-limit/plus-one behavior;
- panic isolation for private test hooks.

The assessment’s terminal separation (`null`, `[]`, missing, malformed complete, partial capture, transport/protocol failure, cancellation) is appropriate, but those are prospective Call Hierarchy custody states. Existing D/R/B4 tests do not establish every state for all three methods.

DRR-R04 is directionally adequate but must identify which limits belong to the shared recorder versus directional adapters. Shared accounting must not borrow directional occurrence budgets, production accounting, or semantic work budgets. Refusal-before-effect must include the B3 zero-effect assertions at the actual private entry seam.

## Private enablement and production unreachability

Current enablement fields and hooks are package/private or `json:"-"`, and the inspected command tree contains no references to the private capture switches. This supports a private, default-off design. It is not a permanent proof of repository-wide unreachability.

DRR-R09 and DRR-R14 must remain static census tests and must reject CLI, MCP, config, environment, build-tag, registry, public constructor/API, production import, capability catalog, schema/manifest/evidence, graph normalization, publication/admission, and semantic-vector changes.

## Semantic transparency and directional authority

The recorder may retain only transaction custody and lifecycle facts. Enabled versus disabled operation must preserve the same production `RoundTripResult`, error, ordering, cancellation, diagnostics, manager lifecycle, and production accounting. Recorder panic, callback failure, exhaustion, allocation/hash refusal, or missing custody must not retry, suppress, replace, or reclassify the managed transaction.

Incoming and outgoing adapters remain separate semantic owners. A shared direction switch, union profile enum, shared CALLS occurrence assembler, reversed-edge inference, shared evidence/review type, or cross-direction GO token is forbidden. Equal retained bytes do not transfer semantic authority.

## Candidate write-set and RED assessment

The candidate remains correctly closed against public/CLI/MCP/catalog/schema/manifest/evidence/semantic-vector/production work, but it is not adequate until B1–B4 are corrected.

No new recorder package is justified. The corrected smallest core remains an extension of existing private `sessionruntime` machinery, including the minimum existing assembly seam needed to construct and retire the current owners without duplication.

## Evidence ceiling

This review used exact source and focused tests at the stated HEAD. It did not execute tests or claim runtime behavior. Repository and structural searches that truncated are treated as uncertainty. No source completeness, provider authentication, workspace identity, production reachability proof, CALLS truth, directional qualification, or Stage 3–7 authority is inferred.

## Gate

`ADR0011_CALLHIERARCHY_DR_RECORDER_REUSE_GO` is not issued.

The exact disposition is:

```text
ADR0011_CALLHIERARCHY_DR_RECORDER_REUSE_BLOCKED
```

Unblock only by revising the assessment to classify workspace explicitly, include or precisely replace the existing private B4 assembly seam in the candidate write-set, and add the arbitrary-method zero-effect plus cross-transaction/workspace REDs. This review grants no implementation authority beyond acceptance of a corrected assessment.
