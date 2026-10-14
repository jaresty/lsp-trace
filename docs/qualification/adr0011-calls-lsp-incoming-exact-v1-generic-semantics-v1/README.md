# ADR0011 CALLS_LSP_INCOMING_EXACT_V1 generic semantics v1 corrected

Status: `GENERIC_INCOMING_SEMANTICS_AUTHORED_NOT_GO`  
Profile: `CALLS_LSP_INCOMING_EXACT_V1`  
Authoring HEAD: `bcc96022dcfe0c7d559be387d45e92d204602a13`  
Authority: `0`; accepted: `false`; provider GO: `false`; incoming provider GO: `false`; production GO: `false`.

This confined docs/evidence package freezes independently reviewable provider-neutral generic incoming CALLS semantics for exact LSP `callHierarchy/incomingCalls` evidence. It corrects the independent reviewer blocker recorded as `CALLS_LSP_INCOMING_EXACT_V1_SEMANTICS_NO_GO`: returned `from` endpoint identity was under-specified and endpoint mutation tests were weak.

## Verdict

`GENERIC_INCOMING_SEMANTICS_AUTHORED_NOT_GO`

## Preserved accepted findings

- Normative source pin/checksums are preserved for LSP 3.17 call hierarchy text.
- Missing/null `fromRanges` is malformed.
- Present-empty `fromRanges: []` retains the incoming member/relationship and emits zero occurrence rows.
- Top-level `null` and `[]` remain distinct.
- Definition/Reference occurrences are not retyped as CALLS.
- Go/gopls behavior is not generic semantic authority.

## Correction summary

- `NORMATIVE_SOURCES.md` now pins and quotes the `CallHierarchyItem` field contract: required `name`, `kind`, `uri`, `range`, `selectionRange`, and optional `tags`, `detail`, `data`.
- Returned `from.range` and `from.selectionRange` are explicitly endpoint identity/display spans retained on relationship/member rows; they are not CALLS occurrence rows.
- Occurrence evidence comes solely from `fromRanges[]`.
- Fixtures/oracles now cover missing/null/malformed `from`, missing required endpoint fields, malformed endpoint URI/kind/range/selectionRange, valid endpoint mutations for URI/kind/range/selectionRange, optional endpoint metadata retention, absent/null/present-empty/nonempty `fromRanges`, malformed containers/ranges, duplicate ranges, and atomic refusal.
- The verifier validates endpoint fields independently and has ten negative tamper tests, including four tests proving valid endpoint mutation erasure is detected semantically rather than by fixture-hash mismatch.

## Files

- `SEMANTICS.md` — semantic decision table, endpoint identity/display rule, normalization order, and NO-GO correction notes.
- `NORMATIVE_SOURCES.md` — LSP 3.17 source pins and excerpts for `CallHierarchyItem` and incoming members.
- `DEPENDENCIES.sha256` — accepted-contract and normative-source pins.
- `fixtures.json` — executable fixture inputs.
- `ORACLES.json` — expected classifications, relationship endpoint rows, occurrence rows, diagnostics, and atomic-refusal expectations.
- `verify_generic_incoming_semantics.py` — deterministic verifier and negative tamper tests.
- `RECURSIVE_PRESUMPTION_LEDGER.md` — Presumed/Revised/Depends-on ledger including NO-GO lineage.
- `ROLE_CLOSURE.md` — role completeness and explicit non-roles.
- `REVIEW_REQUEST.md` — independent review boundary.
- `MANIFEST.json` — package manifest.
- `logs/correction.log` — correction and verification log.
- `CHECKSUMS.sha256` — package checksums excluding itself.

## Verify

```bash
python3 docs/qualification/adr0011-calls-lsp-incoming-exact-v1-generic-semantics-v1/verify_generic_incoming_semantics.py --source /private/tmp/adr0011-incoming-calls-v1-6yiY4m
```

Expected status is `PASS`; this is a verifier for self-authored fixtures/oracles only, not provider behavior.

## Explicit boundaries

No delegation, commit, provider qualification, outgoing semantics, composition semantics, source-runtime behavior, public surface, custody ownership, ceiling, implementation, tests, schema, registry, or production admission is changed by this package. The existing untracked failed provider candidate is excluded from semantic authority.

`GENERIC_INCOMING_SEMANTICS_AUTHORED_NOT_GO`
