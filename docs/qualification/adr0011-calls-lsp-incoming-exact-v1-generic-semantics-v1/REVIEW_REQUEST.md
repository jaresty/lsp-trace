# Independent review request: CALLS_LSP_INCOMING_EXACT_V1 generic semantics v1 corrected

Requested token: `CALLS_LSP_INCOMING_EXACT_V1_GENERIC_SEMANTICS_GO` or `REVISE`/`REJECT`  
Prior independent result: `CALLS_LSP_INCOMING_EXACT_V1_SEMANTICS_NO_GO`  
Producer verdict before review: `GENERIC_INCOMING_SEMANTICS_AUTHORED_NOT_GO`  
Authority: `0`; accepted: `false`; provider GO: `false`; incoming provider GO: `false`; production GO: `false`.

Review only the exact docs/evidence package bytes in `docs/qualification/adr0011-calls-lsp-incoming-exact-v1-generic-semantics-v1/` at authoring HEAD `bcc96022dcfe0c7d559be387d45e92d204602a13`.

## Required checks

1. Verify `DEPENDENCIES.sha256`, `MANIFEST.json`, and `CHECKSUMS.sha256` from exact bytes.
2. Verify the package location/name reconciles with the accepted amendment future order: this is step 2, freezing/reviewing `CALLS_LSP_INCOMING_EXACT_V1` generic contract fixtures/evidence only.
3. Verify no file outside this package is modified and the existing untracked failed provider candidate is not cited as semantic authority.
4. Verify LSP 3.17 source pin/excerpts establish `CallHierarchyItem` required endpoint fields `name`, `kind`, `uri`, `range`, `selectionRange`, optional `tags`, `detail`, `data`, incoming method `callHierarchy/incomingCalls`, result `CallHierarchyIncomingCall[] | null`, member `from: CallHierarchyItem`, and `fromRanges: Range[]` relative to `this.from`.
5. Verify returned `from.range` and `from.selectionRange` are retained endpoint identity/display spans and never become CALLS occurrence rows.
6. Verify occurrence rows are emitted solely from supplied `fromRanges[]` elements; present-empty `fromRanges: []` retains relationship/member/endpoint identity and emits zero occurrence rows.
7. Verify absent/null/malformed `from` cases refuse atomically and fixtures/oracles cover missing required endpoint fields plus malformed URI/kind/range/selectionRange.
8. Verify valid distinct mutations of `from.uri`, `from.kind`, `from.range`, and `from.selectionRange` are retained in oracle relationship output and do not synthesize occurrence ranges.
9. Verify absent and null member `fromRanges` are malformed for semantic admission; raw absence/null may be retained only as malformed evidence/custody, not as admitted relationship or occurrence.
10. Verify nonempty `fromRanges` preserves order, duplicate multiplicity, member ordinal, range ordinal, exact range, and caller-side ownership without synthesis or de-duplication.
11. Verify relationship/member evidence is distinct from occurrence/range evidence throughout fixtures and oracles.
12. Verify malformed member/range/container cases refuse atomically and do not salvage a valid prefix.
13. Verify no Definition/Reference occurrence, documentSymbol result, source text, outgoing reversal, runtime trace, provider heuristic, `from.range`, or `from.selectionRange` is retyped as CALLS occurrence evidence.
14. Run `python3 docs/qualification/adr0011-calls-lsp-incoming-exact-v1-generic-semantics-v1/verify_generic_incoming_semantics.py --source <repo-root>` and confirm PASS plus ten tamper tests, including the four valid endpoint mutation-erasure tests.
15. Hard-stop as `REVISE` if the review requires public schema/API/selector/runtime mutation, provider qualification, outgoing/composition/source-runtime/custody/ceiling change, or any expansion beyond this package.

## Permitted result

A reviewer may issue `CALLS_LSP_INCOMING_EXACT_V1_GENERIC_SEMANTICS_GO` only for this exact package digest and dependency closure, or return `REVISE`/`REJECT`. The token would be generic incoming semantic evidence only. It would not qualify Go/gopls, issue provider GO, authorize implementation/tests, publish schemas, alter public surfaces, qualify outgoing, or admit production behavior.

`GENERIC_INCOMING_SEMANTICS_AUTHORED_NOT_GO`
