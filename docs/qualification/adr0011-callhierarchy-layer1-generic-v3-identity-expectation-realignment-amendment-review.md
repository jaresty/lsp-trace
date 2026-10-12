# ADR-0011 Call-Hierarchy Layer-1 Generic V3 Identity Expectation Realignment — Independent Review

Status: **GO**

- Reviewed HEAD: `cd83a04c9f67787a4bfe978991be7db6ce17db6a`
- No corpus, RED, source, tests, prototype, or C15 execution was modified.

Independent recomputation joined all 85 accepted ledger records to all 85 corpus occurrences by structural address and exact JSON-typed request ID. All request IDs retained integer type and exact value.

Results:

- 45 `opaque-incoming` ledger bindings;
- 40 `opaque-outgoing` ledger bindings;
- 47 current expected bindings already agree;
- exactly 38 current expected bindings disagree;
- exactly 19 changes in each direction;
- profile mismatches: 0;
- workspace-nonce mismatches: 0.

The amendment enumerates the complete 38-row set with exact address, request ID, old expectation, and ledger-authoritative replacement. The accepted ledger predates this comparison and was produced without method or expected-output access, so the comparison cannot feed back into ledger assignment.

Future corpus authority is limited to adding 85 explicit driver identity objects and changing exactly these 38 expected binding literals. No method-derived rewrite or other expected change is admitted.

```text
ADR0011_CALLHIERARCHY_LAYER1_GENERIC_V3_IDENTITY_EXPECTATION_REALIGNMENT_GO
```
