# ADR-0011 Call-Hierarchy Layer-1 Owned-Method-Pair Write-Set Amendment Review

## Scope

Independent review of exact repository HEAD `9132d0759f49f9748c23442f31a1cd15dfe12343` and the sole pre-existing untracked amendment:

- `docs/qualification/adr0011-callhierarchy-layer1-owned-method-pair-writeset-amendment.md`

This review creates only this document. It makes no source, test, corpus, generated-artifact, amendment, activation, or commit change.

## Findings

### 1. The eight-symbol set is the exact minimal generic set exercised by the active retained tests

The active, non-skipped tests require exactly:

1. `(*Manager).privateCallHierarchyRequestIdentity`
2. `newCallHierarchyObservationLimits`
3. `callHierarchyObservationAccounting.validAgainst`
4. `callHierarchyObservationTerminal.valid`
5. `newCallHierarchyOriginalRequestFrameObservation`
6. `callHierarchyOriginalRequestFrameObservation.copyFrameBytes`
7. `newCallHierarchyOriginalResponseFrameObservation`
8. `callHierarchyOriginalResponseFrameObservation.copyFrameBytes`

Those symbols cover only Manager-owned request identity, bounded generic limits/accounting, the existing terminal enumeration, and opaque complete-frame retention. The four skipped directional tests require different allowlist, token/subspan, raw selected/handoff, predecessor-chain, and observation-request symbols. Those excluded symbols can remain inert.

### 2. The amendment's file-ownership premise is incorrect

All eight authorized symbols are already declared as inert stubs in tracked `sessionruntime/owned_method_pair.go` at the pinned HEAD. `git ls-files` records that path, and commit `86974767ec5ebe1a90691ba13a6ec8dc1ced0ea0` added the inert Call-Hierarchy vocabulary and stubs to the existing file.

Therefore:

- `sessionruntime/owned_method_pair.go` is not a new file;
- the eight definitions are not absent from the repository;
- Go does not require a newly added file to implement them; and
- the necessary amendment, if otherwise admissible, would authorize modification of an existing third production file alongside the two previously approved files.

The proposed rationale in lines 9–17 and the instruction to "Add exactly" that file are factually incompatible with the pinned repository state. A GO cannot issue from a write-set amendment that misstates whether its admitted path and definitions already exist.

### 3. The frame constructors can remain complete-frame generic

The request and response constructors can satisfy the active generic test without parsing or classifying JSON tokens or exposing subspan bytes. They can be constrained to:

- reject an unavailable request identity, empty frame, zero or exceeded byte limit, arithmetic overflow, or an out-of-frame supplied span;
- defensively copy the complete frame on ingress;
- record exact complete-frame length and SHA-256;
- retain only the supplied body and payload offset/length metadata after validating those spans are within the complete frame; and
- defensively copy only the complete frame on accessor egress.

The existing `paramsForm`, `resultForm`, body/payload digest fields, `copyParamsBytes`, and `copyResultBytes` need not be populated or activated by this generic layer. No token-form, subspan semantic, directional, raw selected/handoff, predecessor, method-allowlist, target, relation, or publication authority follows from validating numeric span bounds.

### 4. No directional, raw, predecessor, or allowlist authority is admitted

The four tests below remain explicit pinned skips at the reviewed HEAD:

- `TestCallHierarchyDirectionalMethodAllowlistTriples`
- `TestCallHierarchyDirectionalTokenFormAndResultSubspan`
- `TestCallHierarchyDirectionalRawSelectedAndHandoffCommitments`
- `TestCallHierarchyDirectionalRichPredecessorRequestChain`

Their supporting constructors/accessors remain inert. The eight active symbols neither require nor authorize method triples, incoming/outgoing meaning, token-form interpretation, raw selected/handoff bytes, predecessor chains, or observation-request assembly.

### 5. The C15 no-candidate premise is not plausible

The retained experiment `docs/qualification/evidence/adr0011-callhierarchy-scaffold-c15-experiment-v1/README.md` evaluated the Layer-1 retained-copy lifecycle in the same relevant production area. For its exact patch, regeneration changed the C15 surface from 3,805 to 3,838 candidates and classifications: `+33` candidates and `+33` classifications, all added classifications excluded, with no removals or changed predecessor rows.

That experiment is explicitly non-authoritative for exact final IDs or counts, but it is direct evidence that the required retained defensive-copy paths are candidate-producing. The resumed three-file implementation necessarily includes retained complete-frame copy behavior in `sessionruntime/owned_method_pair.go` and owner/view copy behavior in `sessionruntime/b4_owned_method_pair_lease.go`. A zero candidate/classification delta therefore cannot be presumed or treated as plausibly inevitable from this amendment.

The amendment itself requires an immediate hard stop if implementation adds or changes any C15 candidate or requires classification or binding. Because a nonzero candidate delta is already evidenced for the relevant retained-copy lifecycle, the work must stop for a separate finite-delta plan and independent review. The writer may not classify, bind, regenerate, suppress, rename, inline, relocate, or otherwise evade that delta under this amendment.

The broader source-pin manifests also record `sessionruntime/owned_method_pair.go` by byte length and SHA-256, so any implementation changes that tracked file. This reinforces that verification must distinguish historical C15 artifact preservation from successor source/candidate accounting rather than asserting an unqualified zero manifest/digest delta.

## Required correction before reconsideration

A replacement amendment must:

1. describe `sessionruntime/owned_method_pair.go` as an existing tracked third production file containing the eight inert definitions;
2. authorize exactly one writer across only:
   - `sessionruntime/sessionruntime.go`;
   - `sessionruntime/b4_owned_method_pair_lease.go`; and
   - `sessionruntime/owned_method_pair.go`;
3. preserve the eight-symbol generic ceiling and every directional/raw/predecessor/allowlist exclusion;
4. identify the exact candidate-producing copy sites from the proposed final implementation;
5. provide a separately reviewed finite C15 candidate/classification delta and successor-artifact plan; and
6. retain a hard stop for any actual delta beyond that exact reviewed finite set.

No implementation may resume under the reviewed amendment.

## Disposition

`ADR0011_CALLHIERARCHY_LAYER1_OWNED_METHOD_PAIR_WRITESET_BLOCKED`
