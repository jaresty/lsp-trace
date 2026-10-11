# ADR-0011 Call Hierarchy Layer-1 RED Oracle Corpus v1 — Independent Final Strict Rereview v3

**Verdict: `ADR0011_CALLHIERARCHY_LAYER1_ORACLE_CORPUS_DESIGN_BLOCKED`**

Reviewed exact repository HEAD `4f2b5504c1e4f4e559f48db7ab8e0c5b8ca70fa9`, the corrected untracked design `docs/qualification/adr0011-callhierarchy-layer1-red-oracle-corpus-v1.md`, and the two prior blocked reviews. The worktree already contained the unchanged 454-insertion RED modification and those three untracked documents before this review. This review creates only this v3 review and does not modify those inputs.

This verdict authorizes no corpus bytes, loader, RED edit, source edit, testdata, implementation, generated/runtime output, or commit.

## Strict decision boundary

GO requires the design itself to determine one complete canonical corpus byte sequence: two independent authors must be able to materialize every registry constant, recursively resolve every worksheet reference, validate every nested value, and emit all 25 world files and the downstream manifest without inventing a value, selecting among admissible encodings, consulting runtime behavior, or relying on a promised future materialization step.

The corrected design is substantially improved: §7 supplies ordered closed object shapes; §10 supplies the intended disabled composite; §11 supplies an exact world-16 comparison object; and the Layer-1 opaque, source-pin, request-scoped-ledger, and no-runtime-authority boundaries are explicit. It nevertheless does not satisfy the complete-byte criterion because the normative registry is not closed and several recursive semantic forms remain non-canonical.

## Mechanical reference-resolution simulation

The simulation applies the design's own rule: a worksheet contains one `{"$ref":"const:NAME"}`; `NAME` must resolve exactly once in the registry defined as the fenced `NAME := <canonical JSON or closed expression>` definitions in §§8–11; expressions may use only pure constructors and earlier entries; all references and constructor tokens must disappear before schema validation and canonical encoding.

### Representative success: world 01

1. `WORLD_01_INCOMING_SUCCESS_EMPTY_ARRAY` is named in §11 but has no `WORLD_01_INCOMING_SUCCESS_EMPTY_ARRAY := ...` registry definition.
2. Its table row references `IDENTITY_101_INCOMING`, `FRAMES_101_EMPTY`, `EXTERNAL_101_EMPTY`, `TERM_SUCCESSFUL_EMPTY`, `ACCOUNT_101`, `TRANSPORT_101`, and `LED_101`; none has a normative `NAME := ...` definition.
3. `RESULT_EMPTY_ARRAY`, `ERR_NONE`, `COMMITMENTS_OK`, `CONTROL_ON`, `REFUSAL_NO`, and `LIFECYCLE_COMPLETE` do resolve.

**Result:** resolution stops at the root and, even if the table is treated as an implicit root constructor, stops again at seven undefined names. Exact world bytes are not derivable.

### Representative refusal: world 17

1. `WORLD_17_REFUSAL_SELECTED_UNAVAILABLE` has no registry definition.
2. The combined row 17–23 refers to ranges—`IDENTITY_117`…`IDENTITY_123`, `FRAMES_117`…`FRAMES_123`, `TRANSPORT_117`…`123`, `LED_117`…`123`, `CONTROL_REFUSAL_117`…`123`, and `REFUSAL_117`…`123`—rather than seven actual definitions.
3. The prose requires separately materialized constants and forbids range notation, but requiring future materialization does not supply the missing values now.
4. The row says “unique external success” without naming seven exact external constants.

**Result:** the refusal root and its distinguishing record do not resolve. An author must invent registry definitions and exact external values.

### Disabled control: execution 302

1. §10 gives a total field list parameterized by name, paired-world name, `RID_n`, and `TX_n`.
2. It says those inputs “are materialized in each `CONTROL_DISABLED_302`, `304`, …, `322` registry constant,” but no such `NAME := ...` definitions exist.
3. World 25 refers only to `CONTROL_MATRIX_22`, also without a definition containing the eleven literal pairs.

**Result:** the intended disabled state is understandable and fixes every listed nested field, but the normative reference graph has no target for execution 302 or the enclosing matrix. The design cannot distinguish byte-for-byte between a correctly materialized record and an author-created materialization because it supplies neither canonical bytes nor a closed constructor invocation.

### World 16 comparison

The exact comparison literal is authorable. Its RFC-6901 paths resolve in the expanded root:

- `/comparison/left/transaction/local_hex` versus `/comparison/right/transaction/local_hex`;
- selected identity and SHA-256 on left versus right;
- handoff identity and SHA-256 on left versus right.

All five `not-equal` predicates are true from the displayed literals, and the containing `Comparison` key order matches §7. The comparison introduces no method, direction, decoded payload, source frame, or semantic label into the opaque commitment objects.

However, `WORLD_16_PREPARE_OUTGOING_SEPARATION`, `IDENTITY_116_PREPARE`, `FRAMES_116_PREPARE_EMPTY`, `EXTERNAL_116_EMPTY`, `TERM_SUCCESSFUL_EMPTY`, `ACCOUNT_116`, `TRANSPORT_116`, and `LED_116` are undefined registry names. The exact comparison subtree therefore does not make the complete world authorable.

### Concurrency worker 201

1. `CONCURRENCY_16` and `WORKER_201` are required names but have no registry definitions.
2. The prose fixes broad behavior—outgoing method, request/empty-response frames, empty-array result, available empty external arrays, complete lifecycle, opaque commitments, and a before-zero ledger—but does not name the exact worker identity, frames, external, accounting, transport, terminal, or ledger constants.
3. “Aggregate ledger is the field-wise sum already specified in the prior verified arithmetic” is not a closed expression over sixteen named literal worker ledgers in this design.

**Result:** worker 201 and the aggregate cannot be recursively resolved. The author must choose missing constants and arithmetic operands.

## Exact blocking defects

### D1 — The normative constant registry is incomplete

Sections 8–11 define the registry exclusively by fenced `NAME := ...` definitions, yet the world roots and most table-selected constants have no definitions. This affects all 25 roots, not an isolated example. Statements that constants “must materialize,” “are materialized,” or “names a unique materialized constant” specify a future operation but do not define its output.

At minimum, undefined families include world roots; most `IDENTITY_*`, `FRAMES_*`, `EXTERNAL_*`, `TERM_*`, `ACCOUNT_*`, `TRANSPORT_*`, and `LED_*` names; refusal controls/reasons; all workers; the concurrency aggregate; enabled/disabled controls; and the control matrix. Consequently the validator's exactly-one-target rule fails before recursive schema validation.

**Required correction:** supply every referenced constant as one unique normative `NAME := <canonical JSON or closed expression>` definition, in dependency order, or supply an equally exact closed indexed constructor grammar that is explicitly admitted by the registry rules and fixes every argument. Requiring a later builder to materialize values is insufficient.

### D2 — Several permitted constructors are not total byte-producing functions

`ACCOUNT_FAILURE(req,res?)` branches on the prose condition `res fixed?` without defining the argument type or the two complete invocation forms. `TRANSPORT(reqFrames,responseFrames)` describes sums “as Measure” but does not formally select `MEASURE_Z` versus `MEASURE_P(n)` for every zero/nonzero sum. The concurrency aggregate refers to “prior verified arithmetic” rather than a closed expression over named operands. These leave author choices even if all missing names were added mechanically.

**Required correction:** define total constructors with typed arguments and one exact output for every admitted input, then provide every invocation as a named registry definition. Aggregate values must be closed expressions over the exact sixteen worker constants in this document.

### D3 — The recursive schema is closed structurally but not canonical semantically

The `object(...)` notation fixes key order, requiredness, types, and no extras. It does not supply complete cross-field predicates for every composite/tagged form:

- `Error` permits contradictory combinations of `available`, `present`, `class`, code/message/data fields, and `serialized_hex`; the named rows cover some values but the schema itself does not restrict the selected form.
- `Lifecycle` does not require its booleans to agree with `sequence` or constrain which sequences are valid.
- `Control` does not state the complete enabled/disabled cross-field relation; §10 fixes disabled records, but enabled records are described by a future clone materialization.
- Available `External` values are constrained partly by prose rather than by an exhaustive canonical validity relation.

Thus “complete §7 schema and all cross-field predicates” cannot be implemented solely from §7, and different validators can admit different trees.

**Required correction:** add exhaustive formal cross-field validity predicates for every availability/tagged/composite type, including `Error`, `Lifecycle`, `Control`, and `External`, and bind every world constant to one valid row.

### D4 — The 25-world worksheet is a selection table, not 25 deterministically expandable worksheets

Every world file is said to reference a root constant, but no root constant is defined. The table contains combined ranges (17–23), broad descriptors (“unique external success”), replacement prose for worlds 24–25, and promises of complete materialization rather than closed values. Therefore all 25 worksheet files are syntactically trivial but semantically unresolved.

**Required correction:** make each of the 25 root references resolve through the actual registry to a complete literal `World`, with no range, implied clone, prose-selected value, or absent target. Demonstrate resolution for all 25 names, not only representative primitives.

### D5 — World 24 and world 25 containers are promised but absent

The design gives exact cardinalities and order for 16 workers and 11 control pairs/22 executions. It does not provide the complete named worker records, pair records, aggregate ledger, or matrix constant. For enabled controls, “complete clone” remains an instruction whose output is not present. For disabled controls, the field-by-field template is strong but its eleven required materializations are absent.

**Required correction:** provide the complete canonical registry definitions for all 16 workers, `CONCURRENCY_16`, all 22 executions, eleven pairs, and `CONTROL_MATRIX_22`, including independently checkable aggregate arithmetic.

## B1–B11 closure rereview

| Prior blocker | v3 disposition |
|---|---|
| B1 fixed inputs | **Partial.** Primitive identity rules are deterministic, but the required identity/world/worker/control constants are not actually materialized in the normative registry. |
| B2 source pins | **Closed textually and preserved.** Exact HEAD, fixture, RED content/excerpt, binary diff digest, and 454/0 scope remain pinned; this review did not alter them. |
| B3 frame bytes | **Partial.** Frame grammar and pinned examples are exact; most named per-world frame constants are undefined. |
| B4 failure/error | **Partial.** Named error literals improve value coverage, but `Error` lacks an exhaustive cross-field validity relation and several selected external constants are undefined. |
| B5 accounting/ledger | **Blocked.** Request-scoped dimensions and formulas are directionally correct, but named per-world/worker ledgers and the aggregate operands are absent; some constructors are not total. |
| B6 complete worlds | **Blocked.** The 25 names and selection table exist, but none of the 25 root constants is defined as a complete expandable `World`. |
| B7 identity reference | **Closed.** Request identity, transaction identity, and recursive equality are explicit and structurally coherent. |
| B8 availability/forms | **Blocked.** Structural unavailable forms are improved, but exhaustive semantic cross-field constraints are incomplete and enabled composite materializations are absent. |
| B9 Layer-1 selected/handoff opacity | **Closed.** Published selected and handoff values remain `{available,identity,length,sha256}` only. |
| B10 predecessor opacity | **Closed.** Predecessors use the same opaque schema; transaction custody remains separate and generic. |
| B11 manifest/derivation closure | **Blocked.** Manifest ordering is noncircular, but unresolved roots and non-total constructors prevent canonical world bytes and hashes. |

## Representative calculations and boundaries

The displayed request/response/cancellation frame examples, opaque input lengths/digests, zero/positive measure forms, and ledger relation `after=before+delta` are deterministic where all operands are literal. This review found no need to change the prior representative primitive calculations. That does not close missing per-world and aggregate operands.

The accepted Layer-1 boundary remains opaque: availability, identity, length, and SHA-256 only for selected, handoff, and predecessor commitments. World 16's comparison compares only transaction custody and opaque fields. Source pins remain immutable authoring authorities; Manager, RoundTrip, attachments, leases, current behavior, recordings, generated fixtures, update/bless output, and runtime-produced expectations remain prohibited. Ledger state is request/worker scoped with independently before-zero dimensions; no process-global runtime counters are authorized.

Nothing in this review asserts current runtime output, production behavior, source completeness, cross-capture CALLS, or production authority.

## Final disposition

`ADR0011_CALLHIERARCHY_LAYER1_ORACLE_CORPUS_DESIGN_GO` is **not issued**.

Complete corpus byte authoring is not deterministic because representative success, refusal, disabled-control, world-16, and concurrency reference walks all encounter undefined registry targets; additionally, several constructor and recursive semantic-validity choices remain unresolved. The prior BLOCKED boundary therefore remains in force.

A later GO, if earned after correction and another independent strict review, may authorize independently authored corpus bytes only. It may not authorize implementation, runtime-derived expectations, production behavior, or any file outside:

```text
sessionruntime/testdata/callhierarchy-layer1-red-v1/manifest.json
sessionruntime/testdata/callhierarchy-layer1-red-v1/worlds/<world-name>.json
```

## Model interpretation

The review treats a promised future materialization as non-authoritative until its exact output is present as a closed registry definition or a total admitted constructor invocation. It distinguishes structurally closed object shapes from semantically canonical tagged/composite values.

Key assumptions:

- The phrase defining the registry as fenced `NAME := ...` definitions is normative and controls over later prose saying values “must” be materialized.
- A selection table can reference constants but cannot create undefined registry targets.
- Exact world-16 comparison data can pass locally while its containing world still fails global authorability.

Gaps and verification items:

1. Add and independently enumerate the complete registry target set, then prove every referenced name has exactly one definition and no unused ambiguity-bearing definition exists.
2. Resolve all 25 roots to literal trees and independently compare canonical bytes before constructing the manifest.
3. Add exhaustive cross-field validity predicates and rerun the five representative reference walks recorded above.

Improved framing: authorize corpus bytes only when the design contains a closed, total, acyclic mapping from each of the 25 root names to one complete schema-valid literal tree and one canonical byte sequence.
