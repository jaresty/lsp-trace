# ADR-0011 Call-Hierarchy Layer-1 Generic V3 Attachment-Enable Causal-Input Amendment V2 — Independent Review

Status: **DOCUMENTATION-ONLY GO / MUTATION AND PRODUCTION BLOCKED**

- Reviewed repository: `/Users/schwa/dev/lsp-trace-adr0011-c17`
- Reviewed starting HEAD: `0d807a1d717a26fdda809975473bec3acea4039c`
- Reviewed amendment: `docs/qualification/adr0011-callhierarchy-layer1-generic-v3-attachment-enable-causal-input-amendment-v2.md`
- Accepted replacement lineage: `docs/qualification/adr0011-callhierarchy-layer1-generic-v3-causal-lineage-replacement-v2.md`
- Review mode: independent documentation-only mechanical and causal-boundary review
- Mutation by this review: this document only
- Tests run: none
- Corpus, RED, source, test, prototype, generated-artifact, write-set, writer, commit, activation, carrier-acceptance, semantic, public, release, deployment, and production authority: **not issued**

## Input and scope check

The reviewed starting commit resolves exactly to `0d807a1d717a26fdda809975473bec3acea4039c`. Before this review was authored, the only worktree mutation was the untracked amendment-v2 named above. This review adds only the requested review document.

The superseded v1 amendment review blocked a universal all-85 control-agreement rule because all 16 World-24 worker records physically omit `expected.control`. Amendment v2 replaces that contradictory rule with an occurrence-sensitive physical-domain rule and consistently carries it through its decision, schema/accounting, proposed RED, rematerialization, future mutation, and review-gate sections.

## Independent occurrence and bit census

An independent parse of the accepted causal-input registry produced:

```text
records       85
unique IDs    85
roots         25
W24 workers   16
W25 executions 22
W25 pair members 22
```

Selecting only IDs matching `w25-pair/01/disabled` through `w25-pair/11/disabled` yields exactly 11 records. The other 74 records are the exact complement. Therefore the amendment's independently authored proposed mapping is exact:

```text
false = 11 disabled W25 pair members
true  = 74 other accepted occurrences
total = 85
```

The amendment makes the boolean required and non-null in each future self-contained envelope at `driver.request.attachment_enabled`. Registry and occurrence identity are offline authoring inputs only. Runtime registry lookup, occurrence/world/fixture/pair decoding, and derivation from expected values, commitment availability or content, attachment zero value, transport, result, error, or any implementation observation are expressly forbidden. The bit is consequently the sole proposed enablement operand rather than a recovered observation.

## Physical control domain and W24 omission

An independent traversal of the accepted v3 corpus reconstructed the same 85 structural occurrences and found:

```text
controls physically present  69
controls physically absent   16
```

The 69 present controls are exactly the 25 roots, 22 W25 executions, and 22 W25 pair members. The 16 absent controls are exclusively `w24-worker/01` through `w24-worker/16`.

Amendment v2 correctly limits agreement checks for `expected.control.enabled`, `expected.control.attachment_available`, and `expected.control.observation_available` to those 69 physical-control occurrences. For the 16 W24 workers it requires structural omission validation only and forbids cross-checking, borrowing, inheritance, manufacture, defaulting, projection from `root/24`, or interpreting absence as false or unavailable. This closes the superseded review's blocker without changing expected data.

## Unchanged schema and accounting

The amendment adds only a proposed driver input. It adds no expected path, removes no expected path, and preserves the accepted expected ownership and denominator:

```text
A = 116
B = 20
C = 9
union = 145 retained expected paths
removed/deferred = 17
```

Its occurrence-sensitive arithmetic is exact:

```text
A = 84*116 + 81 =  9,825
B = 84*20  + 55 =  1,735
C = 85*9        =    765
union              = 12,325 = 85*145
```

The three physically omitted W24 worker control members remain part of the nine-path C ownership schema without being materialized in those worker envelopes. Omission validation neither creates a value nor changes ownership, denominator, or the accepted 17 absences.

## Causal RED and self-contained-driver boundary

The proposed future RED use is correctly one-way: strict-decode the independently materialized bit, validate its presence and type, and use it only to set the exact private enabled carrier. Expected data, control data, world/occurrence/fixture/pair identity, commitment metadata or bytes, request/transport/result/error/terminal content, production output, and prior RED output cannot derive, select, repair, or rewrite it.

Commitment and transport inputs remain independently owned driver inputs. The enabled bit may change only private publication eligibility through the proposed carrier. For the 69 physical-control occurrences, C comparison is downstream agreement only; for the 16 W24 workers, RED validates physical omission and performs no C comparison. Expected values never become runtime operands, and no world feed or pair dispatch is introduced.

The future corpus boundary is also self-contained: all 85 envelopes receive the explicit driver bit; every other decoded driver value and the complete expected subtree remain unchanged; the registry is not a runtime dependency; only affected v3 canonical lengths, hashes, and manifest aggregates may be recomputed under separate authority; v1/v2 artifacts remain immutable.

## Finite proposed carrier and accepted replacement lineage

The proposed production seam is finite and matches the accepted replacement lineage:

```go
type callHierarchyLayer1RequestAttachment struct {
    enabled bool
}

func (attachment callHierarchyLayer1RequestAttachment) privateCallHierarchyLayer1WithEnabled(enabled bool) callHierarchyLayer1RequestAttachment
```

The setter value-returns a copy changing only `enabled`; the zero value remains disabled; direct comparability is preserved. No public field, registry lookup, side table, options object, callback, content policy, alternate attachment, or hidden carrier is admitted.

At the first executable line of `(*Manager).RoundTrip`, false keeps the attempt builder nil and true creates exactly one `newCallHierarchyLayer1GenericAttemptLineage(req)`. The accepted replacement then carries that same pointer explicitly through the finite wrapper lineage, finalizing refusal or admitted outcomes through its named paths. Expected values, world/pair identity, commitment content, execution output, and hidden state cannot create or replace the builder. These names remain proposals only; this review does not accept a source carrier or issue production authority.

## W25 external equality

The accepted World-25 corpus contains 11 disabled/enabled pairs, and an independent parse found `external_equal=true` and `transport_equal=true` on every pair. Amendment v2 preserves the accepted boundary: equality must be established from the two independently executed actuals, with nil/empty distinctions preserved. The enabled bit changes only private publication eligibility and cannot alter request, transport, result, error, or external pair behavior. No expected-to-actual, expected-to-expected, world-fed, pair-fed, or shared-execution equality is admitted.

## Disposition

The amendment is internally consistent with the accepted replacement lineage and resolves the prior physical-control contradiction. The exact `74 true / 11 false` mapping, 69/16 physical-domain split, no-borrow and no-feed rules, self-contained future driver, unchanged 145-path schema and 12,325 assertion accounting, proposed RED use, finite carrier, and W25 actual-to-actual external equality all pass this bounded documentation review.

This GO approves only the amendment's documentation-level causal-input contract. It authorizes no corpus rematerialization, RED mutation, source or test edit, prototype, generated artifact, write set, implementation, execution, carrier acceptance, commit, activation, release, deployment, or production use.

```text
ADR0011_CALLHIERARCHY_LAYER1_GENERIC_V3_ATTACHMENT_ENABLE_CAUSAL_INPUT_V2_GO
```
