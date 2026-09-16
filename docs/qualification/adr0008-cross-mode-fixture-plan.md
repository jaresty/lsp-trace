# ADR 0008 canonical cross-mode fixture plan

Status: **FIXTURE_IMPLEMENTED — QUALIFICATION_NOT_EXECUTED**

Governing decision: [`HYBRID_SHARED_ALGEBRA`](../../qualification/adr0008-source-projection-matrix.v1.json)

This document records the smallest implemented retained/live contract-adjudication fixture for exact-source projection and the required successor fixture extension for structural display ranges and bounded multi-document live acquisition. The existing `crossmode-v1` files do not yet implement that extension. This document does not claim any qualification `PASS`, authorize runtime implementation, or qualify retained, live, or semantic behavior.

## Product boundary

The immediate product target is exact-source projection through one canonical `lsp_trace_v2_structural_context` operation. Its target is an exclusive union of one exact symbol or one exact URI/line/character position. Semantic Describe, Embed, and indexing are downstream consumers and do not gate this contract.

```text
symbol target
  -> exact workspace-symbol lookup
  -> exact URI plus original symbol
  -> shared structural/projection core

position target
  -> exact URI plus line and character
  -> shared structural/projection core

shared structural/projection core
  -> server-reported graph and occurrences
  -> Select structural units
  -> preserve exact evidence and server item ranges
  -> Select required document URIs
  -> exactly one retained or live document Resolve contract
  -> resolve optional structural display ranges from admitted bytes
  -> Project and Assemble
```

Symbol query text, candidate lists, resolved workspace-symbol ranges, and lookup diagnostics are locator-only. They add no graph facts, cannot manufacture `CALLS`, and do not participate in logical projected-unit identity. Omitted projection retains location-only behavior on the new unified contract. Both legacy symbol operations leave product discovery and routing; their immutable predecessor schema bytes remain historical readers. `lsp_trace_v1_trace` remains retained acquisition, and no operation 44 is added.

## Fixture inventory

The implemented fixture adds one directory:

`internal/sourceprojection/testdata/crossmode-v1/`

| Artifact | Sole ownership |
|---|---|
| `source.ts` | Exact source bytes. |
| `structural.json` | One endpoint/node, one server-reported recursive `CALLS`, one call-site occurrence, and authority invariants. |
| `retained-v5.json` | Exact Graph Provenance V5 carrier and capture identity. |
| `retained-availability.json` | V5 binding plus retained custody/availability and immutable source-object identity; no projection semantics. |
| `live-admission.json` | Exact managed session/generation and admitted URI/version/content identity. |
| `requests.json` | TARGET and bounded NEIGHBORHOOD requests from symbol-façade and exact-target entry points. |
| `variants.json` | Body, metadata-only, withheld, unavailable, byte-limit, range-limit, successful-empty, and input-permutation cases. |
| `expected.json` | Candidate canonical projection records, independent accounting, citations, identities, and semantic-execution-absent placeholders. |

This is one logical fixture over one shared projection engine, not separate retained and live projection implementations. A successor `crossmode-v2` extension must add at least three source documents: the target document, one same-document caller declaration, and one cross-document caller declaration. It must include distinct selection, item, call-site, and enclosing-declaration ranges and must not replace or rewrite the V1 fixture.

## Exact source specimen

`source.ts` contains these exact UTF-8 bytes, including the final newline:

```ts
function target() {
  "😀"; target();
}
```

- byte length: `42`
- SHA-256: `sha256:b2f615ccc7e60a8994f75a1e5cae0a73e0e7f34967a71288eabaad7840d121d5`
- fixture URI: `file:///fixture/src/source.ts`
- public logical source: `src/source.ts`
- position encoding: `utf-16`
- endpoint declaration range: `[0:0,3:0)`
- recursive call-site range: `[1:8,1:16)`
- emoji boundary: `[1:3,1:5)` in UTF-16

The call-site range selects exact text `target()`: 8 UTF-8 bytes. The endpoint range selects all 42 bytes. Therefore the fixture must preserve both counters:

- logical selected-byte sum: `50` (`42 + 8`);
- unique emitted-byte count: `42` because the call-site span lies inside the endpoint span.

The fixture retains two selections/citations despite physical overlap.

## Structural contents and invariants

Use stable fixture labels for one node, one relation, and one occurrence. The relation must be `CALLS` with `SERVER_REPORTED` evidence, and caller and callee must both be the fixture endpoint. Labels are not canonical hashes.

Every expected result preserves:

```text
authority = 0
graph_facts_added = 0
source_graph_complete = UNKNOWN
```

Source projection, names, proximity, overlap, and source text cannot create or repair `CALLS`.

## Custody pair

The retained representation binds:

- exact Graph Provenance V5 bytes and digest;
- capture identity;
- custody/availability manifest identity;
- exact immutable source identity: SHA-256 plus byte length;
- exactly one retained resolver kind.

The live representation models ephemeral metadata and deterministic expected digests only; fixture source files are synthetic test inputs and do not authorize production retention of live bytes. Production multi-document supplies exist only in memory for one request.

The live representation binds:

- exact managed session ID and generation;
- a mandatory target document plus a bounded canonical set of additional selected document URIs;
- exact URI, document version, declared `utf-16` encoding, content SHA-256, and byte length for every acquired supply;
- target-first then lexicographic document ordering;
- independent document candidates, selected, acquired, unavailable, withheld, limit-omitted, and byte accounting;
- exactly one live resolver kind for each selected document and no resolver fallthrough;
- a terminal assertion that no raw live supply is written to a manifest, source-object store, publication root, cache, log, fixture, or other durable carrier.

Private roots and arbitrary host selectors never appear. Retained and live physical projection identities must differ even though their logical unit and exact source bytes match. Placeholder digest labels are permitted in the planning fixture only when explicitly marked noncanonical; real goldens require the selected canonical preimage.

## Request pairs

The fixture provides four requests:

1. symbol-target TARGET;
2. position-target TARGET;
3. symbol-target bounded NEIGHBORHOOD;
4. position-target bounded NEIGHBORHOOD.

After removal of locator-only records, each symbol/position pair must produce identical structural facts and logical projected units. Physical identities remain custody-specific.

Projection omission must preserve location-only behavior on the unified contract. Body projection with publicly distinct evidence, server-item, and structural-display ranges plus multi-document custody requires successor immutable request/result/unified-result/envelope identities; predecessor schema bytes remain immutable.

## Variant expectations

The exact public vocabulary remains pending. The fixture must nevertheless distinguish these outcomes without collapsing causes:

| Variant | Candidates / selected / omitted | Logical bytes | Unique emitted bytes |
|---|---:|---:|---:|
| body returned | 2 / 2 / 0 | 50 | 42 |
| metadata only | 2 / 2 / 0 | 0 | 0 |
| withheld by policy | 2 / 0 / 2 | 0 | 0 |
| source unavailable | 2 / 0 / 2 | 0 | 0 |
| byte limit 41 | 2 / 0 / 2 | 0 | 0 |
| range limit 1 | 2 / 1 / 1 | 42 | 42 |
| successful empty | 0 / 0 / 0 | 0 | 0 |

Whole-range selection forbids shortening the 42-byte endpoint to satisfy a 41-byte limit. Successful empty is not source unavailable. Input permutation must yield byte-identical canonical projection output.

Structural acquisition, projection, and semantic accounting remain independent. Semantic execution is absent in every fixture variant; all semantic counters are zero, with `authority=0` and `accepted=false`.

## D1–D12 adjudication packet

The fixture must distinguish these decisions before implementation:

| Decision | Fixture comparison | Planned direction |
|---|---|---|
| D1 request versioning | exclusive symbol/position target plus omitted projection vs explicit TARGET | New immutable unified request schema; omission preserves location-only behavior. |
| D2 result versioning | unified location result vs body result | New immutable result/envelope schema identity. |
| D3 logical-unit identity | retained endpoint vs live endpoint | Shared role, graph subject, logical source, range, and encoding; custody excluded. |
| D4 citation identity | endpoint vs relation occurrence | Distinct endpoint/relation citations; occurrence identity participates. |
| D5 physical identity | retained vs live with identical bytes | Custody-specific resolver identity participates; identities unequal. |
| D6 status accounting | withheld vs unavailable vs limits | Projection causes remain typed and independent of structural status. Exact names/precedence remain pending. |
| D7 privacy | metadata-only vs withheld | Body opt-in and policy eligibility are distinct. Exact vocabulary remains pending. |
| D8 budgets | byte limit vs range limit | Independent named limits, whole ranges, one exact omission cause per unit. |
| D9 overlap | endpoint alone vs endpoint plus call site | One emitted span may support two retained selections and citations. |
| D10 empty | successful empty vs unavailable | Valid zero candidates is successful, not unavailable. |
| D11 canonicalization | normal vs reversed request arrays | Canonical ordering produces byte-identical output. Exact key order awaits goldens. |
| D12 semantic cache | retained/live semantic placeholders | Defer semantic cache preimages; freeze only cross-custody inequality and absent semantic execution. |
| D13 evidence/display separation | identifier selection, server item, and enclosing declaration | Citation identity binds exact evidence; display identity additionally binds structural range, provenance, adapter version, and custody. |
| D14 display provenance | identifier-sized gopls item range vs Go AST declaration range | Server item ranges are preserved but never relabeled as declarations; structural display uses an explicit qualified provenance such as `GO_AST_ENCLOSING_DECLARATION`. |
| D15 document acquisition | target, same-document caller, cross-document caller, unavailable caller | Document URIs derive only from admitted units; target is first and remaining URIs are lexical; each selected URI has one terminal acquisition disposition. |
| D16 document budgets | max documents, per-document bytes, total bytes, requests, messages, work | Documents and display ranges are admitted atomically; acquisition, projection, and response limits remain independent and exactly reconciled. |

## Ordered RED catalogue

A later implementation begins with assertion-specific failures in this order:

1. `ASSERT_CROSSMODE_FIXTURE_EXACT_BYTES_UTF16_RANGES`
2. `ASSERT_SERVER_ONLY_CALLS_OCCURRENCE_BINDING`
3. `ASSERT_V5_EXACT_CUSTODY_AND_SOURCE_OBJECT_BINDING`
4. `ASSERT_LIVE_EXACT_SESSION_DOCUMENT_ADMISSION`
5. `ASSERT_UNIFIED_SYMBOL_POSITION_TARGETS_DELEGATE_SHARED_CORE`
6. `ASSERT_UNIFIED_PROJECTION_OMISSION_PRESERVES_LOCATION_BEHAVIOR`
7. `ASSERT_BODY_OPT_IN_CHANGES_PROJECTION_ONLY`
8. `ASSERT_RETAINED_LIVE_PHYSICAL_IDENTITIES_DIFFER`
9. `ASSERT_ENDPOINT_RELATION_OVERLAP_ATTRIBUTION_EXACT`
10. `ASSERT_METADATA_WITHHELD_UNAVAILABLE_DISTINCT`
11. `ASSERT_BYTE_AND_RANGE_LIMITS_ADMIT_WHOLE_RANGES`
12. `ASSERT_SUCCESSFUL_EMPTY_NOT_UNAVAILABLE`
13. `ASSERT_INPUT_PERMUTATION_CANONICAL_OUTPUT`
14. `ASSERT_STRUCTURAL_PROJECTION_SEMANTIC_ACCOUNTING_INDEPENDENT`
15. `ASSERT_NO_RETRY_FALLBACK_REPAIR_OR_HIDDEN_LIMIT`
16. `ASSERT_AUTHORITY_GRAPH_FACTS_COMPLETENESS_INVARIANT`
17. `ASSERT_PREDECESSOR_SCHEMA_DIGESTS_UNCHANGED`
18. `ASSERT_41_CANONICAL_12_COMPACT_NO_OPERATION_44`
19. `ASSERT_SELECTION_ITEM_DISPLAY_RANGES_REMAIN_DISTINCT`
20. `ASSERT_IDENTIFIER_SIZED_SERVER_RANGE_NOT_DECLARATION`
21. `ASSERT_GO_AST_DISPLAY_RANGE_USES_ADMITTED_BYTES_ONLY`
22. `ASSERT_LIVE_DOCUMENT_SELECTION_TARGET_FIRST_LEXICAL`
23. `ASSERT_CROSS_DOCUMENT_CALLER_ACQUIRED_ONCE`
24. `ASSERT_UNAVAILABLE_CALLER_HAS_NO_FILESYSTEM_OR_RETAINED_FALLBACK`
25. `ASSERT_DOCUMENT_AND_DISPLAY_BUDGETS_ATOMIC_RECONCILED`
26. `ASSERT_ADDITIONAL_DOCUMENT_FAILURE_CANNOT_REWRITE_STRUCTURAL_RESULT`
27. `ASSERT_MULTI_DOCUMENT_DIRECT_GATEWAY_CLI_PARITY`
28. `ASSERT_LIVE_DOCUMENT_SUPPLIES_EPHEMERAL_NO_DURABLE_WRITE`

## Planned repository scope

The implemented fixture owns:

- `internal/sourceprojection/testdata/crossmode-v1/`;
- `internal/sourceprojection/crossmode_fixture_test.go`.

The planned additive extension should use a distinct `crossmode-v2` fixture directory or an equivalently immutable successor identity. It must not alter V1 source bytes or expected vectors.

Future assertion-specific REDs belong in the unified operation-36 schema, registry, direct-MCP, gateway, and CLI tests. Evidence references may be added to `qualification/adr0008-source-projection-matrix.v1.json` without changing a cell to `PASS` before execution qualifies it.

## Remaining blockers

1. Final additive request/result schema identities and dispatch mechanism.
2. Canonical preimage encoding for projected-unit, citation, and physical projection IDs.
3. Reviewed finite privacy and projection-status vocabularies.
4. Production retained manifest validator and resolver qualification.
5. Live document-byte admission and retention boundary.
6. Assertion-specific RED execution and implementation.
7. Direct MCP, execute gateway, CLI, and host-rendering qualification.
8. Independent retained/live qualification evidence.
9. Frozen successor request/result/envelope identities for structural-display policy and multi-document custody.
10. Qualified language-adapter contract, beginning with Go in-memory parsing over admitted bytes.
11. Canonical multi-document fixture bytes and D13–D16 goldens.
12. Real managed-server evidence for same-document and cross-document caller hydration.
13. ADR 0007 admission, privacy, ownership, evaluation, and execution prerequisites; none gate exact-source bodies.
