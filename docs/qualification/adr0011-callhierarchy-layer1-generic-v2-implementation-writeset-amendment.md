# ADR-0011 Call-Hierarchy Layer-1 Generic V2 Implementation Write-Set Amendment

Status: **REVIEW_GATE_PENDING / NO GO**

- Repository: `/Users/schwa/dev/lsp-trace-adr0011-c17`
- Pinned HEAD: `c321a96e5ce6ffea8f8550ae0d8d9695cece53c8`
- Required starting state: clean worktree at the pinned HEAD
- Qualified RED and disposable blocker evidence: `/tmp/adr0011-layer1-generic-v2-c15-impact-v1`
- Authority: review of a finite disposable experiment boundary only
- Disposable prototype writer authority: **not issued by this document**
- Production writer authority: **not issued**
- Commit authority: **not issued**

## Decision and required ordering

This amendment remains review-pending and issues no GO. A review of this amendment may authorize exactly one disposable isolated prototype writer, solely to obtain a compiling RED and enumerate canonical C15. That authorization, if issued, is limited to exact custody of these three existing files in a newly created disposable worktree:

1. `sessionruntime/sessionruntime.go`
2. `sessionruntime/owned_method_pair.go`
3. `sessionruntime/b4_owned_method_pair_lease.go`

The disposable prototype is evidence tooling, not production implementation. It must not be committed. After the canonical C15 enumeration and evidence preservation, all prototype changes and the disposable worktree must be removed.

The mandatory sequence is:

1. a separate review may issue disposable experiment GO and name exactly one disposable prototype writer with exclusive custody of the exact three files above;
2. that writer may make only the finite changes stated below, only far enough to obtain a compiling RED and run the canonical C15 enumerator;
3. the exact prototype symbols, canonical candidates, and complete candidate/classification delta must be independently reviewed as finite sets;
4. the prototype and disposable worktree must be disposed of with no production commit; and
5. only after that independent review may a separate production gate freeze the final finite production file/symbol/candidate write-set and name exactly one production writer.

No production writer, production custody, production mutation, implementation GO, or commit authority exists before step 5. The disposable prototype writer is not the production writer and receives no presumptive production authority.

The retained blocked review remains preserved and authoritative as the reason for this correction:

- `docs/qualification/adr0011-callhierarchy-layer1-generic-v2-implementation-writeset-amendment-review.md`
- disposition: `ADR0011_CALLHIERARCHY_LAYER1_GENERIC_V2_IMPLEMENTATION_WRITESET_BLOCKED`

## Preserved qualified RED and immutable inputs

The retained normal and race commands executed all 85 required records and stopped at the same prerequisite:

```text
ASSERT_CH_GENERIC_V2_EXECUTION_COUNTERS roots=25 workers=16 executions=22 pair_members=22 total=85
ASSERT_CH_GENERIC_V2_ATTACHMENT_PREREQUISITE contract=RoundTripResult.privateCallHierarchyLayer1GenericLease/View reason=field:callHierarchyLayer1Generic
```

The retained RED discovers the exact contract names:

- field: `RoundTripResult.callHierarchyLayer1Generic`;
- acquisition method: `privateCallHierarchyLayer1GenericLease() (lease, bool)`;
- lease methods: `View() (view, bool)`, `Callback(func(view) error) error`, and `Release() bool`.

The review, retained RED, tests, corpus, fixtures, testdata, manifests, digests, sidecars, generated artifacts, historical C15 artifacts, and all other documentation are immutable during the disposable experiment. A failing immutable oracle is evidence for review, never permission to repair or weaken it.

## Exact disposable prototype boundary

### `sessionruntime/owned_method_pair.go`

The disposable prototype may implement exactly these eight existing inert symbols from the accepted predecessor experiment plan and review, and no others:

1. `(*Manager).privateCallHierarchyRequestIdentity`
2. `newCallHierarchyObservationLimits`
3. `callHierarchyObservationAccounting.validAgainst`
4. `callHierarchyObservationTerminal.valid`
5. `newCallHierarchyOriginalRequestFrameObservation`
6. `callHierarchyOriginalRequestFrameObservation.copyFrameBytes`
7. `newCallHierarchyOriginalResponseFrameObservation`
8. `callHierarchyOriginalResponseFrameObservation.copyFrameBytes`

No additional constructor, type, method, accessor, helper, or declaration is authorized in this file. Existing directional, selected-preimage, handoff-preimage, predecessor, allowlist, token-form, params/result subspan, occurrence, relation, and semantic symbols remain inert.

If compiling RED requires a ninth `owned_method_pair.go` symbol, the disposable experiment must hard-stop before canonical C15 enumeration and report:

```text
BLOCKED — owned_method_pair.go ninth symbol required; write-set amendment and independent review required before C15
```

### `sessionruntime/sessionruntime.go`

Disposable changes are limited to these exact attachment/plumbing seams:

1. add only the private field `callHierarchyLayer1Generic` to `type RoundTripResult struct`, typed as `*callHierarchyLayer1GenericOwner`;
2. in `(*Manager).RoundTrip`, receive or construct that completed owner and assign it to the returned `RoundTripResult`;
3. only if direct assignment in `(*Manager).RoundTrip` cannot compile, thread that same owner through an already-existing private round-trip terminal return value and `(*Manager).finishRoundTrip`; and
4. on every existing early or terminal branch reached after owner creation, either transfer the owner into the returned result or terminally release it exactly once.

No other field, type, constructor, transaction path, transport behavior, result/error behavior, accounting behavior, diagnostic, retry, serialization, parser, hash, public API, exported identifier, feature flag, CLI/MCP behavior, or `internal/lspwire` change is authorized. The existing `callHierarchyLayer1` and `callHierarchyObservation` attachments must not be repurposed.

### `sessionruntime/b4_owned_method_pair_lease.go`

The disposable prototype may add or implement exactly this finite owner/view/lease symbol list required by the retained RED:

1. `type callHierarchyLayer1GenericOwner struct`
2. `type callHierarchyLayer1GenericView struct`
3. `type callHierarchyLayer1GenericLease struct`
4. `newCallHierarchyLayer1GenericOwner`
5. `(RoundTripResult).privateCallHierarchyLayer1GenericLease`
6. `(callHierarchyLayer1GenericLease).View`
7. `(callHierarchyLayer1GenericLease).Callback`
8. `(callHierarchyLayer1GenericLease).Release`
9. `(*callHierarchyLayer1GenericOwner).requestTerminal`
10. `(*callHierarchyLayer1GenericOwner).waitReleased`

No additional owner, view, lease, constructor, method, closure, drain helper, lifecycle helper, or alternate attachment type is authorized. These symbols may only reuse the adjacent mutex/condition-variable lifecycle pattern to provide immutable projection, defensive copies, acquisition refusal, callback admission and drain, panic containment, transparent callback errors, terminal closure, one release winner, and fail-closed subsequent operations.

If compiling RED requires any additional symbol in this file, the experiment must stop and report the exact missing symbol before canonical C15 enumeration.

## Semantic ceiling and cross-file invariants

The disposable prototype and any later production proposal must preserve:

1. **Default-off transparency:** absent or inadmissible generic input produces no attachment and no observable transport/result/accounting change.
2. **Refusal before effect:** invalid input is refused before allocation, reservation, lease publication, callback, diagnostic, transport, or accounting effect.
3. **Exact external behavior:** enabled and disabled executions retain identical externally returned result/error, failure, messages, bytes, terminal behavior, and timing semantics within the frozen contract.
4. **Defensive ownership:** no caller-owned mutable byte slice aliases retained state or a repeated view/callback result.
5. **Lifecycle closure:** terminal closure prevents new callbacks, admitted callbacks drain, exactly one release succeeds, and later acquisition/view/callback/release attempts fail closed.
6. **No semantic promotion:** generic transport retention establishes no direction, target, occurrence, grouping, relation, selected-item meaning, handoff meaning, method-family meaning, token-form meaning, qualification, publication, authority, completeness, or production use.
7. **No parallel machinery:** reuse the existing Manager transaction, private accounting/reservation primitives, and lifecycle patterns.

The prototype must not parse or classify payload meaning; retain selected-item or handoff preimage bytes; introduce public/exported APIs, persistence, replay, catalog behavior, logging, telemetry, configuration, activation, CLI/MCP/schema behavior, or diagnostics; mutate tests/corpus/C15 artifacts; or classify, bind, suppress, rename, inline, relocate, or otherwise evade C15 candidates.

## Canonical C15 experiment and independent review

Canonical C15 enumeration is an experiment step after compiling RED, not “during implementation.” It may occur only under the accepted predecessor experiment-plan constraints, including its temporary enumerator boundary, canonical helpers, ordering, external evidence, cleanup, and no-commit rules.

The disposable writer must preserve and report, without changing retained artifacts:

- the exact three-file prototype patch and its path/length/SHA-256 accounting;
- the exact implemented symbol set;
- canonical baseline and generated candidate/classification sets;
- the complete finite `ADDED`, `REMOVED`, and `CHANGED` delta by candidate ID;
- source locations, AST hashes, and pre/post classifications for every affected candidate; and
- any blocked or ambiguous disposition.

An independent review must accept or reject the finite symbol set, candidate set, and complete delta. The experiment cannot itself classify or bind rows, mutate retained C15 artifacts, authorize a successor, or authorize production implementation.

Any delta beyond the independently reviewed finite set, any inaccessible required input, any semantic requirement, any fourth source file, or any extra symbol is an immediate hard stop. Record the exact missing symbol and owning file; do not improvise a workaround.

## Final production gate

Only after the disposable prototype has been removed and the finite symbols/candidates/delta have passed independent review may a separate gate:

1. freeze the exact final production file, symbol, and candidate write-set;
2. preserve the immutable tests, corpus, review, source inputs, and C15 history;
3. name exactly one production writer with exclusive custody of that frozen production write-set; and
4. state a separate production verification and commit disposition.

This amendment does not predetermine that final symbol set, does not authorize reuse of the disposable patch, and does not name a production writer.

Until that separate gate is issued:

```text
ADR0011_CALLHIERARCHY_LAYER1_GENERIC_V2_IMPLEMENTATION_WRITESET_GO — NOT ISSUED
ADR0011_CALLHIERARCHY_LAYER1_GENERIC_V2_PRODUCTION_WRITER — NOT NAMED
```

Disposition: **REVIEW_GATE_PENDING / NO GO**
