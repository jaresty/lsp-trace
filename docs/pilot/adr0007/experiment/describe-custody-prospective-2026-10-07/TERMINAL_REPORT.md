# ADR 0007 prospective Describe custody terminal report

- Generation: `g-9a5fd445d96fbea3b1c37e9a33fb609b09dda81f765e216bd9ec0108dcb4ac0f`
- Producer attempts: **48/48 COMMITTED**
- Reviewer attempts: **48/48 COMMITTED**
- Unique attempt keys / receipts: **96 / 96**
- Exact producer/pair/evidence lineage errors: **0**
- Retry / substitution / repair / missing: **0 / 0 / 0 / 0**
- Reviewer verdicts: **47 ACCEPT, 1 REJECT**
- Qualification: **BLOCKED**
- Disposition: `DESCRIBE_CUSTODY_BLOCKED_SEMANTIC_REPLAY_DISAGREEMENT`

The persisted rejection is `reviewer-primary-item-13`: primary omits the replay lane's `exactly one parameter` proposition, so the reviewer found the outputs not mutually entailing. No result was retried, repaired, normalized, or replaced. Authority and acceptance ceilings remain unchanged.
