# Backend-neutral protocol outline

Status: `PREREQUISITES_DRAFT`; pilot: `PILOT_DISABLED`. This is a proposed direction, not an implemented, frozen, qualified, approved, executable, or public schema.

## Prerequisite architecture direction

The semantic worker is a separate, backend-neutral, network-denied subprocess behind a JSON/NDJSON protocol boundary. It is never part of the `lsp-trace` core process or core `go.mod`. Backend-specific types, including Yzma types, never cross the boundary. This rejects the research report's proposed in-process Go `LocalModel` seam for every protocol-visible request; a backend adapter may exist only behind the worker boundary after later gates.

Yzma remains an unfrozen, replaceable candidate backend. Process isolation is containment only and confers no source authority, semantic authority, acceptance, approval, qualification, safety, or correctness.

## Proposed framing and state machine

The draft direction is strict UTF-8 NDJSON with one JSON object per frame. A worker session begins with exactly one `HELLO`, then permits only finite `DESCRIBE`, `EMBED`, `INDEX_BUILD`, `SEARCH`, `GROUP`, `CANCEL`, and `SHUTDOWN` requests according to a later-frozen state machine. Unknown versions, record types, fields, duplicate identities, invalid transitions, or post-terminal records fail closed under a later-frozen policy. The final line-ending rule, canonical JSON algorithm, limits, response/error shapes, and terminal values are unresolved; this outline does not freeze them.

The caller supplies admitted bytes and admitted relationships. The worker cannot reread source, traverse repository or arbitrary filesystem paths, follow symlinks, fetch URLs, access the network, download artifacts, execute tools, or silently fall back to another backend. Transport grants no filesystem, network, tool, traversal, or authority capability.

## Typed admissions and coverage

Every admission has exactly one typed corpus from a later-frozen corpus set, one finite item type, and one acquisition mode: `TARGET`, `NEIGHBORHOOD`, or `CENSUS`. Records bind submitted-byte digest/length, canonicalization identity, media/schema identity, exact selector, source identity/revision/digest, authority/acceptance/currentness, admission disposition, expansion/scope, exclusions, failures, denominator and provenance. This draft does not add to or freeze the accepted ADR's corpus details.

Coverage claims have strict ceilings: `TARGET` covers only exact admitted targets; `NEIGHBORHOOD` covers only declared bounded expansion from targets; `CENSUS` covers only its declared closed admitted scope. Partial or failed work and bounded zero results never imply broader absence or completeness.

Source records retain source authority. Descriptions, embeddings, rankings, groups, summaries and all generated projections are derived records with exactly `authority=0` and `accepted=false`.

## Identity, cache and lineage

Every admitted input, request, response, member, operation, index, cache entry, backend, runtime, model, policy and schema has an immutable, content-bound identity. The exact cache preimage is a later-frozen ordered encoding of admission and dependency identities; representation; prompt/system/template/grammar; model/tokenizer/chat-template; policy; protocol/schema/enum; runtime/backend/native-library/environment identities; and inference parameters. Each component includes digest and byte length where representable. Only byte-identical preimages may hit cache.

Every record binds predecessor/supersedes references, correction event and actor authority, the versioned neutral `context_state_delta`, affected dependencies/products, and invalidation/rebuild dispositions. Changes append; they never mutate history. Deletion, correction, revocation or dependency change invalidates affected indexes, caches and products and requires a closed rebuild, deletion, unavailable, or externally-retained disposition.

## Operations and accounting

Conceptual records cover Describe, Embed, Index-build, Search and Group. Their operation/member enums and denominator equations are incorporated **by immutable reference and list ID** from ADR 0007’s “Closed terminal states and denominator accounting”; copies are non-normative. This prerequisite draft invents no additional terminal enum.

A future manifest must identify each exact accepted list (`describe-operation-v1`, `describe-member-v1`, `embed-operation-v1`, `embed-member-v1`, `index-build-operation-v1`, `index-build-member-v1`, `search-operation-v1`, `search-member-v1`, `group-operation-v1`, `group-member-v1`) and each equation by digest. Every admitted or begun operation and member closes exactly once under the applicable accepted list; duplicate, omitted, extra and nonterminal members fail accounting. Incomplete operations preserve full denominators and evaluated counts without implying completeness or absence.

Cancellation is a protocol-visible request and a closed outcome under accepted accounting, not merely caller disconnection. Later requirements must bind deadlines, cooperative cancellation, forced process-tree termination, output closure, cleanup, restart behavior and cancellation races. Resource limits must cover the worker and descendants. Privacy, retention, deletion, backup, correction and revocation policies apply to inputs, outputs, logs, temporary files, indexes and caches.

## Conformance outline

Canonical vectors must test framing, ordering, state transitions, duplicate handling, schema rejection, exact identity/cache misses, source/projection authority, revision coexistence, correction/invalidation/deletion/revocation, each accepted terminal state, balanced and deliberately unbalanced denominators, partial operations, coverage qualification, cancellation/resource/privacy behavior, and backend substitution with distinct identities.

## Unresolved freeze decisions

Before G1 can pass, a later decision must freeze at least: protocol and schema versions; JSON versus NDJSON transport scope; the final LF/line-ending rule; canonical JSON and digest algorithms; request/response correlation and ordering; exact field presence and unknown-field policy; size, depth, count and resource limits; cancellation races and shutdown semantics; error and retry policy; corpus and item-type schema details; identity and cache-preimage encoding; privacy, retention, deletion and revocation dispositions; backend capability negotiation; and conformance vectors. Model choice, embedding or generation metric, numeric thresholds and alternate-backend selection also remain unresolved outside this outline.
