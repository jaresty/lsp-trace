# ADR 0007 amendment: caller-provided inference first

- **Status:** Accepted direction; implementation plan below does not grant public enablement
- **Decision authority:** @jaresty (GitHub handle)
- **Decision basis:** The user selected the existing calling LLM as the primary inference provider and authorized this planning revision.
- **Amends:** [ADR 0007](0007-optional-local-semantic-feature-index.md) and the earlier [single-user/local-model amendment](0007-single-user-gpt-amendment.md).

## Decision and precedence

The primary delivery path is an existing LLM session consuming bounded lsp-trace evidence and providing a source-supported explanation. lsp-trace does not invoke a second model for this path. The feature requires no additional model installation, inference-provider credential, or separate provider account beyond those already used by the host session. It is not intrinsically offline, free of inference costs, or independent of the host's provider.

Standalone key-free local inference remains an optional, deferred backend goal. Preserve the local worker, diagnostics, identities, failures, and licensing records; do not remove or present them as qualified. GPT development results do not qualify a local backend, and local backend failures do not disqualify caller-provided interpretation.

This amendment takes precedence over the earlier amendment's requirement that key-free standalone inference be the first deliverable. It also distinguishes ordinary evidence-assisted conversation from an implemented isolated Describe worker: the worker-specific G1–G8 enablement bundle does not gate this conversational path. Those gates remain applicable to the corresponding standalone worker/index capabilities, not silently marked complete or waived globally.

## Responsibility split

### lsp-trace: bounded evidence

Reuse existing discovery, server-reported relationships, retained inspection, and full-definition projection. Preserve source and artifact identities, exact ranges and position encoding, custody, session/generation or retained provenance, acquisition scope, omissions, and limits. Source search locates targets; it does not establish CALLS. Tool completion does not establish graph completeness.

Consumer selection is structural preprocessing or explicit user selection under the existing selection policy, never an inferred model relationship. Preserve alternative callers and unresolved consumer identity. A single bounded projection must not be labeled a complete SCC/consumer-selection computation unless that preprocessing has actually been performed.

### Host LLM: interpretation

The calling LLM explains only what the selected source and relationships support: target inputs, visible returned results/failures, and immediate caller use where shown. It distinguishes visible wrappers from missing delegated implementations and uses evidence-linked limitations rather than inventing absence.

The host may perform explicitly requested tool acquisition under its normal permissions. It cannot silently expand a particular explanation's evidence boundary: new tool results become separately identified inputs. This is a deliberate distinction from the isolated worker's tool-free contract.

Ordinary conversation contains prior context. Therefore it is not the original ADR's item-independent Describe execution or a reproducible isolated worker simply because it uses the same evidence. Material source claims must cite the selected evidence; unsupported host knowledge cannot be presented as packet-derived truth.

### Local validation, where results are retained

Mechanical validation can check schema shape, citation membership, exact quotations, ceilings, and identity bindings. It cannot establish semantic entailment, usefulness, accurate limitations, or feature identity. Do not claim validation occurred for ordinary prose merely because such validators exist elsewhere.

An ordinary conversational answer need not be serialized into a new artifact. If retention is requested, preserve the actual generated output and bind it to exact evidence identities, host/model identifiers where exposed, and relevant instruction/context provenance where available. Missing context or provider identities limit replay claims. Do not claim general reproducibility or qualified Describe execution. Retained correction is additive; never rewrite historical outputs.

## Authority, disclosure, and scope

@jaresty remains the operator and human approval authority. Generated explanations remain non-authoritative (`authority=0`, `accepted=false`) with completeness unknown except for explicit bounded accounting. They do not establish runtime execution, source authentication, ownership, feature identity, stakeholder acceptance, or correctness of a proposed change.

Evidence exposed to a remote host LLM is remote source disclosure. Existing access to a repository is not blanket permission to send all its contents to a provider. Use only selected material authorized for that host, preserve resource/privacy constraints, and never describe remote inference as local. Existing local-only packet restrictions remain in force until explicitly changed; this amendment does not silently permit uploading historical restricted packets for a demonstration.

No new tool names, schemas, public MCP/CLI operations, service, registry, model backend, index, or automatic code execution are selected here. Existing tool use is distinct from enabling a new product surface. Standalone processing, embedding, indexing, grouping, census, and public shipment remain separately scoped.

## Minimal implementation plan

### 1. Document an existing-tool recipe

Prepare concise host-facing guidance using the currently advertised contracts:

1. Identify the exact question and target.
2. For live evidence, list sessions and use the exact-workspace READY session; derive a registered worktree session only when required and supported. For retained evidence, use its exact supported selector/identity without hidden live repair.
3. Obtain bounded server-reported incoming relationships and source-bearing projection. The advertised `lsp_trace_v2_structural_context` supports `PROJECTED`, `INCLUDE`, and `FULL_DEFINITION`; `lsp_trace_v1_inspect_hydrated` supports retained inspection. Advertisement establishes available contract shapes, not successful execution or accepted selection for a particular request.
4. Preserve consumer alternatives and limitations; distinguish caller nomination from qualified deterministic selection. If required selection cannot be established by the returned evidence, report that gap rather than infer it.
5. Explain target contribution and caller use with source citations. State what is absent; do not generate a full-system purpose from one edge.

Start with documentation and normal host tool calls. No new request builder, inference transport, provider credential flow, correction harness, or target-packet resolver is required merely to demonstrate this recipe.

### 2. Verify one end-to-end example

Choose one target whose disclosure is approved for the current host. Reuse existing admitted evidence only if those disclosure permissions and identities are known; otherwise acquire an explicitly authorized fresh bounded packet. Capture exact tool inputs, results, target/consumer selection basis, source identities, omissions, and the host's answer.

Review source support and explanatory value separately from mechanical validity. A review must see the complete selected evidence, not only the model's chosen citations, so an abstention can be judged fairly. Describe the result as one host-assisted example, not general model or worker qualification. No numeric reliability claim follows from one case.

### 3. Implement only a demonstrated gap

If the example exposes a missing capability, first check existing operations and publication/inspection forms. A documentation or selection-guidance gap does not justify a service. Any actual code plan must identify the exact seam from managed structural evidence and define a narrow test before mutation. This document establishes no new cross-file implementation relationship.

Potential retention work, if requested, must distinguish semantic packet ID from content digest and storage selector. Artifact registration is not occurrence admission or semantic acceptance. Do not build retention machinery for a conversational example that does not need it.

## Completion and stops

The first milestone is one useful, cited explanation in the user's existing LLM session, using attributable bounded evidence and no second inference backend. A real evidence limitation or warranted abstention is reported honestly, not forced into success.

That milestone has one bounded demonstration in the [host-assisted caller inference example](../pilot/adr0007/host-assisted-example.md). Existing managed tools supplied live source and a server-reported caller relationship; the host produced the explanation; independent review found concrete citation defects; and later mechanical corrections repaired those defects without constituting a second semantic acceptance. The example remains `authority=0`, `accepted=false`, and `completeness=UNKNOWN`. It establishes neither isolated item-independent Describe execution nor qualification, feature acceptance, reproducibility, completeness, or public enablement.

Stop for unavailable required evidence, unapproved disclosure, semantic ambiguity that cannot be represented, or a need for a new public/API surface or materially broader architecture. Routine documentation and example preparation should not produce repeated approval loops. Do not resume local-model tuning or broad qualification campaigns to close this milestone.

This amendment selected and documented the delivery direction; the linked example separately demonstrates the first conversational milestone. Neither document uploads restricted historical source, edits runtime code, enables an isolated pilot, authorizes shipment, or commits, merges, or pushes changes.

## Describe semantic qualification threshold amendment

A stakeholder amendment made after observing the prospective custody results distinguishes evidence safety from exact replay phrasing. Describe qualifies bounded semantic usefulness when all custody and mechanical checks pass, every reviewer confirms disposition support, source support, citation validity, limitation validity, ceiling preservation, and evidence binding, and at least 90% of cases pass replay agreement in both lanes. Replay-only failures within the remaining 10% are retained as explicit nondeterminism limitations; they do not become accepted feature identity or authority.

This threshold is recorded transparently as a post-run policy decision rather than a predeclared property of earlier runs. It cannot excuse malformed output, unsupported claims, invalid citations, weakened limitations, changed ceilings, broken lineage, missing attempts, retry, repair, or substitution. Search and Group remain separately gated, and Describe qualification under this threshold does not authorize public enablement or change `authority=0`, `accepted=false`, `completeness=UNKNOWN`, or `featureIdentity=UNRESOLVED`.

The consolidated Describe identity is main `40440412`, source `8ae7f308`. Its bounded threshold is **22/24 cases with all 96 attempts accounted**. The two non-passing cases and all blocked, rejected, superseded, or unevaluated predecessor records remain immutable; the threshold does not impute success or authorize another run.

A conditional zero-effect repair applies only to deterministic custody infrastructure, never semantic recovery. It must be pre-semantic with no observable semantic output, infrastructure-only, preserve the original failure, receive independent zero-effect adjudication, account for the failed attempt, retain the same assignment and freeze, and pass deterministic verification replay. Regeneration may verify deterministic custody after the repair; it may not regenerate, repair, retry, normalize, replace, or substitute semantic output.

## Proposed bounded source-text search extension

ADR 0007 should add a private, default-off Source Text Search stage for exact literal search over immutable source-admission bytes. Its purpose is to turn source occurrences into attributable ranges that the separately qualified Location contract can intersect with bounded Group candidates:

```text
admitted source bytes
→ exact literal UTF-8 matches
→ immutable match ranges
→ Location RANGE_UNION
→ overlapping Group candidates
```

The first version is limited to a nonempty, case-sensitive literal query. It returns every match, including overlaps, in deterministic path and byte-offset order. Each result binds the exact path, revision, source digest, byte offsets, and half-open UTF-16 LSP range. It must fail closed for unavailable or mismatched admission, invalid UTF-8, digest disagreement, impossible range conversion, cancellation, deadline, and exceeded query, source-byte, file, match, work, or output limits. Request, result, policy, limits, accounting, custody, and replay records are canonical and versioned.

Regex, case folding, stemming, fuzzy or semantic search, language-aware tokenization, comment/string filtering, backend source fetching, and implicit workspace expansion are outside the first version. Source Text Search locates exact text occurrences; it does not establish CALLS, feature identity, feature membership, completeness, acceptance, or semantic relevance. Its safe claim is: “these bounded structural candidates overlap these exact source occurrences.”

This extension remains separately gated. Design and private implementation may proceed to an immutable `SOURCE_TEXT_SEARCH_DESIGN_GO` freeze, but qualification execution, production dispatch, CLI/MCP exposure, public schemas, release, and push require separate authorization. Existing ceilings remain `authority=0`, `accepted=false`, `completeness=UNKNOWN`, and `featureIdentity=UNRESOLVED`.

## Candidate-generation publication decision

A future durable-publication adapter publishes one immutable qualified **candidate-group generation**, not an accepted feature inventory or a bundle that silently combines independently qualified products. Existing `PutCandidateGroup`, `GetCandidateGroup`, selector, receipt, and verified-generation publication contracts remain authoritative. The adapter must be additive and use the existing `PublishVerifiedGeneration` path rather than reviving or cherry-picking historical publication machinery.

Only an explicit repository-local owner operation may advance the current-generation selector after strict receipt verification. Qualification success, caller input, filesystem recency, or ambient “latest” selection cannot advance it automatically. The selector transition is ordered after durable artifact and receipt commitment, preserves cancellation boundaries and truthful committed-success semantics, detects tampering and immutable collisions, supports restart-safe retrieval and idempotent repetition, and fails closed under noncanonical input.

A private publication package owns the additive schema and canonical decoding. Its receipt binds the exact candidate artifact and byte digest, qualification or grouping-policy identity, source revision, generation identity, and predecessor selector. Representative status remains separately typed and is never inferred or upgraded by publication. Unresolved Group candidates may be published only with explicit unresolved status while preserving `authority=0`, `accepted=false`, `completeness=UNKNOWN`, and `featureIdentity=UNRESOLVED`.

The initial adapter is repository-private. It defines no public schema, CLI/MCP advertisement, production dispatch, release, or push authority. Historical artifacts remain immutable and are neither migrated nor normalized; any future migration policy, public exposure, or accepted-inventory publication requires a separate decision and qualification. This section selects policy boundaries only and does not authorize implementation.
