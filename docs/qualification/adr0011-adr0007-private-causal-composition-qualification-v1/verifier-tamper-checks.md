# Verifier and tamper checks

The wrapper rejects before candidate effect when any of the following are observed:

- missing incoming or outgoing envelope
- duplicate direction envelope
- unknown direction
- invalid direction status
- package/artifact/profile/evidence/provider-manifest drift
- ordinal semantics/count/hash drift
- any nonempty selected ordinal set in a success envelope
- failure envelope with selected ordinals
- failure envelope without direction-prefixed failure code and failure stage
- mark on a failed direction
- mark for unknown `MatchID`
- duplicate marked `MatchID`
- invalid mark action
- non-`CALLS` relation kind
- profile/evidence drift between mark and envelope
- mark ordinal identity/count/hash drift
- nonzero mark `SelectedOrdinal`
- string/collection/work/reference resource exhaustion
- context cancellation

Mutation gates prove that tests fail if key verifier branches are removed:

- membership omission branch
- order branch
- failure preservation branch
- pin check, including ordinal identity fields
- mark ordinal identity check
- MaxReferences enforcement
- identity-restraint fields
- idempotence normalization
- pre-ADR0007 cancellation check

The wrapper stores direction dispositions even when composition fails, preserving committed bidirectional envelope metadata without producing a candidate.
