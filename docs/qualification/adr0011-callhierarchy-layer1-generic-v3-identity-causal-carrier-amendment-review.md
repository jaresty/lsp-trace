# ADR-0011 Call-Hierarchy Layer-1 Generic V3 Identity Causal Carrier — Independent Review

Status: **DOCUMENTATION GO ONLY**

- Reviewed HEAD: `f8839f0f5a543ace0403d29ea6cb30987cbf6d01`
- Reviewed amendment: `adr0011-callhierarchy-layer1-generic-v3-identity-causal-carrier-amendment.md`
- No corpus, RED, source, tests, prototype, or C15 execution was modified or authorized.

## Findings

Mechanical recursive census confirms exactly 85 physical occurrences: 25 roots, 16 World-24 workers, 22 World-25 standalone executions, and 22 World-25 pair members. All 85 currently lack `driver.request.identity`.

The frozen expected-value distribution is:

- profile `corpus-profile-go`: 85;
- binding `opaque-incoming`: 45;
- binding `opaque-outgoing`: 40;
- workspace nonce `01` repeated 32 bytes: 85;
- other bindings: 0.

The binding split exactly matches the method split. The amendment correctly forbids deriving binding from method, direction, expected data, world identity, request ID, commitment content, or runtime output. A separately reviewed all-85 literal/provenance ledger remains mandatory.

Strict nonce handling is sufficient: exactly 64 lowercase hexadecimal characters, exact 32-byte decode, and all-zero rejection. Duplicate-member rejection requires duplicate-aware token processing; `DisallowUnknownFields` alone is insufficient.

The three attachment fields preserve comparability. The value-returning setter changes only profile, binding, and workspace identity while preserving every other field and predecessor pointer identity. Attempt lineage copies these values verbatim into immutable observation/view custody.

The exact proposed C15 addition is closed: three field candidates, one setter-method candidate, and one existing constructor-body change, with no new wrapper or allocation candidate. The exact three-file production boundary remains sufficient.

Authority ordering is non-transitive. This review accepts only the documentation amendment. Corpus bytes, RED, implementation, C15, and execution remain unauthorized.

```text
ADR0011_CALLHIERARCHY_LAYER1_GENERIC_V3_IDENTITY_CAUSAL_CARRIER_GO
```
