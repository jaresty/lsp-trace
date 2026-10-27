# Fixed inspect/filter product pilot — prospective private policy

Status: preparation only; authority **0**, accepted **false**, feature identity
**UNRESOLVED**, completeness and source-graph completeness **UNKNOWN**.
This is a bounded product pilot, not repository coverage or an accepted inventory.

## Question and fixed boundary

Can a developer inspect retained seed evidence and use two seeds' exact typed
reference intersections and differences to make a useful, correctable provisional
capability description without asserting that overlap establishes a shared feature?

Nomination sources are exactly:

- `cmd/lsp-trace/inspect_command.go`
- `cmd/lsp-trace/filter_command.go`
- `internal/inspection/projection.go`
- `internal/filter/filter.go`

Context only, never nomination sources:

- `cmd/lsp-trace/main.go`
- `internal/schema/schema.go`

**All other paths are excluded as nomination sources.** Callers, dependencies,
framework behavior, generated behavior and external relationships outside these
four files remain explicitly tagged frontier/context gaps. Context bytes do not
expand the nomination set. Neither labels nor reachable code silently enlarge it.

The scope-freezing implementation prepares inputs in memory only. It reads all six known files;
missing, empty, nonregular, symlinked, substituted, oversized or cancelled evidence
fails closed with no bundle. Source roots may be `/private/tmp`; durable output
roots must not be temporary. The private policy is **2 MiB per file, 8 MiB total,
partition size 2**, with equality admitted and plus-one rejected. These are chosen
private bounds, not measurements, public defaults or demonstrated adequacy for a
real acquisition. No seed count, traversal size, campaign resolution or resource
budget is established as a public default by this document.

The lowercase 40-hex revision is a caller assertion, not authenticated Git checkout
provenance. Optional expected pins compare path, role, digest and length against
actual bytes; they do not admit evidence. The scope manifest records the asserted
root and the admitted root's device/inode identity, observed per-file identities,
mode, modification/change times, derived SHA-256 and byte length. Each returned source
body is an owned copy. Fixed-order JSON encoding (no trailing newline) is the
private manifest's canonical encoding; it has its own schema and domain. It is
not the historical custody manifest. The selection and plan preserve exact
canonical byte linkage using the existing input constructors and Build.

Descriptor-relative no-follow traversal admits ordinary repository modes without
requiring 0600, owner-only ancestors or single links. Retained parent descriptors
revalidate named descendants and pre/open/post file identity/metadata; root
substitution during admission rejects. A successfully admitted root can later be
renamed without redirecting the capability. This is stable byte observation,
**not absolute immutability**, a simultaneous six-file snapshot, continuous actor
monitoring, arbitrary-I/O interruption or an elapsed-time guarantee. Cancellation
is checked at safe boundaries. All descriptor close failures invalidate output.
Darwin/Linux are supported; other platforms fail closed. No publication APIs or
published-content read policies are relaxed.

## Evidence-to-review chain (future execution, not performed by this unit)

1. Retain the scope, actual source bodies, selection and partition plan durably
   before execution. Enumerate independent **Census** entry points within exactly
   the four nomination sources; retain failures, successful-empty states, bounds
   and denominators. No source completeness follows from successful enumeration.
2. Acquire permitted retained per-seed evidence under a separate admitted execution
   contract. **ProgramC/Leiden and instability** may nominate review arrangements
   only where their input/admission contracts hold. A capture set is not direct
   Leiden admission and creates no cross-capture CALLS or single-capture custody.
   Preserve unsupported or unqualified analysis as a blocker, not a skipped fact.
3. **Qualified Describe** explains bounded technical behavior using exact typed
   references and source bytes. A relation is CALLS only when server-reported.
   Describe qualification and source completeness remain separate from semantics.
4. Construct external **candidates** with provisional working labels, individual
   seed attribution, exact reference namespaces, alternatives, contrary evidence,
   withdrawal conditions and missing/failed evidence. Never borrow the union graph
   as the closure of each seed.
5. Form **reversible groups**, preserving direction, constituent attribution,
   unstable membership, ungrouped candidates and unresolved merge/split questions.
6. Present a **provisional inventory** for independent usefulness review and
   stakeholder correction. Semantic acceptance requires an explicit external
   decision-maker, scope/revision, proposition types, unresolved-state treatment,
   reopening procedure and coverage ceiling. Technical tools do not supply it.

| ID | Working capability | Status | Seeds | Typed evidence | Alternatives | Gaps | Revision |
| --- | --- | --- | --- | --- | --- | --- | --- |
| pilot-inspect-compare | Inspect retained evidence and compare exact seed references | UNRESOLVED | Not yet acquired | Not yet established | Distinct workflows sharing mechanics; one capability; wrapper/context overlap only | Acquisition, downstream qualification, semantic review | Proposed 1 |

This row is a proposed review record shape, not an observed capability finding.
Every populated record must link exact native IDs, source bodies and retained
receipts without rewriting native evidence. Failed, successful-empty, bounded
zero, unavailable and UNKNOWN remain distinct. Absence cannot be inferred.

## Independent usefulness review and correction

An independent pass must not inherit the grouping conclusion. It should verify:

- A developer can answer the stated comparison question from the retained packet,
  without recapture or hidden assumptions; otherwise usefulness is unestablished.
- Each table row has individual seed support, typed reference joins, alternatives,
  gaps, scope/revision and a discriminating rejection condition.
- Intersections/differences are exact within each reference namespace, not textual
  name similarity, transitive-as-direct support or union-as-seed attribution.
- Unresolved candidates and ungrouped evidence stay visible. Community stability
  is not feature confidence. Navigation/layout never changes identity or status.
- A reviewer can withdraw a semantic claim without modifying native evidence or
  losing its predecessor, contrary interpretation or finite revision delta.

Correction scenario: revision 1 proposes “these seeds implement one shared
feature.” A reviewer finds that only exact reference overlap is established.
Revision 2 withdraws the shared-feature claim, retains its predecessor ID and
unchanged native evidence, records the finite inventory-state delta, and replaces
it with “these seeds have these exact typed-reference intersections/differences;
feature identity is UNRESOLVED.” Neither a rename nor a stakeholder label
retroactively authenticates source or production behavior.

The first checkpoint's gap ledger is an accounting outcome, **not inventory
success**. A useful completed pilot needs the independently reviewed comparison
and correction surface, even if its semantic outcome remains unresolved.

## Prospective practical-safety revision

The historical [input/preflight contract](../../qualification/census-lineage-pilot-input-contract.private.md)
remains immutable evidence. Its monitored-runner dependency statements remain
true of that historical qualification; this prospective product policy supersedes
the proposed operational prerequisite for this new pilot only. It does not
rewrite old receipts or claim that existing preflight now admits execution.

Before any downstream execution, provide:

- Durable **outside-tmp content-addressed primary retention**, independently
  verified backups and exact manifest/source/input linkage before acquisition.
- Hash-verified resume from retained bytes; reject changed/missing objects rather
  than treating a stored filename or an asserted revision as custody proof.
- Owned-scratch cleanup only: confined to explicitly owned disposable paths,
  never primary evidence, backup objects or unowned checkout content.
- An explicit storage budget, including retained rounds, sources, outputs,
  receipts and verified backups; stop before exceeding it.
- Dependency-safe **finite retention**: do not expire/delete a referenced source,
  plan, receipt or ancestor while retained descendants/resume depend on it.
  Define the finite retention schedule and dependency check before execution.

Comprehensive actor monitoring or a privileged vault is **not a prerequisite**
for this prospective practical model. It makes **no monitoring-qualified claim**
and no claim to resist arbitrary deletion by another process with the same user
privileges. Backups and replay detect/recover bounded evidence loss; they do not
prove protection from an unrestricted same-user actor. Scope freezing performs none
of these retention, backup, expiration, deletion or cleanup operations; the additive
one-object storage slice below is separate.

## Exact blockers and smallest next edge

Preparation and fixture qualification do not establish actual pilot primary/
backup retention or workflow execution admission. The additive coordinator below
qualifies only private bundle storage, linked hash-resume and its finite logical
budget. None of these units supplies Census evidence, ProgramC/instability
admission, qualified Describe, a populated candidate inventory, independent
usefulness judgment or stakeholder acceptance. The historical preflight remains
unchanged and still returns MONITORING_NOT_QUALIFIED; the new private head does
not add monitoring qualification or runtime admission.

The private coordinator section below adds fixture-qualified finite-plan
coordination and hash-resume verification. It does not connect an acquisition
executor. The remaining edge is a separately admitted actual retention/acquisition
run and its retention-expiry dependency policy; no public wiring or acquisition
is authorized merely by this document.

## Additive private retention-plan slice

`deriveScopeRetentionPlan` now derives a read-only, opaque plan from the exact
in-memory fixed scope. It checks source lengths and raw-byte hashes, fixed roles,
canonical selection/partition linkage and the complete canonical scope manifest.
It does not reopen source paths or authenticate the caller-asserted revision.
Malformed, zero and inconsistent forged scopes return no plan. Private package
state is not an authentication boundary against code running inside the package.

References are ordered: the six actual source bodies in `featureScopePaths`
order, then selection, partition plan and scope manifest. All nine references
remain in active dependency metadata, including all six roles when source bytes
are identical. Only raw-byte storage identity is deduplicated, in first-reference
order; equal digests also require exact byte equality. Every storage digest is
SHA-256 of the exact raw object, **not** the domain-separated `ManifestDigest`.
Returned metadata, references and object bodies are owned copies.

The repository-owned logical ceiling is 64 MiB, with a separate 512 MiB
byte-processing work ceiling. The reservation is twice (unique object bytes +
64 KiB bounded metadata + largest object staging allowance). Work reserves thirty-two
times the nine-reference input bytes plus two metadata allowances. The arithmetic
is checked for overflow. Available budgets may only restrict these ceilings;
equality is admitted and shortage returns no plan. These conservative bounds
cover this finite source/input slice, not future captures, receipts or rounds.
They are **not filesystem free-space promises**, elapsed-time bounds or a global
storage reservation. No write is possible through this plan-only API.

### Additive one-object pair storage

`publication.ContentRetentionRoot` privately owns the exact capability returned
by `OpenObservedRoot`. Construction writes nothing. `ObservedRoot` stays read-only;
no Root, raw descriptor, arbitrary selector or directory-creation API escapes.
The later coordinator adds only fixed private head/attempt namespaces and a
scalar directory-identity accessor; it does not broaden the one-object API.
Only exact root-level lowercase SHA-256 content IDs and nonempty objects up to
2 MiB are publishable. Current-user private no-follow admission is unchanged;
fixture storage roots are outside temporary storage under test-owned home paths.

Reads use the already-existing **close-aware** `ReadContentObserved`, not the older
`ReadVerifiedBoundFile`. It checks 0600, regular/single-link/current-owner custody,
no-follow identity, bounds, hash and all file closes. The old reader is unchanged
and is not a blocker for this unit.

`retainScopeObject` selects one internal storage ordinal from the opaque plan.
Before writing it re-derives the complete plan, checks both available budgets,
validates both roots and rejects the same opened directory identity, including
separately opened aliases. Primary and backup publication are **two independent
storage operations**, not an atomic workflow transition. Outcomes distinguish
NOT_ATTEMPTED, INDETERMINATE, NAMESPACE_COMMITTED, EXISTING_UNVERIFIED and
EXISTING_VERIFIED. Committed receipts survive every later error, including
cancellation. No rollback, deletion or automatic retry occurs.

`PublishBoundFile` receipts separately record namespace commit, directory sync,
close and late verification. A new publication qualifies for subsequent reads
only when all statuses are successful; a nil error alone is insufficient. The
existing directory-sync status explicitly says `NO_CRASH_GUARANTEE`; this unit
does not strengthen that into an unconditional crash-durability promise. Existing
targets are close-aware read/hash/exact-byte verified **before** invoking Publish;
known-equal preexisting objects receive EXISTING_VERIFIED without publication.
Only confirmed os.ErrNotExist admits an absent-object write; other read errors,
including absence joined with close/cancellation errors, fail closed.

After an attempted Publish, **every ErrExist fails closed**, even when a competitor
installed equal bytes. There is no success-producing readback on that error path.
The unchanged Darwin/Linux legacy primitive can discard a precommit duplicated-
parent close failure on late EEXIST, so even pure ErrExist cannot qualify that
attempt. Missing receipts remain INDETERMINATE, backup is not attempted after a
primary failure, and earlier committed receipts are preserved. This is a stop,
not safe-retry admission; a later orchestrator must not silently reinterpret an
uncertain attempt as retryable. No retry, rollback, deletion or overwrite occurs.

The regression chain uses real publication hooks to insert an equal competitor
after confirmed absence and inject the duplicated-parent close failure (source
close succeeds), observing the legacy pure ErrExist with no receipt. Separate
pair tests feed that same pure error after competitor insertion and require no
verified-copy evidence, no readback and no retry. Legacy publication APIs and
primitive semantics are deliberately unchanged.

After both operations, the helper closes/reopens object handles twice on **both
exact pinned roots**, checks hashes, lengths and exact bytes, then returns opaque
per-object verified-copy evidence with authority zero and the exact reference
metadata digest. Root capabilities remain pinned and borrowed throughout; this
is not ambient pathname root reopening. The owner must separately close the
roots, account for close errors, and not use them concurrently. Wrapper Close
joins both owned capability-close errors and invalidates the capability.

The one-object unit itself publishes no retention metadata, durable head, CAS or
workflow transition; one-object success is never whole-bundle success. The
additive coordinator below supplies the separate bundle condition, reference
metadata, finite budget and partial/close accounting. No acquisition or execution
admission is supplied by either unit.

Independence means separately read/hash-verified copies, not separate administrative
or disaster domains. Authority remains zero; there is no monitoring, immutable
vault, unrestricted same-user attacker resistance, accepted inventory or complete
workflow qualification. Metadata is finite active-reference accounting only:
there is no expiry, automatic deletion, retention schedule execution or cleanup.
The tests are fixture-only and do not acquire or retain actual pilot evidence.

## Finite private bundle coordinator contract

`newScopeBundleOwner` accepts only the opaque frozen scope and three exact,
existing outside-tmp primary/backup/journal paths. It derives the plan and key;
callers supply no ordinal, stage, raw plan, expected head, policy override or trust
flag. Binding includes SHA-256 of the **raw scope manifest**, retention metadata,
the repository policy version and all three admitted directory device/inode
identities. Lookup within the journal uses frozen inputs/policy, not copy-root
identities: a substituted copy root must find and reject the existing head instead
of minting a new absent key. All directories must differ. Every invocation owns and closes its
acquired capabilities; only scalar identities escape the publication wrapper.

One advance invocation performs **at most one head CAS and one pair attempt**.
ABSENT/READY(i) claims CLAIMED(i); only a fully verified, unequivocally committed
CAS invoker executes the internally selected item. Data objects are first-reference
ordered and deduplicated; metadata containing **all nine references** is a separate
last item, never an added ordinal in the validated data plan. Object-root closes
and their causes are recorded before publishing the immutable attempt receipt.
The journal receipt is reopened and checked for exact bytes, digest and physical
selector before the journal closes and a live, owner-bound pending capability is
returned. A journal-close failure prevents that capability from escaping and is
preserved in the partial result; its already-written receipt cannot authorize
finalization. Publication and physical copies are not atomic head transitions.

The **next** invocation consumes that exact capability, checks the reopened receipt
and exact claim predecessor, reconstructs the complete linked prefix, verifies both
copies and closes object roots before its one READY(i+1)/FAILED CAS. Unknown CAS or
receipt publication outcomes produce INDETERMINATE_EXECUTION, never pair retry or
recovery authority. CONFLICT requires the concrete primitive's empty noncommitted
receipt plus an exclusively stale-predecessor error tree (bounded to 64 inspected
nodes). Its NOT_COMMITTED outcome instead accompanies post-stage cancellation;
neither that outcome alone nor a stale cause mixed with cancellation/other causes
establishes conflict. Unknown, missing or contradictory return shapes fail closed
as INDETERMINATE_EXECUTION. Known committed flags preserve the exact replacement
revision and CAS evidence on every error, never becoming conflict. Conflicting
claims perform no pair effects. Known commit
revisions and copy/receipt observations survive errors and cancellation. A close
failure after a known CAS commit preserves that revision but fails the returned
status closed; it does not claim nothing committed.

**Crash gap:** a fresh owner finding CLAIMED without the live pending capability
returns INDETERMINATE_EXECUTION without scanning receipts, reading copy objects,
writing, retrying or discovering/finalizing an orphan. Even a physically complete
orphan receipt is not recovery authority. This intentionally sacrifices automatic
crash recovery to prevent duplicate execution. No source-continuation coordinator
is called. There is no generic workflow engine or internal advance loop.

Heads and attempts have new strict canonical private schemas and distinct fixed
root-level namespaces (`feature-scope-bundle-head-private-v1-<key>.json` and
`feature-scope-bundle-attempt-private-v1-<sha256>.json`). Receipt links are logical
SHA-256 IDs resolving only in that attempt namespace; publication receipts retain
the exact physical selector. Duplicate/unknown/trailing/noncanonical wire values,
empty persisted heads, overflow and inconsistent reconstructed predecessors reject.
Reopen follows only head-linked receipts and verifies exact primary/backup bytes,
including metadata, without consulting checkout contents or historical candidates.
Returned wire accessors deep-copy bytes, including partial linked observations.

Repository policy reserves the whole finite run before effects: at most ten pair
items/receipts and twenty head transitions, with a 64 MiB logical byte ceiling.
Reservation includes both copies, metadata, largest-object staging and 32 × 64 KiB
for bounded receipts/head staging/lock overhead. Per-owner work is conservatively
reserved as 24 × (64 × nine-reference-plus-metadata bytes + 128 × 64 KiB), capped
at 16 GiB of byte-processing work. Every advance, including read-only observations,
consumes one checked work reservation; an owner admits at most 24 calls. These are
upper-bound logical reservations, not measured CPU work or filesystem free-space
promises. Generation/item bounds constrain persisted evidence growth across owners;
read-only work is bounded per invocation and owner, not a global quota against
unlimited creation of new owners. No automatic retry, deletion or expiry occurs.
CAS cooperative lock waits and the one receipt-cleanup publication have a fixed
five-second cancellation deadline which callers cannot raise. Cancellation checks
do not promise interruption of arbitrary filesystem I/O.

RETAINED_INPUT_BUNDLE means only exact two-copy observations plus linked receipts:
authority **0**, accepted **false**, identity **UNRESOLVED**, completeness **UNKNOWN**.
It supplies no runtime, monitoring, Census, adaptive5 or production admission.
Expiry remains a later dependency-safe policy. All qualification here is disposable
fixture evidence, not acquisition or retention of actual pilot inputs.
