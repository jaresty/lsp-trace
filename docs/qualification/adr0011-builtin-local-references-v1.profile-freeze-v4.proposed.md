# ADR 0011 — built-in local references V1 profile freeze candidate v4

**PROPOSED / UNACCEPTED.** V3 (`adr0011-builtin-local-references-v1.profile-freeze-v3.proposed.md`, 4,323 bytes, SHA-256 `762ec8fbda8100102b3aed578bea716d6e2a1ef978bf0ecfcde560a40ea4d37b`) remains unchanged and received **REVISE**: its selection text contradicted itself when crossing outer ranges both contained a common strict inner range. V4 supersedes only the V3 selection paragraph below. All V3 pins, limitations, implementation gaps, original-byte replay and earlier unchanged conditions apply.

## Exact input pins (unchanged from V3)

| Input | Bytes | SHA-256 |
|---|---:|---|
| `profiles/adr0011-builtin-local-references-v1.proposed.json` | 4628 | `e5b6dd5ad3a0c3c97d3a7201f3d1891d46470116bafa2f4390a66dcc97e22b19` |
| `schemas/adr0011-builtin-local-references-v1.proposed.schema.json` | 10593 | `d6bed3453c002d7d509a8b7fbfb17a78aeb61fee0a576f9728ac78dccaf0bdce` |
| `policies/adr0011-document-symbol-target-policy-v1.proposed.json` | 547 | `9e9ff102363ed75075dc97eb4bbb2e56e9748cbca553b9326adc33949d4c21e3` |
| `schemas/adr0011_builtin_local_profile_proposed_test.go` | 11280 | `775215b2a91da238bdb1fa2c8ad40cf1176d25c39b4b0251c22c7c8c586b8825` |

## Ordered selection rule (clarification of policy label, not a new input byte)

First validate the entire bounded hierarchical document-symbol result and every range, independent of target selection. Collect the symbols whose nonempty `selectionRange` contains the query point, start inclusive/end exclusive. If the set is empty, target is unresolved. **Before selecting any symbol**, compare *each pair* of containing selection ranges: they must be strictly nested (one is a proper subset of the other), except that a single containing range has no pair to compare. If any pair is equal (including duplicate provider items) or crosses/is incomparable, the target is unresolved—even if a third, narrower range is strictly inside both. Only after the pairwise chain check, select the unique strict innermost range (or the sole containing range). No name, input order, display-range specificity, or references result may break ties. An outer and strict nested inner pair selects the inner; two crossing outers and a common inner are unresolved. On unresolved identity, **no references WRITE** or fabricated target/final.

This conservative pairwise-chain interpretation is proposed for independent contract review against the accepted gate's `unique strict most-specific nested selection containment`. If the gate instead permits a common inner despite crossing outers, that is an identity-authority interpretation question, not a silent implementation choice; return `REVISE` with the specific counterexample and seek maintainer adjudication. The current `SelectDocumentSymbolCandidate` still rejects a second containing range and **does not conform**; private implementation and adversarial positive/negative selector tests remain prerequisites to owner integration. Synthetic schema tests do not exercise replay or cross-process exclusion.

The product direction is unchanged: built-in immutable V1 defaults automatically selected on explicitly invoked future references operation, not overridden by request or result; future V2 cannot replace old V1 on replay; no unsolicited capture or retention. A file measurement proves at most opened-file byte consistency, not producer authentication or running-memory correspondence. Exactly 21 immediate final refs, historical CALLS bytes, `authority=0`, `accepted=false`, `completeness=UNKNOWN`, default-off privacy, fail-closed public publication and 0/162 qualification all remain unchanged. No live provider, row execution, `owner.go` wiring, public enablement, commit, or push is authorized.

Requested independent read-only verdict: `ACCEPT_BUILTIN_PROFILE_FOR_PRIVATE_IMPLEMENTATION` or `REVISE` with a decisive first counterexample. Acceptance is private/default-off contract scope only.
