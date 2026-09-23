# ADR 0011 — definition schema and policy boundaries (proposal)

**Status: PROPOSED DESIGN, NOT REGISTERED OR ACCEPTED.** These are closed-record shapes to review before registering immutable schemas or implementation. Literal draft IDs below are **not** production schema URLs and their hashes are not yet pinned. Neither these definitions nor a matching self-hash qualify admission.

## Canonical record rule proposed for every role

One UTF-8 JSON object encoded by a versioned fixed-field struct in the order listed below, `encoding/json` Go string/number encoding, no omitted required fields, no unknown/duplicate decoded keys at any nesting level, no floats, no unordered maps, and exactly one trailing LF; exact complete bytes (including LF) ≤1,500,000. Arrays retain declared/original order, with separately documented sorted dependency lists; canonical replay independently decodes strictly, re-encodes, compares bytes and verifies role-separated SHA-256 selector/digest. Empty required strings, noncanonical integer spelling, changed source or policy, and trailing content fail closed. The proposed schema records below must become actual pinned machine-readable schemas with byte fixtures before contract acceptance; a free-text list alone is insufficient.

| Draft role/version | Required ordered fields and replay checks |
| --- | --- |
| `definition.capability.v1` | version, session ID/generation, managed READY/workspace root, provider/profile/executable identity as reported, negotiated encoding, advertised definition support present/absent, initialize evidence selector/digest, capability policy ID/digest. No invocation if absent. |
| `definition.query-occurrence.v1` | version, predeclared query ID, URI/point, exact range and `GO_IDENTIFIER_RANGE_V1` or separately qualified language adapter ID/digest, source URI/version/digest/length and immutable payload selector, before-Git observation, capability selector/digest, adapter terminal outcome, full bounded derivation-work counters. Replayed before definition invocation. |
| `definition.method.v1` | version, query and capability selector/digest, method name, exact raw params bytes/digest, session/generation/key/invocation, completed request write, matched read or explicit failure kind, result presence/length/exact payload selector/digest, source/revision/Git observations, provider/adapter as reported, method/privacy policy ID/digest. |
| `definition.evaluation.v1` | version, method/query selector/digest, bounded result digest/shape, ordered `QUERY_BEGIN`, optional `TOP_LEVEL_COUNT`, per-ordinal `ELEMENT_BEGIN` and `ELEMENT_EVALUATED` events with half-open raw-byte spans/digests/work, whole-result result/parse event, internal N/B/E/E_B/E_T/P and explicit UNKNOWN E. No reader may infer omitted events. |
| `definition.terminal.v1` | version, method/query/evaluation selectors/digests, single closed method outcome and query/element dispositions where issuable, N/B/T/E/E_B/E_T/P/A and diagnostic prefix separately, privacy/disposition and publication status. `T=0` if this record cannot independently verify. |
| `definition.occurrences.v1` | version, all predecessor selectors/digests, canonical typed `RESOLVES_TO_DEFINITION` list with exact query→target direction, original result shape/ordinal/URI/ranges, source and cross-document target selectors/digests, transaction-scoped candidate ID and distinct occurrence ID, authority 0/accepted false/completeness UNKNOWN/no producer authentication. |
| `definition.dependencies.v1` | version, monotonic root generation, previous verified manifest selector/digest, decision ID, sorted unique occurrence/derivative and query/method/target-source/terminal/payload selector/digest dependencies, `ACTIVE | TOMBSTONED | QUARANTINED`, effective visibility event, attempted and failed deletion inventory. An empty or stale dependency set rejects. |

Every nested observation, range, identifier, digest, policy, and event has its own closed struct/enum and independent substitution guard; strings are not arbitrary `any`. Records bind exact source/encoding/revision/policy profiles without claiming provider authentication. Diagnostic failure artifacts have separate redacted schema; a failed committed record has **no verified receipt**, even if bytes exist at its selector.

## Policy digest boundaries to freeze independently

- `GO_GOPLS_DEFINITION_EXACT_V1`: provider/capability/encoding/revision and root admission; future CUE profile separate.
- `GO_IDENTIFIER_RANGE_V1` and `DEFINITION_QUERY_OCCURRENCE_V1`: source lexical derivation and query receipt, not grouping endpoint resolution.
- `DEFINITION_METHOD_V1`: exact request/result/shape/strict nested grammar and bound precedence.
- `DEFINITION_EVALUATION_V1`: event ordering/offsets/counts/work and partial-wire handling.
- `DEFINITION_PRIVATE_EXACT_V1`: raw payload privacy, diagnostic redaction and withholding rules.
- `DEFINITION_OCCURRENCE_V1`: direction, ordinal, identity and all-or-none cross-document admission.
- `DEFINITION_DEPENDENCIES_V1`: root lock, reader lease, manifest/tombstone and revoke ordering.

Hash **complete exact canonical policy bytes** separately for each ID; bind all selected IDs/digests into every dependent record. A changed field, schema byte, executable policy implementation or numeric cap requires a new digest and independent test. Pin each proposed schema's exact registered ID, version, bytes, digest and implementation digest only after independent review, before implementation acceptance. Keep references schemas and CALLS-only bytes unchanged. Missing schema/policy bytes are `INCOMPLETE`, never an inferred match.
