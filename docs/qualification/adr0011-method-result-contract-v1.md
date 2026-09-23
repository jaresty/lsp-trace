# ADR 0011 private method-result grammar v1

**State:** private design freeze for bounded parser tests. No parser, receipt, occurrence admission, public schema, CLI/MCP operation, grouping-policy qualification, or semantic acceptance is established by this document. Changing a rule below requires a new private grammar version and corresponding fixture expectations. ADR 0011 remains controlling for acquisition, custody, request/member aggregation, source context, replay, and claim ceilings.

## Input and parsing boundary

A parser receives an exact method name, the bounded successful raw JSON result of one managed `textDocument/definition` or `textDocument/references` transaction, and an explicit positive candidate limit supplied by its caller. The transport's typed outcome must be `TRANSPORT_SUCCESS` before parsing. A JSON-RPC error, timeout, cancellation, unsupported capability, or over-limit transport result is **not** a method result and must not be coerced into `null` or `[]`. No numeric candidate limit is selected by this grammar; a parser cannot run without an explicit limit and must stop before retaining more items than that limit. Resource exhaustion is not an empty result.

Parse exactly one JSON value. Reject trailing data, malformed JSON, duplicate object keys at any depth, incorrect field types, and any result shape outside the method-specific forms below. Unknown extension fields may be retained in the raw result for diagnosis, but cannot supply a required standard field, alter identity, or create an occurrence. Reject non-integral, negative, or greater-than-2³¹−1 line/character coordinates. Ranges are half-open, have nonnegative endpoints, and require start ≤ end; an empty server-reported range remains exact input evidence, not a fabricated nonempty definition or context span. Do not normalize URI strings, rewrite ranges, resolve documents, or infer source bodies in this parser. A required URI is a nonempty string; later URI/custody admission is separate.

## Definition result: `textDocument/definition`

Accepted top-level forms are precisely `null`, one `Location`, a homogeneous array of `Location`, or a homogeneous array of `LocationLink`. `null` and `[]` are well-formed **zero returned targets**. A lone `LocationLink` object, mixed arrays, nested arrays, and unrelated JSON objects/scalars are malformed. Empty `[]` has no member kind and needs none.

A `Location` requires `uri` and `range`. Preserve its exact URI and range as one returned target item. A `LocationLink` requires `targetUri`, `targetRange`, and `targetSelectionRange`; `originSelectionRange` is optional. Preserve `targetRange` as the server item envelope and `targetSelectionRange` as the exact target selection; require the selection range to lie within `targetRange`. Preserve any `originSelectionRange` separately as a server-reported hint, never as a replacement for the request's exact query occurrence. Neither target range is automatically a server-reported *full-definition display* range; that requires a separately bounded document-symbol resolution receipt and admitted source supply.

Return one parsed item per array element in original zero-based response order; a scalar `Location` has ordinal zero. Do not choose the first target, deduplicate repeated items, rewrite a URI, or collapse equal endpoint pairs. Parsing alone neither binds an item to an admitted query occurrence nor creates `RESOLVES_TO_DEFINITION` evidence.

## References result: `textDocument/references`

Accepted top-level forms are precisely `null` or an array of `Location`. Both `null` and `[]` are well-formed **zero returned locations**. Reject a scalar `Location`, every `LocationLink`, mixed arrays, and unrelated JSON values. Each location requires its exact `uri` and `range`; preserve its original zero-based array ordinal and all repeated items. The request's `context.includeDeclaration` remains in its own method receipt: do not insert, delete, or relabel returned locations based on its value.

A well-formed references array does not identify the referenced-symbol target. An admitted query-target receipt binding a pre-existing canonical identity to the exact query occurrence remains mandatory before any `REFERENCES_SYMBOL` occurrence can be admitted. A returned location cannot repair an absent, ambiguous, mismatched, or unverifiable query-target receipt.

## Malformed and claim boundaries

A required field that is absent, null, duplicated, of the wrong type, or carries an invalid range makes that member malformed. A malformed top-level form makes the whole parse malformed. A mixed array is malformed even when its individual items look valid. Retain raw bytes and exact offending ordinals for private diagnosis within the transport/privacy bounds; do not expose malformed members as occurrences. Independently well-formed members may be reported for diagnosis, but this parser cannot admit them, decide request `COMPLETE`/`COMPLETE_EMPTY`/`PARTIAL`, or manufacture terminal member outcomes. ADR 0011's separate acquisition/admission layer applies request precedence and complete denominator accounting; any required malformed provider output makes its request `MALFORMED`, not `COMPLETE`.

`null` means zero *returned items from this exact successful query*, not absence elsewhere, workspace completeness, or a complete request by itself. Exact ranges are not body bytes. Method parsing does not authenticate document supply, establish target identity, infer `CALLS`, choose a grouping policy, authorize Leiden, or accept a feature. All later presentations remain `authority=0`, `accepted=false`, `completeness=UNKNOWN` unless separately authorized under their own contracts.

## Qualification boundary

This grammar is ready to be implemented and falsified by test-only method fixtures, including scalar and array `Location`, `LocationLink[]`, both null variants, duplicate and mixed members, missing/wrong-type/duplicate fields, trailing data, range boundaries, explicit candidate exhaustion, and preservation of duplicate ordinals. No such parser tests have been run by this document. Receipt shape, canonical occurrence IDs, admission, source-context supply, terminal per-candidate accounting, public schemas and policy qualification remain separate design/verification gates. Historical omitted-selector `CALLS_ONLY` bytes and behavior are unchanged.
