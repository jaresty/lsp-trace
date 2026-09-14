# Backend-neutral protocol outline

Status: `PREREQUISITES_DRAFT`; pilot: `PILOT_DISABLED`. This is a proposed direction, not an implemented, frozen, qualified, approved, executable, or public schema.

## Prerequisite architecture direction

The semantic worker is a separate, backend-neutral, network-denied subprocess behind a serialization-neutral protocol boundary. It is never part of the `lsp-trace` core process or core `go.mod`. Backend-specific types, including Yzma types, never cross the boundary. This rejects the research report's proposed in-process Go `LocalModel` seam for every protocol-visible request; a backend adapter may exist only behind the worker boundary after later gates.

Yzma remains an unfrozen, replaceable candidate backend. Process isolation is containment only and confers no source authority, semantic authority, acceptance, approval, qualification, safety, or correctness.

## Deferred framing, vocabulary and lifecycle

ADR 0007 defers public names and schemas. This prerequisite therefore assigns no wire identity to any operation, control, record, enum, message, or lifecycle label. Any descriptive terms used below name required capabilities only; they are not protocol-visible names, tokens, enums, framing, or state transitions and may change when G1 freezes the protocol.

A future protocol must provide bounded, unambiguous framing, negotiation, request/response correlation, ordering, validation, lifecycle control and fail-closed rejection. Whether a frame is a complete JSON value or an NDJSON record remains unresolved. The negotiation mechanism, operation/control vocabulary, state transitions, cancellation representation, final LF/line-ending rule, and canonicalization algorithm also remain unresolved freeze decisions, together with limits, response/error shapes and terminal values.

The caller supplies admitted bytes and admitted relationships. The worker cannot reread source, traverse repository or arbitrary filesystem paths, follow symlinks, fetch URLs, access the network, download artifacts, execute tools, or silently fall back to another backend. Transport grants no filesystem, network, tool, traversal, or authority capability.

## Typed admissions and coverage

Every admission has exactly one typed corpus from a later-frozen corpus set, one finite item type, and one acquisition mode: `TARGET`, `NEIGHBORHOOD`, or `CENSUS`. Records bind submitted-byte digest/length, representation identity, media/schema identity, exact selector, source identity/revision/digest, authority/acceptance/currentness, admission disposition, expansion/scope, exclusions, failures, denominator and provenance. This draft does not add to or freeze the accepted ADR's corpus details.

Coverage claims have strict ceilings: `TARGET` covers only exact admitted targets; `NEIGHBORHOOD` covers only declared bounded expansion from targets; `CENSUS` covers only its declared closed admitted scope. Partial or failed work and bounded zero results never imply broader absence or completeness.

Source records retain source authority. Descriptions, embeddings, rankings, groups, summaries and all generated projections are derived records with exactly `authority=0` and `accepted=false`.

## Identity, cache and lineage

Every admitted input, request, response, member, operation, index, cache entry, backend, runtime, model, policy and schema has an immutable, content-bound identity. The exact cache preimage is a later-frozen ordered encoding of admission and dependency identities; representation; prompt/system/template/grammar; model/tokenizer/chat-template; policy; protocol/schema/enum; runtime/backend/native-library/environment identities; and inference parameters. Each component includes digest and byte length where representable. Only byte-identical preimages may hit cache.

Every record binds predecessor/supersedes references, correction event and actor authority, the versioned neutral `context_state_delta`, affected dependencies/products, and invalidation/rebuild dispositions. Changes append; they never mutate history. Deletion, correction, revocation or dependency change invalidates affected indexes, caches and products and requires a closed rebuild, deletion, unavailable, or externally-retained disposition.

## Operations and accounting

The protocol must support the abstract capabilities to describe admitted inputs, derive vector representations, build bounded indexes, search them, and propose groups. These descriptions are capability statements only, not operation names or wire labels. The accepted operation/member lists and denominator equations are incorporated **by immutable reference and list ID** from ADR 0007’s “Closed terminal states and denominator accounting”; copies are non-normative. This prerequisite draft invents no protocol-visible name or additional terminal enum.

A future manifest must identify each exact accepted list (`describe-operation-v1`, `describe-member-v1`, `embed-operation-v1`, `embed-member-v1`, `index-build-operation-v1`, `index-build-member-v1`, `search-operation-v1`, `search-member-v1`, `group-operation-v1`, `group-member-v1`) and each equation by digest. Every admitted or begun operation and member closes exactly once under the applicable accepted list; duplicate, omitted, extra and nonterminal members fail accounting. Incomplete operations preserve full denominators and evaluated counts without implying completeness or absence.

The protocol must support an explicit, accountable way to request that work stop and to close affected accounting; caller disconnection alone is insufficient. The representation and lifecycle semantics of that capability are deferred and confer no wire identity. Later requirements must bind deadlines, cooperative stop handling, forced process-tree termination, output closure, cleanup, restart behavior and races. Resource limits must cover the worker and descendants. Privacy, retention, deletion, backup, correction and revocation policies apply to inputs, outputs, logs, temporary files, indexes and caches.

## Conformance outline

Conformance vectors must test the later-frozen framing, ordering, lifecycle rules, duplicate handling, schema rejection, exact identity/cache misses, source/projection authority, revision coexistence, correction/invalidation/deletion/revocation, each accepted terminal state, balanced and deliberately unbalanced denominators, partial operations, coverage qualification, stop/resource/privacy behavior, and backend substitution with distinct identities.

## Unresolved freeze decisions

Before G1 can pass, a later decision must freeze at least: protocol and schema versions; framing as complete JSON values versus NDJSON records; the negotiation mechanism; operation and control vocabulary; state transitions; cancellation representation and races; the final LF/line-ending rule; canonicalization and digest algorithms; request/response correlation and ordering; exact field presence and unknown-field policy; size, depth, count and resource limits; shutdown semantics; error and retry policy; corpus and item-type schema details; identity and cache-preimage encoding; privacy, retention, deletion and revocation dispositions; backend capability negotiation; and conformance vectors. Model choice, embedding or generation metric, numeric thresholds and alternate-backend selection also remain unresolved outside this outline.
