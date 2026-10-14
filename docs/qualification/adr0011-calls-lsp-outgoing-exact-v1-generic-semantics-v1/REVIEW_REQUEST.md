# Review request: CALLS_LSP_OUTGOING_EXACT_V1 generic semantics

Requested review token: `CALLS_LSP_OUTGOING_EXACT_V1_GENERIC_SEMANTICS_REVIEW`  
Producer disposition: `GENERIC_OUTGOING_SEMANTICS_AUTHORED_NOT_GO`  
Authority: `0`; accepted: `false`; provider GO: `false`; production GO: `false`.

## Review scope

Please independently review only the provider-neutral generic outgoing semantics package at:

`docs/qualification/adr0011-calls-lsp-outgoing-exact-v1-generic-semantics-v1/`

Review should check:

1. Normative LSP pin and excerpts are sufficient and exact for `callHierarchy/outgoingCalls`, `CallHierarchyOutgoingCall`, `CallHierarchyItem`, and `Range[]` occurrence evidence.
2. Orientation is correct: queried caller `A` plus returned `to=B` yields normalized `A -> B`.
3. `fromRanges` is required, non-null, and the only occurrence evidence.
4. Present empty `fromRanges: []` retains the member/relationship with zero occurrences, unless normative evidence contradicts it.
5. Missing/null/malformed `to`, missing/null/malformed `fromRanges`, malformed ranges, and malformed endpoint fields cause atomic refusal with no prefix salvage.
6. Nonempty ranges preserve exact order, duplicates, member ordinals, and range ordinals without synthesis or deduplication.
7. `to.range` and `to.selectionRange` are endpoint identity/display only and never occurrence evidence.
8. D/R evidence, source text, documentSymbol, incoming reverse edges, provider heuristics, and endpoint ranges are not retyped as CALLS.
9. Multi-item prepare selection/target binding is present and outgoing-specific.
10. The package does not merge incoming/outgoing authority and does not use provider/runtime evidence.
11. Verifier and tamper tests detect endpoint mutation erasure, direction reversal, duplicate collapse, empty-array refusal, missing-field admission, prefix salvage, and endpoint-range-as-occurrence synthesis.
12. Manifest, dependency pins, and checksums bind all files without self-hashing `CHECKSUMS.sha256`.

## Explicit non-scope

Do not qualify `CALLS_GO_GOPLS_OUTGOING_EXACT_V1` or any provider binding. Do not infer runtime support, source completeness, fake-LSP behavior, gopls behavior, workload-corpus claims, public schema changes, composition changes, custody changes, ceiling changes, production readiness, or GO from this package.

## Expected review outcomes

- `CALLS_LSP_OUTGOING_EXACT_V1_GENERIC_SEMANTICS_GO` only if the exact generic outgoing bytes and review scope pass.
- `CALLS_LSP_OUTGOING_EXACT_V1_GENERIC_SEMANTICS_NO_GO` if any semantic, authority, fixture, verifier, checksum, or boundary defect remains.

`GENERIC_OUTGOING_SEMANTICS_AUTHORED_NOT_GO`
