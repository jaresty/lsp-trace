# ADR-0011 Call-Hierarchy Layer-1 Generic V3 Exhaustive Causal-Lineage Inventory — Independent Review

Status: **BLOCKED**

- Reviewed repository: `/Users/schwa/dev/lsp-trace-adr0011-c17`
- Reviewed HEAD: `4add2ac3f9cf556789869067b35170dd2c9f6276`
- Reviewed inventory: `docs/qualification/adr0011-callhierarchy-layer1-generic-v3-exhaustive-causal-lineage-inventory.md`
- Additional untracked inputs reviewed:
  - `docs/qualification/adr0011-callhierarchy-layer1-generic-v3-attachment-enable-causal-input-amendment.md`
  - `docs/qualification/adr0011-callhierarchy-layer1-generic-v3-attachment-enable-causal-input-amendment-review.md`
- Review mode: independent documentation-only mechanical and semantic review
- Mutation by this review: this document only
- Corpus, RED, write-set, source, test, prototype, generated-artifact, execution, writer, commit, activation, semantic, and production authority: **not issued**

## Review method and authority ceiling

This review compared the proposed inventory against the committed v3 causal-input contract and independent review, committed corpus-bytes review, committed RED source, reviewed raw-response and semantic-residue amendments, and reviewed disposable C15/write-set documents. Counts were reconstructed as finite sets and then challenged semantically. No count, hash, successful decode, existing expected value, fixture label, disposable result, or prior GO was treated as proof of a lawful production carrier.

The inventory is a documentation artifact. It cannot accept attachment enablement, production implementation, semantic commitment validity, or production authority.

## Mechanical retained-path and occurrence result

The inventory's retained-path partition is mechanically consistent with the committed reviewed contracts:

```text
ordinary occurrence:  A=116, B=20, C=9, total=145
World-24 root:         A=81,  B=55, C=9, total=145
removed/deferred:      17 paths
```

The World-24 exception is exact: only the 35 members of seven ledger dimensions times five fields migrate from A to B at `root/24`; they remain A for all 16 World-24 workers and every other occurrence.

The occurrence census is also exact:

```text
25 roots
16 World-24 workers
22 World-25 executions
22 World-25 pair members
85 total occurrences
```

The inventory's occurrence-keyed arithmetic reproduces:

```text
A = 84*116 + 81 = 9,825
B = 84*20  + 55 = 1,735
C = 85*9        =   765
union                 = 12,325 = 85*145
```

The 17 globally removed/deferred schema paths are identified exactly as the four request-body, four request-payload, four response-body, four response-result paths, plus `result.form`. The inventory does not admit any of them into the retained 145.

This is a **mechanical PASS**, not the requested final GO. The accepted contracts classify schema paths occurrence-sensitively; physical optional-field absence must remain distinct from ownership classification. In particular, all 16 World-24 workers physically omit `expected.control`, and the inventory correctly does not manufacture or borrow those values.

## Five-element lineage review

The inventory supplies the five required columns for each A, B, C, and World-24 family:

1. independently frozen driver input or actual observable source;
2. production transfer seam or `NONE`;
3. resulting observation/classification;
4. unavailable, disabled, nil, empty, order, or cardinality semantics; and
5. anti-feed boundary.

All B and C rows state `NONE`, as required. The 35 World-24 aggregate paths also state `NONE` and derive only from the 16 independently executed worker actual ledgers.

However, not every A row supplies an exact production symbol/file lineage. Several use family-level descriptions rather than an exact seam:

- accounting: “existing Manager accounting primitives” and `S-OWNER via S-RESULT`;
- error/external/refusal/transport: broad `(*Manager).RoundTrip` descriptions without the exact producing field/method branch;
- terminal and terminal-ledger: multiple broad owner/result seams without a one-to-one producing symbol for each outcome;
- ordinary ledger dimensions: “existing Manager ledger primitives” rather than exact before/after snapshot symbols.

Those descriptions are useful inventory groupings, but they do not satisfy an exact production-seam proof for every conditional outcome. They must not be interpreted as accepted carrier authority. This precision gap is secondary to, and does not cure, the substantive attachment blocker below.

## Conditional-branch coverage

The inventory explicitly lists the requested branch families:

- attachment enabled/disabled and owner publication/absence;
- selected, handoff, and ordered predecessor commitment availability, cardinality, and order;
- request params nil/empty/present;
- valid, malformed, and no-response-frame cases;
- result unavailable/available-empty/present;
- error unavailable/present;
- refusal false/true;
- terminal unavailable/value;
- accounting, transport, external, and ledger availability/known-zero/value behavior;
- acquisition, view, callback success/error/panic containment, release, concurrent drain, terminal closure, and defensive copying;
- World-24 worker aggregation; and
- World-25 disabled/enabled pairing plus actual-to-actual transport and external equality.

The nil/empty and unavailable/known-zero distinctions are preserved. Malformed emitted response bytes remain raw custody; control-event-only no-frame cases do not fabricate a response frame; available-empty external response sequences remain distinct from unavailable. Predecessor order and unavailable members remain literal. World-24 aggregation propagates unavailability rather than substituting zero. World-25 equality is computed only after two independent executions.

No expected value, world/name/pair label, commitment content, hidden registry/runtime side table, sibling actual, or normalized content is authorized as an execution operand. The inventory preserves the no expected/world/pair/content/hidden-feed boundary.

## Production-owned conditional outcomes lacking a lawful accepted carrier

### 1. Attachment enablement and disabled publication

The production-owned decision “publish the private Layer-1 generic owner/observation or publish none” lacks a lawful accepted causal carrier for the disabled W25 pair members.

The committed v3 driver has no `driver.request.attachment_enabled` field. The committed RED constructs the attachment from selected, handoff, and predecessor commitment inputs and has no accepted enable operand. Commitment availability and content are expressly non-policy and cannot supply this decision. Expected controls, pair labels, occurrence IDs, names, worlds, transport content, result content, and hidden state are forbidden operands.

The proposed `driver.request.attachment_enabled` plus `S-ENABLE` would be a finite independent candidate carrier, with the deterministic mapping `11 false / 74 true`, but it is untracked, unaccepted, absent from frozen corpus bytes, absent from the committed RED, absent from accepted production source authority, and blocked by its own independent review.

Therefore these dependent production-owned outcomes remain **BLOCKED**:

- disabled request publishes no private generic owner;
- disabled request exposes no private generic observation/lease;
- enabled request is eligible to publish independently of all commitment availability/content;
- W25 disabled/enabled private-publication contrast while external transport remains equal.

### 2. Exact per-outcome production lineage

For accounting, error, external, refusal, terminal, transport, and ordinary ledger outcome branches, the inventory identifies plausible existing broad runtime sources but does not bind every outcome to an exact producing symbol/file branch. Until that lineage is made exact and checked against the accepted final production write set, those family-level descriptions cannot independently establish lawful accepted carriers.

This is a documentation-lineage omission, not authority to add source. It does not imply that the generic Manager lacks all such behavior; it means the inventory has not proved the exact carrier at the required granularity.

No other carrier may be invented from commitments or expected values. In particular, commitment availability cannot cause refusal, error, terminal, lifecycle, or ledger policy.

## Attachment amendment and World-24 cross-rule blocker

The proposed amendment requires all 85 occurrences to validate:

```text
expected.control.enabled
expected.control.attachment_available
expected.control.observation_available
```

against the independently materialized enable bit while also preserving all expected values, path accounting, and absences. That universal rule is unsatisfiable because every World-24 worker physically omits `expected.control`.

The inventory correctly preserves this blocker. It does not:

- add the three controls to workers;
- borrow the root's controls;
- treat absence as false;
- weaken “all 85” silently to “where present”; or
- use C metadata to derive the enable bit.

A corrected amendment must define an exact occurrence-sensitive validation domain that preserves W24 absence, or separately authorize and account for exact expected-path additions. Until corrected and independently re-reviewed, `attachment_enabled` and `S-ENABLE` remain **BLOCKED**.

## Semantic disposition

The inventory does not infer commitment validity, direction, target, relation, source completeness, producer authenticity, qualification, feature identity, publication authority, or production use. It correctly treats opaque commitment metadata as finite custody/equality input only.

Nevertheless, exhaustive mechanical accounting is insufficient. The accepted production carrier for attachment enablement is missing, the containing amendment has an unresolved World-24 cross-rule contradiction, and several A families do not yet identify exact per-outcome symbol/file lineage.

Exact omissions blocking GO:

1. no accepted self-contained `driver.request.attachment_enabled` in the frozen v3 corpus;
2. no accepted RED decode/validation and driver-only request construction for that bit;
3. no accepted production `enabled bool` field, value-returning setter, admission read, and owner-publication propagation;
4. no satisfiable occurrence-sensitive C agreement rule for the 16 World-24 workers that omit `expected.control`;
5. no independently reviewed replacement amendment closing items 1–4;
6. no accepted final production write-set/writer authority for the enable seam; and
7. no exact symbol/file producing-seam binding for each conditional member of the broad accounting/error/external/refusal/terminal/transport/ordinary-ledger A families.

## Final disposition

```text
ADR0011_CALLHIERARCHY_LAYER1_GENERIC_V3_CAUSAL_LINEAGE_INVENTORY_GO — NOT ISSUED
ADR0011_CALLHIERARCHY_LAYER1_GENERIC_V3_CAUSAL_LINEAGE_INVENTORY_BLOCKED
```

The inventory passes finite path, occurrence, ownership, World-24 migration, and branch-enumeration review, but it does not pass semantic carrier acceptance. Production authority remains blocked regardless of this review.
