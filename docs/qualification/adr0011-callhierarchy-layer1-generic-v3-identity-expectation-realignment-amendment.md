# ADR-0011 call-hierarchy layer-1 generic v3 identity expectation realignment amendment

Status: **REVIEW_REQUIRED**

## 1. Scope, baseline, and non-authority

This documentation-only amendment was prepared against exact clean repository HEAD `cd83a04c9f67787a4bfe978991be7db6ce17db6a` in `/Users/schwa/dev/lsp-trace-adr0011-c17`.

It grants **no mutation authority** for corpus, RED, source, tests, runtime, activation, production, or acceptance. It does not mutate those artifacts. A separately authorized Stage 2 may use only the finite change set in §5, and only after review of this amendment.

The new independent causal authority is the accepted immutable ledger:

- ledger: `docs/qualification/adr0011-callhierarchy-layer1-generic-v3-identity-ledger-v1.json`;
- ledger file SHA-256 at this HEAD: `bfdad9b0e5505e40e23c52accb1ad53525c9245f0eef69f9b4f1b8ad50987667`;
- canonical `records` digest: `d626cbda96d875a2f3d0c04fa1a00d20c72481c5b74f742e0af8b5c4943802c5`;
- ledger authoring base HEAD: `bf8741551899882efa06c3bee847337c794b7503`;
- independent review disposition: `ADR0011_CALLHIERARCHY_LAYER1_GENERIC_V3_IDENTITY_LEDGER_GO`.

The independent review reproduced all 85 assignments from structure and exact JSON-typed request IDs without reading expected identity, method, world/pair semantic names, implementation behavior, sibling values, or mismatch diagnostics. Its post-verdict comparison found 38 binding disagreements; that comparison did not influence ledger production or acceptance.

## 2. Mechanical all-85 comparison

A fresh mechanical traversal at the stated HEAD joined each of the ledger's 85 records to exactly one physical corpus envelope by its exact `occurrence_address`, confirmed the exact JSON-typed `request_id`, and compared the ledger literals to that envelope's current downstream `expected.identity` values.

| Check | Result |
|---|---:|
| Ledger records | 85 |
| Unique joined corpus occurrences | 85 |
| Request-ID mismatches | 0 |
| `expected.identity.profile.value` mismatches | 0 |
| `expected.identity.request_identity.workspace_nonce_hex` mismatches | 0 |
| `expected.identity.binding.value` mismatches | 38 |
| Binding matches | 47 |
| Ledger bindings | 45 `opaque-incoming`; 40 `opaque-outgoing` |

Profile and workspace nonce expectations therefore remain unchanged. Their exact mismatch count is zero. The only stale expected values are the following 38 binding scalars.

## 3. Exact binding realignment inventory

Each address identifies the envelope through its `driver.request`; the allowed expected scalar is the sibling envelope path `expected.identity.binding.value`.

| # | Occurrence address | Request ID | Old expected binding | New ledger binding |
|---:|---|---:|---|---|
| 1 | `sessionruntime/testdata/callhierarchy-layer1-red-v3/worlds/world-04.json#/driver/request` | `104` | `opaque-incoming` | `opaque-outgoing` |
| 2 | `sessionruntime/testdata/callhierarchy-layer1-red-v3/worlds/world-05.json#/driver/request` | `105` | `opaque-outgoing` | `opaque-incoming` |
| 3 | `sessionruntime/testdata/callhierarchy-layer1-red-v3/worlds/world-07.json#/driver/request` | `107` | `opaque-incoming` | `opaque-outgoing` |
| 4 | `sessionruntime/testdata/callhierarchy-layer1-red-v3/worlds/world-08.json#/driver/request` | `108` | `opaque-incoming` | `opaque-outgoing` |
| 5 | `sessionruntime/testdata/callhierarchy-layer1-red-v3/worlds/world-11.json#/driver/request` | `111` | `opaque-incoming` | `opaque-outgoing` |
| 6 | `sessionruntime/testdata/callhierarchy-layer1-red-v3/worlds/world-12.json#/driver/request` | `112` | `opaque-incoming` | `opaque-outgoing` |
| 7 | `sessionruntime/testdata/callhierarchy-layer1-red-v3/worlds/world-15.json#/driver/request` | `115` | `opaque-incoming` | `opaque-outgoing` |
| 8 | `sessionruntime/testdata/callhierarchy-layer1-red-v3/worlds/world-18.json#/driver/request` | `118` | `opaque-incoming` | `opaque-outgoing` |
| 9 | `sessionruntime/testdata/callhierarchy-layer1-red-v3/worlds/world-19.json#/driver/request` | `119` | `opaque-incoming` | `opaque-outgoing` |
| 10 | `sessionruntime/testdata/callhierarchy-layer1-red-v3/worlds/world-20.json#/driver/request` | `120` | `opaque-incoming` | `opaque-outgoing` |
| 11 | `sessionruntime/testdata/callhierarchy-layer1-red-v3/worlds/world-23.json#/driver/request` | `123` | `opaque-incoming` | `opaque-outgoing` |
| 12 | `sessionruntime/testdata/callhierarchy-layer1-red-v3/worlds/world-24.json#/driver/request` | `124` | `opaque-outgoing` | `opaque-incoming` |
| 13 | `sessionruntime/testdata/callhierarchy-layer1-red-v3/worlds/world-24.json#/expected/concurrency/workers/1/driver/request` | `202` | `opaque-outgoing` | `opaque-incoming` |
| 14 | `sessionruntime/testdata/callhierarchy-layer1-red-v3/worlds/world-24.json#/expected/concurrency/workers/2/driver/request` | `203` | `opaque-outgoing` | `opaque-incoming` |
| 15 | `sessionruntime/testdata/callhierarchy-layer1-red-v3/worlds/world-24.json#/expected/concurrency/workers/4/driver/request` | `205` | `opaque-outgoing` | `opaque-incoming` |
| 16 | `sessionruntime/testdata/callhierarchy-layer1-red-v3/worlds/world-24.json#/expected/concurrency/workers/6/driver/request` | `207` | `opaque-outgoing` | `opaque-incoming` |
| 17 | `sessionruntime/testdata/callhierarchy-layer1-red-v3/worlds/world-24.json#/expected/concurrency/workers/9/driver/request` | `210` | `opaque-outgoing` | `opaque-incoming` |
| 18 | `sessionruntime/testdata/callhierarchy-layer1-red-v3/worlds/world-24.json#/expected/concurrency/workers/14/driver/request` | `215` | `opaque-outgoing` | `opaque-incoming` |
| 19 | `sessionruntime/testdata/callhierarchy-layer1-red-v3/worlds/world-24.json#/expected/concurrency/workers/15/driver/request` | `216` | `opaque-outgoing` | `opaque-incoming` |
| 20 | `sessionruntime/testdata/callhierarchy-layer1-red-v3/worlds/world-25.json#/driver/request` | `125` | `opaque-outgoing` | `opaque-incoming` |
| 21 | `sessionruntime/testdata/callhierarchy-layer1-red-v3/worlds/world-25.json#/expected/comparison/executions/2/driver/request` | `303` | `opaque-outgoing` | `opaque-incoming` |
| 22 | `sessionruntime/testdata/callhierarchy-layer1-red-v3/worlds/world-25.json#/expected/comparison/executions/4/driver/request` | `305` | `opaque-incoming` | `opaque-outgoing` |
| 23 | `sessionruntime/testdata/callhierarchy-layer1-red-v3/worlds/world-25.json#/expected/comparison/executions/5/driver/request` | `305` | `opaque-incoming` | `opaque-outgoing` |
| 24 | `sessionruntime/testdata/callhierarchy-layer1-red-v3/worlds/world-25.json#/expected/comparison/executions/6/driver/request` | `307` | `opaque-outgoing` | `opaque-incoming` |
| 25 | `sessionruntime/testdata/callhierarchy-layer1-red-v3/worlds/world-25.json#/expected/comparison/executions/12/driver/request` | `313` | `opaque-incoming` | `opaque-outgoing` |
| 26 | `sessionruntime/testdata/callhierarchy-layer1-red-v3/worlds/world-25.json#/expected/comparison/executions/13/driver/request` | `313` | `opaque-incoming` | `opaque-outgoing` |
| 27 | `sessionruntime/testdata/callhierarchy-layer1-red-v3/worlds/world-25.json#/expected/comparison/pairs/0/enabled/driver/request` | `301` | `opaque-incoming` | `opaque-outgoing` |
| 28 | `sessionruntime/testdata/callhierarchy-layer1-red-v3/worlds/world-25.json#/expected/comparison/pairs/1/disabled/driver/request` | `303` | `opaque-outgoing` | `opaque-incoming` |
| 29 | `sessionruntime/testdata/callhierarchy-layer1-red-v3/worlds/world-25.json#/expected/comparison/pairs/2/disabled/driver/request` | `305` | `opaque-incoming` | `opaque-outgoing` |
| 30 | `sessionruntime/testdata/callhierarchy-layer1-red-v3/worlds/world-25.json#/expected/comparison/pairs/3/disabled/driver/request` | `307` | `opaque-outgoing` | `opaque-incoming` |
| 31 | `sessionruntime/testdata/callhierarchy-layer1-red-v3/worlds/world-25.json#/expected/comparison/pairs/3/enabled/driver/request` | `307` | `opaque-outgoing` | `opaque-incoming` |
| 32 | `sessionruntime/testdata/callhierarchy-layer1-red-v3/worlds/world-25.json#/expected/comparison/pairs/5/disabled/driver/request` | `311` | `opaque-outgoing` | `opaque-incoming` |
| 33 | `sessionruntime/testdata/callhierarchy-layer1-red-v3/worlds/world-25.json#/expected/comparison/pairs/6/disabled/driver/request` | `313` | `opaque-incoming` | `opaque-outgoing` |
| 34 | `sessionruntime/testdata/callhierarchy-layer1-red-v3/worlds/world-25.json#/expected/comparison/pairs/6/enabled/driver/request` | `313` | `opaque-incoming` | `opaque-outgoing` |
| 35 | `sessionruntime/testdata/callhierarchy-layer1-red-v3/worlds/world-25.json#/expected/comparison/pairs/7/disabled/driver/request` | `315` | `opaque-outgoing` | `opaque-incoming` |
| 36 | `sessionruntime/testdata/callhierarchy-layer1-red-v3/worlds/world-25.json#/expected/comparison/pairs/7/enabled/driver/request` | `315` | `opaque-outgoing` | `opaque-incoming` |
| 37 | `sessionruntime/testdata/callhierarchy-layer1-red-v3/worlds/world-25.json#/expected/comparison/pairs/8/disabled/driver/request` | `317` | `opaque-incoming` | `opaque-outgoing` |
| 38 | `sessionruntime/testdata/callhierarchy-layer1-red-v3/worlds/world-25.json#/expected/comparison/pairs/9/disabled/driver/request` | `319` | `opaque-outgoing` | `opaque-incoming` |

## 4. Oracle direction and anti-feedback proof

`expected.identity.*` values are downstream comparison oracles. They are not causal inputs and do not gain authority merely because they already exist in accepted corpus bytes. Where an expected binding disagrees with the independently produced and reviewed immutable ledger, the expected binding is stale; the ledger is not wrong by definition and **must not be changed to satisfy expected**.

The causal direction is one-way:

```text
accepted immutable ledger literal
  -> explicit driver.request.identity literal
  -> real execution and private owner/view observation
  -> comparison against downstream expected.identity literal
```

It is never:

```text
expected.identity -> driver identity
method/direction -> driver or expected binding
mismatch diagnostic -> ledger rewrite
```

Anti-feedback is established by the ledger's production and review sequence:

1. all 85 assignments were frozen before the Stage 2 expected comparison;
2. assignment inputs were limited to exact structural address and exact JSON-typed request ID;
3. expected values, methods, semantic world/pair labels, implementation behavior, sibling values, and diagnostics were forbidden authoring inputs;
4. the independent review reproduced the 85-record bijection, 45/40 partition, and canonical records digest;
5. only after that verdict did a separate diagnostic identify the 38 stale expected bindings.

The ranking procedure is retained authoring provenance and an independent consistency check only. It is not a runtime rule and does not authorize deriving identity from request ID, address, method, or any other field during execution.

## 5. Exact allowed Stage 2 expected changes

If and only if separately authorized, Stage 2 may perform one simultaneous corpus rematerialization with both of these inseparable parts:

1. insert one explicit `driver.request.identity` object into each of all 85 physical envelopes, containing the three direct literals from that occurrence's immutable ledger record:
   - `profile = "corpus-profile-go"`;
   - `binding =` that record's exact `binding_identity` literal;
   - `workspace_nonce_hex = "0101010101010101010101010101010101010101010101010101010101010101"`;
2. change exactly the 38 `expected.identity.binding.value` scalars enumerated in §3 from the stated old value to the stated new ledger value.

The 38 expected changes are allowed only simultaneously with explicit all-85 driver identity insertion. Expected values must not be changed first, independently, or used to populate the driver. The ledger literals must populate the driver directly; after execution, expected remains only the comparison oracle.

No other expected path may change. In particular:

- zero `expected.identity.profile.value` changes;
- zero `expected.identity.request_identity.workspace_nonce_hex` changes;
- zero method changes;
- zero generation, request key, session, transaction, availability, frame, result, error, external, control, lifecycle, accounting, commitment, transport, terminal, refusal, concurrency, comparison, or round-trip-ledger expected changes;
- no method-derived, direction-derived, name-derived, world-derived, pair-derived, sibling-derived, request-ID-derived, registry-derived, runtime-derived, or expected-derived identity rewrite;
- no wildcard, default, inheritance, shared expansion, fallback, or hidden side table;
- no ledger mutation, regeneration, repair, or successor substitution to make it agree with current expected values.

Any mismatch outside this exact finite set is a hard stop and requires new separately reviewed authority.

## 6. Corpus-byte review obligations

Current pre-mutation corpus pins at HEAD `cd83a04c9f67787a4bfe978991be7db6ce17db6a` are:

- manifest: 4,733 bytes, SHA-256 `7d381e2a81c98d9af678eb7f5a38ff42630928e5baf8b435c4ef964a904b056f`;
- ordered concatenation of the exact 25 world files: 582,734 bytes, SHA-256 `b195c00a1325340a3c400bccafc1dc641ce7858f310c1c2b23b7e6cb7bf9d3af`;
- ledger canonical records digest: `d626cbda96d875a2f3d0c04fa1a00d20c72481c5b74f742e0af8b5c4943802c5`.

These are pre-mutation evidence, not target hashes. A future authorized rematerialization necessarily changes affected world bytes and therefore per-world pins, aggregate length/digest, and manifest bytes/digest. Its independent corpus-byte review must, without trusting generated totals:

1. strictly decode and enumerate exactly 85 physical envelopes: 25 roots, 16 W24 workers, 22 W25 standalone executions, and 22 W25 pair members;
2. prove a bijection with all 85 immutable ledger records using exact occurrence address and exact JSON-typed request ID;
3. recompute and match the immutable ledger records digest;
4. prove exactly 85 explicit driver identity insertions, each equal to its direct ledger literals;
5. prove exactly 38 and only 38 expected decoded-value changes, all at the listed `expected.identity.binding.value` paths;
6. prove zero profile mismatches, zero workspace-nonce mismatches, and zero request-ID mismatches;
7. prove every non-identity driver value and every non-listed expected value remains decoded-value identical to this pre-mutation baseline;
8. reproduce every changed world byte length/digest, the ordered aggregate length/digest, and the rebuilt manifest bytes/digest;
9. preserve secure path, canonical serialization, world order, no-extra/no-missing-file, and immutable predecessor-tree checks; and
10. fail closed on any extra path, count, type, value, hash, address, or join difference without changing the ledger.

The earlier corpus-byte review's historical hashes remain true for its reviewed bytes but are not the present HEAD baseline and must not be reused as post-rematerialization claims.

## 7. RED consequences

No RED change is authorized here. If Stage 2 is separately authorized, the existing RED must initially fail for the intended causal reason: the current corpus lacks the mandatory explicit all-85 `driver.request.identity` input, and the 38 stale binding expectations disagree with the accepted ledger-backed causal values.

The permitted eventual RED behavior is:

- strict validation of exactly one explicit three-literal identity object per physical envelope;
- construction of production identity only from those driver literals;
- all 85 real executions before expected comparison;
- post-execution comparison of actual owner/view identity to expected identity;
- mutation guards proving profile, binding, and nonce independently alter only their owned actual identity projections; and
- guards proving expected values and methods cannot feed, select, repair, normalize, or validate causal driver identity.

A test or fixture helper that creates expected from driver, driver from expected, both from a shared method mapping, or either from execution output invalidates the anti-feedback proof. Any GREEN obtained through such a path is not qualifying GREEN.

## 8. Review gate

Disposition remains:

```text
REVIEW_REQUIRED
NO_MUTATION_AUTHORITY
```

Review must confirm the 85-record join, exact 38-row inventory, 0/0 profile-and-nonce mismatch counts, current byte pins, immutable-ledger custody, and the simultaneous finite Stage 2 boundary before any corpus, RED, source, or test mutation is considered.
