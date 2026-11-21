# Legacy opt-in durable one-item Describe

**Current default:** ordinary folder invocation now uses the multi-item durable
inventory flow documented in [census-durable-inventory.md](census-durable-inventory.md).
The partial-flow descriptions below apply only to hidden compatibility parsing of
`--durable-describe`. This option is not advertised in normal help and is not an
alias for the full inventory. Its explicit environment-store and caller-declaration
requirements, and legacy continuation diagnostic formatting, remain exceptions;
new callers should use plain `census [FOLDER]`.

This is a **partial durable continuation**, not the completed inventory workflow.
It supports genuine normal-folder preparation, producer exit, bounded evidence
reads from another process, a single Describe submission, and verified description
reopen. Multi-item suspend/advance, grouping, separate sequential review and final
inventory publication are **not implemented in this path**. Larger lead sets
refuse with `DURABLE_DESCRIBE_ONE_ITEM_ONLY`; no lead is silently dropped and no
budget is multiplied. The existing connected caller/controller remains available
through its explicitly configured IPC/legacy routes; without such configuration,
the ordinary folder command now starts the durable multi-item flow.

## Local authority and custody

`LSP_TRACE_CENSUS_STORE` configures an existing absolute private directory owned by
the current user, outside the source workspace. It is application configuration,
not an artifact path accepted in a task request. Provision it with mode 0700. Files
are private 0600 and accessed through existing verified no-follow publication
readers. Run/task identifiers are random lookup handles in this configured store;
they cannot import caller-provided prepared records, paths, checksums or manifests.

The sole issuance function accepts an opaque `PrivateSelectedResultV2` minted by
the existing owner after source cleanup, Finish and accounting readback. Issuance
is one-shot even if a later write is uncertain. It validates through the existing
`PrepareSelectedDescribeV3` and stores exact selected row, unit/body, Describe
request/prompt, source and query bindings, completed M/P/S head/authorization,
three attempt identities, fourteen accounting currencies, handoff/checkpoint,
caller pins and declared review policy. Exact-byte readback precedes the handle.

Reopening admits only a bounded semantic attempt to the existing
`CompleteSelectedDescribeV3`. It never reconstructs a native admission,
`PrivateSelectedResultV2`, source/provider capability, or acquisition grant.
The original DTO/opaque-JSON rejection contract remains unchanged.

This deliberately trusts the application-owned local store. Digests detect
corruption; they do **not** authenticate arbitrary bundles. There is no external
issuer registry, supervisor requirement, signature PKI, or claim of protection
against malicious same-user replacement/rollback of the whole store. Restoring
old filesystem snapshots can restore spent state: rollback prevention is outside
this local trust model. Preserve the store; do not manually copy or edit its
records as a continuation procedure.

## Start

On Darwin, configure the private store and a bounded caller declaration:

```sh
export LSP_TRACE_CENSUS_STORE=/absolute/private/lsp-trace-store
export LSP_TRACE_CENSUS_CALLER='{"Protocol":"lsp-trace.census-caller.v1","Subagents":false,"Interpreter":"main","Reviewer":"main","Pins":{"worker_sha256":"sha256:<64 lowercase hex>","model_sha256":"sha256:<64 lowercase hex>","grammar_sha256":"sha256:<64 lowercase hex>","prompt_sha256":""}}'
lsp-trace census /absolute/source-folder --durable-describe \
  --request-timeout 30s --timeout 180s
```

Replace pin placeholders with actual **caller-declared** values. They are not
execution authentication. For declared distinct roles, set `Subagents:true` and
use different Interpreter/Reviewer names. The mode, identities and existing
interpreter/reviewer role-prompt hashes are frozen before task exposure. Task
metadata explicitly says review has not occurred; names alone do not prove
semantic independence. The full role prompts are available in the prepared
artifact's `Policy` field. This milestone does not execute the review stage.

Normal parsing, source traversal, controlled/native acquisition, cleanup,
retained transfer, original owner M/P/S and the existing controller/preparer seam
are used. No model credentials, model client, Python, Node or Pi is required.
The process returns a `lsp-trace.durable-describe-task.v1` record with `RunID`,
`TaskID`, `PENDING`, absolute deadline and artifact sizes, then exits. It is **not**
an inventory terminal. A caller connected by the older optional IPC is not needed.

## Ordinary successive requests

Keep the same store configuration. No caller declaration or source/provider is
needed for these commands; the original declaration is already retained.

```sh
lsp-trace census durable task RUN_ID TASK_ID
lsp-trace census durable read RUN_ID TASK_ID source 0 32768
lsp-trace census durable read RUN_ID TASK_ID request 0 32768
lsp-trace census durable read RUN_ID TASK_ID prepared 0 32768
lsp-trace census durable submit RUN_ID TASK_ID interpreter < answer.jsonl
lsp-trace census durable result RUN_ID TASK_ID
```

Each invocation may be a new process. `read` returns `{RunID,TaskID,ArtifactID,
Offset,Total,Data}`; JSON `Data` is base64 encoding of the exact page bytes. Advance
by the decoded byte length, not the encoded length. A page is 1..32768 bytes.
`source` contains the exact source-unit JSON; `request` contains the exact Describe
request; `prepared` includes their original bindings, accounting and role policy.
These are copies, never mutable application state or admission tokens. A client
may read only the artifacts it needs. Each read reserves persistent usage before
loading/verifying the bounded original record.

`answer.jsonl` must use the existing supplied terminal adapter grammar: one
canonical JSON object plus LF, `status: COMPLETE`, and `text` containing canonical
Describe semantic JSON. The selected source-only consumer admits semantic
`verdict: COMPLETE`, `consumer_need: {status: UNRESOLVED, value: ""}`, and
`provided_behavior.consumer_relative: false` only. The generic parser/request
prompt's broader vocabulary is not lifecycle admissibility: ABSTAINED, RESOLVED
or consumer-relative responses refuse after claim. If evidence cannot support a
truthful COMPLETE description, stop/report inability without submitting; do not
fabricate COMPLETE or treat ABSTAINED as success. This is not a replacement
prepared artifact. The CLI reads
at most 65537 bytes, and the stored normal-folder adapter ceiling is 65536 bytes
(semantic text ceiling 32768). Overlong or malformed supplied answers consume the
claimed task and cannot be corrected by resubmission. Before reading stdin, the
CLI obtains the pending task's stored deadline and combines it with its request
context. A pipe held open with partial input is canceled at that deadline. Input
uses an owned duplicate pollable descriptor, joins its cancellation callback,
restores shared nonblocking flags and closes the duplicate; it does not abandon a
reader goroutine. A transport read failure does not submit the partial bytes or
claim the task. Only known bounded memory readers and OS files are accepted by
the internal input helper; arbitrary blocking `io.Reader` implementations refuse.

`result` returns a verified bounded description (maximum 8 MiB record), with
`Status: DESCRIBED_NOT_REVIEWED`, `Authority:0`, `Accepted:false`, and
`Completeness:UNKNOWN`. It is not an accepted feature inventory. Repeated result
reopen is safe but charged; it does not replay Describe.

## Lifetimes and accounting

- Native acquisition/preparation retains its existing <=15-second deadline and
  remains spent. No native work occurs in later commands.
- The normal-folder application wall deadline is fixed at invocation start to
  `min(--timeout,180s)`, before acquisition/preparation; the independent Describe
  task interval begins after preparation and is at most
  `min(--request-timeout,30s,remaining run time)`.
- These are absolute **wall deadlines including waiting between processes**.
  Pending reservations, claims, final CAS and confirmation use the minimum caller,
  stored task and stored run deadlines, including waiting on the real OS CAS lock.
  Verified result reads use their current caller context and persistent allowance,
  not the expired semantic task deadline.
  Nothing pauses a deadline. A slow agent may miss it; the task becomes EXPIRED
  and cannot be replayed. This does not claim arbitrary human/agent waits fit.
- The entire task interval is reserved once. This one-item slice uses no more
  than 30 seconds of the existing 120-second cumulative semantic allowance.
- Logical/copy allowance <=64 MiB, work <=`1<<32`, and <=64 artifact/result/submit
  packets are persisted across processes. Conservative reservations can stop a
  run before all otherwise legal pages are read. No per-process balance reset.
- Metadata/status is a bounded (<=16 KiB stored head) observation and can be
  repeated without spending an answer. Result reads remain possible after expiry
  only for already completed tasks, within persistent read/packet allowances.
- These are logical accounting limits, not OS memory/CPU containment or physical
  elapsed-work measurements. Each CLI continuation invocation also has a <=30s
  execution context. The existing original acquisition/controller admission
  remains separately accounted.

## State transitions and failures

The authoritative head uses existing cross-process publication CAS:

`PENDING -> CLAIMED -> COMPLETE -> verified completion confirmation`

There is one successful claimant. Missing/foreign run, task, artifact or role,
invalid paging, stale CAS, corrupt/truncated content and invalid hashes refuse.
Malformed answers, crashes after claim, or incomplete final confirmation expose
`UNCERTAIN_SPENT`; no automatic resubmit/replay is offered. A confirmation is
written only after the COMPLETE CAS has passed its receipt/readback checks. Missing
confirmation never yields a result. A late uncertain response must be resolved by
read-only task/result inspection, not by repeating submission. If a verified
complete result exists, inspection can return it without repeating effects.

Exact-FD publication and directory sync retain their existing platform limits;
there is no exactly-once distributed or power-loss durability claim. Tests cover
missing final confirmation and claimed/spent state, not exhaustive crash injection
at every syscall. Confirmation now uses a candidate plus a separate proof that
candidate CAS/readback/sync/close finished before the stored deadline. Candidate-only
and actually killed late-candidate writers refuse without requiring cleanup.
Timely proof reopens after expiry. This replaces observed-failure invalidation;
it does not qualify every fsync/close or power-loss boundary. Concurrent safe reads may encounter a stale reservation and
retry within remaining allowance; answer submissions must not be automatically
retried after uncertainty.

## Qualification boundary

The controlled Go fixture runs real normal folder preparation/acquisition with a
controlled LSP transport, exactly one admitted definition query and observed-empty
references. A exits and deletes its source workspace. Separate processes read
source/request pages, submit scripted Describe, reopen a verified description and
reject a duplicate. Same-thread and distinct-role declarations are tested; no
live actor independence is claimed. One test waits a real 16 seconds after A exits,
past native expiry but inside the semantic task deadline. Another waits past the
semantic deadline and requires refusal. Lower-level process tests cover exact
original evidence, malformed/spent state, corruption/truncation, concurrent
submission and missing completion confirmation. Test-only filesystem copies are
fault controls, not a supported import path.

Additional repair guards use two barrier-released OS submit processes (one winner),
reopen the verified result after real task expiry, kill a process after its malformed
answer persisted a claim, and hold the real publication flock in another process
past reservation/claim/final/confirmation deadlines. Killing the lock holder releases
its kernel lock. A genuine normal-folder CLI subprocess with partial, held-open
stdin returns after the stored deadline without submitting the partial answer.
These are bounded Darwin runtime witnesses, not exhaustive crash qualification.

The successor default route now implements shared applicationinventory transitions,
run-wide accounting, sequential review and actual publication/reopen. See
`census-durable-inventory.md` for its interface and qualification boundaries.
This explicit one-item option itself remains DESCRIBED_NOT_REVIEWED.
