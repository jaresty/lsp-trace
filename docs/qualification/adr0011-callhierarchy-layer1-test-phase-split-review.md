# ADR0011 Call Hierarchy Layer 1 test phase-split review

## Scope

Independent review against exact HEAD `751b0276c46dbccc4899cfec32adf513c123b57f` in `/Users/schwa/dev/lsp-trace-adr0011-c17`.

Before this review, the worktree had exactly one untracked file:

- `docs/qualification/adr0011-callhierarchy-layer1-test-phase-split.md`

This review creates only this review document. No source, test, corpus, fixture, generated artifact, or other qualification document is changed, and no commit is made.

This review grants no behavior, semantic, directional, implementation, production, completeness, custody, or acceptance authority.

## Classification review

The proposed source classification is mechanically complete and minimal for the six current `TestCallHierarchy*` tests in `sessionruntime/owned_method_pair_test.go`:

- retain `TestCallHierarchyManagerMintsExactCustody` as generic manager-custody identity evidence;
- retain `TestCallHierarchyLimitsAndAccountingBoundaries` as generic limits/accounting evidence;
- split `TestCallHierarchyLiteralFrameCommitments`, retaining exact request/response frame bytes, lengths, SHA-256 values, and defensive-copy assertions while deferring only response JSON token form/result-subspan interpretation;
- retain `TestCallHierarchyTerminalClassificationIsExact` as a closed terminal-enum validity check;
- defer the exact incoming/outgoing method triples and their ordering/refusal policy;
- defer raw selected-item/handoff commitments and their relationship;
- defer predecessor/request-chain construction and binding meaning.

The literal-frame split is necessary and correctly placed. The current response assertion combines generic exact-frame evidence with `resultForm == callHierarchyJSONTokenFormEmptyArray`; moving only token form and the required subspan inputs beneath a skip preserves the generic frame evidence without granting JSON-result semantics.

Exactly four deferred tests are therefore complete and minimal. No fifth current test contains an additional semantic/directional assertion that must be skipped, and none of the four proposed deferred units can remain active without asserting method-family direction, JSON result form, selected/handoff meaning, or predecessor/request-chain meaning.

The proposed names and shared marker prefix are stable and reviewable. The later gate is explicit: all four exact `t.Skip` statements must be removed, all four tests rerun, and zero `ADR0011_CALLHIERARCHY_DIRECTIONAL_GATE_PENDING:` markers remain. The proposal also correctly requires deferred bodies and assertions to stay source-visible and prohibits deletion, weakening, vacuous replacement, or citation as passing evidence.

## Preservation checks

The proposed implementation surface is limited to `sessionruntime/owned_method_pair_test.go`; therefore it does not admit edits to the accepted corpus, `sessionruntime/b4_owned_method_pair_lease_test.go`, or the separate owner lifecycle tests.

Current owner lifecycle verification remains green:

```text
go test ./sessionruntime -run '^(TestPrivateB4OwnedMethodPair|TestPrivateOwnedMethodPair)' -count=1
Go test: 10 passed in 1 packages
```

The actual accepted corpus-backed Layer 1 causal RED remains source-visible and causally red:

```text
go test ./sessionruntime -run '^TestCallHierarchyLayer1AcceptedCorpusRED$' -count=1
Go test: 0 passed, 28 failed in 1 packages
```

The failures reach the expected `ASSERT_CH_API_GAP_*` implementation-gap family across the accepted worlds and aggregate cases. No compile, fixture, corpus-integrity, or skip failure displaced that RED.

## Blocking verification defects

### 1. The proposal pins the wrong causal RED

The accepted corpus-backed Layer 1 oracle is `TestCallHierarchyLayer1AcceptedCorpusRED` in `sessionruntime/b4_owned_method_pair_lease_test.go`, as recorded by the accepted opacity-correction review. The proposal instead names and runs:

```text
TestADR0011C17C15AtomicBatchObserverOwnedPairSameInstanceCausalRED
```

That C15 atomic-batch test is not the accepted Layer 1 corpus oracle, and at exact HEAD it is already green:

```text
go test ./sessionruntime -run '^TestADR0011C17C15AtomicBatchObserverOwnedPairSameInstanceCausalRED$'
Go test: 1 passed in 1 packages
```

Consequently, the statements that this named test is the corpus-backed causal test and remains expected RED are false at the reviewed baseline. The verification contract does not currently protect the accepted corpus RED.

Required correction: replace the wrong causal-test identity with `TestCallHierarchyLayer1AcceptedCorpusRED`, preserve its existing causal RED before authorized production implementation, and require later GREEN without editing or skipping that test or its corpus.

### 2. The retained generic PASS is impossible under the authorized test-only amendment

At exact HEAD, all four proposed retained generic tests are RED:

```text
go test ./sessionruntime -run '^(TestCallHierarchyManagerMintsExactCustody|TestCallHierarchyLimitsAndAccountingBoundaries|TestCallHierarchyLiteralFrameCommitments|TestCallHierarchyTerminalClassificationIsExact)$'
Go test: 0 passed, 4 failed in 1 packages
```

The failures are the current implementation gaps `ASSERT_CH_MANAGER_MINT_CUSTODY`, `ASSERT_CH_LIMITS_AT_BOUNDARY`, `ASSERT_CH_REQUEST_FRAME_EXACT_BYTES`, and `ASSERT_CH_TERMINAL_VALID`. Renaming the mixed frame test, moving only token-form/subspan evidence, and adding four skips cannot make these generic assertions pass. The proposal nevertheless says they may be verified now and requires PASS after the test-only amendment, while prohibiting production changes and assertion weakening.

Required correction: state the generic tests' actual pre-implementation RED state, separate test-classification verification from later production GREEN, and require their unchanged generic assertions to become GREEN only after the separately authorized Layer 1 implementation. Do not obtain PASS by changing expectations or deleting assertions.

### 3. Full-package green with four pinned skips is not presently an honest phase-split verification

Exactly four pinned directional skips would honestly describe only the deferred semantic/directional inventory. They cannot explain or conceal the still-RED retained generic tests or accepted corpus oracle. A full `sessionruntime` GREEN is valid only after separately authorized production work makes the retained generic tests and `TestCallHierarchyLayer1AcceptedCorpusRED` green without changing their evidence. The current verification sequence omits the actual accepted oracle and therefore could report a misleading green boundary.

Required correction: make full-package GREEN explicitly contingent on unchanged retained generic tests and the actual accepted corpus oracle becoming GREEN through separately authorized production implementation, while pinning exactly the four directional skips independently. Also retain the existing owner-lifecycle family as a mandatory green preservation control.

## Disposition

The four-test semantic/directional split, names, marker prefix, source-visible preservation rules, and later reactivation gate are accepted as a minimal classification. The amendment is blocked because its causal-RED identity and phase verification claims are materially incorrect at exact HEAD.

```text
ADR0011_CALLHIERARCHY_LAYER1_TEST_PHASE_SPLIT_BLOCKED
```
