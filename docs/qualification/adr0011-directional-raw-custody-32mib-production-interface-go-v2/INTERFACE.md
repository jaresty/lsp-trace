# Production interface contract v2

## Basis and boundary

Assumption: the accepted ledger-partition package fixes architecture but not an exact Go declaration. Basis: its architecture selects a dedicated directional subledger in the same Manager accounting domain and permits a private successor interface. This package therefore freezes only the exact proposed patch and its compile/lifecycle-plumbing evidence.

## Accounting interface

- The same `Manager` owns one real `privateB4ByteAccountV2`; no second recorder or terminal authority is introduced.
- `GENERAL_C15_V2` remains the existing general ledger: maximum 33,554,432 bytes, including the current 24,600-byte legacy ledger baseline and unchanged 1,024 general slots.
- `DIRECTIONAL_RAW_V1` is distinct account-owned state with maximum 33,554,432 bytes, distinct live/cumulative/generation counters, and one occupied slot.
- Checked combined live bytes are `general.live + directional.live <= 67,108,864`; overflow and limit checks occur before mutation.
- Exact 32 MiB directional reservation is admitted while the normal general baseline remains charged because the baseline belongs to the independent general maximum, not the directional maximum.

## Opaque lease interface

`privateB4DirectionalLeaseV1` is package-private and binds the real account pointer, immutable directional partition ID, reservation generation, capacity, and active state. Alias validates and remains in the same partition with zero additional bytes. Transfer accepts only the same directional partition. General↔directional transfer, rebinding, capacity borrowing, and release-offsetting refuse before effect.

## Precedence

Eligibility/terminal/identity and slot checks precede arithmetic. Directional +1 precedes allocation; general-limit and checked combined/global +1 or overflow precede account/owner/source mutation. A refusal leaves counters, slot, generation, leases, source, owner, and terminal state unchanged.

## Stage A

Exactly 20 predecessor rows remain `PLANNED_RESERVATION`, `PLANNED_COPY`, `PLANNED_ALIAS`, `PLANNED_TRANSFER`, or `PLANNED_RELEASE`. The wrappers are migration hooks only. Planned IDs do not become source-derived actual IDs and no `NECESSARY`, `REUSE`, Stage B, or semantic C15 closure is claimed.

Authority: `PRODUCTION_INTERFACE_LIFECYCLE_PLUMBING_ONLY`.

AUTHORED_NOT_GO
