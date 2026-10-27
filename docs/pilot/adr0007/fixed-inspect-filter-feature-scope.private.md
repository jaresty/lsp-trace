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

The implementation prepares inputs in memory only. It reads all six known files;
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
prove protection from an unrestricted same-user actor. This helper performs none
of these retention, backup, expiration, deletion or cleanup operations.

## Exact blockers and smallest next edge

Implemented preparation does not supply durable primary/backup retention,
hash-resume admission, a storage/retention budget, workflow execution binding,
Census evidence, ProgramC/instability admission, qualified Describe, a populated
candidate inventory, independent usefulness judgment or stakeholder acceptance.
The current historical preflight still returns MONITORING_NOT_QUALIFIED; no
monitoring qualification, manifest/head transition or runtime admission is added.

Smallest next edge: privately bind the returned exact scope/source/selection/plan
bytes to a durable outside-tmp content-addressed primary plus independently
verified backup receipt under this prospective policy. Test reopen/hash-resume
and budget/dependency refusal before connecting any acquisition executor. No
public wiring or acquisition is authorized merely by this document.
