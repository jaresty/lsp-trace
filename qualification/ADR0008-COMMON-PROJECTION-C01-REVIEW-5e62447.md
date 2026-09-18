# ADR 0008 common projection C01 qualification review

- Reviewed revision: `5e624471011cb679f731bbc4079a4229f2f986b7`
- Execution record: `qualification/adr0008-common-projection.execution.5e624471011cb679f731bbc4079a4229f2f986b7.json`
- Predecessor: `qualification/adr0008-common-projection.execution.ceac4ab9ada237029e513a4947b464a5f46e1d50.json`
- Governing matrix: unchanged
- Result: `C01–C06=PASS`; `COMMON_PROJECTION=PASS`; `implementation_qualified=false`

## C01 evidence

The assertion-specific corpus independently mutates every candidate identity field:

- role;
- graph subject;
- relation occurrence;
- logical source identity;
- range start and range end;
- position encoding;
- privacy classification.

Each mutation changes both the projected unit identity and its citation identity. The implementation factors identity derivation into testable helpers while preserving the exact historical endpoint and relation preimage shapes and therefore existing identity bytes.

The integrated composite-identity corpus additionally mutates source digest, source byte length, custody mode, custody binding, request policy identity, and source availability. Each mutation changes the required physical, custody, policy, or disposition carrier rather than relying on an unrelated serialized difference.

One byte-level permutation oracle reverses candidate, endpoint/relation unit, endpoint/relation citation, emitted-span, and omission order. V2 assembly now sorts copied unit, citation, span, and omission collections deterministically without mutating caller input. Both LIVE and RETAINED custody produce byte-identical canonical output under those semantically irrelevant permutations, while custody identities remain distinct across modes.

The focused C01 execution and full repository suite passed at the reviewed revision. Log paths and SHA-256 digests are recorded in the execution record. Released schemas and historical qualification records are unchanged.

## Common-track disposition

C01 is PASS. C02–C06 are carried forward after full-suite re-execution at the same reviewed revision. Therefore `COMMON_PROJECTION=PASS` at `5e624471011cb679f731bbc4079a4229f2f986b7`.

Historical `L04=FAIL` is unchanged. This review does not qualify the overall implementation, and `implementation_qualified=false` remains required.
