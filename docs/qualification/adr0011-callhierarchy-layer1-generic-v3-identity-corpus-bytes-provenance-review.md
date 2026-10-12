# ADR-0011 Call-Hierarchy Layer-1 Generic V3 Identity Corpus Bytes and Provenance — Independent Review

Status: **GO**

- Reviewed base HEAD: `84edc6318b31396f7d875bdeb77b43b7862a973e`
- Review scope: uncommitted `sessionruntime/testdata/callhierarchy-layer1-red-v3/**` identity amendment
- No RED, production source, v1/v2 corpus, tests, prototype, or C15 execution was modified.

Independent mechanical verification confirmed:

- changed source paths are confined to the v3 corpus;
- v1 and v2 bytes are unchanged;
- occurrence categories are exactly 25 roots, 16 World-24 workers, 22 World-25 standalone executions, and 22 World-25 pair members;
- all 85 occurrences join the accepted ledger by structural address and exact JSON-typed request ID;
- all 85 contain a strict direct identity input with the ledger-authoritative profile, binding, and workspace nonce;
- bindings are exactly 45 `opaque-incoming` and 40 `opaque-outgoing`;
- the semantic decoded delta is exactly 85 driver identity additions plus the 38 independently approved expected-binding replacements;
- the other 47 expected bindings remain unchanged and agree with the ledger;
- no method, request identity, commitment, control, profile expectation, nonce expectation, or other decoded value changed;
- every world byte count and SHA-256 digest was independently recomputed and agrees with the updated manifest;
- manifest changes are limited to the affected per-world and aggregate byte/hash accounting.

The accepted ledger remains the causal provenance. Corpus methods and expected outputs did not define the driver binding values.

```text
ADR0011_CALLHIERARCHY_LAYER1_GENERIC_V3_IDENTITY_CORPUS_BYTES_PROVENANCE_GO
```
