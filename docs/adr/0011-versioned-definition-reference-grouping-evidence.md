# ADR 0011: Add versioned definition and reference evidence as a Program C grouping input

- **Status:** Accepted
- **Date:** 2026-09-21
- **Accepted:** 2026-09-22
- **Decision owners:** LSP Trace maintainers
- **Extends:** [ADR 0007](0007-optional-local-semantic-feature-index.md)
- **Related:** [ADR 0009](0009-provenance-bounded-semantic-discovery.md), [ADR 0010](0010-provenance-bounded-feature-attribution.md)
- **Implementation authorization:** Granted 2026-09-22 for staged, additive implementation, schema work, and qualification tests under the contracts below. This does not qualify a grouping policy, admit definition/reference evidence to Leiden, enable a public CLI/MCP surface, migrate historical artifacts, or accept feature identities.

## Context

ADR 0007 permits immutable, bounded structural evidence to nominate reversible provisional groups. Those groups and every generated derivative remain `authority=0`, `accepted=false`, and `completeness=UNKNOWN`. Grouping does not establish feature identity, ownership, purpose, runtime use, or completeness.

The existing public normalized relation vocabulary excludes definition and reference evidence. When relation selectors are omitted, the historical meaning is `CALLS_ONLY`. Existing Program C occurrence admission and its claim ceiling are correspondingly `CALLS`-specific, even though the Leiden kernel consumes generic directed `PairWeight` values. Its present logical digest binds the canonical partition but does not bind the input family and version, acquisition and method receipts, occurrence digest, or grouping-policy digest. Existing V6 custody roles `TARGET` and `CALLER`, including incoming occurrence semantics, are also `CALLS`-specific.

Definition and reference results have different semantics. They are language-server responses to bounded queries, not call observations and not runtime evidence. Treating them as `CALLS`, or fitting them into `TARGET`/`CALLER` custody, would erase the distinction that ADR 0007 depends on.

References are nevertheless selected here as a grouping input, not merely evidence displayed after grouping. The architecture must therefore admit their exact occurrence multiplicity into a separately versioned Program C input family while preserving direction and protecting Leiden from high-degree or ubiquitous reference structure. Numeric protections cannot be chosen from intuition; they require a frozen evaluation and must fail closed until a policy pins them.

ADR 0010 later consumes exact immutable anchors and attribution results. This decision must make definition/reference occurrences and grouping products stable inputs for that later join without moving attribution into Program C.

## Decision

Add a staged, additive **definition/reference evidence family** and an explicitly versioned **Program C grouping-input family**. Historical behavior remains unchanged:

- omitted relation or input-family selection means `CALLS_ONLY`;
- existing CALLS schemas, selectors, snapshots, packets, checkpoints, replay, CLI, and MCP behavior retain their historical bytes and meaning;
- the new relations never become `CALLS` and never inherit CALLS custody or runtime claims;
- representative selection and outward-consumer selection remain CALLS-only until a separately versioned and qualified successor says otherwise.

This ADR authorizes staged additive implementation, schema registration for the new versioned family, and qualification work under these contracts. Implementation must fail closed until the specified method, custody, admission, policy, and replay gates pass. Acceptance does not qualify any grouping policy or authorize public enablement, migration of historical artifacts, or changes to historical CALLS-only bytes and omitted-selector meaning.

## Authority and claim ceiling

Every acquisition record, admitted occurrence, grouping input, partition, community, representative status, terminal result, and derived packet introduced here carries:

```text
authority = 0
accepted = false
completeness = UNKNOWN
```

Definition/reference evidence means only that a named language-server method returned a bounded result under the recorded request, provider, session or retained custody, and limits. It does not establish:

- runtime execution, dynamic dispatch, or `CALLS`;
- accepted symbol identity or semantic correctness;
- ownership, product purpose, feature identity, or production use;
- repository, source-graph, reference, or definition completeness;
- absence outside the exact bounded acquisition.

No transformation, community score, agreement, multiplicity, or repeated provider return may raise this ceiling.

## Stage 1: relation and occurrence semantics

Introduce two relation kinds in a new evidence-family version. Neither is a `CALLS` subtype or alias.

### `REFERENCES_SYMBOL`

Orientation is normative:

```text
referencing occurrence or referencing symbol -> referenced symbol
```

The preferred source endpoint is the exact reference occurrence. A symbol-level source endpoint is permitted only when the provider contract supplies no occurrence range and the admission records that lower precision explicitly. Reversing this edge for display or algorithmic projection does not change its canonical orientation.

`textDocument/references` returns reference locations; it does not independently establish the canonical identity of the referenced-symbol target. Every references request must therefore name a pre-existing canonical `referenced_symbol_identity` bound to the exact query occurrence by an admitted query-target receipt. That receipt states the versioned derivation used to identify the query target, such as exact document-symbol containment or a separately admitted definition result, and binds its method evidence, custody, URI, range, encoding, document version, and digest. The references response may add occurrences to that target identity but cannot create, replace, or infer it. An absent, ambiguous, mismatched, or unverifiable query-target receipt terminates as `TARGET_IDENTITY_UNRESOLVED` and contributes no `REFERENCES_SYMBOL` occurrence. Any secondary definition or symbol request is a separate bounded acquisition with its own immutable receipt; it is never an implicit fallback inside references acquisition.

### `RESOLVES_TO_DEFINITION`

Orientation is normative:

```text
query or reference occurrence -> server-returned definition target
```

The source identifies the exact query occurrence or reference occurrence submitted to `textDocument/definition`. The target identifies exactly one server-returned definition target. A result returning several targets emits one occurrence per returned target; it does not select a preferred definition.

### Exact multiplicity

One normalized logical edge never erases occurrence multiplicity. Every admitted provider-returned occurrence is immutable and independently identified by at least:

- evidence-family and schema version;
- method (`textDocument/references` or `textDocument/definition`) and method-policy identity;
- request and acquisition receipt identity;
- source/query occurrence identity and exact range when supplied;
- target symbol/definition identity and exact range when supplied;
- logical URI, position encoding, document version, workspace revision/custody, session generation, provider, and adapter identities as applicable;
- canonical ordinal after deterministic ordering;
- terminal disposition and limitations.

Repeated responses that normalize to the same endpoint pair remain distinct occurrences. Pair-weight composition may count, cap, normalize, or exclude them only under a named grouping policy; it cannot rewrite the occurrence ledger.

No definition/reference occurrence may be serialized, counted, projected, labeled, or advertised as `CALLS`.

## Stage 2: bounded capability-aware acquisition

Acquisition is live, bounded, and capability-aware. It is separate from deterministic offline grouping.

A request names exact targets or query occurrences, the managed session and generation, expected workspace revision/custody when supplied, method policy, provider/adapter policy, position encoding, privacy policy, and hard limits for documents, requests, messages, bytes, candidates, occurrences, time, and work.

Before invoking a method, the acquisition layer records whether the selected server advertises the required capability. It may invoke only:

- `textDocument/references` for `REFERENCES_SYMBOL`;
- `textDocument/definition` for `RESOLVES_TO_DEFINITION`.

Request outcomes and member outcomes are separate closed vocabularies. A request receives exactly one outcome under this precedence and aggregation contract:

1. `REVISION_MISMATCH` or `POLICY_MISMATCH` when pre-execution identity or policy admission fails; no member evaluation begins.
2. `UNSUPPORTED` when the required capability is unavailable or the selected method is explicitly unsupported; no member evaluation begins.
3. `CANCELLED`, `TIMEOUT`, `RESOURCE_LIMIT`, or `PROVIDER_FAILURE` when execution terminates before any provider output can be admitted; no unevaluated member outcome is manufactured.
4. `MALFORMED` when any provider output needed for the request violates the selected method contract. Malformed output contributes no occurrence; valid independently admitted members may remain in the immutable ledger for diagnosis, but the request is not complete.
5. `PARTIAL` when at least one member evaluation or provider result is retained but the declared request denominator did not reach terminal, well-formed evaluation within bounds.
6. `COMPLETE_EMPTY` when capability and policy checks passed, the complete declared request denominator was evaluated, every returned member was well formed, and zero occurrences were admitted.
7. `COMPLETE` when the same completeness predicate holds and at least one occurrence was admitted.

Member outcomes use a separately versioned closed set that records admitted, empty, malformed, excluded, limited, or failed evaluation as applicable. Request aggregation binds the declared member denominator, evaluated-member count, admitted-occurrence count, and exactly one terminal member outcome for every member whose evaluation began. `COMPLETE_EMPTY` and `COMPLETE` are therefore disjoint. Mixed valid and empty members may produce `COMPLETE` when the denominator balances and at least one occurrence is admitted; any unfinished member makes the request `PARTIAL`. A whole-request failure cannot manufacture outcomes for members whose evaluation did not begin.

`COMPLETE_EMPTY` is not absence beyond the exact query. `UNSUPPORTED` is not empty. `PARTIAL` is not complete. `MALFORMED` cannot contribute malformed occurrences.

### Custody

Do not reuse V6 `TARGET`/`CALLER` custody or incoming CALLS occurrence semantics. Introduce method-specific custody roles in the new family, including:

- `QUERY_OCCURRENCE`;
- `REFERENCING_OCCURRENCE`;
- `REFERENCING_SYMBOL` when occurrence precision is unavailable;
- `REFERENCED_SYMBOL`;
- `DEFINITION_TARGET`.

Custody records the server-reported method result, its exact request context, and its limitations. It neither authenticates semantic truth nor transfers CALLS custody.

## Stage 3: immutable grouping-input admission and composition

Introduce an immutable versioned `ReferenceGroupingInput` family, distinct from historical CALLS-only Program C admission.

A canonical input contains:

- input-family name and version;
- exact admitted acquisition and method receipt selectors, digests, schema identities, and byte lengths;
- complete occurrence-ledger selector and occurrence digest;
- exact node identities and endpoint-custody references;
- relation-kind eligibility and occurrence-admission policy;
- composition policy and ordered contribution records;
- privacy policy and resource limits;
- denominator, exclusions, duplicates, terminal outcomes, and truncation accounting;
- `authority=0`, `accepted=false`, and `completeness=UNKNOWN`.

Admission is closed. Unknown relation kinds, unknown family versions, unbalanced occurrence accounting, missing receipt bindings, custody substitution, duplicate keys, trailing content, noncanonical ordering, and digest mismatch fail closed.

Composition preserves every contributing occurrence and receipt. It may derive generic directed `PairWeight` records for the existing Leiden kernel only after admission and policy qualification. A `PairWeight` is an algorithm input, not a structural claim and not a normalized public relation.

Historical Program C requests and artifacts that omit an input-family selector resolve exactly to `CALLS_ONLY`. They cannot acquire, admit, or compose the new family, and their canonical bytes, selectors, claim ceiling, and replay behavior remain unchanged.

## Stage 4: versioned grouping policy

Every definition/reference grouping run selects one immutable grouping-policy identity and digest.

### Normative direction

Canonical direction is always retained in the occurrence ledger, grouping input, receipts, and identity. Leiden projection may be:

- directed as recorded;
- symmetrized under a named rule;
- reversed under a named rule;
- split into relation-specific layers and composed under a named rule.

The projection is policy-controlled and must not silently overwrite normative orientation. A partition alone cannot reveal which projection was used.

### Degree and ubiquity protections

The policy must explicitly address:

- high-degree referenced symbols;
- shared helpers and shared definitions;
- imports, re-exports, generated bindings, and aliases;
- builtins, standard-library symbols, SDK/framework symbols, and unresolved external targets;
- file-, package-, language-, and provider-specific fan-in artifacts;
- repeated occurrences from one source and duplicate provider returns;
- cross-language and cross-revision identity boundaries.

Available policy actions are closed and versioned, for example `INCLUDE`, `EXCLUDE`, `CAP`, `NORMALIZE`, `DOWNWEIGHT`, or `SEPARATE_LAYER`. Each action preserves the original occurrence ledger and emits exact accounting and rationale codes.

### Qualification before thresholds

This ADR chooses no numeric degree, multiplicity, weight, or ubiquity threshold. A policy version may become eligible only after a digest-pinned qualification plan:

1. defines corpora, languages, providers, revisions, expected groupings, and held-out evaluation data;
2. freezes candidate thresholds and projection rules before held-out execution;
3. compares CALLS-only, references-only, definition-only, and declared composite inputs;
4. reports false merges, fragmentation, stability, unmatched/no-community rates, shared-node effects, and resource behavior;
5. includes adversarial high-degree, import, builtin, generated, and shared-helper cases;
6. records uncertainty and an independent acceptance decision.

Qualification produces an immutable, canonical `GroupingPolicyQualificationReceipt` owned by the LSP Trace maintainers as the decision authority. The receipt binds the exact grouping-policy identity and digest, evaluation-plan and corpus-manifest identities, fixture and held-out partitions, implementation and environment identities, metric definitions and complete metric results, qualification report identity, independent reviewer identities, decision timestamp, and every predecessor byte length and digest.

Its closed decision vocabulary is `QUALIFIED`, `REJECTED`, and `INCOMPLETE`. `QUALIFIED` is valid only when every mandatory corpus and adversarial fixture completed under the frozen plan, all denominator equations balance, deterministic replay and input-permutation checks pass, every pre-registered acceptance criterion is satisfied, and the required independent maintainer acceptance is recorded. A missing criterion, missing artifact, identity mismatch, incomplete run, or absent acceptance deterministically yields `INCOMPLETE`; a completed evaluation that violates any acceptance criterion or is declined by the decision authority yields `REJECTED`. The policy contract—not ambient configuration—defines the required number and identity class of independent acceptances.

Admission permits Leiden only when a strictly verified receipt is `QUALIFIED` and its policy digest exactly matches the selected grouping policy. `REJECTED`, `INCOMPLETE`, absent, unknown, or mismatched receipts return `GROUPING_POLICY_UNQUALIFIED`. Admission must not choose defaults, infer thresholds from the input, treat report prose as acceptance, or run Leiden.

## Stage 5: identity and logical digest

A successor grouping-result identity and logical digest bind, in canonical order:

```text
grouping_result_identity = digest(
  input_family_name,
  input_family_version,
  acquisition_receipt_identities,
  method_receipt_identities,
  occurrence_ledger_digest,
  grouping_policy_digest,
  pairweight_composition_digest,
  algorithm_name,
  algorithm_implementation_version,
  algorithm_profile,
  numeric_policy,
  seed,
  resource_limits,
  canonical_partition
)
```

The partition remains canonical, but it is no longer sufficient identity. Any change to the input family/version, receipts, occurrences, policy, composition, algorithm/profile, numeric contract, seed, limits, or partition creates a distinct result.

A result also records canonical input and output byte lengths, schema identities, executable/library identities where applicable, replay environment, terminal accounting, and immutable predecessor/supersession references. Digest equality establishes byte/identity equality under the canonical contract only; it does not establish truth, completeness, or acceptance.

## Stage 6: CALLS-only consumers and no-CALLS communities

Representative selection and outward-consumer selection remain CALLS-only in the initial version.

A definition/reference grouping result may identify candidate communities, but it cannot use `REFERENCES_SYMBOL` or `RESOLVES_TO_DEFINITION` to select:

- the community representative used by existing CALLS-specific logic;
- the nearest outward consumer defined by ADR 0007;
- a caller boundary, runtime entry point, or execution direction.

When a community has no eligible admitted CALLS evidence, it receives the terminal status:

```text
NO_CALLS_REPRESENTATIVE
```

The status is a successful, explicit terminal condition, not an error and not permission to substitute reference degree, lexical prominence, source order, file size, or model judgment. Any future reference-aware representative or outward-consumer rule requires a separately versioned contract, identity binding, claim ceiling, and qualification.

## Stage 7: additive artifacts and interfaces

Introduce additive versions rather than widening historical contracts in place.

### Immutable artifacts

Version separately:

- acquisition request and receipt;
- method-member and occurrence ledger;
- reference grouping input and composition receipt;
- grouping policy;
- Program C result and logical digest;
- snapshot;
- target/community packet;
- checkpoint;
- replay request and replay result.

Every artifact carries exact family/version discriminators. Readers reject unknown versions. Historical readers continue to read historical CALLS-only artifacts unchanged.

### Snapshot, packet, and checkpoint

A successor snapshot binds the exact definition/reference receipts and occurrence ledger. A packet preserves canonical occurrence direction, multiplicity, policy actions, projected `PairWeight` contributions, and exclusions. A checkpoint binds every predecessor identity needed for workspace-free replay and discloses whether CALLS evidence exists for representative/outward-consumer processing.

### Replay

Replay uses only immutable named artifacts. It cannot query a language server, inspect the ambient workspace, substitute current documents, reacquire references/definitions, infer a missing policy, or use a newer algorithm/profile. It produces byte-identical canonical grouping bytes under the declared replay environment or one typed replay failure.

### CLI and MCP

Future CLI and MCP surfaces, if separately authorized, use additive versioned request/result contracts. They expose explicit input-family and grouping-policy selectors. Omission means `CALLS_ONLY` at every compatibility surface.

Capability discovery reports support by exact family/version/method/policy tuple. `SUPPORTED` means an implementation contract is present, not that the current server advertises definition/references capability or that a policy is qualified for every corpus.

Transport envelopes and human projections do not alter the underlying canonical identity. CLI and MCP adapters share one admission, composition, Leiden, identity, and replay implementation; neither adapter implements fallback grouping.

## Stage 8: qualification

No new family or interface is enabled until CUE and Go qualification agree on canonical contracts and outcomes.

### CUE matrix

CUE qualification covers:

- every relation orientation and custody role;
- occurrence multiplicity, canonical ordering, and digest stability;
- empty, unsupported, partial, malformed, timeout, cancellation, resource-limit, and policy-mismatch outcomes;
- closed unions, unknown fields, duplicate keys, trailing content, and identity substitution;
- historical omitted-selector `CALLS_ONLY` fixtures;
- new family/version discriminators and additive artifact versions;
- authority, acceptance, and completeness constants;
- balanced denominators and terminal outcomes.

### Go matrix

Go qualification covers:

- capability-aware bounded definition and references acquisition with representative servers;
- exact method receipts and workspace-revision/session-generation custody;
- immutable admission and occurrence-to-`PairWeight` composition;
- direction-preserving policy projections;
- deterministic canonical ordering, identities, partition bytes, and replay;
- CLI/MCP parity over the shared kernel when those surfaces are authorized;
- historical CALLS-only fixtures remaining byte-identical.

### Adversarial high-degree collapse cases

Qualification includes graphs where:

- one builtin or standard-library symbol is referenced by most nodes;
- one import/re-export hub connects otherwise unrelated packages;
- one generated binding or registry is referenced across domains;
- shared helpers dominate occurrence counts;
- duplicate provider returns inflate one pair;
- aliases or multi-target definitions create dense cross-links;
- a community contains reference evidence but no CALLS evidence.

A candidate policy fails if these cases collapse unrelated expected groups, hide unmatched members, manufacture a representative, or become nondeterministic under input permutation.

Qualification must demonstrate deterministic replay for exact input bytes, receipt set, occurrence ledger, policy, algorithm/profile, numeric contract, seed, limits, and environment. Input permutation must produce the same canonical result. Every result remains `authority=0`, `accepted=false`, and `completeness=UNKNOWN`, including a passing qualification result.

## Sequencing

1. **ADR 0007 first:** retain its immutable provisional-inventory, authority, correction, and CALLS-only outward-consumer contracts. This ADR extends only the available grouping-input families.
2. **This ADR next:** staged additive implementation and qualification are authorized by the 2026-09-22 decision. Freeze exact acquisition, occurrence, admission, policy, identity, artifact, replay, and qualification contracts before enabling each dependent stage. A definition/reference grouping policy remains ineligible for Leiden until its own immutable qualification receipt is strictly verified as `QUALIFIED`; this ADR's acceptance is not that receipt.
3. **ADR 0010 after immutable anchors/results exist:** exact attribution may consume the new immutable occurrence anchors, grouping inputs, constituent memberships, and results. It does not reacquire evidence or reinterpret grouping.
4. **ADR 0009 after ADR 0010:** semantic discovery may index the new immutable evidence and attribution artifacts as typed inputs. Ranking or synthesis cannot repair, broaden, or upgrade them.

This sequence does not authorize ADR 0010 or ADR 0009 implementation and does not alter their existing gates.

## Consequences

### Positive

- References and definitions can influence provisional grouping without being mislabeled as calls.
- Exact occurrence multiplicity and canonical direction remain inspectable.
- The generic Leiden kernel can be reused behind a family-specific admission and claim ceiling.
- Historical CALLS-only artifacts and omitted-selector behavior remain compatible.
- Identity distinguishes equal partitions produced from different evidence, policies, methods, or seeds.
- ADR 0010 receives stable immutable anchors and results for later exact attribution.

### Costs and risks

- Live acquisition adds provider capability, revision, multiplicity, and partial-result complexity.
- Reference graphs can be dominated by shared, imported, builtin, or generated symbols.
- Policy qualification requires representative multilingual corpora and independent labels.
- Separate additive schemas and replay artifacts increase contract and maintenance surface.
- Users may overinterpret reference-based communities as runtime or feature truth despite the claim ceiling.

These costs are accepted only under fail-closed admission, qualification before numeric thresholds, immutable receipts, complete terminal accounting, and explicit authority limits.

## Alternatives considered

### Reclassify definitions or references as `CALLS`

Rejected. Language-server definition/reference results do not establish runtime invocation and have different orientation, custody, multiplicity, and failure semantics.

### Use references only as post-grouping evidence

Rejected. The selected requirement is to use references as an actual grouping input. Evidence-only display would not satisfy that requirement.

### Widen existing CALLS schemas in place

Rejected. It would change omitted-selector meaning, V6 custody assumptions, claim ceilings, logical identity, and historical replay.

### Collapse occurrences into one unweighted edge before admission

Rejected. It destroys exact multiplicity and prevents policy evaluation, correction, and replay from accounting for every provider-returned occurrence.

### Choose high-degree thresholds now

Rejected. No frozen evaluation establishes defensible numeric values. The system fails closed until a qualified policy pins them.

### Select representatives by reference degree

Rejected for the initial version. Representative and outward-consumer semantics are CALLS-specific; reference prominence is not consumer or runtime evidence.

## Unresolved policy decisions

1. What exact public schema and artifact names should the additive versions use?
2. Which language servers, languages, repositories, and revision classes form the calibration and held-out qualification corpora?
3. Which policy action applies to imports, re-exports, builtins, generated bindings, external targets, aliases, and shared helpers in each qualified profile?
4. Which candidate numeric thresholds and normalization formulas should enter pre-registered evaluation?
5. Should the first qualified Leiden projection remain directed, symmetrize by relation kind, or use a multilayer composition?
6. How should a symbol-level reference source with no occurrence range be weighted relative to exact occurrence evidence?
7. Which replay environment dimensions are required for byte-identical numerical replay across supported platforms?
8. Which future ADR, if any, may define reference-aware representatives or outward consumers?
9. Which CLI/MCP names and advertisement profiles should expose the new family after qualification and separate implementation authorization?

Authorized implementation and qualification work may proceed while these decisions remain open. Until the relevant contracts and policy decisions are frozen and the matching grouping policy is independently qualified, definition/reference grouping remains disabled and admission fails closed with `GROUPING_POLICY_UNQUALIFIED`. Public CLI/MCP enablement requires its separate qualification and authorization; ADR 0010 attribution still waits for the immutable inventory and anchors.