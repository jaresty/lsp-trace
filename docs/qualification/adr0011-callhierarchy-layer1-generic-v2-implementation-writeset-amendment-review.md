# ADR-0011 Call-Hierarchy Layer-1 Generic V2 Implementation Write-Set Amendment Review

Status: **BLOCKED**

- Repository: `/Users/schwa/dev/lsp-trace-adr0011-c17`
- Reviewed HEAD: `c321a96e5ce6ffea8f8550ae0d8d9695cece53c8`
- Reviewed amendment: `docs/qualification/adr0011-callhierarchy-layer1-generic-v2-implementation-writeset-amendment.md`
- Review mutation: this document only
- Writer authority: **not issued**
- Commit authority: **not issued**

## Independent mechanical findings

### Qualified RED and immutable inputs

The retained test and corpus are unchanged from reviewed HEAD. The normal and race commands were run from the exact worktree:

```sh
go test ./sessionruntime -run '^TestCallHierarchyLayer1GenericV2CorpusCausalRED$' -count=1 -v
go test -race ./sessionruntime -run '^TestCallHierarchyLayer1GenericV2CorpusCausalRED$' -count=1 -v
```

Both reached the qualified causal RED shape:

```text
ASSERT_CH_GENERIC_V2_EXECUTION_COUNTERS roots=25 workers=16 executions=22 pair_members=22 total=85
ASSERT_CH_GENERIC_V2_ATTACHMENT_PREREQUISITE contract=RoundTripResult.privateCallHierarchyLayer1GenericLease/View reason=field:callHierarchyLayer1Generic
```

The RED therefore executes the complete 85-record path before stopping at the exact missing attachment. The implementation writer must not modify `*_test.go`, corpus, fixture, testdata, manifest, digest, sidecar, generated, C15, or documentation inputs.

### Exact symbols and minimum physical write-set

The exact three existing production files are mechanically sufficient as a physical ownership boundary:

1. `sessionruntime/sessionruntime.go`
   - owns `type RoundTripResult struct`, `(*Manager).RoundTrip`, existing private round-trip plumbing, and `(*Manager).finishRoundTrip`;
   - may own only the exact private `callHierarchyLayer1Generic` result attachment and minimum generic attachment transfer/release plumbing;
   - must not own generic view/lifecycle behavior or call-hierarchy semantics.
2. `sessionruntime/owned_method_pair.go`
   - owns the existing private generic identity, frame, terminal, limits, accounting, observation, view, constructors, copy accessors, and `(*Manager).privateCallHierarchyRequestIdentity` declarations;
   - may construct transport-only observations without parsing or promoting payload meaning.
3. `sessionruntime/b4_owned_method_pair_lease.go`
   - already owns adjacent mutex/condition-variable owner, view, callback, terminal-close, drain, and release patterns;
   - may own the unexported generic-v2 owner/view/lease and exact methods `privateCallHierarchyLayer1GenericLease`, `View`, `Callback`, and `Release`.

No fourth production file is demonstrated by the retained RED or declarations. If implementation requires one, the hard-stop result is:

```text
BLOCKED — exact missing symbol and owning file required; write-set expansion not authorized
```

### Exact retained contract

The immutable test discovers these names and signatures:

- field `RoundTripResult.callHierarchyLayer1Generic`;
- `privateCallHierarchyLayer1GenericLease() (lease, bool)`;
- `View() (view, bool)`;
- `Callback(func(view) error) error`;
- `Release() bool`.

After acquisition, it requires all 145 registered expected paths to compare with zero skips/errors, defensive-copy behavior, callback success/error/panic containment, one sequential and one concurrent release winner, terminal reacquisition refusal, and callback drain before release completion.

### Forbidden semantics and hard stop

The proposed semantic ceiling is otherwise correct: no direction, target, occurrence, grouping, relation, selected-item meaning, handoff meaning, method-family meaning, token-form meaning, payload parsing/classification, qualification, publication, authority, public API, CLI/MCP/schema/configuration/activation, persistence, replay, telemetry, diagnostic, transport retry, or `internal/lspwire` ownership is admitted. Existing `callHierarchyLayer1`, `callHierarchyObservation`, and exported owned-method-pair contracts may not be repurposed.

A failing immutable oracle, unavailable exact frame/result/accounting/lifecycle input, extra file, or semantic requirement is review evidence and an immediate hard stop, never authority to repair tests, broaden files, or improvise behavior.

## Blocking defects

### 1. C15-before-writer ordering is internally contradictory

The amendment says canonical C15 enumeration may occur only **after a compiling, focused implementation exists** (`C15 boundary`), but later says the implementation gate must name the single writer with exclusive custody **after C15** (`Gate and writer custody`). A production implementation cannot exist before any production writer receives custody while also requiring C15 to finish before that custody.

The reviewed predecessor C15 plan already provides the non-circular sequence: an authorized disposable isolated three-file prototype, canonical C15 enumeration, finite delta classification and independent review, disposal, then a later production implementation gate. The amendment must state that sequence explicitly and must not authorize a production writer or production-file mutation before the disposable C15 successor decision is complete.

### 2. The `owned_method_pair.go` symbol ceiling is broadened without C15 evidence

The reviewed C15 experiment boundary permits exactly eight existing inert symbols in `owned_method_pair.go`. The amendment instead permits implementing or extending generic types and constructors for the complete 145-field view and adding an additional transport-only constructor. That may be necessary, but it is not the same finite symbol boundary and no completed C15 delta evidence in this review admits the expansion.

The amendment must either:

- bind production work to an independently reviewed finite post-experiment symbol/candidate set; or
- retain the exact predecessor eight-symbol ceiling and hard-stop when the RED proves another symbol necessary.

It may not grant an open-ended “any additional unexported generic transport-only constructor” allowance before C15 enumeration and review.

## Required correction

Before GO, revise and independently review the amendment so that it:

1. orders the already-authorized disposable C15 prototype/enumeration and finite delta review before naming the production writer or granting production custody;
2. binds `owned_method_pair.go` to the exact independently reviewed finite symbol/candidate set produced by that experiment;
3. preserves the exact three-file production boundary, generic-only `sessionruntime.go` plumbing, owner/view/lease ownership, forbidden-semantic ceiling, immutable tests/corpus/C15 artifacts, and hard-stop language; and
4. names exactly one production writer only after those prerequisites pass.

## Disposition

`ADR0011_CALLHIERARCHY_LAYER1_GENERIC_V2_IMPLEMENTATION_WRITESET_BLOCKED`

No implementation GO, writer authority, commit authority, test mutation, C15 artifact mutation, or production-file mutation is issued.
