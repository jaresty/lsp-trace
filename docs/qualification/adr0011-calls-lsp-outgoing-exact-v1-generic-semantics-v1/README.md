# CALLS_LSP_OUTGOING_EXACT_V1 generic outgoing semantic freeze

Status: `GENERIC_OUTGOING_SEMANTICS_AUTHORED_NOT_GO`  
Authority: `0`; accepted: `false`; provider GO: `false`; production GO: `false`.

This package freezes provider-neutral generic outgoing LSP call-hierarchy semantics for `CALLS_LSP_OUTGOING_EXACT_V1`. It is a separate package from incoming semantics. It does not qualify any provider, does not cite Go/gopls behavior as semantic authority, does not modify incoming files, and does not change public schema, runtime behavior, composition, custody, or authority ceilings.

## Normative basis

Assumption: the LSP 3.17 callHierarchy source pinned in `NORMATIVE_SOURCES.md` is the normative method/type evidence. Basis: the pinned source has byte length `6812` and SHA-256 `5fd617bd3f3d6cb5d6dbd754febeeaebe6525808ff20bbb4ca85cee6c832eaf3`, matching the previously used incoming generic source pin.

Assumption: the accepted generic directional amendment is historical CALLS architecture context, not a substitute for the LSP source. Basis: it states independent incoming/outgoing profile IDs and explicitly rejects direction inheritance.

Structural precedent only: `adr0011-calls-lsp-incoming-exact-v1-generic-semantics-v1` informed file roles, checksum conventions, and verifier shape. Its incoming direction semantics do not supply outgoing authority where direction differs.

## Frozen outgoing semantic summary

For queried prepared caller `A`, a valid `callHierarchy/outgoingCalls` member with returned `to=B` retains the relationship `A -> B`. The member's `fromRanges[]` are the only call-site occurrence evidence, because the LSP says they are ranges relative to the caller item passed to `callHierarchy/outgoingCalls`. Returned `to.range` and `to.selectionRange` are endpoint identity/display metadata only.

A complete top-level `null` is distinct from a complete top-level `[]`. A member must contain valid required `to: CallHierarchyItem` and present non-null `fromRanges: Range[]`. Missing, null, or malformed required member content is an atomic malformed transaction with no valid-prefix salvage. A present empty `fromRanges: []` retains the otherwise-valid `A -> B` member/relationship and emits zero occurrence rows. Nonempty `fromRanges` preserves exact supplied order, duplicates, member ordinals, and range ordinals without synthesis, sorting, or deduplication.

Definition/Reference evidence, source text, documentSymbol ranges, provider heuristics, incoming reverse edges, and endpoint `to.range`/`to.selectionRange` never create or repair CALLS occurrence rows.

## Package files

- `NORMATIVE_SOURCES.md` pins and excerpts the LSP source.
- `SEMANTICS.md` gives the exact decision table.
- `fixtures.json` and `ORACLES.json` freeze executable examples.
- `verify_generic_outgoing_semantics.py` deterministically verifies fixtures, oracles, checksums, scope, and adversarial tamper tests.
- `ROLE_CLOSURE.md` states included roles and non-roles.
- `RECURSIVE_PRESUMPTION_LEDGER.md` records reversible assumptions and revision triggers.
- `REVIEW_REQUEST.md` asks for independent semantic review without provider qualification.
- `MANIFEST.json`, `DEPENDENCIES.sha256`, and `CHECKSUMS.sha256` bind the package identity and dependency closure.

## Ceiling

This package establishes no provider behavior, runtime behavior, source completeness, public schema, registry entry, implementation write-set, incoming semantics, composition behavior, custody change, provider qualification, or production admission. The only producer disposition is `GENERIC_OUTGOING_SEMANTICS_AUTHORED_NOT_GO`.
