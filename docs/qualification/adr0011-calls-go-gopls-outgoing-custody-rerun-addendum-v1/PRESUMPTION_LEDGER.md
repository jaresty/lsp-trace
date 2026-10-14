# Recursive presumption ledger — outgoing custody rerun addendum v1

Disposition: `OUTGOING_CUSTODY_RERUN_ADDENDUM_AUTHORED_NOT_GO`. This ledger records only author-side presumptions and corrections. It grants no GO.

| id | presumption | correction / current disposition |
| --- | --- | --- |
| P1 | A committed incoming custody package could authorize outgoing. | Rejected. It is implementation precedent only; outgoing receives fresh namespace/profile/authority and independent review. |
| P2 | Exact HEAD self-reference could prove package identity. | Rejected. The package requires immutable base ancestry plus tracked descendant confinement and package checksum integrity. |
| P3 | Existing untracked outgoing provider package could be used as dependency. | Rejected. It remains untouched and non-authoritative. |
| P4 | Public custody summaries may expose paths/env for reviewer convenience. | Rejected. Public artifacts are allowlist-only; private exact custody retains raw values. |
| P5 | Test-only env probe or dry5900 plan could count as denominator evidence. | Rejected. Both are marked TEST_ONLY, no gopls launch, and not denominator evidence. |
| P6 | Final seals can trust path strings. | Rejected. Reads use no-follow descriptor traversal plus fstat and O_NONBLOCK/O_NOFOLLOW where available; nonregular and symlink artifacts fail closed. |
| P7 | Outgoing ledger validation can inherit incoming event checks. | Rejected. The sealer checks `direction == "outgoing"`, `ledgers/outgoing.attempts.jsonl`, and exact outgoing plan direction. |
