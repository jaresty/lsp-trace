# Backend-neutral protocol outline

Status: `PREREQUISITES_DRAFT`. This is not an implemented or approved schema.

A future closed protocol uses UTF-8 NDJSON: one canonical JSON object per line, explicit `protocol_version`, `record_type`, `record_id`, `sequence`, and content digest. Requests and responses are append-only; unknown versions or fields fail by frozen policy. Transport conveys records only and grants no filesystem, network, tool, traversal, or authority capability. Yzma, if later selected, is a replaceable isolated process backend.

## Typed admissions and coverage

Every admission has exactly one corpus (`REVISION_BOUND_CODE_STRUCTURAL`, `ACCEPTED_DECISION_REQUIREMENT`, or `WORKING_CONTEXT`), one finite item type, and one acquisition mode: `TARGET`, `NEIGHBORHOOD`, or `CENSUS`. Records bind submitted-byte digest/length, canonicalization, media/schema identity, exact selector, source identity/revision/digest, authority/acceptance/currentness, admission disposition, expansion/scope, exclusions, failures, denominator and provenance.

Source records retain source authority. Descriptions, embeddings, rankings, groups, summaries and all projections are derived records with exactly `authority=0` and `accepted=false`. Process separation is containment, not semantic or adjudicative authority.

## Identity, cache and lineage

The exact cache preimage is the ordered canonical encoding of: admission and dependency identities; representation; prompt/system/template/grammar; model/tokenizer/chat-template; policy; protocol/schema/enum; runtime/backend/native-library/environment identities; and inference parameters. Each component includes digest and byte length where representable. Only byte-identical preimages may hit cache.

Every record binds predecessor/supersedes references, correction event and actor authority, the versioned neutral `context_state_delta`, affected dependencies/products, and invalidation/rebuild dispositions. Changes append; they never mutate history.

## Operations and accounting

Conceptual records cover Describe, Embed, Index-build, Search and Group. Their operation/member enums and denominator equations are incorporated **by immutable reference and list ID** from ADR 0007’s “Closed terminal states and denominator accounting”; copies are non-normative. A future manifest must identify each exact list (`describe-operation-v1`, `describe-member-v1`, `embed-operation-v1`, `embed-member-v1`, `index-build-operation-v1`, `index-build-member-v1`, `search-operation-v1`, `search-member-v1`, `group-operation-v1`, `group-member-v1`) and each equation by digest. Every begun member closes exactly once; incomplete operations preserve full denominators and evaluated counts without implying completeness or absence.

## Conformance outline

Canonical vectors must test framing, ordering, duplicate handling, schema rejection, exact identity/cache misses, source/projection authority, revision coexistence, correction/invalidation, each terminal state, balanced and deliberately unbalanced denominators, partial operations, coverage qualification, and backend substitution with distinct identities.
