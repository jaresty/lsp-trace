# Role closure

Status: `GENERIC_INCOMING_SEMANTICS_AUTHORED_NOT_GO`; authority `0`.

| Role | File(s) | Closure statement |
|---|---|---|
| Exact profile identity | `README.md`, `SEMANTICS.md`, `MANIFEST.json` | Names only `CALLS_LSP_INCOMING_EXACT_V1`; no outgoing, provider, public, or production profile is qualified. |
| Semantic decision table | `SEMANTICS.md` | Covers top-level null/empty, absent/null/present-empty/nonempty `fromRanges`, malformed containers, range validation, duplicates, order, and atomic refusal. |
| Normative source pins/excerpts | `NORMATIVE_SOURCES.md`, `DEPENDENCIES.sha256` | Pins LSP 3.17 call hierarchy source and quotes the incoming method/result/member text. |
| Accepted-contract dependency pins | `DEPENDENCIES.sha256` | Pins accepted amendment future order, outgoing present-empty/malformed symmetry, and D/R generic envelope/transport/applicability patterns. |
| Executable fixtures/oracles | `fixtures.json`, `ORACLES.json` | Covers absent/null, present-empty, nonempty, duplicates, top-level null/empty, malformed container/ranges, mixed-prefix atomic refusal, and multi-member relationship/occurrence separation. |
| Deterministic verifier | `verify_generic_incoming_semantics.py` | Recomputes fixture outcomes and checks checksums without LSP/provider execution. |
| Negative tamper tests | `verify_generic_incoming_semantics.py` | Detects absent-admitted, empty-refused, duplicate-collapsed, direction-reversed, prefix-salvaged, endpoint URI-erased, endpoint kind-erased, endpoint range-erased, endpoint selectionRange-erased, and endpoint-range-as-occurrence tampering. |
| Recursive presumption ledger | `RECURSIVE_PRESUMPTION_LEDGER.md` | Records Presumed/Revised/Depends-on state for initial hypotheses and dependencies. |
| Review request | `REVIEW_REQUEST.md` | Asks for only generic incoming semantic review; reviewer may return GO token for exact package or REVISE/REJECT. |
| Manifest/checksums | `MANIFEST.json`, `CHECKSUMS.sha256` | Provides deterministic file inventory and package checksums. |

## Explicit non-roles

- No provider qualification, including no Go/gopls incoming provider GO.
- No outgoing semantic freeze or outgoing qualification.
- No composition/source-runtime/public/custody/ceiling changes.
- No public schema/API/selector/registry/build-tag mutation.
- No production/test code mutation.
- No D/R occurrence retyping as CALLS.
- No use of the existing untracked failed provider candidate as semantic authority.
