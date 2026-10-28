# Prospective private partial-inspection serialized budget v2

Status: local deterministic qualification only; independent review pending. No new
live authorization, configuration, operational roots, replay, recovery, server
execution, commit or push accompanies this proposal.

## Measured boundary, not a source/query repair

Read-only witness: `/Users/schwa/Library/Application Support/lsp-trace-scoped-partial-boundary-lu4v9etp/reports/live.native.log`,
18,433 bytes, SHA-256 `eb99b571b80ce76ec0b3520a86ae0427a8dc0e90e8f61fba5392e2c98d4c15ab`.
The log reports one producer call, zero admitted constituents, INDETERMINATE,
and a `bounded-partial-artifact` record of **3,487,940 bytes**, empty ID,
`verified:false`, `private serialized output budget exceeded`. It also reports
`partial artifact serialized cap after producer; no heap bound claim`.
Outcome ID: `sha256:0d79f889e24f8ed44a315e81365164e3460458edacf1afc2da28e9918fb55f0f`.
The authorized corrected attempt is spent. Input-32 unchanged and six verified
object pairs are supplied handoff facts, not newly reverified here.

The producer returned an actual partial artifact; the blocking observation is
serialized-budget refusal, not evidence of a new external-query or source bug.
The artifact was not retained; this work neither reconstructs it nor assigns it
a historical content ID. Views were not generated: their live sizes remain
**unknown**. No future success is inferred from the measured artifact size alone.

## Closed prospective variation

- Config schema: `lsp-trace.scoped-rehearsal.private.v4`.
- Request policy: `one-partial-inspection-private-v2`.
- Claim schema: `lsp-trace.scoped-execution-claim.private.v3`.
- Claim policy explicitly includes `serialized_artifact_bytes:4194304`.
- Config's existing exact closed `budget` tuple is
  `{artifact_bytes:4194304,total_bytes:16777216,objects:8,staging_bytes:33554432,read_hash_bytes:268435456}`.

This is a fixed private proposal, not optional numeric tuning or public defaults.
The existing `artifact_bytes` field is generic per serialized record: v4 allows
4 MiB for all records. There is no new optional config field or artifact-role
exception. The outcome encoder retains its separate 256 KiB bound.
Historical v1/v2/v3 config encoding and expected budgets remain unchanged;
v3 still selects partial-v1 at 2 MiB. Historical claim bytes omit the new field.
Unknown policies, changed/missing explicit v2 cap, mixed claim schema/name,
unknown config fields, changed budget components and noncanonical config bytes
refuse. Strict parsing and finite purpose checking precede root opening/startup.
Full-purpose entrypoints accept only config v1/v2; partial accepts only v3/v4.
The shared coordinator selector is unchanged: policy changes do not yield a fresh
claim slot, resume authorization or reconstructed pending capability.

The storage capability's existing Read/Publish and 2 MiB source/input ceiling
are unchanged. Only explicitly named `ReadPartialInspectionV2Private` and
`PublishPartialInspectionV2Private` permit 4 MiB with the same close-aware,
no-replace, exact-byte/hash receipt checks. The test-only v4 bridge selects them;
no general storage budget interface or alternate root framework is added.

## Independent ceilings and arithmetic

`2,097,152 < 3,487,940 < 4,194,304`; prospective per-record headroom for this
observation is 706,364 bytes. This does not estimate view sizes or physical heap.

Aggregate serialized admitted bytes remain at most 16 MiB, independently of the
eight-object maximum; eight objects need not all fit at 4 MiB simultaneously.
Nonterminal admission stops at 16 MiB minus 256 KiB and seven records; the
terminal outcome may use the reserved eighth slot and aggregate remainder.
Failed publication attempts remain charged; no retry or automatic expansion.

Two-copy staging reservation is still `2 * 16 MiB = 32 MiB`; conservative
sixteen-byte-equivalent read/hash reservation is still `16 * 16 MiB = 256 MiB`.
Neither is derived from `8 * 4 MiB`. The pair operation and verification passes
are unchanged, so raising the individual bound does not require changing those
aggregate reservations. These are logical accounting ceilings, not measured
heap, elapsed-time or filesystem free-space guarantees.

Unchanged: 32 KiB config, 16 KiB claim, 256 symbols, four nomination files, six
supplies, depth 64, 63 targets, down/up 1/1; 30 s adapter, 5 s request, 256 requests,
64 wire messages, 2 MiB wire response and 8 MiB captured responses. No CALLS
truncation, evidence compression or source/wire/request limit increase.

## Deterministic qualification

`TestScopedPartialBudgetV2SerializedPrivate`: generic synthetic serialized bytes
of exactly 3,487,940 bytes refuse under v3 and retain/read/hash-verify on both roots
under v4; 4 MiB succeeds, 4 MiB+1 refuses before ID/receipt publication. Old 2 MiB
boundaries remain. Size refusal leaves bounded terminal outcome persistence usable.
This is **byte-budget testing, not the actual historical workload**.

`TestScopedPartialBudgetV2AggregatePrivate`: individually valid records fill
16 MiB minus reserve; nonterminal +1 and total 16 MiB+1 refuse; exact 16 MiB
including reserved outcome succeeds, with staging/read-hash tuples unchanged.
`TestScopedPartialBudgetV2ObjectReservePrivate`: seven nonterminal records,
eighth outcome, ninth refusal. Canonical tests cover mixed schemas, all budget
component drift, claim explicit cap and unchanged selector/old bytes.

`TestContentRetentionPartialV2Private` independently covers new storage exact
read/receipt sizes, old method refusal, 4 MiB+1 rejection and no-replace behavior.
`TestScopedPartialBudgetV2DriverPrivate` runs the real partial rehearsal with the
existing small fake-wire graph, verifies eight paired records and explicit
4 MiB output/unchanged wire and time limits. It also observes full/v4 refusal
with zero traffic; the startup purpose matrix and entrypoint AST checks cover
prestartup gating. This is not a heavy-graph or future-live view-size test.

Local normal and race checks target Scoped Partial/Rehearsal/ExecutionClaim and
ContentRetention families; vet covers both affected packages. Independent review
must precede any parent commit decision or separately authorized future live run.
A later view/aggregate failure must fail honestly, never auto-expand budgets.
