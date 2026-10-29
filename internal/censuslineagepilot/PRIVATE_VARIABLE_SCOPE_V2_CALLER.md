# Private variable-scope V2 preparation caller

**Default OFF; test-only; authority=0; accepted=false.** Explicit `execute-once`
requires its separate reviewed scope/configuration and execution approval; no
real provider was started during this assignment. This connects the existing
V2 freeze → managed readiness → original active lease → discovery → every target
batch → shutdown → complete producer-packet pair → qualification/derivation →
all-batch instability/preparation → pending post-publication pair APIs. It does
not use the old fixed-six/fixed-25 custody path, ordinary admission, public
ExecuteAuthorized/ExecutePlannedBatch, a new CLI/MCP operation, or a source DTO
import route. Source/comments and interpretation text are untrusted data.

No real provider was run while implementing this caller. A later live source
operation requires independent parent review/adoption, a reviewed exact operation
configuration/scope/limits and explicit execution permission. A config is NOT
execution permission. Readonly preflight cannot establish runtime custody.

## Invocation

From the reviewed checkout, without either variable:

```sh
go test ./internal/censuslineagepilot -run '^TestPrivateVariableScopeRealV2$' -count=1 -v
```

This SKIPs before config IO, manager creation or writes. Invalid mode fails before
config IO. Only these two exact sentinel values are recognized:

```sh
GOWORK=off GOENV=off GOFLAGS= \
LSP_TRACE_PRIVATE_V2_CALLER=preflight \
LSP_TRACE_PRIVATE_V2_CALLER_CONFIG=/ABSOLUTE/REVIEWED/config.json \
go test ./internal/censuslineagepilot -run '^TestPrivateVariableScopeRealV2$' -count=1 -v
```

**Do not run the following without independent parent adoption and the separate
reviewed operation configuration/scope/limits and explicit execution approval.
The sentinel is a deliberate execution opt-in, not authority conferred by a config:**

```sh
GOWORK=off GOENV=off GOFLAGS= \
LSP_TRACE_PRIVATE_V2_CALLER=execute-once \
LSP_TRACE_PRIVATE_V2_CALLER_CONFIG=/ABSOLUTE/REVIEWED/config.json \
go test ./internal/censuslineagepilot -run '^TestPrivateVariableScopeRealV2$' -count=1 -v
```

Real execution uses the existing LocalDarwinSupervisor and is Darwin-only. Linux
builds permit owned pipe-provider tests and readonly preflight, not a claim of
real-backend support. No independently configured language server or managed MCP
replacement is started. A future MCP caller must independently satisfy its exact
registered-workspace READY/derive policy; this test is not an override.

## Configuration contract

The source of truth is `variableCallerConfig` in
`variable_scope_real_v2_caller_test.go`; all fields are required in the exact
`encoding/json.Marshal` struct field order (including empty arrays). No trailing
newline, whitespace variation, omitted fields, case aliases, duplicate keys,
unknown fields, alternative numeric encodings or trailing JSON are accepted.
Input ≤64 KiB; UTF-8; depth ≤16; token work ≤2048; arrays ≤32 and object members
≤64 before typed decoding; strings ≤8192 bytes (keys ≤128), with narrower field
bounds afterward. No expansion or implicit zero/default limits.

| Fields | Required meaning |
|---|---|
| `schema`, `purpose` | `private-variable-scope-v2-caller.v2`, `PRIVATE_VARIABLE_SCOPE_V2_PREPARATION_ONLY` |
| `code_root`, `code_head` | Canonical checkout path matching runtime.Caller's source path; exact lowercase 40-hex reviewed HEAD. Index-only diff and untracked checks are supplemental, never evidence of working byte equality; working-tree diff conversion is prohibited: the separate bounded HEAD-object input verifier below is mandatory. The driver and all relevant repository inputs must be reviewed/committed before real preflight succeeds. This is not proof of which bytes compiled the executing binary. |
| `scope` | Existing ScopeSpecPrivateV2: canonical root, 40-hex revision assertion, explicit members `{path,role,language_id,expected_sha256,byte_length}`, partition size and every effective-policy field. 1–8 nominations, 0–8 context, ≤16 members, go only. Member SHA/length must match before claim. Discovery ≤256 symbols; targets/batches ≤63. Existing policy bounds remain unchanged. |
| `session` | Existing SessionIdentity JSON fields `SessionID`, `Generation`; must exactly match the one Start result. |
| `selector` | Existing runtimeprofile.Selector JSON fields `TrustDomain`, `Workspace`, `Profile`, `EnvironmentReference`, `Options`; trust domain/profile/environment reference explicit; workspace exactly scope.root. Environment reference is a selector label, not ambient environment inheritance. |
| `server`, `server_sha256`, `args` | Canonical absolute executable regular file, required `sha256:` lowercase byte digest, empty `[]` only. No remote/network/redirect flags. Read ≤64 MiB, no-follow final open and identity checks before/after read; executable permission required. |
| `environment` | 1–32 explicit UTF-8 `KEY=VALUE` entries, each ≤4096 bytes, valid unique keys, no NUL; no unspecified inherited variables. |
| `prep_limits` | All existing prep.Limits fields explicitly positive: targets ≤64, part bytes 2048..2 MiB, total ≥part and ≤8 MiB, parts ≤256, source bytes ≤2 MiB. |
| `post_limits` | All existing PostPreparationLimits fields positive and within their unchanged private ceilings (64 constituents, 4096 ledger targets, 256 submissions/reviews, 16 claims/item, 32 citations/claim, 64 KiB string, 1 MiB item, 16 MiB input/output, 262144 evidence records, 10000000 work). |
| `seeds` | Ordered unique uint64 list, length 2..6. Order is preserved. |
| `operation_timeout_ms`, `readiness_timeout_ms`, `shutdown_timeout_ms` | Explicit positive ceilings 240000, 60000, 10000 respectively. The shutdown context bounds the caller's join/cleanup allowance, including readiness waiting; it is not the response read timer or a global shutdown bound. No retry/reset. |
| `shutdown_response_timeout_ms` | Required positive representable integer, at most `shutdown_timeout_ms`. Strict v2 incompatibility: absent/zero/negative/overflow/excess is refused before claim. No migration of retained v1 configurations or artifacts. Constructor-created private response allowance is not acquisition authority. Its single deadline is capped by the remaining cleanup context; shutdown write time consumes it. Synchronous shutdown/exit writes and teardown/close/readjoin are not globally bounded; context expiry may return nonjoined failure. The placeholder's 1000ms/10000ms values are examples, not live approval or a promise of success. |
| `operation_directory` | Existing fresh empty canonical private 0700 directory outside `/tmp` and `/private/tmp`. |
| `producer_outputs`, `post_outputs` | Each exactly two existing fresh empty canonical private 0700 directories outside temporary roots. No mkdir or output-root creation by caller. |
| `protected_roots` | Explicit `[]` or ≤15 additional existing protected private roots (source plus additional ≤16). These supplement, never replace, source protection and the exact two genuine producer input roots. |
| `attempt_marker_path`, `report_path` | Distinct initially absent single direct children of operation_directory, not config/server/source/output selectors. |

All source, operation, producer, post and additional protected roots must be
physically distinct with no equality/ancestry in either direction. Canonical
component checks reject symlink aliases. Source protection uses
SourceProtectionRootPrivateV2 (source can be 0755 or under a temporary root), not
an output-root capability. Preflight opens only readonly topology constructors,
closes all of them and propagates close errors. It does not create an attempt,
report, publisher, manager, Census request or directory. It may Freeze the exact
finite pinned source and emits a bounded `t.Log` JSON observation, not a disk report.

### Placeholder example, not an operation config

`private_variable_scope_v2.example.json` is the single canonical-encoding
placeholder example. It round-trips byte-for-byte through the config struct but
intentionally fails semantic validation: placeholders are NOT asserted hashes,
HEADs, source lengths or reviewed paths. Its numeric ceilings illustrate existing
bounds, not approved operation limits. Replace every placeholder and review all
fields, including `byte_length`, environment and limits, then serialize once with
`json.Marshal`. Never execute the example or infer permission from it.

## HEAD-matching repository input observation

The private `HEAD_REPOSITORY_INPUTS_PRIVATE_V1` policy observes **HEAD-matching
repository build inputs before startup**, not a clean-index proxy. It never
calls `go list`, build, generate, a compiler, shell or network service. Git is
always the deliberately fixed `/usr/bin/git`, never PATH lookup. Commands use
`--no-pager`, `--no-optional-locks`, `core.fsmonitor=false`, disabled hooks and
untracked cache, a fixed noninteractive environment and no lazy fetch. Every
command also sets both `--no-replace-objects` and `GIT_NO_REPLACE_OBJECTS=1`;
caller environment cannot re-enable replacement refs. The command-shape allowlist
admits only the required raw object/config/status forms, not cat-file conversion,
hash-object conversion, checkout or arbitrary Git commands. Working-tree diff
is **not executed**. Index-only diff disables external diff, textconv and
submodule work. No source clean conversion is used to establish byte equality.

Builtin `git config --includes --get-regexp` queries observe **effective** local,
worktree and included configuration, with no `--local` scope restriction; system
and global config are disabled. Clean-filter and partial-clone/promisor settings
are refused before index comparison or object traversal. Worktree-specific
includes may read configuration metadata outside CodeRoot, never execute it or
add it to repository source attestation. This avoids both the local-only blind
spot and relying on a later byte rejection to undo an earlier helper write.
All commands and file IO share the finite operation deadline.

Starting at `internal/censuslineagepilot`, it verifies actual working bytes
against raw unreplaced Git commit/tree/blob objects, independent of working-index
bits or ignore lists. The literal commit is canonically hashed against CodeHEAD;
its original tree OID is retained as `CommitTreeOID`. Each binary tree object is
canonically hashed and decoded under that parent OID, and each raw blob is also
hashed. Thus commit→tree→child-tree→blob binding is checked, not inferred from a
mutable HEAD label or only from leaf blob IDs. Verified Go files are parsed with ImportsOnly; all platform
files and internal/external `_test.go` imports are included conservatively, then
local module imports are traversed. The complete direct package-file envelope is
**every visible regular direct file**, but accepted local compilation is strictly
**pure Go**. Compiler-consumed assembly, C/C++/Objective-C/Fortran, SWIG,
precompiled object/library/resource/IR inputs and native cgo compiler directives
are refused explicitly, as is import `C`. There is no assembler/preprocessor
parser, header resolver or toolchain-header allowlist. `.s`/`.S` inputs are
refused regardless of `#include`, `.include`, `%include`, incbin, macro syntax or
absence of include text; even `textflag.h` in assembly is not allowlisted.
Headers and incidental documentation may remain byte-pinned opaque auxiliaries
only because the admitted local build has no native consumer. Embedded native
text is data, not permission to compile or interpret it. No transitive native
include-closure claim is made. Both
HEAD-expected missing files and actual untracked/ignored additions fail. Ordinary
unreferenced subdirectories are not traversed; only local imports or embed
patterns authorize traversal. Package filenames beginning `.` or `_` follow Go's
ignore rule; explicit embed matches and `all:` retain Go embed semantics, while
recursive ordinary embed directory walks exclude `.`/`_` children and VCS dirs.
Go comments are scanned as syntax for `//go:embed`, never instructions. Embed
patterns/trees are bounded; every matched file, including ignored/untracked
candidates, must have matching HEAD bytes. Source component symlinks, submodule
traversal and nested-module indirection are refused.

`go.mod` is mandatory and HEAD-pinned; optional `go.sum`, `go.work`, and
`go.work.sum` must have identical actual/HEAD presence and bytes. This first
profile then refuses workspace files, replace directives, vendoring and cgo
include indirection rather than silently trusting another source root. Actual
preflight requires explicit caller `GOWORK=off GOENV=off`, empty GOFLAGS, and
module mode; explicit server environment also refuses GOFLAGS and external
GOWORK/GOENV overrides. No source code is read outside CodeRoot.

Limits: **4096 input files, 4 MiB/file, 64 MiB aggregate input bytes, 256 local
packages, 2048 directories, 65536 combined actual/HEAD entries, 12288 code-verifier
Git commands, depth 64 and 256 embed patterns per Go file**. Independent legacy
preflight Git checks remain a fixed finite list; index-only diff adds one bounded
effective readonly config query. Relevant working inputs must match HEAD exactly;
staged changes and reported untracked files are refused. Unrelated unstaged
files outside the declared input envelope are not represented as globally clean. HEAD object sizes reserve per-file/remaining aggregate bytes before body reads;
buffers are bounded before use. Limits refuse oversized
closures, not an invitation to expand/retry. A zero-byte tracked auxiliary file
is valid; zero never selects a default resource budget.

The report and attempt marker retain the code witness's digest, counts, profile,
HEAD/original commit-tree/module/root and limits, not 64 MiB of source. The digest commits to sorted
path/blob/SHA/length records and policy. Actual byte verification is repeated
before the claim and again after claim immediately before manager construction;
a changed witness fails without startup, and a claimed operation stays spent.
The 64 MiB limit is unique input bytes per verification, not cumulative IO or heap
metering: at most three verification passes each compare working and HEAD bytes.

Proof limits: this is a practical same-user readonly observation, not continuous
monitoring, adversarial self-attestation, authenticated Git provenance or proof
of the source that compiled the running binary. Toolchain, standard library,
external module dependencies, native SDKs and build-cache/binary provenance are
not attested. Unsupported local build indirection fails closed. The compiled
source-path check cannot attest an arbitrarily self-modified checker. These
limits do not waive repository-input byte and membership verification.

Owned legacy protocol fixtures use an explicitly named private fake code-input
backend (`OWNED_FAKE_REPOSITORY_INPUTS_ONLY`), never selectable by the real
entrypoint. New disposable-Git fixtures exercise real `/usr/bin/git`, HEAD/blob
comparison, import/embed closure and zero-write snapshots below only the separate
compiled-path framing gate. Linked-worktree direct/included clean-filter receipts,
replacement A→B objects and an assume-unchanged outside-header assembly case were
observed RED before repair and GREEN afterward. They do not claim to be the
executing checkout.

## Clarified cleanup contract

The parent clarified that `NoManageraftershutdown` prohibits new provider/source
requests, acquisition, lifecycle execution, restart or rebind after Shutdown;
it explicitly permits readonly terminal status observations needed to report
cleanup errors. No public Manager API or global Shutdown semantics were changed.
`Stop` asynchronously owns teardown; `Shutdown` joins that worker rather than
stopping a READY child by itself. The caller issues **one Stop**, joins through
**one Shutdown** using the explicit shutdown deadline, then performs at most
**one readonly Operation(intentID)** observation. This lookup neither enqueues
work nor grants permission. No readiness/start/acquisition/rebind/restart/second
Stop or provider request follows Shutdown.

Success requires the exact pinned session/generation, exact intent/caller,
non-restart COMPLETE operation with empty failure, successful join and exactly
one Stop/Shutdown/read. Missing intent, unavailable operation, pending/unknown/
failed state, foreign identity, nonempty failure or join error is conservative
failure, never verified cleanup. The final report records this observation and
all returned cleanup errors before any report write. Child-close failure is
visible through the joined terminal operation even when Shutdown itself returns
nil. A timeout is an incomplete cleanup observation, not proof of process death;
there is no retry, second Stop or hidden recovery. Any already-running Stop
worker is not claimed terminated by a timed-out join.

## One attempt and final artifacts

The following execution behavior is implemented and verified through owned
injected Managers. The real entrypoint retains the same explicit mode, strict
configuration and preflight/claim boundaries; no actual real candidate config
was created or executed during implementation.

After readonly preflight and execute-once selection, the caller rechecks config
identity/full bytes, executable digest, frozen expected source bytes, operation
root identity and report absence. An exclusive no-follow direct-child marker is
created **before manager construction/start**. It contains config SHA, CodeHEAD,
scope SHA, the code-input witness and preparation limits, `RUN_CLAIMED_NOT_RECOVERABLE`, authority=0,
accepted=false. File and directory sync are attempted. Any existing marker,
including a failed/partial one, spends the operation permanently; no deleting,
replacing, reset, replay, retry, restart or output cleanup is supported.

One manager/Start and readiness attempt, exact session/generation/command check,
original leased DiscoverScopePrivateV2, and one AcquireScopeBatchPrivateV2 per
actual batch are used. After lease exit, a single explicit Stop is issued before Shutdown joins its
worker, followed only by a readonly exact terminal-operation lookup. This is
necessary because Manager.Shutdown alone does not tear down a READY child. The
complete shutdown procedure finishes before any retain, qualify, derive, project,
instability or preparation call. No provider requests
are made during these offline stages. Borrowed producer/source/post capabilities
remain alive until all dependent reads finish. Nil submissions and nil reviews
produce PENDING: no manufactured LLM claims, reviews, semantic independence,
leads or accepted feature identity. Prep/resource/source gaps retain their real
per-target dispositions; invalid preparation or retention errors fail spent.

Final report is exclusive-create once, ≤1 MiB. It preserves exact config and
successful marker bytes/hashes, source and code-input witnesses, actual discovery, runtime
observations, phases, errors/close errors, producer copy receipts/budget,
qualification, preparation SHA/length/coverage, pending ledger and post-pair
receipts/envelope identity. Existing APIs verify exact full physical copy bytes;
references and receipts are inspectable without replay. Source revision,
CodeHEAD and executable byte hash are assertions/pins, **not authenticated
provider identity**, OS monitoring, source completeness or production authority.
Ordinary native and Describe remain NOT_ATTEMPTED.

Stop terminal failure (including child resource-close failure), returned Manager
shutdown errors and borrowed-capability close errors are collected before
the report attempt, including on startup/cancellation/readiness/capture failures.
Report write/sync/close errors propagate but cannot be inserted into the already
written immutable report itself. Likewise the final operation-capability close
error is returned after reporting. Marker/output commits are never rolled back,
including partial pair copies or report failure. Namespace pairs are not atomic;
sync is practical durability only, not a crash/power-loss/authenticated-history
claim. Concurrent namespace replacement is outside the observation guarantee.

## Owned verification

```sh
go test ./internal/censuslineagepilot -run '^(TestPrivateVariableScopeRealV2|TestVariableCallerV2)' -count=1 -v
go test -race ./internal/censuslineagepilot -run '^TestVariableCallerV2' -count=1
```

These tests inject only a package-private test factory and bounded owned git
responses; pipe providers never execute the pinned fake executable. They cover
mode/default-off, readonly preflight (zero factory calls/writes), strict parsing,
pins, topology, permissions, marker/report absence, exact startup environment,
1/0 and 2/1 all-batch pending physical pairs/report lineage, startup/readiness/
malformed-symbol/resource/source-gap/report-close/shutdown-close failures,
claimed cancellation, late report ErrExist preservation and spent replay.
The readonly positive case also accepts source-only 0755 temporary roots while
continuing to reject temporary or nonprivate output roots.
The readonly guard was counterfactually observed failing when a factory call was
inserted into preflight, then restored and observed passing. The conservative
terminal-status guard was also observed failing when its COMPLETE-only check was
weakened to allow unknown status, then restored and observed passing. Owned
asynchronous Stop barriers prove no output/report is written before the join;
actual resource-close failure appears in both returned error and final report.
Owned counters verify one start, Stop, teardown, Shutdown and readonly terminal
observation; zero transport writes after child close is stronger than zero after
the Shutdown that joined that close. Missing/unknown/pending/foreign terminal
classifications are tested against detached observations of an actual owned
successful operation, not fabricated runtime custody. No live candidate config,
source query, provider or ordinary admission was used by this verification.
