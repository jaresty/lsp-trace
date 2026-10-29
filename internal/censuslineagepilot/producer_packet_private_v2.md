# Complete-origin producer packet — private V2 checkpoint

This checkpoint connects the existing opaque V2 producer to paired retention,
independent offline qualification, and a **new in-process derived seal**. It does
not implement ProjectV2, ordinary NativeV5 admission, G1–G8, workflow/Describe,
source-analysis confidence, feature acceptance, or source completeness.

## APIs and ownership

1. `publication.OpenSourceProtectionRootPrivateV2` acquires a topology-only
   directory descriptor by no-follow ancestor traversal. Unlike ObservedRoot,
   the source can be temporary or mode 0755. The capability has no os.Root,
   content reader, publication method, serialization, or caller-info constructor.
   Its path and device/inode accessors are observations, not a namespace lease.
2. `NewProducerPacketRootsPrivateV2` borrows two existing strict output caps, the
   mandatory source cap, and at most 15 optional ObservedRoot caps. Close/use must
   not overlap. Supplied optional roots are not a complete hidden-root inventory.
3. `RetainCompleteScopePrivateV2` accepts only original opaque frozen, enumeration,
   and successful capture handles. Its outcome retains per-copy observations and
   any actual publication receipts on error. `VerifiedPair()` is zero unless the
   complete protocol succeeds. No arbitrary-byte/DTO/receipt constructor mints it.
4. `QualifyProducerPacketPrivateV2` independently reopens both exact pinned roots,
   verifies exact length/SHA/bytes, rejects same-inode objects, validates the
   complete packet again, and binds it to the original producer pointers. The
   detached report records actual root paths/devices/inodes and object identities,
   lengths and selectors observed through the caps before/final qualification;
   observation DTOs are never accepted as seal inputs.
5. `DeriveProducerPacketPrivateV2` mints the new opaque derived state only from
   the qualified state. `ReadDerivedProducerPacketPrivateV2` requires that actual
   handle and returns detached packet/report/per-batch source and native graph
   bytes. No second base64 wire envelope is created.
6. `PureParseProducerPacketPrivateV2` returns only a mutable inspection DTO. It
   cannot create a seal or acquisition capability.

The success mint now records the actual enumeration pointer, ordinal, and actual
attach/release observations after verified release. The spent batch also retains
its unique success pointer. Equal JSON, seeds, artifacts, or declared session IDs
from another execution do not substitute for this relation. None of the offline
operations consults an active Manager lease, replays acquisition, or reads source
bodies. Required source topology is compared to the original frozen path/dev/ino
at operational boundaries, even when the Manager has already shut down.

## Packet and independent validation

The new schema is `lsp-trace.complete-scope-producer-packet.private.v2`. Original
spec/manifest/selection/file-plan/policy/origin, observed file bodies/records, and
each batch's original seeds/capture-origin/source-artifact/manifest-input are
explicit base64 byte fields. An ordered role manifest binds index, member/batch,
SHA and length. Scope is shared once; batch boundaries remain intact.

The read-only validator reconstructs the canonical spec, nomination selection,
file partitions, actual discovery ledger, symbol/noncallable membership, target
batches, seeds, manifest inputs, request context IDs, query membership and source
policy. It uses the private V2 source parser and exact retained-process graph
codec, preserving original native bytes, directed call occurrences, duplicates,
null/empty distinctions, ranges and URIs. Present readable source receipts are
compared to frozen bodies; policy-withheld endpoints never become absence claims.
Whole-origin batch coverage is explicitly **not source completeness**.

## Finite private profile

Hard ceilings: serialized packet 16 MiB (including base64 and JSON), each original
source artifact 4 MiB, metadata carriers 128 KiB, 1–8 nominations, 0–8 context
members, at most 16 members, 1–63 actual targets/batches. Freeze's 8 MiB source
capacity remains separate from successful capture's 4 MiB source-body preflight.

The named private profile also caps cumulative logical reservations at 64 MiB,
read/hash reservations at 256 MiB, and validation work at 1,000,000 units. Callers
may narrow but not increase these limits. Reservations precede the governed
passes. The wrapper is streamed with finite array/depth/token bounds before
base64 decoding and typed allocation. Exact encoded-size preflight marshals only
metadata and accounts for nil versus empty values and base64 expansion.

The retention schedule reserves 16 logical packet passes and 32 read/hash passes
for the shared six-read/two-publication algorithm, its clones/verifiers and
commitments. Parsing/source-codec/topology passes have separate reservations.
These are named logical work reservations, **not CPU, peak heap, or kernel-I/O
meters**. A packet below the 16 MiB wire ceiling can exhaust the independent work
profile and is then `NOT_RETAINED_OVER_BOUND` before storage; no limit is raised.
Outcome `BudgetSpent()` reports reservations made, including unused worst-case
reservations when a later operation fails, not exact executed CPU/I/O work.

The new publication methods have their own fixed 16 MiB family. Legacy 2/4/192
MiB and post-preparation contracts are not enlarged. Common pair mechanics are
reused without adopting the legacy verification token. Pure absence alone
permits one no-replace attempt; late ErrExist never triggers reread/retry,
overwrite, deletion, or rollback. Both roots undergo two reopen passes. Directory
sync claims remain scoped to actual receipts, without power-loss/atomic-pair or
comprehensive-monitoring guarantees. All observations are boundary observations,
not protection against arbitrary future same-user namespace mutation.

## Owned verification

All runtime traffic used owned Manager/fake-pipe fixtures. Successful 1/0, 2/1 and
5/2 scopes, heterogeneous bodies, multiple target batches, noncallables, non-default
process metadata, null/empty replies and duplicate call occurrences traversed the
connected pipeline. Managers were shut down before offline operations. Outputs
were private owned persistent test directories outside tmp; temporary mode-0755
sources were protected separately and all caps/fixtures were cleaned up.

Named fault tests cover missing/duplicate/extra or foreign same-byte successes,
seed/terminal substitution, packet/role/source tampering, budget overflow and
one-over bounds, wrapper array preflight, topology aliases/replacement/closure,
partial writes, late collisions, joined absence/close errors, second-copy faults,
corrupt reads, cancellation, failed receipts and hardlinked copy objects.

One declared counterfactual temporarily removed the ordered-role gates:
`TestProducerPacketPrivateV2InspectorMutations/missing-role` failed with
`ASSERT_PACKET_INSPECTOR_REFUSES_TAMPER missing-role`; restoring the checks
returned it to GREEN. This is one bounded RED/GREEN witness, not exhaustive
mutation coverage.

Final Darwin runs (counts include parent tests and subtests):

- Packet + existing V2 producer/V1 retention/publication/golden focused selection:
  **261 passed**.
- Focused packet/producer/retention/topology race selection: **165 passed**.
- Existing source-withholding V1/V2 lower-profile selection: **97 passed**.
- Entire publication package: **215 passed**.
- `TestFeatureScopeManifestGolden` and
  `TestManifestCanonicalGoldenAndNoAuthorityFields`: **2 passed**.
- `go vet ./internal/censuslineagepilot ./internal/publication` and
  `git diff --check`: clean.

No live providers, real retained roots, acquisition restart/retry, repository-wide
CI, commits or pushes were used. Linux runtime qualification, exhaustive fault
interleavings/mutations, ProjectV2 and all downstream acceptance remain outside
this checkpoint. Independent parent review is still required.
