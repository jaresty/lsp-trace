# ADR 0008 common projection C02 qualification review

- Reviewed revision: `c23ff0b2245a5a278c5cbb2dbf8f9a1ab5789677`
- Execution record: `qualification/adr0008-common-projection.execution.c23ff0b2245a5a278c5cbb2dbf8f9a1ab5789677.json`
- Predecessor: `qualification/adr0008-common-projection.execution.6020d43fcb09bcee4f6b5cdcd6cd0c7d2780038d.json`
- Governing matrix: unchanged
- Result: `C02=PASS`, `C03=PASS`; `COMMON_PROJECTION=BLOCKED`; `implementation_qualified=false`

## C02 evidence

The integrated corpus executes UTF-8, UTF-16, and UTF-32 over exact UTF-8 bytes containing a BOM, a non-BMP rune, CRLF, and LF. Separate position tests also execute CR. It proves:

- empty and cross-line half-open ranges;
- exact non-BMP boundaries under every supported position encoding;
- a whole-document endpoint display range;
- a cross-line server-reported relation occurrence;
- distinct endpoint and relation citation identities and subjects;
- independent evidence and display ranges;
- one overlapping emitted span retaining both unit IDs;
- exact bodies without repair, widening, shortening, splitting, or role substitution;
- identical logical units, citations, spans, and accounting under LIVE and RETAINED custody.

Focused C02 suites and the full repository suite passed at the reviewed revision. Log paths and SHA-256 digests are recorded in the execution record.

## Remaining common-track blockers

- C01: exhaustive identity-field sensitivity.
- C04: complete source-mutation graph-byte oracle.
- C05: complete privacy/non-disclosure marker corpus.
- C06: exhaustive malformed-input and page-mutation RED corpus.

Historical `L04=FAIL` is unchanged. This review does not qualify the full common track or implementation.
