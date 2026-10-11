# ADR 0011 Call Hierarchy Layer 1 test phase split

Status: **GATE PENDING**  
Scope: test-only amendment for `sessionruntime/owned_method_pair_test.go`  
Baseline: clean `751b0276c46dbccc4899cfec32adf513c123b57f`

## Purpose

Freeze the test-only split between the generic Layer 1 contract that may be verified now and the semantic/directional scaffold that must remain deferred until a later directional gate. This amendment preserves the deferred evidence in source; it does not delete it, weaken it, or rewrite it to pass.

This document authorizes no behavior, production design, semantic interpretation, directionality, completeness, custody, or runtime claim. Passing the retained tests establishes only their bounded generic assertions. Skipped tests remain unresolved evidence, not accepted behavior.

## Permitted edit surface

The later implementation of this amendment may edit exactly:

- `sessionruntime/owned_method_pair_test.go`

No production file, other test file, fixture, corpus, generated file, or documentation file may be changed as part of that implementation. In particular, the existing private-B4 owner lifecycle tests and `TestCallHierarchyLayer1AcceptedCorpusRED` must remain unchanged.

Permitted edits inside the named test file are limited to:

1. rename `TestCallHierarchyLiteralFrameCommitments` to `TestCallHierarchyCompleteFrameBytesAndDigest`;
2. leave its complete request/response frame byte-copy, frame-length, defensive-copy, and SHA-256 assertions active;
3. move only the response token-form/subspan assertion into the deferred test named below;
4. add the exact `t.Skip` calls specified below at the beginning of the four deferred tests, before setup or assertions execute; and
5. rename deferred tests only as specified below so the skip inventory is stable and reviewable.

Imports may be adjusted only if these test-only changes make an existing import unused. Assertions may be physically moved only where this document explicitly requires the literal-frame split; their values and intended evidence must not be weakened.

## Retained generic Layer 1 tests

The following tests remain active with their current assertions, except for the exact literal-frame split described above:

- `TestCallHierarchyManagerMintsExactCustody`
- `TestCallHierarchyLimitsAndAccountingBoundaries`
- `TestCallHierarchyCompleteFrameBytesAndDigest`
  - retains complete request-frame bytes, length, digest, and defensive-copy checks;
  - retains complete response-frame bytes, digest, and defensive-copy checks;
  - does **not** assert JSON token form, result subspan, selected-item meaning, handoff meaning, predecessor meaning, or request-chain meaning.
- `TestCallHierarchyTerminalClassificationIsExact`

These are generic carrier, identity, boundary/accounting, complete-frame integrity, and terminal-enum checks only. All four are currently RED before Layer 1 production implementation. The test-only phase split does not make them pass, and neither their names nor any later passing status grants semantic or directional behavior authority.

## Deferred directional/semantic scaffold

The following tests must remain source-visible in `sessionruntime/owned_method_pair_test.go`. Each must begin with its exact `t.Skip` marker. The bodies and assertions below the skip remain present as evidence and must not be removed, weakened, replaced with vacuous assertions, or rewritten merely to pass.

| Exact deferred test name | Exact first executable statement |
|---|---|
| `TestCallHierarchyDirectionalMethodAllowlistTriples` | `t.Skip("ADR0011_CALLHIERARCHY_DIRECTIONAL_GATE_PENDING: method allowlist triples")` |
| `TestCallHierarchyDirectionalTokenFormAndResultSubspan` | `t.Skip("ADR0011_CALLHIERARCHY_DIRECTIONAL_GATE_PENDING: token form and result subspan")` |
| `TestCallHierarchyDirectionalRawSelectedAndHandoffCommitments` | `t.Skip("ADR0011_CALLHIERARCHY_DIRECTIONAL_GATE_PENDING: raw selected and handoff commitments")` |
| `TestCallHierarchyDirectionalRichPredecessorRequestChain` | `t.Skip("ADR0011_CALLHIERARCHY_DIRECTIONAL_GATE_PENDING: rich predecessor and request chain")` |

The exact classification is:

- rename and defer `TestCallHierarchyExactPerBindingMethodAllowlists` as `TestCallHierarchyDirectionalMethodAllowlistTriples`;
- create `TestCallHierarchyDirectionalTokenFormAndResultSubspan` from the token-form/result-subspan portion removed from `TestCallHierarchyLiteralFrameCommitments`; the existing `callHierarchyJSONTokenFormEmptyArray` assertion remains visible beneath the skip, together with any existing exact subspan inputs needed by that assertion;
- rename and defer `TestCallHierarchySelectedAndHandoffCommitments` as `TestCallHierarchyDirectionalRawSelectedAndHandoffCommitments`;
- rename and defer `TestCallHierarchyRequestPreservesCommitmentChain` as `TestCallHierarchyDirectionalRichPredecessorRequestChain`.

The shared marker prefix is stable and reserved for the later gate:

```text
ADR0011_CALLHIERARCHY_DIRECTIONAL_GATE_PENDING:
```

No additional test in `sessionruntime/owned_method_pair_test.go` may use that prefix under this amendment.

## Explicitly unchanged evidence

This amendment must not alter, skip, rename, relax, or make green:

- existing private-B4 owner lifecycle coverage, including its production/corpus ownership and lifecycle checks;
- `TestCallHierarchyLayer1AcceptedCorpusRED`;
- the corpus and corpus-backed tests rooted at `sessionruntime/testdata/callhierarchy-layer1-red-v1` and `sessionruntime/b4_owned_method_pair_lease_test.go`; or
- any other existing RED, fixture provenance, digest, byte count, assertion marker, or lifecycle expectation.

The accepted-corpus oracle remains expected RED until the separately authorized Layer 1 production implementation satisfies it. This phase split is not permission to change its expectation or evidence.

## Verification contract

No verification is executed by this documentation-only change. When the test-only amendment is later implemented, verification proceeds in this order:

1. Verify the phase split itself by source inspection of `sessionruntime/owned_method_pair_test.go`. Confirm that the four retained generic tests remain active and unchanged except for the authorized literal-frame split; the four deferred tests remain source-visible with their exact names, bodies, assertions, classifications, and first-executable `t.Skip` markers; and no assertion has been weakened. This verification preserves, rather than resolves, the expected RED state.

2. Run the focused retained generic tests before Layer 1 production implementation:

   ```sh
   go test ./sessionruntime -run '^(TestCallHierarchyManagerMintsExactCustody|TestCallHierarchyLimitsAndAccountingBoundaries|TestCallHierarchyCompleteFrameBytesAndDigest|TestCallHierarchyTerminalClassificationIsExact)$'
   ```

   Expected: RED for the existing pre-implementation reasons. The test-only split does not make any of these tests pass.

3. Run the accepted-corpus oracle before Layer 1 production implementation:

   ```sh
   go test ./sessionruntime -run '^TestCallHierarchyLayer1AcceptedCorpusRED$'
   ```

   Expected: RED for its existing oracle reason, not a compile failure, fixture failure, skip, or rewritten expectation.

4. Only after a separately authorized Layer 1 production implementation, rerun the same four retained generic tests and the unchanged accepted-corpus oracle.

   Expected: all five become GREEN without changing their retained assertions, the oracle, or the corpus.

5. Run the full package only after those five tests and the unchanged private-B4 owner lifecycle tests are GREEN:

   ```sh
   go test ./sessionruntime
   ```

   Full `sessionruntime` GREEN is contingent on all of those tests remaining GREEN and exactly four independently pinned directional skips. The implementation must inspect verbose test output (or equivalent machine-readable test events) and pin exactly four skips bearing the stable marker prefix:

   - `TestCallHierarchyDirectionalMethodAllowlistTriples`
   - `TestCallHierarchyDirectionalTokenFormAndResultSubspan`
   - `TestCallHierarchyDirectionalRawSelectedAndHandoffCommitments`
   - `TestCallHierarchyDirectionalRichPredecessorRequestChain`

   Required pinned count: **4**. Missing, renamed, duplicate, newly added, or silently passing directional tests fail this qualification. Unrelated pre-existing skips are outside this count; no new skip is authorized by this amendment.

## Later directional gate

A later, explicit directional-gate adjudication must reactivate all four deferred tests by removing their exact `t.Skip` statements. Reactivation must preserve the source-visible assertions and may strengthen them only under separate authority. The gate must run the four reactivated tests and the full `sessionruntime` package with zero remaining skips carrying `ADR0011_CALLHIERARCHY_DIRECTIONAL_GATE_PENDING:`.

Until that gate is explicitly accepted, the deferred tests must remain skipped and must not be cited as passing evidence. The gate is pending; this document does not satisfy it.

## Prohibitions

This amendment does not permit:

- production-code changes;
- source behavior changes;
- edits outside `sessionruntime/owned_method_pair_test.go` when implementing the split;
- deletion or weakening of deferred assertions;
- conversion of deferred assertions into comments, fixtures without tests, or unconditional passes;
- changes to private-B4 ownership/lifecycle evidence or the immutable corpus;
- changes to the accepted-corpus oracle to obtain green;
- inference of call-hierarchy semantics or directionality from the retained generic tests; or
- any claim of behavior authority, production readiness, completeness, or gate acceptance.
