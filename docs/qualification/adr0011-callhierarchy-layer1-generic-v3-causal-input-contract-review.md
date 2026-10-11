# ADR-0011 Call-Hierarchy Layer-1 Generic V3 Causal-Input Contract — Independent Review

Status: **GO**

- Repository: `/Users/schwa/dev/lsp-trace-adr0011-c17`
- Reviewed HEAD: `8ff8bdf617f514f4f1eadc2e6e77ceb78758ad66`
- Reviewed contract: `docs/qualification/adr0011-callhierarchy-layer1-generic-v3-causal-input-contract.md`
- Reviewed registry: `docs/qualification/adr0011-callhierarchy-layer1-generic-v3-causal-input-registry.json`
- Preserved blocked inputs: `docs/qualification/adr0011-callhierarchy-layer1-generic-v2-oracle-assertion-ownership-amendment.md` and `docs/qualification/adr0011-callhierarchy-layer1-generic-v2-oracle-assertion-ownership-amendment-review.md`
- Review mutation: this document only
- Corpus/RED/source/implementation/C15/activation/commit authority: **not issued**

## Independent method

This review parsed the canonical v2 162-row path registry and the blocked v2 A/B/C inventories as finite sets; parsed the v3 registry without trusting its stated totals; independently regenerated every available commitment from the frozen seed, occurrence ID, role, and index; independently recomputed availability and predecessor cardinality from global ordinal; reconstructed canonical JSON bytes; and inspected all 85 retained v2 occurrences for the value of `external.execution_expected`.

The review did not treat names, prose totals, hashes, successful JSON decoding, prior blocked conclusions, or structural adjacency as semantic authority.

## Exact registry result

The checked-in v3 registry is canonical and complete:

```text
byte length       51098
SHA-256           8c660fa8450ddaf79ba6237fb6cb82695d28f4ea0c929420f5c80022def50933
terminal LF       yes
canonical rebuild byte-equal
records           85
unique IDs        85
root              25
w24-worker        16
w25-execution     22
w25-pair-member   22
```

The IDs exactly cover `root/01..25`, `w24-worker/01..16`, `w25-execution/01..22`, and both explicit members of each `w25-pair/01..11`. Record shape, class-local ordinal, pair ordinal, and disabled/enabled member ordering are exact. There are no aliases or duplicate IDs.

Independent schedule recomputation produced predecessor cardinalities `{0: 28, 1: 29, 2: 28}`. Selected availability is true for 80 and false for 5 records; handoff availability is true for 81 and false for 4; predecessor availability is true for 79 and false for 6. Every materialized availability bit, unavailable normalization, predecessor count, and predecessor order matches the frozen ordinal rules.

For every available selected, handoff, and predecessor occurrence, independent domain-separated regeneration reproduced identity, length, generated opaque bytes, and SHA-256 metadata exactly. No v1/v2 expected value or implementation observation was needed. Unavailable commitments uniformly contain `available=false`, empty identity, 64 zero digest characters, and zero length.

## Causal independence and pair-member inputs

Availability and predecessor cardinality are fixed by the v3 ordinal schedule before and independently of expected commitment values. Identity, length, material, and digest are fixed by distinct domain-separated hashes over the seed, exact occurrence ID, role, and one-based role index. They are not retrofitted from v1/v2 expectations.

The disabled and enabled members of every World-25 pair are separate records with distinct occurrence IDs. Their commitment values are independently generated explicit inputs; neither member inherits from, aliases, or is derived from the other. Pair labels and relationship declarations remain fixture organization only. Pair equality must still be computed after two actual executions and cannot be manufactured by shared expected operands.

Generated material is explicitly opaque. It is neither retained as an expected semantic preimage nor interpreted as call-hierarchy meaning. The registry contains metadata only; future envelopes contain the exact driver bytes. The contract forbids production or RED dispatch by world, occurrence, record, name, pair, control label, or expected value.

## Exact ownership partition

The canonical v2 registry independently parses as 162 unique paths:

```text
PRESERVE GENERIC  133
PRESERVE OPAQUE    12
REMOVE/DEFER       17
retained total     145
```

Applying the v3 contract's sole reclassification to the blocked v2 exact inventories yields:

```text
A                         116
B                          20
C                           9
union                     145
A ∩ B                       0
A ∩ C                       0
B ∩ C                       0
retained paths missing      0
paths outside retained      0
removed paths admitted      0
```

The only category delta is `external.execution_expected`: absent from A and present in C. It is absent from B. All 85 v2 occurrences carry the fixed value `true`, consistent with the v3 requirement that each envelope represents a real harness execution. The path is fixture intent, not a Manager-produced observation or production field.

All 17 removed/deferred paths remain absent. The 12 commitment expectation paths derive mechanically from explicit `driver.commitment_inputs`. The other 133 expected paths are required to project unchanged at decoded-value level from accepted v2. This review grants design authority for that future projection; because no v3 corpus exists yet, exact per-envelope projection remains a mandatory corpus-review obligation rather than a presently executed fact.

## Self-contained envelope and immutability boundary

The future envelope contract is self-contained: exactly `driver` and `expected` at the top level, with complete commitment bytes under `driver.commitment_inputs`. Runtime execution has no registry reference, occurrence lookup, side-table key, inheritance rule, or ambient resolver. The authoring registry is provenance only and cannot become a runtime dependency.

V1 and v2 remain immutable historical evidence. HEAD `8ff8bdf617f514f4f1eadc2e6e77ceb78758ad66` adds only the three documented v2 disposable-write-set qualification files. The two v3 design files and two preexisting blocked-v2 ownership files were review inputs; this review does not modify or supersede them.

## Circularity and runtime-authority challenge

The design is not circular merely because commitment expectations are derived from the same independent authoring generation that supplies driver bytes. The two roles are separated:

1. authoring deterministically creates explicit opaque causal bytes and independently checkable expected metadata;
2. the harness submits only the envelope's driver inputs;
3. production may retain only actual opaque metadata from that submitted attachment; and
4. the RED compares actual production observations to expected metadata.

This would become circular if runtime reconstructed inputs from expected, consulted the registry or occurrence ID, copied expected metadata as the actual, fabricated an observation, or dispatched by fixture labels. The contract explicitly forbids each route and requires all 85 real executions. Passing future corpus hashes or schema checks cannot establish runtime causality; the separately reviewed RED must demonstrate driver-only request construction, actual A observations, independently computed B assertions, structural-only C validation, and no-world-dispatch behavior.

The contract therefore creates no production authority. It authorizes only the bounded v3 design for later separately reviewed corpus and RED work. Any registry runtime dependency, expected-fed input, semantic interpretation of generated material, failure to execute both pair members independently, change to the other 133 expectations, or weakening of v1/v2 immutability is a hard stop.

## Disposition

`ADR0011_CALLHIERARCHY_LAYER1_GENERIC_V3_CAUSAL_INPUT_CONTRACT_GO`

The finite registry, deterministic causal-input derivation, self-contained envelope design, corrected `116/20/9` ownership partition, v1/v2 immutability boundary, and runtime-authority ceiling pass independent strict review. This GO issues design authority only and grants no corpus, RED, test, source, implementation, C15, activation, production-writer, commit, or semantic-promotion authority.
