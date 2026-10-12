# ADR-0011 Call-Hierarchy Layer-1 Owned-Method-Pair Write-Set Amendment

## Status and frozen input

This is a corrected minimal production write-set amendment proposal against exact clean HEAD `9132d0759f49f9748c23442f31a1cd15dfe12343`.

The amendment remains blocked. It does not authorize implementation, source or test mutation, a commit, activation, or semantic expansion. No implementation GO is granted. The one writer may not resume implementation until the separately reviewed finite C15 delta experiment plan has received its own explicit experiment GO, the disposable experiment has been completed and reviewed, and a later explicit implementation authorization has been issued.

## Corrected existing-file premise

`sessionruntime/owned_method_pair.go` is an existing tracked production file at the frozen HEAD. It already contains inert declarations for all eight generic symbols listed below. This amendment does not add a file or add symbol vocabulary; it proposes permitting the one existing writer to implement those inert declarations within their current generic ceiling.

The implementation write-set, if later authorized, is exactly three existing production files:

1. `sessionruntime/sessionruntime.go`
2. `sessionruntime/b4_owned_method_pair_lease.go`
3. `sessionruntime/owned_method_pair.go`

Exactly one writer may own changes across those three files. No other production file, test, fixture, corpus, generated artifact, manifest, digest sidecar, documentation file, or configuration file is in the implementation write-set.

## Exact symbol ceiling in `owned_method_pair.go`

Within `sessionruntime/owned_method_pair.go`, implementation authority remains limited to these eight existing inert generic symbols:

- `(*Manager).privateCallHierarchyRequestIdentity`
- `newCallHierarchyObservationLimits`
- `callHierarchyObservationAccounting.validAgainst`
- `callHierarchyObservationTerminal.valid`
- `newCallHierarchyOriginalRequestFrameObservation`
- `callHierarchyOriginalRequestFrameObservation.copyFrameBytes`
- `newCallHierarchyOriginalResponseFrameObservation`
- `callHierarchyOriginalResponseFrameObservation.copyFrameBytes`

No ninth symbol is authorized. Discovery that another declaration, method, field, helper, accessor, or file must be added or changed is a hard stop requiring separate review.

The other two files retain only their previously reviewed responsibilities:

- `sessionruntime/sessionruntime.go` may contain only the bounded Manager integration required to supply the generic observations under existing lifecycle and refusal behavior.
- `sessionruntime/b4_owned_method_pair_lease.go` may contain only the bounded owner/lease custody, accounting, callback-drain, refusal-atomicity, and lifecycle-joining behavior already assigned to that file.

This amendment does not broaden either file's authority.

## Authorized generic behavior

If a later implementation GO is issued, authority remains limited to behavior required by the retained generic and corpus-backed RED cases:

- mint the private request identity from Manager-owned workspace/session/generation/request-key custody;
- construct and validate bounded generic observation limits;
- validate independently available accounting measures against those limits, preserving unavailable versus available-zero;
- validate only the existing generic terminal enumeration;
- construct original request-frame and response-frame observations from complete frames;
- defensively copy complete frame bytes on constructor ingress and copy-accessor egress;
- record and validate exact complete-frame byte length and SHA-256 digest;
- validate only supplied complete-frame span metadata needed to establish transport custody; and
- preserve the previously reviewed private owner/lease lifecycle, accounting, callback drain, refusal atomicity, and stale/terminal admission behavior.

The frame constructors must treat supplied complete frames as opaque bytes. They may not parse, normalize, reconstruct, reinterpret, or expose semantic payload content. Existing generic copy primitives may be reused only when the finite C15 experiment and subsequent review show that reuse preserves ownership, accounting, and candidate classification requirements.

## Explicitly unavailable and deferred

This amendment does not authorize:

- a method allowlist or method triples;
- incoming/outgoing or any directional semantics or directional symbols;
- token-form, result-form, params-form, or payload-form interpretation;
- result or params subspan interpretation;
- subspan byte-copy accessors;
- `copyParams` or `copyResult`;
- selected-item raw bytes or commitments;
- handoff raw bytes or commitments;
- predecessor commitments or predecessor chains;
- observation-request construction or aggregation;
- target identity, selection, normalization, occurrence, grouping, relation, qualification, publication, or semantic-vector behavior;
- new parsing, serialization, reconstruction, canonicalization, logging, telemetry, configuration, activation, CLI, MCP, public API, exported symbol, or schema behavior; or
- any source, test, fixture, corpus, generated-artifact, manifest, digest-sidecar, or commit change outside the exact three-file production write-set.

The following retained tests must remain pinned skips and must not be enabled, altered, weakened, or used to justify generic implementation:

1. `TestCallHierarchyDirectionalMethodAllowlistTriples`
2. `TestCallHierarchyDirectionalTokenFormAndResultSubspan`
3. `TestCallHierarchyDirectionalRawSelectedAndHandoffCommitments`
4. `TestCallHierarchyDirectionalRichPredecessorRequestChain`

## C15 finite-delta prerequisite

No zero-delta presumption is made. The required complete-frame defensive-copy paths may add, remove, or change C15 candidates or classifications, and existing source pins may require successor accounting even when historical artifacts remain immutable.

Implementation is blocked pending the separately reviewed finite experiment plan in `docs/qualification/adr0011-callhierarchy-layer1-c15-delta-experiment-plan.md`. That experiment must determine the exact candidate/classification delta produced by a disposable prototype of the final generic three-file shape and distinguish necessary candidates, reusable already-classified paths, and false positives. The experiment may inform a later amendment or implementation decision; it cannot itself authorize implementation, classification, binding, corpus mutation, artifact succession, or a commit.

Until that experiment is run under its explicit gate and independently reviewed, the writer must not classify, bind, regenerate in the main worktree, suppress, rename, inline, relocate, or otherwise evade a candidate delta.

## Review sequence

1. Independently review this corrected amendment and the finite C15 delta experiment plan.
2. If and only if the exact experiment gate is granted, run the disposable experiment without mutating the main worktree.
3. Review the experiment's exact candidate rows, locations, AST hashes, classifications, C15 v2 comparison, preserved patch evidence, and necessity/reuse/false-positive determinations.
4. Draft and review any required finite candidate/classification or successor-artifact amendment.
5. Issue a separate explicit implementation GO only after all C15 consequences are admitted or resolved.

No test run, verification success, experiment result, or review comment implicitly supplies implementation or commit authority.

## Gate disposition

`ADR0011_CALLHIERARCHY_LAYER1_OWNED_METHOD_PAIR_WRITESET_AMENDMENT_PENDING`

Implementation remains blocked. This document supplies no implementation GO and no commit authority.
