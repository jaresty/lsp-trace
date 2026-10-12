# ADR-0011 Call-Hierarchy Layer-1 Generic V3 Identity Ledger V1 — Independent Review

Status: **GO**

- Reviewed base HEAD: `bf8741551899882efa06c3bee847337c794b7503`
- Reviewed ledger: `adr0011-callhierarchy-layer1-generic-v3-identity-ledger-v1.json`
- Reviewed provenance: `adr0011-callhierarchy-layer1-generic-v3-identity-ledger-v1.md`
- No corpus, RED, source, tests, prototype, or C15 execution was modified or authorized.

Independent reproduction confirmed:

- exactly 85 unique structural addresses and exact JSON-typed occurrence identities;
- a complete bijection with a structure/request-ID-only corpus projection;
- byte-defined identity canonicalization, SHA-256 ranking, and tie-break reproduction for all 85 assignments;
- `corpus-profile-go` ×85;
- `opaque-incoming` ×45;
- `opaque-outgoing` ×40;
- workspace nonce `01` repeated 32 bytes ×85;
- direct per-occurrence literals with no runtime defaults, inheritance, shared expansion, or method/expected/world/pair/implementation derivation;
- fail-closed mismatch handling and immutable-ledger correction discipline;
- canonical records digest matching the value frozen in the ledger.

A separate post-verdict Stage 2 diagnostic found 38 ledger-binding mismatches against the frozen expected bindings. This diagnostic did not influence ledger production or acceptance and does not authorize expected-value changes. It requires a separately justified and reviewed expectation amendment before corpus mutation can realign those values.

```text
ADR0011_CALLHIERARCHY_LAYER1_GENERIC_V3_IDENTITY_LEDGER_GO
```
