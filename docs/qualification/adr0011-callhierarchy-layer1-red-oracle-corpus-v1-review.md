# ADR-0011 Call Hierarchy Layer-1 RED Oracle Corpus v1 — Independent Review

**Verdict: `ADR0011_CALLHIERARCHY_LAYER1_ORACLE_CORPUS_DESIGN_BLOCKED`**

Reviewed repository state: exact HEAD `4f2b5504c1e4f4e559f48db7ab8e0c5b8ca70fa9`, the untracked design `docs/qualification/adr0011-callhierarchy-layer1-red-oracle-corpus-v1.md`, and the pre-existing 454-line modification to `sessionruntime/b4_owned_method_pair_lease_test.go`.

This verdict does not authorize edits to the design, RED, source, testdata, or commit. It does not authorize corpus bytes.

## Decision

A narrow test-only corpus amendment is justified in principle, and its only acceptable future write scope is exactly:

```text
sessionruntime/testdata/callhierarchy-layer1-red-v1/**
```

The separate corpus is necessary to keep manually authored frame bytes, byte spans, digests, availability forms, and ledger values independent from runtime attachment serialization and production behavior. The proposed design correctly excludes generated/update fixtures, runtime recording, production authority, and files outside that directory.

The design is nevertheless **not frozen enough to author independent corpus bytes**. The following defects are blocking.

## Blocking defects

### B1. The purportedly fixed inputs are not supplied or immutably pinned

**Basis:** Design §8 step 2 says corpus identities come from a “reviewed world worksheet,” but no worksheet or literal per-world identity table is present. The design does not freeze request IDs, workspace nonce, session, generation, request-key form/value, local transaction digest, profile, binding, predecessor identity/ordinal, selected ordinal, successor identity, or per-worker transaction identities.

**Defect:** An author must invent values, inspect runtime values, or create an additional authority. Independent authors cannot derive identical bytes from the accepted protocol, fixed fake inputs, and accounting/refusal policy alone.

**Required correction before GO:** Put every corpus-only identity and request ID into an immutable literal table in the design, including all 16 concurrency identities and every selected/handoff/predecessor identity used by every world. Pin the table itself.

### B2. The source pins do not pin the actual RED or all fixture inputs

**Basis:** `provenance.baseline_head` pins tracked source at HEAD, but the 454-line RED is an uncommitted modification and therefore is not identified by that commit. The provenance fields name source locations but provide no content digest or exact line-range digest. The design also refers generally to “the immutable RED setup visible at the baseline,” although that setup is not contained in the baseline commit.

**Defect:** A future author cannot prove that the RED and fake-input literals used for manual derivation are the reviewed inputs. Location strings are not immutable source pins.

**Required correction before GO:** Freeze SHA-256 and byte length for the exact RED diff/content and each accepted fixture/source excerpt, or include their exact accepted bytes in the design. State how line-ending normalization is handled; preferably hash repository blob/diff bytes without normalization.

### B3. Exact frame bytes are not derivable because request serialization is underspecified

**Basis:** Design §8 requires manually constructing JSON-RPC requests in the fixture’s “declared key order,” but the fixture does not declare a complete corpus request-body literal or a corpus-specific key-order table. It also does not freeze every request ID. `Content-Length` and all downstream offsets, lengths, and digests depend on those missing choices.

**Defect:** The design freezes a recipe, not the exact input strings needed to execute it uniquely. Multiple compact JSON bodies can satisfy the semantic request while producing different bytes.

**Required correction before GO:** Provide the exact request-body byte string, or a complete literal field-order and escaping rule plus every literal field value, for each distinct request form. Do the same for cancellation frames and every writer-generated response/error body whose exact bytes are expected.

### B4. Exact failure and error expectations are incomplete

**Basis:** Only the protocol error fixes code `-32603` and message `boom`. Cancellation, deadline, shutdown, transport-unavailable, malformed, and refusal worlds do not freeze all required `error` members (`available`, `present`, `class`, `code`, `message`, `data_form`, `data_base64`, `serialized_hex`) and exact `external.failure`. The schema says absent external server errors serialize as `null`, but does not completely specify the interaction among `error.available`, `error.present`, `present` availability, and `serialized_hex` for every failure/refusal world.

**Defect:** Exact world JSON cannot be authored without choosing values not fixed by the design or observing current behavior.

**Required correction before GO:** Add a complete per-world literal error/failure table covering every required field, including exact strings and exact serialized bytes.

### B5. Complete accounting and ledger values are not frozen

**Basis:** The design defines equations and defaults but supplies no literal per-world `before`, `after`, `delta`, `available`, and `known_zero` values for all dimensions. It likewise omits explicit per-world values for observation `bytes`, `diagnostics`, `objects`, and `work`; transport `bytes`, `messages`, `request_bytes`, and `request_messages`; terminal before/after; and external byte counts. “Known-zero unless explicitly exercised” does not freeze the nonzero baselines. The concurrency world specifies 16 workers but no per-transaction or aggregate ledger vector.

**Defect:** `delta` can sometimes be derived, but `before` and `after` cannot be derived from frames alone. Sampling current counters is forbidden, so the missing values have no admissible derivation source.

**Required correction before GO:** Supply a complete literal ledger/accounting vector for every world and every concurrent transaction, with the manual equation beside each value. If absolute baselines are intentionally not protocol facts, revise the schema to freeze only admissibly derivable quantities rather than inventing absolute values.

### B6. The world inventory names coverage but does not freeze every world’s complete expected value

**Basis:** The 25 names cover success, protocol/malformed failure, cancellation/deadline/shutdown/transport unavailability, nil/empty/nonempty payloads, mutation, prepare/outgoing separation, seven refusals, concurrent lifecycle, and enabled/disabled controls. However, §7 gives distinguishing summaries rather than a complete value assignment for every schema field. The enabled/disabled aggregate does not identify the eleven primary worlds in a literal list or freeze each pair’s complete expected external and attachment state. Mutation and lifecycle worlds do not freeze all frames, identities, terminal states, accounting values, and commitments.

**Defect:** Coverage is present, but byte-authorable expected worlds are not. An author must fill unspecified fields by judgment or runtime observation.

**Required correction before GO:** Add a complete per-world worksheet assigning every required schema field, with no “same as expected/current/control” shorthand unless the referenced immutable value is identified exactly.

### B7. The schema contains an impossible identity reference

**Basis:** Design §5.1 defines `transaction` as `{available, local_hex, request_key}`. Design §5.2 then requires frame `request_identity` to equal `identity.transaction.request` when available. No `request` member exists in the declared transaction schema. The text also calls `request_identity` a “complete identity tuple” without enumerating its object schema.

**Defect:** Two conforming authors cannot agree on the JSON shape, and a strict loader cannot enforce the stated equality.

**Required correction before GO:** Define one exact request-identity object schema and either add it to `transaction` or change the equality to an existing field. Enumerate every member in canonical key order and define all availability constraints.

### B8. Availability/form rules are not complete for every nested object

**Basis:** The design gives general rules, but several objects lack exhaustive cross-field validity tables: frame/body/token forms; request-key unavailable/number/string combinations; result available versus nil/empty/nonempty; error available/present/data combinations; external availability versus failure/notifications/responses; terminal availability; selected/handoff availability; and refusal identity suppression.

**Defect:** Invalid and ambiguous combinations remain representable, so “unknown fields forbidden” does not make the schema immutable.

**Required correction before GO:** Freeze an exhaustive validity matrix for every tagged union and availability-bearing object, including which unavailable fields are `null`, `0`, `""`, or forbidden. Every world must select one row explicitly.

### B9. Selected and handoff commitments violate the requested Layer-1 opaque boundary

**Basis:** The RED’s request attachment intentionally limits `callHierarchyOpaqueCommitmentMetadata` to identity, SHA-256, length, and availability. In contrast, design §5.6 exposes selected `exact_bytes_hex`, ordinal, source frame/request, and source offset; handoff exposes nested selected content, successor bytes, successor request/transaction, and successor offset. Design line 178 explicitly says tests compare “identity, order, bytes, offsets, lengths, and digest.”

**Defect:** That is broader than the required Layer-1 commitment boundary of identity/digest/length only. It promotes semantic/custody internals from the observation layer into this Layer-1 oracle and creates direction/sequencing commitments the narrow corpus was told not to add.

**Required correction before GO:** For Layer 1, reduce selected and handoff commitment expectations to `{available, identity, sha256, length}` (with exact availability rules). Do not include raw commitment bytes, decoded content, ordinals, source/successor spans, or nested semantic identities in this corpus. If those fields belong to a higher accepted layer, qualify them separately rather than importing them here.

### B10. Predecessor commitments are not generic transport identities

**Basis:** The design requires predecessor `method`, `request_key`, `request_ordinal`, transaction, and request identity, while the RED supplies semantic identity `"document-symbol"` and method `"textDocument/documentSymbol"`. The design does not establish that these semantic labels are permitted generic transport identities, nor does it constrain predecessor exposure to an opaque identity/digest/length representation.

**Defect:** The predecessor shape leaks semantic phase and direction information across the Layer-1 boundary and is not frozen as a generic transport identity.

**Required correction before GO:** Either remove predecessor commitments from Layer 1 or represent each only by an explicitly generic transport identity plus digest and length. If transaction identity is permitted by the accepted protocol, define it as generic transport custody and exclude method, call-hierarchy direction, decoded payload, and semantic phase labels.

### B11. Canonical manifest construction is circularly incomplete without literal world bytes

**Basis:** The manifest procedure is sound only after exact canonical world files exist, but B1–B10 prevent deterministic world bytes. The future loader literal is therefore also not derivable. The design does not provide a complete manual derivation ledger mapping each expected value to one accepted authority.

**Defect:** Two independent corpus authors cannot be expected to produce identical world files, hashes, manifest bytes, and manifest digest.

**Required correction before GO:** After correcting the schema and per-world tables, include a value-level derivation ledger: field/path, literal expected value or deterministic formula, exact accepted source pin, and independent arithmetic/hash check. Corpus creation must remain manual and must not use Manager, RoundTrip, observation leases, current attachment serialization, generated fixtures, update flags, or recording.

## Coverage determination

The design names all requested world classes, but naming a world is not equivalent to freezing it. The success/failure/refusal/payload/mutation/separation/lifecycle/concurrency/enabled-disabled matrix is therefore **coverage-complete by label but expectation-incomplete by value**.

The canonical JSON rules, ordering, null distinctions, manifest structure, no-update rule, source/runtime authority separation, and narrow test-only directory are directionally correct. They do not cure missing literals, missing source digests, incomplete tagged-union constraints, incomplete ledger vectors, or the Layer-1 opacity violation.

## Authorization boundary

`ADR0011_CALLHIERARCHY_LAYER1_ORACLE_CORPUS_DESIGN_GO` is **not issued**.

This BLOCKED verdict requires design correction and a new independent review. It authorizes no corpus bytes, no loader, no RED edit, no behavior change, no testdata creation, and no commit.

## Model interpretation

The review treats “frozen enough to author bytes” as requiring two independent authors, using only accepted protocol text, immutably pinned fake inputs, and frozen accounting/refusal rules, to produce byte-identical corpus files without observing runtime or current attachment serialization.

Key assumptions:

- The requested Layer-1 opaque boundary controls over broader semantic structures visible in the preexisting RED.
- A source pathname at a commit does not pin an uncommitted RED modification.
- A derivation recipe is insufficient when literal inputs or absolute ledger baselines remain unspecified.

Gaps and verification items:

1. Confirm the accepted protocol permits any predecessor transport identity beyond opaque identity/digest/length.
2. Re-review a corrected exhaustive tagged-union matrix and per-world literal worksheet.
3. Have two independent authors derive one representative success world and one refusal world before authorizing the full corpus.

Improved framing: qualify the design only when every corpus byte is a deterministic function of an explicitly pinned authority and the Layer-1 schema contains no semantic commitment beyond opaque custody metadata.
