# Durable inventory from an ordinary folder invocation

On Darwin, `lsp-trace census [FOLDER]` now prepares the bounded originals,
issues a durable task and exits. No store environment variable, caller daemon,
model credentials, Pi, Node or Python is required. FOLDER defaults to the current
directory; existing server/profile selection rules still apply. It does not author
semantic answers: the calling agent reads evidence and submits judgments.

## Successive processes

```sh
lsp-trace census /absolute/source-folder
# Save RunID and TaskID from the returned lsp-trace.durable-inventory-task.v1.
lsp-trace census durable task RUN_ID
lsp-trace census durable read RUN_ID TASK_ID input 0 32768
lsp-trace census durable submit RUN_ID TASK_ID interpreter < answer.jsonl
# A successful submit returns the next task, with its predecessor and sequence.
# Repeat Describe as needed, then GroupDecision, then a separate Reviewer task.
lsp-trace census durable submit RUN_ID REVIEW_TASK_ID reviewer < review.json
lsp-trace census durable result RUN_ID PUBLISHED_TASK_ID
lsp-trace census durable read RUN_ID PUBLISHED_TASK_ID input 0 32768
```

`task RUN_ID` observes the current task without resubmitting a previous answer.
Returned `Operations.current_task` and its `task` alias supply exactly this
current-run argv; `Operations.exact_task` adds TaskID and is only an exact-current-ID
check, which becomes stale after advancement. If a submit response is lost or
ambiguous, **do not automatically repeat this command**: execute the preceding
task's `current_task` argv, preserving its explicit root, to observe the successor,
PUBLISHED result handle, EXPIRED or UNCERTAIN_SPENT status. Never resubmit the
predecessor. Observation makes no semantic submission or accounting reservation.
Passing a TaskID requires an exact match. Every command may be a fresh process;
no source/provider access occurs after the initial preparation process exits.

`read` returns bounded copies: `Data` is base64 in JSON, `Offset` and `Total` are
byte positions. Advance by decoded length. Pages are 1–32768 bytes. Describe input
is artifact `input`; concatenate decoded pages into one JSON object. Its exact
source-body path is `$.Unit.Body`, retained request path is `$.Request`, and reused
request-prompt path is `$.Request.Envelope.Prompt`. It also contains completed
accounting, selected endpoint and host bindings. Read these retained values;
do not reacquire source or invoke a provider. GroupDecision input is the current catalog; Reviewer input is
the proposed catalog. Published input is the corrected catalog. These copies are
not mutable application state or importable native capabilities.

Describe submission uses the existing supplied adapter grammar: canonical JSON
plus LF with `status: COMPLETE` and `text` containing canonical Describe semantic
JSON. This retained-evidence lifecycle admits only semantic `verdict: COMPLETE`,
`consumer_need: {status: UNRESOLVED, value: ""}`, and
`provided_behavior.consumer_relative: false`. The generic parser and reused
prepared request prompt have broader vocabulary (ABSTAINED, RESOLVED and
consumer-relative); parser validity is **not** lifecycle admissibility. Those
three variants refuse after the claim is spent. If a truthful COMPLETE source-only
description cannot be supported, stop and report inability to the caller without
submitting; do not fabricate COMPLETE or submit ABSTAINED as successful completion.
The returned example is an illustrative syntax specimen, not evidence-based prose
to copy blindly. No additional Describe task values need substitution; substantive
prose must be grounded in the actual retained input.

GroupDecision and Reviewer use existing `NativeDecisionV3` JSON fields:
`predecessor`, `actor`, `reason`, `affected` and `groups`, with the current catalog
revision and exact declared actor. Each returned task includes a bounded
`Contract` (`lsp-trace.census-task-contract.v1`): the complete response shape,
byte/encoding rules, a schema-valid example, and ready-to-use `Operations` argv
arrays for current_task/task/exact_task/read/submit/result. Examples illustrate grammar, not evidence-based
judgments; replace the example's revision with the current input revision and
judge the actual evidence. No source-code lookup or user-managed role machinery
is needed. Follow the frozen role prompt too; do not submit a replacement prepared
object or catalog. Describe's outer JSON is canonical JSON+LF, with all adapter
fields in the supplied order (including zero-valued metadata); semantic JSON is
strict but permits whitespace. Decision JSON permits whitespace/final LF. Neither
allows unknown fields or duplicate keys. Contract metadata does not change validators.

The CLI reads at most 65537 bytes. Complete malformed/oversized judgments consume
the claim. A transport error does not submit partial bytes. Held-open stdin uses
owned pollable input, waits for cancellation notification before classifying an OS
deadline, joins cancellation, restores shared descriptor flags and closes its
owned duplicate. It does not accept arbitrary blocking readers or abandon a read
goroutine.

## Store, authority and roles

The default store is `lsp-trace/census/durable` under the OS per-user configuration
or application-support directory. Existing checked ancestor/private-directory
provisioning is reused; source-overlapping stores and aliases are refused.
The one public storage override is `--publication-root ABSOLUTE_PRIVATE_DIRECTORY`.
It selects acquisition `run-*` directories beneath that root and durable records /
publication beneath its `durable` child. The existing private parent must exist;
the application creates private children without chmod/chown of existing trees.
Both storage paths are resolved and validated before provider effects.

Selection precedence is explicit `--publication-root`, then compatibility-only
`LSP_TRACE_CENSUS_STORE`, then the OS per-user location. The compatibility variable
selects its existing private directory directly; acquisition `run-*` children are
also placed there. Invalid explicit or environment selection refuses before
provider effects: it never falls back to another store. The environment variable
is not a second advertised setup requirement.

With an explicit root, repeat **the same** `--publication-root` on every durable
command. Returned `Contract.Operations` arrays already include it. With the
compatibility environment override, keep that environment in later processes.
Default ID-only calls use the same OS user/configuration context. IDs alone do not
discover arbitrary stores; there is no global mapping, artifact import or manual
manifest registration. Moving to a different context does not relocate a run.

```sh
lsp-trace census /absolute/source-folder --publication-root /absolute/private-store
lsp-trace census durable task RUN_ID --publication-root /absolute/private-store
```

Run/task IDs are lookup handles, not authority to import paths, bytes or manifests.

The application owns one persisted manifest, denominator, cursor, predecessor chain,
description references, group state, dispositions and accounting ledger per run.
Each genuine original is retained once through `PrepareSelectedDescribeV3`, with
exact encoded readback and equality against the original owner-prepared value.
Resume calls `CompleteSelectedDescribeV3` and the shared applicationinventory
catalog/review/reconciliation/publication transitions. It does not rerun `Run`,
restore `PrivateSelectedResultV2`, restore a native grant, or repeat M/P/S.

Absent an optional `LSP_TRACE_CENSUS_CALLER` declaration, roles are `main`/`main`,
SAME_THREAD, with explicitly unspecified caller pins—not measured model execution
identities. A configured `Subagents:true` declaration uses distinct interpreter
and reviewer identifiers. Review is always a separate sequential task. Declarations
and names do not authenticate independent actors or establish semantic coverage.
The optional declaration format is documented in `census-durable-describe.md`.

The local application store is the accepted authority. Integrity seals/digests
check corruption, not authenticity of arbitrary imported bundles. Malicious
same-user whole-store replacement or rollback is outside the guarantee. Do not
copy/edit records as a supported continuation procedure. No external issuer,
registry, signing service or renewed approval is required.

## Deadlines and conserved accounting

- Native work remains bounded by its original <=15-second lifetime.
- Application deadline is fixed at initial invocation: `min(--timeout,180s)`.
  The normal durable default is 180 seconds, selected before store/source preparation.
  Explicit `--timeout 60s` remains 60 seconds. The legacy explicit, connected and
  one-item milestone defaults and shared receipt default remain 60 seconds.
  No deadline is renewed after preparation; changing the default does not increase
  the native, request, task or cumulative semantic ceilings.
- Each task receives one persisted absolute window, at most 30 seconds, including
  waiting between calls. The run reserves at most 120 seconds cumulatively. The
  initial task-window allocation also accounts for the admitted task count.
- One ledger enforces <=64 MiB logical/copy allowance, <=`1<<32` work and <=64
  read/result/submit packets. Claim slices are reserved before decoding stored
  state or judgments. Shared semantic budgets include previously spent bytes and
  already-reserved semantic time; unused claimed slices are never refunded.
- `Contract.RunByteLimit` is the actual stored run ceiling (not necessarily the
  maximum 64 MiB); `Contract.RunPacketLimit` is 64. Task `Used` and `Packets` are
  cumulative usage, not remaining allowance. For `InputIndexBytes > 0`, each read
  reserves `8*InputIndexBytes + 6*verifiedChunkBytes + 2*returnedBytes + 16384`,
  32 times that amount of work, and one packet. `verifiedChunkBytes` is the sum
  of complete intersecting 32768-byte chunks (the last chunk has its actual
  length). Index verification never reopens/decodes the whole catalog.
  Published input (`InputIndexBytes=0`) remains optional full-object verification:
  `6*Artifacts[0].Bytes + 2*returnedBytes + 16384`.
  No per-item independent 64 MiB/120-second budget is created.
- Every issued semantic task protects one complete forward delivery **attempt**:
  offset `RequiredInputNextOffset`, limit 32768, until the original byte length.
  Smaller, repeated, overlapping or out-of-order requests are optional charged
  work; they do not advance or spend the protected cursor. Both kinds count real
  packets. Skipping an unconsumed required pass at submission conservatively spends
  its remaining bytes/packets, rather than returning credits.
- The read cursor and spending move in the same authoritative head CAS before
  index/chunk access. Lost output or a crash after that CAS spends the attempt.
  `current_task` safely observes the next funded offset, **not proof of receipt**.
  A reread must pay optional capacity and may refuse; there is no automatic retry,
  deadline extension, credit renewal or receipt guarantee. Concurrent operations
  do not automatically rebase a failed CAS.
- Metadata/status is explicitly uncharged, bounded by the stored head size plus
  fixed contract text and bounded operation arguments; contract serialization is
  guarded below 10 KiB for normal fixed-size IDs. It does not enlarge stored role
  prompts, evidence limits, claim slices or any ceiling. This is not exhaustive
  physical accounting. Published reads remain charged but do
  not repeat semantics or inherit an expired semantic deadline.
- `PublicationRunReservedBytes`, `PublicationRunWork` and
  `PublicationRunPackets` are ledger snapshots at publication. Later result/page
  reads increase current task accounting. `CallerBytes` is the shared publication
  budget's prepublication snapshot, not a refund of conservative ledger slices.

These are logical budgets and wall-clock checks, not OS memory/CPU containment.
Slow human/agent interactions may exceed the fixed deadlines and then refuse.

After genuine preparation, a checked semantic full-sequence quote, the current
required input pass, and a conservative successor construction/delivery escrow
must fit before the first semantic task is issued. Semantic quotes include every
prepared Describe, grouping, separate review, reconciliation/publication and one
verified Result. Evidence delivery uses explicit staged admission; this is not an
unconditional promise that every future maximal legal answer fits. The admitted census N
still determines the semantic window; prepared count P determines the remaining
Describe operations. Neither the census nor its dispositions is reduced to fit.

The completion plan for each issued task is bound into the authoritative run head
and timed acceptance. A submission claims its existing funded stage before sizing
work, then admits a conservative successor top-up and delivery before parsing or
storing its answer. Admission failure remains UNCERTAIN_SPENT, with no semantic
answer effects. The current and all previously spent stages never shrink. Future
unspent semantic bounds may tighten using each supplied canonical adapter's actual
byte length instead of its previously unknown maximum; parser allocation and
accepted grammar remain unchanged. The successor's own construction/delivery
escrow is included before issuance. Optional work cannot consume these protections. Optional/repeated reads cannot consume its remaining bytes, work or
packets. A required stage transfers its entire quote into spent accounting in the
same predecessor CAS that claims it; its inner budget spends that already-funded
slice, not a second outer reservation. Unused slices are never refunded. First
Result has a protected transfer; subsequent Results remain charged optional work.
Metadata observation remains uncharged. Corrupt quote/state bindings fail closed.

Quotes use owned lengths and unchanged response grammar. Catalog seal sizing
counts body + prompt + response record (the existing 24× operand), not an entire
enclosing description. Closed state/catalog metadata and the four-artifact manifest
have explicit tested bounds; full artifact verification and the existing copy,
escaping and verification multipliers remain. Preparation retains the original
source/proof bounds and policy ceiling, without charging the same retained state
again outside its existing preparation reservation. No arbitrary-scale or physical
memory guarantee follows; admission can still refuse a large census or body.
Whole-application completeness remains unknown.

### Verified chunk custody

The original full object remains authoritative for semantic parsing, catalog
validation and publication. Chunking is only a delivery representation. Construction
charges `14*originalBytes + 16*(1024 + 128*chunkCount) + 32768` before original
verification and publication: full-original read/hash, chunk publication/readback,
and index construction/encoding/publication/strict readback. Initial construction
is preparation spending; successor construction is inside the predecessor's single
outer allocation, not a second run charge. Planning has its own charged allowance.

The canonical finite ordered index binds run, task, predecessor, exact original
length/digest and each chunk's exact length/digest. Its immutable stored reference
is included in timed acceptance; changing an index and recomputing a head checksum
cannot substitute it. Fixed chunk sizes/order exclude gaps, overlap and truncation.
Every subsequent page verifies the accepted index and the requested whole chunks.
No arbitrary paths or raw range reads are accepted. Same-user coherent whole-store
replacement remains outside the existing trust model; there is no new PKI.

Paging arguments and returned page bytes are unchanged. Task metadata adds
`InputIndexBytes` and `RequiredInputNextOffset`; updated instructions explain the
accounting.

### Persisted format compatibility

Fresh runs explicitly emit `lsp-trace.durable-inventory-head.v2` and nested
`lsp-trace.inventory-completion-quote.v2`. These versions cover the incompatible
chunk/index, forward-attempt and staged-planning fields. Supported decoding remains
strict and canonical, including the checked envelope digest; the v2 head and plan
remain bound to timed acceptance. Versions are not a migration mechanism.

Recognized canonical v1 heads (both the pre-chunk shape and the subsequently emitted
chunk/staged shape that incorrectly retained the v1 identifier), and a current
head carrying a recognized old quote identifier, refuse with
`UNSUPPORTED_STORED_VERSION`. The standalone closed
`lsp-trace.census-operation-diagnostic.v2` contract permits only this reason,
`task|read|submit|result`, `action:STOP`, and `retry:false`, with no recovery,
paths or raw causes. Existing diagnostic-v1 reason/field bytes are not widened.
Malformed/checksum-invalid representations and unknown version identifiers remain
conservatively `RUN_UNAVAILABLE`, not evidence of a recognized old format.

Refusal does not rewrite stored bytes, transfer allowances, interpret an answer,
renew a run, reacquire originals, restore selected capabilities, or fall back to
the one-item route. **Old artifact files remain, but ordinary task/read/result
retrieval of old Published runs is not restored.** An archival retrieval reader is
separate work and is intentionally not implemented here. Do not relabel old heads
as v2, copy them into new runs, or treat version recognition as compatibility.

## Claims, uncertainty and publication

A CAS reservation makes a task CLAIMED before semantic interpretation. Only one
claimant can advance it. Stale, duplicate, foreign-role and expired submissions
refuse. A crash, invalid answer or failed completion after claim remains spent;
there is no automatic retry or replay.

Controlled process guards now SIGKILL immediately after the actual first submit
claim CAS and separately after the second top-up CAS, before answer interpretation
or storage. A genuine owner-issued three-original store is used. Guards check exact
persisted head bytes, byte/work/packet transfers, semantic cursor, protected state,
unchanged non-head files, UNCERTAIN_SPENT metadata and competing replay refusal
without refunds. These are submit boundaries, not the earlier read-reservation
proxy. They do not qualify every scheduler/fsync/power-loss boundary.

Each next task/publication needs a separate timed completion proof. The candidate
alone is never accepted. The proof records that candidate CAS, exact readback,
directory sync and close had already completed before the persisted deadline.
It is a record of finished verification, not permission to finish later. A late
candidate or writer killed before sealing remains `UNCERTAIN_SPENT`; cleanup is
not required for refusal. A timely verified result reopens after task expiry.

Ordinary description reopen verifies the exact digest/length-bound description
references in accepted canonical controller state, retains structural
ordinal/status/cardinality checks, and applies the existing intrinsic description
and request validation. It does not reread or compare every historical preparation.
Missing old preparation bytes therefore do not invalidate an already accepted
description on reopen; completion still requires and validates its preparation.
This is accepted-description integrity, not an all-originals audit or renewed
native admission. Historical accepted-proof tests may use an optional immutable
closure via `LSP_TRACE_OWNER_FIXTURE`; small pure-data reader/structure tests run
without that fixture and do not pretend to establish native admission.

`result` reopens the actual publication directory and verifies its manifest and
all four artifacts: base catalog, proposed catalog, corrected catalog and report.
The corrected/report digests must also match the durable run's immutable references.
The original denominator and unmatched descriptions are preserved. Publication
is provisional: **Authority 0, Accepted false, completeness UNKNOWN**. PUBLISHED
means verified artifact publication, not accepted feature identity or live truth.

No exhaustive fsync/close interruption, OS power-loss, physical containment or
malicious rollback guarantee is claimed. The proof protocol qualifies the named
candidate/receipt cutpoints, not every possible filesystem failure.

## Legacy routes and platform boundary

Normal start and durable multi-item failures use the separately named
`lsp-trace.census-operation-diagnostic.v1` contract without `--machine`, except the
separate stored-version v2 refusal and Start capacity v3 refusal described below. Normal `--machine` (including `--machine=false`)
is compatibility syntax, not a workflow switch. Census syntax errors are now
structured even for malformed legacy invocations; no raw parser error is emitted.
Explicit legacy acquisition success/runtime-failure formatting still obeys its
original machine flag. Existing connected/private/enumeration modes retain their
explicit machine requirements and configuration contracts; examples for those
modes are not the normal product entry. Help is text, not a task response.

Operation diagnostics have closed `operation`, `reason`, `action`, and `retry`
fields. Retry means permission to repeat the **same failed operation unchanged**.
This version conservatively emits only `retry:false`; it has no established safe
unchanged-repeat error case. Config failures use `CORRECT_CONFIG` with a typed
allowlisted reason (`STORE_ROOT_MISSING`, `STORE_PATH_INVALID`,
`STORE_ALIAS_REFUSED`, `STORE_PERMISSIONS`, `STORE_SOURCE_OVERLAP`, `STORE_IO`, or
`CONFIG_UNCLASSIFIED`). ENOENT from symlink resolution alone is missing/unavailable
root evidence, not proof of an alias. Private-mode/owner sentinels do not classify
closed descriptors or identity errors as permission errors.

After a trustworthy current-run observation, a failed/ambiguous durable operation
uses `OPERATION_UNCONFIRMED` and `OBSERVE_CURRENT_TASK`. Recovery contains only
`{run_id, store_context:"SAME_SELECTION"}`. Reuse the exact store selection from the
preceding invocation and its returned `current_task` operation. No raw paths,
environment values, error strings, stale TaskIDs or submit argv are disclosed.
Malformed/unknown/corrupt run heads use `RUN_UNAVAILABLE` and `STOP`, without
recovery. No exact spent diagnosis is inferred from a broad store error. A PENDING
observation is not blanket permission to replay an ambiguous submit. Task observation
is uncharged; `read` and `result` are charged and are not described as idempotent.
New-run acquisition is never an automatic retry.

Clients must dispatch on `schema_version` and reject unknown versions/fields;
the standalone schemas are `internal/schema/schemas/lsp-trace.census-operation-diagnostic.v1.schema.json`
and `internal/schema/schemas/lsp-trace.census-operation-diagnostic.v2.schema.json`.
Start capacity refusal uses the separate closed
`internal/schema/schemas/lsp-trace.census-operation-diagnostic.v3.schema.json`.
It reports `CAPACITY_INVALID_QUOTE`, `CAPACITY_BYTES`, `CAPACITY_WORK`, or
`CAPACITY_PACKETS`, in that existing guard order, with `operation:start`,
`action:STOP`, `retry:false`, and no recovery/run locator. Invalid quotes carry
no invented metrics. Other reasons carry `unit`, `spent`, `protected`, `limit`,
`available`, and `deficit`; all numeric fields use the stated unit (`bytes`,
`work_units`, or `packets`). Available is limit minus spent; deficit is protected
minus available, strictly positive. Work uses work units, not byte equivalents.
Preparation and retained-object effects have already occurred, but no durable
head/task has been published at this refusal. Stop; this is not permission to
repeat acquisition, shrink the census/replies, or raise a budget.

V3's Draft 2020-12 schema validates structure/ranges/enums. Full arithmetic and
raw-wire validation additionally requires `censusresult.DecodeCapacityDiagnostic`
or composed `schema.ValidateFor(raw, schema.FamilyCensusCapacityDiagnostic, "v3")`.
Standalone JSON Schema cannot compare sibling numeric fields or detect duplicate
keys after parsing; it is not the full semantic validator.
The strict Go codec also rejects duplicate fields and trailing JSON. This is not
an MCP envelope and does not widen MCP schemas. The old census diagnostic DTO,
validator, schema and legacy retry bytes remain unchanged for legacy routes.

Explicit workspace/catalog/caller-host/resume routes remain available. A configured
legacy `LSP_TRACE_CALLER_FD` keeps its connected route. The opt-in one-item
`--durable-describe` milestone is hidden compatibility parsing, not a full-inventory
alias, and still refuses multiple leads;
its result is DESCRIBED_NOT_REVIEWED, not a published inventory. Its environment
store/caller declaration and legacy continuation diagnostics remain compatibility
exceptions; see `census-durable-describe.md`. No Python/adapter files were removed.

Darwin runtime fixtures qualify ordinary-folder two-lead preparation, original
process exit/source deletion, fresh-process Describe/group/review/publication,
stale refusal, accounting corruption refusal and reopening after actual task
expiry, in both declaration modes. No canned publication or native-dispatch
override is used; only the test LSP transport and semantic judgments are scripted.
Returned-contract rehearsal additionally submits the unchanged returned Describe
example through owner-issued retained evidence; independently verifies parser-valid
ABSTAINED/RESOLVED/consumer-relative refusal and permanent spent state; and advances
the plain-default two-lead flow using returned argv while discarding submit output
and recovering only through preceding tasks' current-run argv, including explicit
root mode. This qualifies returned syntax/lifecycle/recovery, **not actual model
usability or evidence-based semantic quality**.

The separate three-prepared-item controlled qualification also completes the plain
no-flag default route, source deletion after process A, fresh-process descriptions,
nonempty grouping, a separate reviewer correction, publication and two verified
Result calls. It checks three actual M/P/S completions, zero selected recapture
credits, exact retained wire identities and produced definition mappings. In one
retained run, preparation was 12,476,190 bytes and protected completion 52,885,909;
after required reads and two Results, total spending was 66,720,451 bytes
(388,413 bytes below 64 MiB). This is bounded fixture evidence, not a general
three-item capacity promise, model-quality result or independent-actor claim.

A separate fixed evidence-size experiment holds the three phase18e scripted legal
replies constant (28672 content / 28876 semantic JSON / 29050 adapter bytes each).
Adding ASCII comments after the original call sites preserves query offsets;
provider ranges are derived from the actual parsed fixture source. The frozen
padding matrix is 0, 256 and 1024 bytes per caller body (nonzero arms additionally
include five comment/separator bytes). Baseline and 256 arms completed publication
and verified Result, spending 58,770,057 and 59,648,135 logical bytes respectively,
17 packets each. The 1024 arm completed the original three M/P/S checks but refused
before task issuance (the initial phase19 run emitted `RUN_UNAVAILABLE`). That
initial public diagnostic did not expose exact preparation or the resource guard.
The diagnostic-only follow-up now identifies `CAPACITY_BYTES` at the actual Start
guard. A retained unchanged-matrix rerun emitted spent 14,825,180, protected
54,463,091, limit 67,108,864, available 52,283,684, deficit 2,179,407 bytes, with
no durable head. Other executions produced different totals: temporary fixture
paths are not fixed, so these numbers are execution-bounded, not golden constants.
The supplied external observation (14,825,416 + 54,463,091, deficit 2,179,643) is
covered as an exact scalar guard test, not claimed as this rerun's measured total.
Baseline and 256 still published. This is useful numeric refusal, not capacity
success; admission, budgets and response sizes are unchanged. See
`capacity-refusal-v3-report.md` for the bounded qualification ledger.
No arm was renewed and no budget was tuned. These controlled source sizes do not
establish general capacity, real model quality or arbitrary production scale.

Linux retains publication/CAS and durable continuation implementations and passes
an amd64 cross-build. Linux native folder lifecycle explicitly refuses; no Linux
runtime qualification was performed. Windows ownership/publication/native lifecycle
remain unsupported. `Stat_t/Uid` is isolated, but the full Windows cross-build is
still blocked by existing `privatehostproductv3` build constraints. A cross-build
is not runtime qualification. Remote CI and live model/actor acceptance were not run.
