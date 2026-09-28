# Host-Git shared-stream rule — implementation-only decision

**Decision: `ACCEPT_HOST_GIT_SHARED_STREAM_RULE_FOR_IMPLEMENTATION_ONLY`.** Exact reviewed `adr0011-host-git-shared-stream-implementation-rule.proposed.md`, complete-byte SHA-256 `ded85eb6f4f41ed5e122dbf0c4f177c33751c2194e319faad68b942cd1de356b`. Independent review found that accepted host-Git records require six *references* and exact replay, not six distinct objects or publication receipt fields. A transaction-scoped owner-held VERIFIED publication map may reuse equal content-addressed stream objects only with the original verified receipt status and fresh exact readback. This changes no frozen schema or selector.

Unknown prior objects, lost owner map, hash-only equality, no-replace collision, nil receipt, or uncertainty after an earlier stream commit cannot become `ABSENT` or VERIFIED. Such paths remain `COMMITTED_UNVERIFIED` where commit is possible. A restart may replay retained bytes but cannot fabricate a missing original publication receipt for a new issuance decision. Zero-byte streams are real immutable objects and may be shared under the same rule.

Implementation and synthetic falsification only. Neither a retained host-Git record nor this decision accepts revision identity, final references issuance, producer authentication, a public selector, or a nonzero `T/A` or authority.
