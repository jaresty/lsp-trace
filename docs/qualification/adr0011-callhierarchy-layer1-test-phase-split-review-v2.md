# ADR0011 Call Hierarchy Layer 1 test phase-split review v2

## Scope

Independent superseding rereview against exact HEAD `751b0276c46dbccc4899cfec32adf513c123b57f` in `/Users/schwa/dev/lsp-trace-adr0011-c17`.

The prior blocked review is preserved unchanged at:

- `docs/qualification/adr0011-callhierarchy-layer1-test-phase-split-review.md`

The corrected proposal reviewed here is:

- `docs/qualification/adr0011-callhierarchy-layer1-test-phase-split.md`

This rereview creates only this v2 review document. No source, test, corpus, fixture, generated artifact, prior review, proposal, or other qualification document is changed, and no commit is made.

This review grants no behavior, semantic, directional, implementation, production, completeness, custody, activation, publication, writer, or commit authority.

## Superseding correction review

The corrected proposal resolves all three defects recorded by the preserved blocked review.

### Actual corpus test identity

The accepted corpus-backed Layer 1 causal RED is correctly identified as:

```text
TestCallHierarchyLayer1AcceptedCorpusRED
```

It is defined in `sessionruntime/b4_owned_method_pair_lease_test.go` and remains rooted at `sessionruntime/testdata/callhierarchy-layer1-red-v1`. The corrected proposal no longer substitutes the unrelated C15 atomic-batch test. It requires the accepted corpus oracle to remain unchanged and RED before separately authorized production implementation, then become GREEN only through that later production work without editing, skipping, or weakening the oracle or corpus.

Focused verification at exact HEAD confirmed the actual oracle remains causally RED:

```text
go test ./sessionruntime -run '^TestCallHierarchyLayer1AcceptedCorpusRED$' -count=1
Go test: 0 passed, 28 failed in 1 packages
```

The failures reached the expected `ASSERT_CH_API_GAP_*` implementation-gap family across the 25 corpus worlds and the World 24 and World 25 aggregate cases. No compile, fixture, corpus-integrity, or skip failure displaced that RED.

### Generic tests remain preimplementation RED

The corrected proposal accurately states that the four retained generic tests remain RED before Layer 1 production implementation:

- `TestCallHierarchyManagerMintsExactCustody`;
- `TestCallHierarchyLimitsAndAccountingBoundaries`;
- `TestCallHierarchyCompleteFrameBytesAndDigest`, renamed from `TestCallHierarchyLiteralFrameCommitments` when the split is implemented; and
- `TestCallHierarchyTerminalClassificationIsExact`.

At exact HEAD, using the current pre-split frame-test name, focused verification confirmed:

```text
go test ./sessionruntime -run '^(TestCallHierarchyManagerMintsExactCustody|TestCallHierarchyLimitsAndAccountingBoundaries|TestCallHierarchyLiteralFrameCommitments|TestCallHierarchyTerminalClassificationIsExact)$' -count=1
Go test: 0 passed, 4 failed in 1 packages
```

The four failures were the existing implementation gaps `ASSERT_CH_MANAGER_MINT_CUSTODY`, `ASSERT_CH_LIMITS_AT_BOUNDARY`, `ASSERT_CH_REQUEST_FRAME_EXACT_BYTES`, and `ASSERT_CH_TERMINAL_VALID`. The corrected proposal does not claim the test-only amendment makes them pass. It separates source-level phase-split verification from later production GREEN and prohibits obtaining GREEN by changing expectations, deleting assertions, or weakening evidence.

### Minimal phase split and pinned directional inventory

Source inspection of `sessionruntime/owned_method_pair_test.go` confirms the corrected split remains mechanically complete and minimal for the six current `TestCallHierarchy*` tests:

- generic manager custody, limits/accounting, complete request/response frame integrity, and terminal-enum checks remain active;
- only the response token-form/result-subspan assertion is split from the mixed literal-frame test;
- exact incoming/outgoing method triples are deferred;
- raw selected-item/handoff commitments are deferred; and
- rich predecessor/request-chain construction and binding meaning are deferred.

The proposal adds exactly four independently pinned directional skips, with stable exact names and exact first-executable markers:

1. `TestCallHierarchyDirectionalMethodAllowlistTriples`;
2. `TestCallHierarchyDirectionalTokenFormAndResultSubspan`;
3. `TestCallHierarchyDirectionalRawSelectedAndHandoffCommitments`;
4. `TestCallHierarchyDirectionalRichPredecessorRequestChain`.

All four use the reserved prefix:

```text
ADR0011_CALLHIERARCHY_DIRECTIONAL_GATE_PENDING:
```

The proposal requires the deferred bodies and assertions to remain source-visible, forbids deletion or weakening, pins the count independently at exactly four, and requires a later explicit directional gate to remove all four skips and leave zero markers bearing that prefix.

### Later production is required for GREEN

The corrected verification sequence is phase-honest:

1. the test-only amendment verifies classification and preservation while the four generic tests and accepted corpus oracle remain RED;
2. separately authorized Layer 1 production implementation must make those five unchanged tests GREEN;
3. full `sessionruntime` GREEN is considered only after that production work, while exactly four directional skips remain independently pinned; and
4. the later directional gate must reactivate all four deferred tests.

The phase split therefore does not conceal retained generic RED or corpus RED behind directional skips and does not claim behavior from source classification.

## Owner controls

The proposal preserves the private-B4 owner lifecycle family as an unchanged mandatory control. Focused verification at exact HEAD passed:

```text
go test ./sessionruntime -run '^(TestPrivateB4OwnedMethodPair|TestPrivateOwnedMethodPair)' -count=1
Go test: 10 passed in 1 packages
```

The permitted later test-only implementation surface remains exactly `sessionruntime/owned_method_pair_test.go`. No production file, corpus, accepted corpus test, private-B4 owner test, fixture, generated artifact, or other test file is admitted.

## Disposition

The corrected proposal accurately names the accepted corpus oracle, preserves the four generic tests as preimplementation RED, limits the phase split to four pinned directional skips plus the frame-test split, requires separate later production work for GREEN, and retains owner lifecycle controls. The prior blocked review remains preserved as historical evidence of the superseded defects.

```text
ADR0011_CALLHIERARCHY_LAYER1_TEST_PHASE_SPLIT_GO
```

This GO accepts only the corrected test-phase-split documentation and its bounded future test-only amendment. It grants no behavior or production authority.
