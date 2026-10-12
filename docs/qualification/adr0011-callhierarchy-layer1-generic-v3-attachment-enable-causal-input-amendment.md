# ADR-0011 Call-Hierarchy Layer-1 Generic V3 Attachment-Enable Causal-Input Amendment

Status: **PENDING / NO GO**

- Repository: `/Users/schwa/dev/lsp-trace-adr0011-c17`
- Pinned starting HEAD: `4add2ac3f9cf556789869067b35170dd2c9f6276`
- Required starting state: clean worktree at the pinned HEAD
- Authority: attachment-enable causal-input and finite-seam design only
- Corpus, RED, test, source, generated-artifact, implementation, C15 execution, writer, commit, activation, and production authority: **not issued**

## Decision

Every one of the 85 v3 structural occurrences SHALL carry one explicit causal boolean at:

```text
driver.request.attachment_enabled
```

`attachment_enabled` is required, non-null, and boolean. It is the sole runtime operand that decides whether the private Layer-1 attachment is enabled for that occurrence. It is independently derived from the accepted v3 causal-registry occurrence kind:

```text
false  exactly for w25-pair/01/disabled through w25-pair/11/disabled  (11 occurrences)
true   for every other accepted v3 occurrence                         (74 occurrences)
                                                                    ----------------
total                                                               85 occurrences
```

No exception may be inferred from commitment availability, identity, digest, length, bytes, expected values, control values, world number, fixture name, occurrence ID text at runtime, pair label, transport contents, result contents, or implementation output. Any different value requires a separately stated, occurrence-specific independent causal justification and independent review before rematerialization; absent such accepted justification, the exact `11 false / 74 true` rule is mandatory.

The registry occurrence kind is an offline authoring operand only. Final corpus envelopes materialize the boolean directly in their self-contained `driver.request`; runtime execution does not consult the registry or decode an occurrence ID to recover it.

## C metadata is agreement-only, never execution input

The expected control values

```text
expected.control.enabled
expected.control.attachment_available
expected.control.observation_available
```

remain category C fixture/control assertions. For every occurrence, strict corpus validation SHALL require them to agree structurally with the independently materialized causal request bit and the existing control contract. In particular, the disabled W25 pair member must retain the disabled/unavailable control shape, and the enabled member must retain the enabled/available control shape required by that contract.

These C values do not feed request construction, attachment mutation, Manager execution, admission, owner publication, observation acquisition, or any actual/B computation. The RED MUST NOT read any `expected.control.*` value to choose the enabled bit, decide whether to attach, or decide whether an owner or observation should exist. Structural agreement is checked only after independently validating `driver.request.attachment_enabled` from the accepted offline occurrence-kind mapping.

`expected`, including `expected.control`, remains forbidden as a runtime operand. A structural disagreement is corpus RED; it is not permission to select execution from expected data.

## Independence from opaque commitments

Attachment enablement and opaque commitment availability are orthogonal causal dimensions.

- A disabled occurrence still carries its exact independently generated selected, handoff, and ordered predecessor commitment inputs through the private attachment construction seam.
- Disabled means that production must not publish the Layer-1 generic owner or observation for that request.
- Enabled means that production may publish the owner and observation under the existing lifecycle, accounting, capacity, and terminal contract.
- Enabled publication is permitted even when selected, handoff, or every predecessor commitment is unavailable, empty, or normalized to zero metadata.
- No commitment field is a validity predicate, enable predicate, admission predicate, refusal predicate, or world-dispatch key.

Production and RED code MUST NOT infer enablement from whether the attachment equals its zero value, from whether any commitment is available, or from commitment content. The explicit boolean is authoritative.

## Required RED construction and causal assertions

A later separately authorized v3 RED revision SHALL perform all 85 real executions and construct each request only from that occurrence's self-contained `driver`:

1. strict-decode and independently validate `driver.request.attachment_enabled` against the accepted offline occurrence-kind mapping;
2. build selected, handoff, and ordered predecessor metadata only from `driver.commitment_inputs`;
3. invoke the exact private value-returning attachment method carrying the explicit enabled bit and the exact commitment metadata before `Manager.RoundTrip`;
4. assign the returned attachment to `RoundTripRequest.callHierarchyLayer1`;
5. execute without consulting `expected`, control metadata, occurrence/world/pair names, or transport/result content for dispatch; and
6. derive actual owner, observation, lifecycle, accounting, and B assertions only from the execution.

The RED must prove:

- all 11 disabled W25 pair-member executions attempt and complete their existing transport script but publish no Layer-1 generic owner or observation;
- all 74 enabled executions are eligible for publication independently of commitment availability, including enabled cases where all commitments are unavailable or carry normalized zero metadata;
- missing publication for an otherwise admissible enabled request is implementation RED, not a reason to rewrite the bit or expected controls;
- publication for a disabled request is implementation RED;
- no content-based validity, inference, or world dispatch occurs; and
- counters remain exact for 25 roots, 16 W24 workers, 22 W25 executions, and 22 W25 pair members, totaling 85 attempts.

The RED interface and call must carry both enablement and commitments exactly. Under the finite seam selected below, the required private interfaces are:

```go
type callHierarchyLayer1GenericV3EnableAttachment interface {
	privateCallHierarchyLayer1WithEnabled(bool) callHierarchyLayer1RequestAttachment
}

type callHierarchyLayer1GenericV3PredecessorAttachment interface {
	privateCallHierarchyLayer1WithPredecessors([]callHierarchyOpaqueCommitmentMetadata) callHierarchyLayer1RequestAttachment
}
```

The RED may invoke these value-returning setters in either order only after proving that each changes solely its owned field and preserves all other attachment fields exactly. It must pass the exact `driver.request.attachment_enabled` value to `privateCallHierarchyLayer1WithEnabled`; no expected or control operand is allowed.

## W25 pair comparability and external transparency

For each of the 11 W25 disabled/enabled pairs, the two members retain independently generated commitments and independently execute their own requests. Their external transport contract remains identical under the existing pair fixture:

- exact request and response transport bytes;
- transport messages and byte accounting;
- externally returned result and error representations;
- terminal behavior; and
- bounded externally visible bytes.

The only intended causal contrast is private Layer-1 publication controlled by `driver.request.attachment_enabled`. Disabled execution must not publish an owner or observation; enabled execution may publish one. The bit must not alter transport, retry, normalization, diagnostics, external result/error, or terminal behavior. Pair equality is computed from the two actual executions, never manufactured from shared expected operands.

## Finite production seam

To preserve direct comparability of `callHierarchyLayer1RequestAttachment`, the finite production representation is exactly one added boolean field:

```go
type callHierarchyLayer1RequestAttachment struct {
	// existing fields remain unchanged
	enabled bool
}
```

The exact private setter is:

```go
func (attachment callHierarchyLayer1RequestAttachment) privateCallHierarchyLayer1WithEnabled(enabled bool) callHierarchyLayer1RequestAttachment
```

It value-returns a copy with only `enabled` replaced and every existing field preserved exactly. It allocates no alternate attachment, owner, lease, lifecycle, parser, policy object, or side table. The zero value remains disabled and comparable. Direct attachment equality remains legal; equality of opaque predecessor metadata remains value-based under the existing holder rules and never pointer-semantic.

The exact additive symbol/field authorization proposed by this amendment is:

1. `callHierarchyLayer1RequestAttachment.enabled bool` in `sessionruntime/b4_owned_method_pair_lease.go`;
2. `(callHierarchyLayer1RequestAttachment).privateCallHierarchyLayer1WithEnabled(bool) callHierarchyLayer1RequestAttachment` in `sessionruntime/owned_method_pair.go`;
3. reading `RoundTripRequest.callHierarchyLayer1.enabled` at the existing `(*Manager).RoundTrip`/generic-owner admission seam in `sessionruntime/sessionruntime.go`; and
4. propagation of that exact bit only through the already-authorized `newCallHierarchyLayer1GenericOwner` path in `sessionruntime/b4_owned_method_pair_lease.go`, without adding a public field or alternate lifecycle.

The previously reviewed exact predecessor symbols remain unchanged:

```text
type callHierarchyOpaqueCommitmentList struct
callHierarchyLayer1RequestAttachment.predecessors *callHierarchyOpaqueCommitmentList
(callHierarchyLayer1RequestAttachment).privateCallHierarchyLayer1WithPredecessors([]callHierarchyOpaqueCommitmentMetadata) callHierarchyLayer1RequestAttachment
```

Combining enablement with the predecessor setter is not authorized by this amendment. A combined signature may replace the separate enable setter only after an exact proposed signature, field-preservation behavior, RED interface, complete call-site impact, C15 delta, and finite symbol count are independently reviewed and accepted. No variadic, options, generic map, callback, interface-valued, content-sensitive, or open-ended setter is permitted.

## No semantic or content-based policy

The enable seam is generic and policy-free. It MUST NOT:

- inspect selected, handoff, or predecessor availability or contents;
- validate identities, lengths, digests, raw bytes, preimages, request params, methods, frames, results, or errors;
- infer validity, direction, target, occurrence, pair membership, world, method family, relation, qualification, publication authority, or downstream meaning;
- dispatch by fixture, corpus, occurrence, world, pair, request, or transport content;
- add parser, serialization, hashing, configuration, feature-flag, CLI, MCP, schema, logging, telemetry, metric, or diagnostic behavior; or
- alter external transport or lifecycle behavior except the private owner/observation publication decision controlled by the explicit bit.

## Corpus rematerialization

A later separately authorized canonical corpus rematerialization SHALL add only the required boolean member `driver.request.attachment_enabled` to each of the 85 self-contained occurrences.

The rematerializer must:

1. verify the accepted v3 manifest, registry pin, and immutable reachable v1/v2 pins;
2. traverse exactly the accepted `25/16/22/22 = 85` structural occurrences;
3. materialize `false` for exactly the 11 `w25-pair/*/disabled` registry occurrence kinds and `true` for the other 74;
4. perform that derivation without reading any `expected` subtree, prototype output, RED output, or production output;
5. preserve every other decoded `driver` value, including all commitment inputs and transport events;
6. preserve the complete `expected` subtree byte-for-byte in decoded value and canonical form if and only if strict cross-rules prove that `control.enabled`, `control.attachment_available`, and `control.observation_available` already agree with the new driver bit and existing control contract;
7. hard-stop on any cross-rule disagreement rather than changing expected data under this amendment;
8. strict-reject missing, null, non-boolean, duplicate, or unknown alternatives to the new field;
9. canonicalize each changed complete envelope and recompute every affected file length and SHA-256 plus manifest aggregate fields; and
10. remain self-contained: no registry reference, occurrence lookup, inherited default, side table, filename rule, or ambient resolver may be required at runtime.

No expected scalar is authorized to change. If any cross-rule fails, rematerialization is blocked pending a separate exact expected-value amendment.

## Ownership and write-set implications

### Driver/C ownership

`driver.request.attachment_enabled` is a causal driver input, not an A, B, or C expected path. It does not change the 145 expected-path denominator, the exact 17 absences, ordinary `116/20/9` ownership, or the accepted World-24 root `81/55/9` occurrence-sensitive ownership.

The three named `expected.control.*` values remain C. Their new cross-rules strengthen structural consistency only; they acquire no runtime authority. Actual attachment/observation availability and lifecycle assertions remain direct actual/B results under the existing ownership rules.

### RED write-set

A future RED amendment is limited to the existing v3 RED and v3 corpus/manifest paths separately authorized for rematerialization. It must add the strict request-field decode/validation, exact private enable-setter interface/call, independent runtime assertions, and no-expected-input guards. It must not mutate v1/v2 tests or corpora, production source, unrelated tests, or generated catalogs under RED authority.

### Production and C15 write-set

The finite production candidate remains exactly the previously reviewed three-file disposable boundary:

```text
sessionruntime/sessionruntime.go
sessionruntime/owned_method_pair.go
sessionruntime/b4_owned_method_pair_lease.go
```

Relative to the accepted v3 disposable boundary, the exact proposed additions are the `enabled bool` field, the exact value-returning enable setter, and the finite reads/propagation named above. These additions may add or change C15 candidates. Therefore the prior zero-delta production assumption does not apply automatically.

Before production authority, a separately authorized disposable prototype must:

1. implement only the exact three-file, finite-symbol seam above;
2. run the canonical C15 enumerator from the accepted baseline;
3. preserve the complete `ADDED`, `REMOVED`, and `CHANGED` candidate/classification delta with source locations and AST hashes;
4. obtain independent review of every delta and exact symbol;
5. dispose of the prototype without commit; and
6. obtain a new production write-set gate naming the final files, symbols, candidates, classifications, and sole writer.

Any fourth production file, additional field, helper, constructor, accessor, alternate setter, public symbol, new candidate, changed candidate, or required classification outside that independently reviewed finite delta is an immediate hard stop. Historical C15 artifacts remain immutable; they are evidence baselines, not regeneration targets.

## Review gate

Independent review must verify:

- exact clean pinned input `4add2ac3f9cf556789869067b35170dd2c9f6276`;
- exact 85-occurrence coverage and `11 false / 74 true` derivation from registry occurrence kind independently of expected data;
- strict self-contained `driver.request.attachment_enabled` materialization;
- unchanged expected values with passing structural cross-rules for the three named C controls;
- no expected/control runtime operand;
- disabled requests retain commitments but publish no owner/observation;
- enabled requests may publish even when all commitments are unavailable or zero-normalized;
- identical external transport behavior for every W25 pair;
- exact separate value-returning setter and preserved attachment comparability;
- no content-based inference, validity policy, semantic dispatch, or world dispatch;
- unchanged expected ownership counts and exact RED assertion ownership;
- complete finite three-file C15 delta treatment; and
- no authority inferred for corpus, RED, test, source, implementation, writer, commit, activation, or production use.

```text
ADR0011_CALLHIERARCHY_LAYER1_GENERIC_V3_ATTACHMENT_ENABLE_CAUSAL_INPUT_AMENDMENT_REVIEW_GO — NOT ISSUED
STATUS — PENDING / NO GO
```
