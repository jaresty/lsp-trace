# Private V2 producer checkpoint

This is a connected **ephemeral producer-lineage checkpoint**, not generalized
completion, production admission, qualification, or durable retention. The existing
fixed six-member V1 route and historical exact-25 qualification are unchanged.

## Closed path

1. `FreezeScopePrivateV2` validates an explicit `ScopeSpecPrivateV2` before source
   IO, then owns descriptor-read bytes and observed identities. The revision is a
   40-lowercase-hex caller assertion, not authenticated Git provenance. Members
   retain declared order. Detached spec, manifest, selection, and file-plan bytes
   are pins, not live custody. Existing V1 selection/partition codecs are reused
   as neutral bytes inside a new private V2 manifest, not relabelled as V2 custody.
2. `DiscoverScopePrivateV2` consumes the frozen owner's one discovery attempt. Its
   closed constructor accepts only that opaque owner, a concrete Manager, session
   identity, and original active lease context. It revalidates source identity
   before/after preparation, compares independently returned captured supply,
   and binds Manager-owned `RetainedScopeGuardPrivateV2` for every member.
3. Actual `DiscoveryAdapter` execution uses nomination-only Supply and the guarded
   adapter client. Incomplete, empty, overflow, and failed enumeration cannot mint
   an origin. A detached report retains actual bounded discovery metadata when
   available. There is no DTO-to-origin constructor or automatic rescan.
4. `Core.PlanDiscovery` derives actual target batches and canonical Seeds V2 from
   successful discovery. These target batches are distinct from the frozen source
   **file partitions**. The origin pins both mappings and actual enumeration.
5. `origin.Batch(ordinal)` returns an opaque handle sharing the original batch's
   authority and mutex-protected spending state. SeedAuthority is minted once per
   planned batch, caller-asserted-local, unauthenticated, empty host receipt,
   retaining its canonical seeds. No API exposes an authority, permit or client.
6. `AcquireScopeBatchPrivateV2` performs exact identity/request/scope/lease and
   source/query declaration preflight before spending. It exhausts the shared
   state **before Consume**, including grant-comparison failures. It owns a real
   directional reservation through acquisition, SourceWithholdingPrivateV2 capture,
   client revocation, and verified attachment/release. Failure returns no successful
   handle and cannot reset or replace the original authority.
7. `OpaqueSuccessfulV2Capture` contains detached origin/seed/artifact bytes only.
   Its capture-origin frame pins artifact and seed SHA-256 and lengths, ordinal,
   and enumeration origin. JSON consistency cannot mint this in-process lineage.

The V5 output-version string in the restricted canonical seed-manifest input is
only an existing binding/validation byte pin. This producer does **not** call the
ordinary V5 engine, ExecuteAuthorized, ExecutePlannedBatch, or any admission path.
The mint scanner permits only `DiscoverScopePrivateV2` in its exact producer file;
other functions in that same file remain forbidden mint call sites.

## Bounds and claims

- Actual members: 1..8 nomination, 0..8 context, <=16 total. Context is never a seed.
- All effective numeric bounds are explicit/nonzero and ceiling-checked; context
  capacity is nonzero even when actual context count is zero. No generic zero
  semantics/default is used. Discovery <=256 symbols, <=63 targets; planning
  batch size <=63, positive down/up depth <=8.
- Freeze: <=2 MiB/file, <=8 MiB source total; URI/path <=8192 bytes, language <=256,
  combined detached scope metadata <=128 KiB. Preflight uses conservative metadata
  bounds before IO. Origin and capture-origin metadata are each <=128 KiB.
- Source supply/capture remains stricter: Manager captured supply <=1 MiB/file;
  capture preflight <=1 MiB/file and <=4 MiB total; artifact <=4 MiB. Freeze success
  alone therefore does not promise that supply/capture will succeed.
- Private prospective path depth <=32 (root and relative member paths), bounding
  retained descriptor growth. This is **not a new public or V1 default**.
- Every component is opened no-follow, regular files only. V2 refuses hardlink
  identity aliases before a second aliased read. Held/named files and ancestors
  are rechecked; close/cancel failures clear outputs. Identical-byte root/file
  substitution fails observed identity comparison.
- Source roots are borrowed, nonconcurrent same-user practical inputs, with
  `BORROWED_NONCONCURRENT_SAME_USER_PRACTICAL_NO_ATOMIC_SNAPSHOT`. Repeated checks
  do not create an atomic snapshot, adversarial filesystem isolation, authenticated
  history, or formal historical independence.
- Authority=0, accepted=false, feature identity UNRESOLVED, source completeness
  UNKNOWN, absence inference PROHIBITED. Native server metadata and ranges are
  evidence, not qualified AST/full-definition/source-analysis claims.

## Active lease and preparation stop semantics

`CheckActiveSessionLeasePrivateV2` is a narrow read-only Manager predicate. Before
producer source revalidation/preparation and at subsequent adapter boundaries it
requires the exact current session/generation/workspace, READY state, no protocol
or lifecycle owner/closing state, exactly one active lease, and the Manager's
opaque token/nonce in the context. It does no IO, grants no lease, and changes
neither generic `validSessionLeaseLocked` nor ordinary V1/V2 guard admission.
Tokenless, foreign, namespace-mismatched, and retired contexts are not producer
inputs. Retirement is checked through the Manager token state even when the old
context's `Err()` remains nil. Legitimate child contexts keep the same token.
Freezing inputs alone does not require or grant a runtime lease.

The constructor validates policy and fixes its overall deadline at entry, BEFORE
source IO. Construction revalidation, supply, binding, and active checks run in
bounded contexts. Each `PrepareDocument` uses a canceled-on-return child bounded
by both the remaining overall deadline and `RequestTimeoutMS`; a successful
preparation does not reset the overall deadline. The adapter stores the ORIGINAL
lease lifetime, not the canceled construction child. Timeout/partial-write
transport poisoning stays Manager-owned. No retry, rebind, or fresh discovery is
performed after a failed frozen-owner attempt.

These are safe-boundary context checks, not an atomic lease/filesystem snapshot
or an absolute wall-clock guarantee for arbitrary blocked filesystem syscalls.
The owned Manager transport interrupts a blocked notification pipe write on its
request deadline; the bounded regression fixture separately verifies that actual
write cancellation, rather than merely noticing a deadline after a delayed write.

## Owned checks

The producer tests use real Manager leases and owned fakepipe children, unrelated
explicit source paths and varied bodies for 1/0, 2/1, 5/2 and 8/8 member cases.
Callable counts vary by file. Actual document-symbol discovery precedes graph
queries; context is captured through relations, not nominated. Raw null/empty
responses, duplicate CALLS, opaque item data, outside endpoints, separate file and
actual-target partitions, copied/concurrent handles and refusal/exhaustion paths
are exercised. Runtime source-supply limits and one-guard-per-lease are preserved.

A controlled counterfactual temporarily disabled the shared exhausted-state
assignment: the post-Consume grant-mismatch assertion failed, then passed after
restoration. This demonstrates that assertion's discrimination, **not** exhaustive
mutation adequacy or comprehensive security qualification.

Review M1's real tokenless RED admitted both origin and capture, with new supply/
query traffic; the fixed test refuses before any such traffic. M2's RED kept a
real notification write blocked after initialization beyond its policy window;
the fixed test returns without its independent external cleanup watchdog. That
fixture uses a 150ms request / 600ms overall policy, a conservative 3s observation
window, and 10s emergency cleanup, not a narrow latency claim. A second case keeps
the 150ms request cap but uses a 5s overall cap: the 3s window also rejects an
implementation that silently drops the request cap. Manager-owned unit
cases cover foreign/retired/other-lease and namespace identities, child deadline
propagation, read-only behavior and unchanged ordinary tokenless semantics.

## Explicitly remaining

V2 RETENTION, QUALIFICATION, DERIVATION, PROGRAM C, PREPARATION, and POSTLEDGER
integration remain unimplemented. No durable pair exists until the existing
publication-root protocol is actually used and completely verified. Nothing here
replaces or enlarges the historical 25-target milestone, merges constituent
graphs, establishes feature identity/completeness, or qualifies a scaled production
workflow.
