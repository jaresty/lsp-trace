# Frozen production and test write-set

Status: `AUTHORED_NOT_GO`

## 1. Hard write-set rule

A future separately authorized RED/implementation phase may modify or create only the paths listed below. The complete allowed set is six production targets plus four new test targets. **No unlisted production or test path may change.** Documentation/evidence output requires a separately reviewed qualification package and is not implied here.

Historical tests are dependencies only: do not edit, rename, regenerate, move, or delete them. No existing fixture, corpus, golden output, public contract test, testdata path, schema, profile, registry, build tag, or package boundary may change.

## 2. Accepted production targets, unchanged

| Path | State at pinned HEAD | Owning seam | Permitted scope only |
|---|---|---|---|
| `sessionruntime/call_hierarchy_directional_raw_v1_private.go` | `ABSENT_REQUIRED_NEW` | package-private Manager directional reservation, scanner/selector, separate assemblers and handoff | New private v1 mechanics frozen by accepted design |
| `sessionruntime/sessionruntime.go` | existing | real Manager `RoundTrip` transaction/read loop under Manager synchronization | reservation-before-effect and transaction-scoped private bound/handoff only |
| `internal/lspwire/read_frame.go` | existing | exact transport scanner | bounded incomplete opaque-prefix result only; defaults unchanged |
| `internal/lspwire/successor_ingress.go` | existing, conditional use | successor scanner/read/result-span path | thread already-reserved private bound/prefix ownership only if the real Manager opted-in path uses it |
| `sessionruntime/b4_owned_method_pair_lease.go` | existing | sole generic Layer 1 owner/terminal | opaque prefix custody and mapping IDs only; no second owner or lineage/history semantic change |
| `sessionruntime/b4_c15_ownership_registry_private.go` | existing generated target | C15 ownership registry | generated only after accepted enumeration; never hand-edit |

The conditional production target remains in the complete write-set. If exact implementation evidence proves it unnecessary, it stays byte-identical; that does not authorize a replacement path.

## 3. New test targets and exact ownership

All four paths must be new files. Existing files may not absorb these tests.

### A. `sessionruntime/call_hierarchy_directional_raw_v1_private_test.go`

Package: `sessionruntime` (same-package private test).

Disjoint normal selector prefix: `TestADR0011DirectionalRawV1Normal`.

Exact anchored normal selector: `^TestADR0011DirectionalRawV1Normal`.

Normal owner for:

- declaration/contract REDs for all frozen private v1 names;
- exact `33554432` admission, `33554433` refusal, checked-add overflow, ledger-unchanged refusal, duplicate release false/no-op, release then reuse with fresh identity and stale lease closure;
- direction-neutral prepared-array scanner and nonzero ordinal/raw-span selection preserving lexical bytes, ordering, duplicate multiplicity, whitespace, key order and token spelling;
- separate incoming and outgoing assemblers embedding byte-identical selected bytes in `{"item":...}`; one-byte mutation refuses before directional write; neither assembler imports/calls the other or shares semantic direction types;
- Manager real-path reservation-before-effect integration and exact request/response/handoff length plus SHA-256 equality;
- typed distinctions for null, empty, nonempty, protocol error/`-32601`, malformed complete bytes, timeout, cancellation, incomplete prefix, absent bytes, pre-effect refusal and complete success;
- exact release/reuse and sole-terminal behavior in deterministic serial cases;
- disabled and `CALLS_ONLY` compatibility snapshots: exact readiness-history entries/bytes/digests/cuts/directions/order/identity, acquired/outstanding bytes, receipt owner/borrower counts, retirement/lifecycle state, predecessor commitments, generic attempt-lineage fields, provider/request/response/graph bytes, and zero prefix-mapping IDs;
- enabled mapping IDs are directionally namespaced and never equal or feed history/predecessor/lineage IDs, digests, ordering, classification or lifecycle;
- C15 relationship assertions: every retained backing has exactly one C15 classification, at most one directional component, aliases charge zero, transfers do not double-charge, directional rows are not C15 owner rows, and C15/directional/relationship deltas reconcile separately.

Required selectors include:

- `TestADR0011DirectionalRawV1NormalDeclarationsRED`
- `TestADR0011DirectionalRawV1NormalLedgerBoundariesRED`
- `TestADR0011DirectionalRawV1NormalScannerSelectorRED`
- `TestADR0011DirectionalRawV1NormalIncomingAssemblerRED`
- `TestADR0011DirectionalRawV1NormalOutgoingAssemblerRED`
- `TestADR0011DirectionalRawV1NormalManagerRealPathRED`
- `TestADR0011DirectionalRawV1NormalTypedStatesRED`
- `TestADR0011DirectionalRawV1NormalCompatibilityRED`
- `TestADR0011DirectionalRawV1NormalC15RelationshipRED`

### B. `sessionruntime/call_hierarchy_directional_raw_v1_race_private_test.go`

Package: `sessionruntime` (same-package private test).

Disjoint race selector prefix: `TestADR0011DirectionalRawV1Race` exclusively.

Exact anchored race selector: `^TestADR0011DirectionalRawV1Race`.

This file has no special build semantics: it has no build tags and compiles and runs under ordinary package testing. Qualification ownership executes it with `go test -race` and the anchored race selector. The normal focused command uses the anchored normal selector, which excludes every race test by construction. File A must not duplicate concurrency schedules.

Race owner for:

- simultaneous incoming/outgoing or same-direction reservation: exactly one v1 slot winner, loser refused before avoidable effect, aggregate never above `33554432`;
- release versus callback/terminal/lifecycle races: callback drain, clear references before decrement/reuse, exactly one release winner, no use-after-return;
- STOP/RESTART retirement versus active prefix/view, stale generation refusal, fresh attempt identity on reuse;
- concurrent duplicate release and stale lease access;
- race-run C15 and directional live/cumulative counters remain internally consistent without merged ownership.

Required selectors include:

- `TestADR0011DirectionalRawV1RaceSingleSlotRED`
- `TestADR0011DirectionalRawV1RaceReleaseDrainRED`
- `TestADR0011DirectionalRawV1RaceStopRestartRED`
- `TestADR0011DirectionalRawV1RaceReuseIdentityRED`

### C. `internal/lspwire/read_frame_bounded_incomplete_v1_private_test.go`

Package: `lspwire` (same-package private test).

Selector prefix: `TestADR0011BoundedIncompleteFrameV1` exclusively.

Normal transport-boundary owner for:

- declaration RED for `boundedIncompleteFrameV1` and `Reader.readWithBoundedIncompleteFrameV1`;
- pre-reserved bounded capture before read; exact cap accepted and plus one never retained;
- malformed header/body/JSON, short EOF, timeout and cancellation return only actually consumed bounded bytes as `INCOMPLETE_OPAQUE_PREFIX`, with actual length, SHA-256, limit, terminal cause and `complete=false`;
- zero bytes read differs from present empty/prefix custody;
- no reconstruction, semantic decode, member/range salvage, or complete-frame/result/message labeling;
- inability to reserve refuses before read/provider effect;
- legacy `Read`, `ReadWithExactFrame`, and `ReadWithFrameIfWithin` behavior remains byte/value identical.

Required selectors include:

- `TestADR0011BoundedIncompleteFrameV1DeclarationRED`
- `TestADR0011BoundedIncompleteFrameV1FailureMatrixRED`
- `TestADR0011BoundedIncompleteFrameV1ExactAndPlusOneRED`
- `TestADR0011BoundedIncompleteFrameV1ZeroVersusPresentRED`
- `TestADR0011BoundedIncompleteFrameV1LegacyCompatibilityRED`

No race selector belongs in this file; Manager/owner concurrency belongs exclusively to file B.

### D. `internal/lspwire/successor_ingress_directional_v1_private_test.go`

Package: `lspwire` (same-package private test).

Selector prefix: `TestADR0011SuccessorIngressDirectionalV1` exclusively.

Normal owner for the accepted conditional successor-ingress target:

- prove the opted-in transaction threads only an already-reserved private bound and incomplete-prefix ownership;
- exact, plus-one, cumulative, message-attempt and short-EOF precedence under the private bound;
- original-frame transfer/refusal remains exact and no alias/copy is double charged;
- nil/disabled/default options remain byte/value identical to historical successor ingress;
- if the real Manager path does not use this scanner, record that the production target is byte-identical and these tests must establish non-use/compatibility rather than invent an adapter.

Required selectors include:

- `TestADR0011SuccessorIngressDirectionalV1ReservedBoundRED`
- `TestADR0011SuccessorIngressDirectionalV1PrecedenceRED`
- `TestADR0011SuccessorIngressDirectionalV1TransferAccountingRED`
- `TestADR0011SuccessorIngressDirectionalV1DisabledCompatibilityRED`

## 4. Selector non-overlap rule

The four prefixes are disjoint and ownership-bearing:

- `TestADR0011DirectionalRawV1Normal` — sessionruntime normal only; anchored selector `^TestADR0011DirectionalRawV1Normal`;
- `TestADR0011DirectionalRawV1Race` — sessionruntime race qualification only; anchored selector `^TestADR0011DirectionalRawV1Race`;
- `TestADR0011BoundedIncompleteFrameV1` — lspwire exact scanner only;
- `TestADR0011SuccessorIngressDirectionalV1` — lspwire successor ingress only.

Focused ownership commands are:

```text
go test ./sessionruntime -run '^TestADR0011DirectionalRawV1Normal'
go test -race ./sessionruntime -run '^TestADR0011DirectionalRawV1Race'
```

The normal anchored selector cannot select race tests, and the race anchored selector cannot select normal tests. The race file has no build tags or special compilation semantics and remains part of ordinary package compilation/testing. `-race` is a qualification execution mode, not file admission or ownership syntax.

No test outside these four new files may use these prefixes. No new file may use historical `TestCallHierarchyLayer1GenericV3`, `TestPrivateB4C15`, `TestReadWithExactFrame`, or `TestSuccessorBoundary` prefixes. No build tags may be added for selector ownership.

## 5. Exact RED obligations and prohibition

Each named RED must initially fail because its named production seam/contract is absent, not because of compilation unrelated to that seam, a fabricated replacement owner, changed global/default limit, direct post-guard injection, fixture mutation, test-only implementation, or altered historical expectation. Current >16 MiB real-path reachability remains `BLOCKED_NOT_RED` until the separately authorized private transaction-scoped path exists; tests may declare the missing contract but may not manufacture reachability.

The focused normal ownership command runs without `-race` using `^TestADR0011DirectionalRawV1Normal`. The focused race qualification command runs with `-race` using `^TestADR0011DirectionalRawV1Race`. Both files still compile and may run during ordinary unfiltered package tests; selector ownership, not build semantics, separates obligations. Passing historical tests are compatibility dependencies, never RED ownership.

## 6. Hard stops

Stop `BLOCKED_OWNERSHIP_OR_PACKAGE_MISMATCH` if:

- any historical test must change;
- any new test cannot be same-package `sessionruntime` or `lspwire` as assigned;
- any obligation requires an unlisted production/test path;
- a public/global/schema/config/profile/registry/build-tag change is required;
- ownership duplicates, a second terminal appears, raw bytes are reconstructed, or history/predecessor/lineage behavior changes;
- incoming evidence satisfies outgoing or vice versa;
- C15 and directional ledgers cannot remain separately exhaustive and reconciled.

No fallback write-set is authorized.
