# Role closure for CALLS_LSP_OUTGOING_EXACT_V1 generic semantics

Status: `GENERIC_OUTGOING_SEMANTICS_AUTHORED_NOT_GO`; authority `0`.

## Included roles

1. **Normative LSP source role** — pinned `callHierarchy.md` at commit `f1b0378161a72018a2abeb5794532d5f6e2aebac`, byte length `6812`, SHA-256 `5fd617bd3f3d6cb5d6dbd754febeeaebe6525808ff20bbb4ca85cee6c832eaf3`.
2. **Accepted historical contract role** — accepted generic directional amendment and accepted CALLS/D/R boundaries, limited to profile independence, no D/R retyping, target-bound preparation, terminal distinctions, and authority ceilings.
3. **Structural precedent role** — generic incoming package file layout, checksum convention, fixture/oracle/verifier pattern, and review packet shape only.
4. **Outgoing semantic role** — this package's semantic table, fixtures, oracles, and verifier for provider-neutral outgoing semantics.
5. **Review role** — independent reviewer must check exact bytes, direction correctness, endpoint/occurrence separation, and non-authority boundaries before any future use.

## Closed non-roles

- Go/gopls behavior is not semantic authority.
- The existing outgoing provider NO_GO package is not semantic authority.
- Incoming generic semantics do not authorize outgoing direction semantics where direction differs.
- Provider evidence, runtime evidence, raw provider captures, workload corpus results, and fake-LSP behavior are not generic semantic authority.
- Definition/Reference occurrences, source text, documentSymbol ranges, endpoint display spans, or repository scans are not CALLS occurrence sources.
- This package does not qualify `CALLS_GO_GOPLS_OUTGOING_EXACT_V1` or any other provider binding.
- This package does not alter public schemas, runtime behavior, composition, custody, ceilings, incoming files, implementation files, tests, or production settings.

## Closure statement

All roles required for `CALLS_LSP_OUTGOING_EXACT_V1` generic semantics are either supplied by this package, explicitly pinned as historical/normative context, or closed as non-authorities. Any future provider qualification must bind to this exact generic outgoing package without adding, deleting, retyping, or relaxing its semantics.
