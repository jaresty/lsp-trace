# ADR-0011 Call-Hierarchy Layer-1 Generic V2 Oracle/Assertion Ownership Amendment — Independent Review

Status: **BLOCKED**

- Repository: `/Users/schwa/dev/lsp-trace-adr0011-c17`
- Reviewed HEAD: `8ff8bdf617f514f4f1eadc2e6e77ceb78758ad66`
- Reviewed amendment: `docs/qualification/adr0011-callhierarchy-layer1-generic-v2-oracle-assertion-ownership-amendment.md`
- Review mutation: this document only
- Corpus authority: **not issued**
- RED mutation authority: **not issued**
- Production writer authority: **not issued**
- Commit authority: **not issued**

## Independent method and authority boundary

This review independently parsed the normative 162-row registry in `docs/qualification/adr0011-callhierarchy-layer1-generic-corpus-red-v2-amendment.md`, parsed the amendment's A/B/C inventories, and compared the resulting sets without treating the amendment's stated totals as authority. It then inspected the immutable v2 driver/expected schema and causal RED execution path in `sessionruntime/b4_callhierarchy_layer1_generic_v2_test.go`.

The classification rule applied here is:

- A is admissible only for an actual production-owned generic observation derived from authorized Manager request, response, terminal, accounting, or opaque-commitment inputs;
- B is admissible only for an actual assertion independently computed by the RED from production observations or actual external/lifecycle execution, never stored in production and never compared expected-to-expected;
- C is admissible only for fixture/control organization and may not dispatch production behavior.

Names do not establish ownership. Corpus values remain expected values and cannot become causal production inputs merely because their paths are assigned to A.

## Mechanical partition result

The partition is mechanically exact:

```text
normative rows                         162
PRESERVE GENERIC                       133
PRESERVE OPAQUE                         12
REMOVE/DEFER                            17
retained expected denominator          145

A listed / unique                      117 / 117
B listed / unique                       20 / 20
C listed / unique                        8 / 8
A ∩ B                                    0
A ∩ C                                    0
B ∩ C                                    0
A ∪ B ∪ C                              145
retained paths missing from partition    0
partition paths outside retained set      0
removed paths admitted by partition       0
```

The 17 removed/deferred paths remain exactly absent from A/B/C:

- `frames.request.body.{available,bytes_hex,length,sha256}`;
- `frames.request.payload.{available,bytes_hex,length,sha256}`;
- `frames.response.body.{available,bytes_hex,length,sha256}`;
- `frames.response.payload.{available,bytes_hex,length,sha256}`; and
- `result.form`.

The corpus and its expected schema are unchanged by this review.

## Semantic assignment challenge

### Supported assignments

- `name`, `comparison.available`, `concurrency.available`, and the `control.*` paths are correctly excluded from the production view. Their permitted use is fixture shape, naming, pairing, classification, and harness organization only. The amendment correctly forbids passing them to `Manager.RoundTrip` or consulting them in production code.
- The `derived.*` paths and `lifecycle.*` paths are correctly assigned to B in principle. Their formulas use actual result, acquisition, view, callback, release, external, and identity observations; the amendment expressly prohibits expected operands and production storage.
- `accounting.*`, `transport.*`, and `round_trip_ledger.*` are supportable as A only as actual Manager observations. The amendment correctly keeps aggregate arithmetic and relationship checks in the RED rather than making corpus arithmetic production state.
- Opaque `commitments.*` are semantically capable of being A fields only when they originate in independently authorized request attachment inputs and production stores only defensive opaque metadata, never expected values or preimage semantics.
- World-25 transport/external equality is correctly specified as a post-execution relationship over two actual executions rather than a production field. World-24 arithmetic is likewise correctly specified as a RED aggregation over actual worker ledgers.

These supported assignments do not cure the two unresolved causal-source defects below.

## Blocking defects

### 1. Commitment A fields have no exact independent causal input source

The amendment forbids constructing production commitments from `expected.commitments`, C metadata, world identity, or a hidden table. That prohibition is necessary. The immutable RED presently violates the intended successor rule at `sessionruntime/b4_callhierarchy_layer1_generic_v2_test.go:594-597`: it constructs the request attachment's selected and handoff commitments directly from `record.Expected.Commitments`.

The immutable v2 driver schema contains request, transport-event, and derived-external inputs, but no selected, handoff, or predecessor commitment input. The amendment says that “independently authorized request attachment inputs” must be supplied before execution, but it does not identify an exact immutable source, schema, per-record mapping, availability rule, or derivation for those inputs. The corpus is declared immutable, and the amendment also forbids a hidden side table. Consequently the successor RED cannot presently construct the production input needed to compare the A `commitments.*` fields without either:

1. continuing the forbidden expected-to-production feed;
2. dispatching from C/world labels; or
3. introducing an unspecified external mapping or hidden table.

That is an authority and derivability gap, not an implementation detail. Before GO, the amendment must name and freeze an independent causal commitment-input source for every executed envelope, including enabled/disabled availability and predecessors, or reclassify/remove the affected ownership claim through a separately authorized corpus/schema amendment. Expected commitment values may remain comparison operands but may not be their own production-input preimage.

### 2. `external.execution_expected` is not demonstrated as an A production observation

`external.execution_expected` is assigned to A, but its name is not the defect; its missing actual source is. The retained corpus type defines it as an expected boolean, and the retained RED historically validates the expected value rather than deriving a Manager-produced “execution expected” observation. The proposed mapping says to retain external execution “availability/expectation,” but it does not identify an authorized Manager request/result/transport fact from which the expectation bit is produced independently.

Whether external execution actually occurred is independently observable from the execution and therefore can be a B assertion. Whether execution was expected is fixture/control intent and therefore can be C metadata. Neither proposition is an actual production observation merely because it is grouped under `external.*`. Without an exact production source and semantics distinct from fixture expectation, this path fails A's production-generic-only rule and makes the stated `117/20/8` semantic partition invalid even though the sets are mechanically exact.

Before GO, classify the proposition explicitly and adjust the finite partition/counts:

- B if the path means an independently computed actual-execution predicate;
- C if it means fixture-declared expectation; or
- A only if a concrete authorized Manager-produced generic fact, availability rule, and storage justification are identified without consulting expected or C data.

## Control and dispatch boundary

`control.enabled` may organize which independently authored causal request is submitted by the harness, but it cannot manufacture that request's attachment values. Until the missing commitment-input source is frozen, saying that the label selects an independently authored enabled/disabled request is circular: the independently authored attachment input has not been specified anywhere outside expected data. The no-world-dispatch proof obligations are otherwise appropriately strict, but they cannot prove independence for an undefined causal input.

The current RED is not evidence that the corrected ownership rule already exists: it compares all 145 expected paths against the production view and sources commitments from expected data. The amendment properly proposes replacing that behavior with `A=117` field comparisons, `B=20` independent computations, and `C=8` structural validations, but no RED edit or implementation is authorized until the blockers above are corrected and independently reviewed.

## Required correction

A successor amendment and independent review must:

1. preserve the mechanically exact 145-path accounting and all 17 absences;
2. freeze an exact non-expected, non-C, non-world-dispatched causal source and mapping for every commitment request input used by every actual execution;
3. re-adjudicate `external.execution_expected` by proposition and authority, then update the exact A/B/C inventories and counts;
4. preserve B as independently computed actuals with no production storage and no expected self-comparison;
5. preserve C as fixture-only organization with no production dispatch;
6. retain exact per-record accounting, no skips, immutable corpus bytes, and no fallback, updater, or golden rewrite; and
7. require another independent semantic and mechanical review before RED mutation, implementation, production custody, or commit authority.

## Disposition

`ADR0011_CALLHIERARCHY_LAYER1_GENERIC_V2_ORACLE_ASSERTION_OWNERSHIP_BLOCKED`

The exact partition passes mechanical set reconciliation, but the commitment-input causality gap and unsupported A ownership of `external.execution_expected` prevent semantic ownership GO. This review grants no corpus, RED, fixture, test, source, generated-artifact, implementation, C15, activation, production-writer, or commit authority.
