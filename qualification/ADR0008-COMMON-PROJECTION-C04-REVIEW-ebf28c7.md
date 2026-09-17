# ADR 0008 common projection C04 qualification review

- Reviewed revision: `ebf28c738ae7a9ee5278f757fecc1a80670578e0`
- Execution record: `qualification/adr0008-common-projection.execution.ebf28c738ae7a9ee5278f757fecc1a80670578e0.json`
- Predecessor: `qualification/adr0008-common-projection.execution.c23ff0b2245a5a278c5cbb2dbf8f9a1ab5789677.json`
- Governing matrix: unchanged
- Result: `C02=PASS`, `C03=PASS`, `C04=PASS`; `COMMON_PROJECTION=BLOCKED`; `implementation_qualified=false`

## C04 evidence

One before/after oracle executes the complete required source mutation set:

- source addition;
- source removal;
- source text change;
- candidate/source reordering;
- overlap change.

Each substantive mutation changes projection output, while reordered candidates preserve canonical projection bytes. After every mutation the oracle independently verifies:

- byte-identical graph bytes and digest;
- unchanged nodes and relations;
- unchanged support;
- `authority=0`;
- `accepted=false`;
- `source_graph_complete=UNKNOWN`;
- `graph_facts_added=0`.

The graph fixture contains a server-reported `CALLS` relation. Projection cannot add, remove, repair, suppress, or otherwise alter it. The focused C04 execution and full repository suite passed at the reviewed revision. Log paths and SHA-256 digests are recorded in the execution record.

## Remaining common-track blockers

- C01: exhaustive identity-field sensitivity.
- C05: complete privacy/non-disclosure marker corpus.
- C06: exhaustive malformed-input and page-mutation RED corpus.

Historical `L04=FAIL` is unchanged. This review does not qualify the full common track or implementation.
