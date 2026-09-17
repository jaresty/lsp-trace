# ADR 0008 common projection C06 qualification review

- Reviewed revision: `ceac4ab9ada237029e513a4947b464a5f46e1d50`
- Execution record: `qualification/adr0008-common-projection.execution.ceac4ab9ada237029e513a4947b464a5f46e1d50.json`
- Predecessor: `qualification/adr0008-common-projection.execution.3679275d41fa043213b34728e4693fdec730cd06.json`
- Governing matrix: unchanged
- Result: `C02–C06=PASS`; `C01=BLOCKED`; `COMMON_PROJECTION=BLOCKED`; `implementation_qualified=false`

## C06 evidence

The assertion-specific corpus executes named compiling mutations for:

- unknown cursor envelope and payload fields;
- duplicate cursor envelope and payload fields;
- missing candidate, target, and paging identities;
- duplicate selected documents, document bindings, candidates, units, citations, and pager records;
- invalid role, range, encoding, mixed encoding, privacy classification, projection status, and paging limits;
- reordered additional documents and continuation records;
- mixed continuation records.

Every mutation reaches its intended assertion and returns the exact zero result. No case is repaired, normalized to success, retried, widened to a broader policy, routed through a fallback, or implicitly continued.

The RED corpus identified and closed three normalization gaps:

- cursor envelope and payload decoding now rejects unknown and duplicate JSON members;
- empty privacy classification fails before projection;
- unknown V2 projection status fails before assembly.

Successful cursor bytes and released schema/result contracts remain unchanged. The focused C06 execution and full repository suite passed at the reviewed revision. Log paths and SHA-256 digests are recorded in the execution record.

## Remaining common-track blocker

- C01: exhaustive identity-field sensitivity and one byte-level permutation oracle across every projected-unit form and both custody modes.

Historical `L04=FAIL` is unchanged. This review does not qualify the full common track or implementation.
