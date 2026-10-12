# ADR-0011 Call-Hierarchy Layer-1 Generic V3 Attachment-Enable Causal-Input Amendment V2

Status: **REVIEW_REQUIRED**

- Repository: `/Users/schwa/dev/lsp-trace-adr0011-c17`
- Pinned starting HEAD: `0d807a1d717a26fdda809975473bec3acea4039c`
- Required starting state: exact clean worktree at the pinned HEAD
- Supersedes: `docs/qualification/adr0011-callhierarchy-layer1-generic-v3-attachment-enable-causal-input-amendment.md`
- Accepted replacement lineage: `docs/qualification/adr0011-callhierarchy-layer1-generic-v3-causal-lineage-replacement-v2.md`
- Mutation authority: this document only
- Corpus, RED, source, test, prototype, generated-artifact, execution, writer, commit, activation, carrier-acceptance, and production authority: **not issued**

## Decision

Every one of the 85 v3 structural occurrences requires the explicit, non-null boolean causal input:

```text
driver.request.attachment_enabled
```

The independent registry occurrence-kind mapping is exact:

```text
false  = w25-pair/01/disabled through w25-pair/11/disabled = 11
true   = every other accepted v3 occurrence                 = 74
total                                                            85
```

The strict driver bit is structurally present on all 85 occurrences. It is independently materialized during offline authoring and is the sole proposed enablement operand. Runtime execution must not consult the registry or decode occurrence, world, fixture, or pair identity to recover it.

Enablement is independent of `expected`, commitment availability or content, attachment zero value, transport, result, error, and every other fixture or implementation observation. The false and true values above are not inferred from any of those values.

## Physical C domains and agreement

C agreement applies only where the relevant C values physically exist.

The root/control physical domain contains 69 occurrences:

```text
25 roots
22 W25 executions
22 W25 pair members
--
69 occurrences physically carrying expected.control
```

For only those 69 occurrences, strict structural validation checks the independently materialized driver bit against the physically present:

```text
expected.control.enabled
expected.control.attachment_available
expected.control.observation_available
```

These checks are agreement-only and occur after independent validation of `driver.request.attachment_enabled`. The C values never select, construct, or alter runtime input, execution, owner publication, observation publication, actual values, or B derivations.

The worker physical domain contains the 16 occurrences `w24-worker/01` through `w24-worker/16`. Those workers preserve the physical omission of `expected.control`. Validation checks that omission structurally. It performs no C cross-check for those workers and must not borrow, inherit, manufacture, default, or project any control value from `root/24` or another occurrence. Physical absence is neither false nor unavailable.

Thus:

- the driver bit is required and physically present for all 85 occurrences;
- C agreement is checked for exactly the 69 occurrences physically carrying `expected.control`; and
- the 16 W24 workers preserve omission with no C cross-check or borrowing.

## Schema, paths, and accounting remain unchanged

This amendment adds no expected path and removes no expected path. The accepted expected schema and path accounting remain exact:

```text
A production generic owner/view fields                     116
B RED-derived assertions from actual executions             20
C fixture/control-only assertions and metadata                9
                                                          -----
retained expected paths                                     145
removed/deferred paths remain absent                         17
```

Occurrence-sensitive accounting remains:

```text
84 ordinary occurrences: 116/20/9
root/24:                  81/55/9

A = 84×116 + 81 =  9,825
B = 84×20  + 55 =  1,735
C = 85×9        =    765
union              = 12,325 = 85×145
```

The nine-path C ownership category remains unchanged even though the three named `expected.control.*` paths are physically omitted from W24 worker envelopes. Structural omission validation does not create values, change ownership, change the denominator, or authorize root-to-worker projection. The exact 17 removed/deferred paths remain absent.

## Proposed RED lineage — no RED authority

A later separately authorized RED may use `driver.request.attachment_enabled` only to set the exact private enabled carrier for that occurrence. It must not derive, select, repair, or rewrite the bit from:

- any `expected` value, including `expected.control`;
- world, occurrence, fixture, or pair identity;
- commitment availability, identity, length, digest, bytes, predecessor cardinality, or predecessor order;
- request, transport, frame, result, error, terminal, owner, observation, or other content; or
- production output or prior RED output.

The RED must construct commitment and transport inputs from their independently owned driver fields. The enabled bit changes only the private enabled carrier. It must never alter expected data, world dispatch, pair selection, commitment content, request content, transport content, result content, or external pair behavior.

For the 69 physical-control occurrences, RED validation may compare C to the independently validated driver bit only as a fixture agreement check. For the 16 W24 workers, it validates physical omission and performs no C comparison. Expected data is never a runtime operand.

This section specifies proposed lineage only. It authorizes no RED file, RED mutation, execution, assertion result, or acceptance claim.

## Finite proposed production seam — no production authority

The finite proposed carrier remains one private boolean field and one exact value-returning setter:

```go
type callHierarchyLayer1RequestAttachment struct {
	// existing fields remain unchanged
	enabled bool
}

func (attachment callHierarchyLayer1RequestAttachment) privateCallHierarchyLayer1WithEnabled(enabled bool) callHierarchyLayer1RequestAttachment
```

The setter returns a copy with only `enabled` replaced and every other attachment field preserved exactly. The zero value remains disabled and the attachment remains directly comparable. No public field, parser, registry lookup, side table, options object, callback, content policy, or alternate attachment is proposed.

The accepted replacement lineage further proposes an attempt builder created only from the exact private enabled carrier:

```text
newCallHierarchyLayer1GenericAttemptLineage(req RoundTripRequest)
```

At the first executable line of `(*Manager).RoundTrip`, a false carrier keeps the attempt builder nil; a true carrier creates exactly one attempt-scoped builder. The same pointer is then proposed to pass explicitly through the finite attempt-builder lineage and to finalize through the replacement's refusal or admitted paths. Expected values, world/pair identity, commitment content, and execution output cannot create or replace that builder.

These field, setter, read, builder, transfer, finalization, and publication names are finite design proposals only. They establish no accepted source symbol, implementation, write set, C15 classification, carrier acceptance, writer, commit, activation, or production authority. A separately authorized source/prototype review must still approve the exact finite seam and every source change before production mutation.

## Exact corpus rematerialization boundary — no corpus authority

A later separately authorized v3 corpus rematerialization may change driver data only:

1. add required `driver.request.attachment_enabled` to every one of the exact 85 self-contained occurrences;
2. set it to false for exactly the 11 disabled W25 pair members and true for exactly the other 74 occurrences, solely from the accepted offline registry occurrence-kind mapping;
3. preserve every other decoded driver value exactly, including request, transport, derived, and commitment inputs;
4. preserve the complete expected subtree unchanged;
5. check agreement only for the 69 occurrences physically carrying `expected.control`;
6. preserve and structurally validate `expected.control` omission for all 16 W24 workers, without borrowing from `root/24`;
7. reject missing, null, non-boolean, duplicate, unknown, or differently derived alternatives to the driver bit;
8. canonicalize only the rematerialized v3 envelopes and recompute the affected v3 lengths, hashes, and manifest aggregates; and
9. leave v1 and v2 bytes, hashes, manifests, tests, and historical evidence immutable.

Any disagreement in the 69-occurrence C domain or any unexpected control presence in the 16-worker omission domain is a hard stop. This amendment does not authorize changing expected data to make a check pass.

No corpus bytes or rematerializer execution are authorized here.

## Exact future RED mutation boundary — no RED authority

If separately authorized, the future RED mutation scope is limited to the existing v3 RED and the exact v3 corpus/manifest paths separately authorized for rematerialization. It may add only:

- strict decode and all-85 structural validation of `driver.request.attachment_enabled`;
- the exact private enabled-carrier interface and value-returning setter call;
- 69 physical C agreement checks;
- 16 W24 physical-omission checks with no borrowing;
- guards proving the bit never mutates expected/world/pair/content inputs; and
- actual-execution assertions independently derived under the accepted replacement lineage.

RED authority must not mutate production source, v1 or v2 corpus/test artifacts, unrelated tests, generated catalogs, or historical qualification evidence. This amendment itself grants none of that future mutation authority.

## Review gate

Independent review must verify:

- exact clean pinned input `0d807a1d717a26fdda809975473bec3acea4039c`;
- this document is the only mutation;
- the blocked v1 attachment-enable proposal is superseded;
- exact all-85 required driver-bit presence and exact `11 false / 74 true` mapping;
- C agreement only on the 69 occurrences physically carrying `expected.control`;
- preserved omission and no C cross-check or borrowing for all 16 W24 workers;
- unchanged expected schema, `116/20/9` ownership, 145-path denominator, 17 absences, and 12,325 total assertions;
- RED use of the bit only to set the exact private enabled carrier, never expected/world/pair/content;
- finite proposed boolean field, exact value-returning setter, and accepted-replacement attempt-builder lineage, all explicitly without production authority;
- exact driver-only v3 rematerialization and bounded future RED mutation scope; and
- immutable v1/v2 corpus, tests, manifests, hashes, and historical evidence.

```text
ADR0011_CALLHIERARCHY_LAYER1_GENERIC_V3_ATTACHMENT_ENABLE_CAUSAL_INPUT_AMENDMENT_V2_REVIEW_GO — NOT ISSUED
STATUS — REVIEW_REQUIRED
CORPUS / RED / SOURCE / TEST / PROTOTYPE / PRODUCTION AUTHORITY — NOT ISSUED
```
