# Host-neutral Go census task/response boundary

The product is the Go command and its existing V3 controller. **Pi, Node, Python,
model APIs and credentials are not runtime requirements.** A caller may implement
this bounded protocol using its own process/IPC tools. The Go standard-library
subprocess client in `cmd/lsp-trace/census_folder_go_host_test.go` exercises the
actual normal-folder command entry with controlled LSP and semantic replies and
an environment whose PATH cannot resolve those optional runtimes. It is a
mechanical integration fixture, not an agent/model or live-host qualification.

## Dependency map

| Path | Required implementation/dependency | Role |
| --- | --- | --- |
| Normal census command | Go binary; existing platform/native and LSP-provider support | Acquires/retains originals, invokes controller once |
| Task/response boundary | Go `net`, bounded framing/JSON and inherited connected UNIX stream | Emits task; accepts exact caller response; checks binding/deadline |
| Fresh normal execution | Existing inventory-successor kernel reserve/reopen/observe | Separate one-shot parent allocation, executable hash and child-grant permit |
| Semantic workflow/publication | Existing Go V3 controller, transport, journal and verifier | Describe/group/separate Reviewer; conservation and readback |
| Calling host | Owned concurrent process handle, duplex bytes, later tool/control-return and cancellation/join | Agent invokes Go, reasons with its own tools, replies |
| Go integration client | Go standard library process/socket APIs; controlled test fixtures | Tests boundary without Pi/Node/Python/provider credentials |
| Optional Pi convenience | `.pi/extensions/census-caller.ts` and Node transport helper | One optional mapping; neither loaded nor launched by Go |
| Prior Python helpers | None on the production path | Unused historical assets preserved pending separately scoped cleanup |

Language-server dependencies remain those of the chosen existing provider; this
change adds no model backend or agent runtime dependency. Host-neutral does not
mean every operating system or arbitrary harness is automatically supported.

## Execution boundary

The calling agent invokes Go. Go never invokes that agent, a model API, or a new
inference backend. Go owns census coordination, original native inputs, M/P/S,
closed response validation, review ordering, accounting, and publication/readback.
The agent supplies Describe, grouping, and a separate sequential Reviewer reply
using its existing reasoning/tools. Caller-supplied identity and pins are not
execution authentication. Authority remains 0, Accepted false, completeness UNKNOWN.

A synchronous shell command cannot wait for semantic work from the very agent
blocked on that command. The required capability is an explicitly **host-owned
concurrent child**, retained across tool returns, with a local duplex channel and
shutdown/cancellation joining. Merely launching `... &`, recording a PID, writing
an opaque proof to disk, or replaying a transcript does not provide this capability.

## Protocol

On macOS the host spawns an attached, non-detached Go child with a connected UNIX
stream on a descriptor >=3 and sets `LSP_TRACE_CALLER_FD` for that child only. Go
checks socket type, UNIX peer and connectedness, and owns a duplicate. It accepts
neither a regular file, stdin/stdout, TCP, listener, nor an unconnected descriptor.
There is no Python dependency. No model credential is transported.

Each message is a four-byte big-endian byte length followed by JSON. The initial
hello (<=4096 bytes) has `Protocol: "lsp-trace.census-caller.v1"`, `Subagents`,
`Interpreter`, `Reviewer`, and `Pins`. Pins use the existing snake_case V2 fields;
`prompt_sha256` must initially be empty because Go binds the actual request prompt.
Other pins must be sha256 digests. Distinct real context identities are required
for SUBAGENTS; SAME_THREAD uses the same context identity for both roles. Choose
mode before setup/sealing. Prefer the caller's actual subagent tools if available;
do not invent tool names/signatures or claim context independence from a label.

Go writes `ACTIVATION_READY`, with the automatically created request location,
run binding, selected review mode and both role prompts. This is metadata, not
permission to prepare opaque inputs elsewhere. The existing controller remains
the sole preparer inside the retained native lifetime.

Go then writes `NEEDS_HOST` containing the existing `InventoryFrameV3`. The host
returns that task to the agent while retaining the live child. A later tool call
sends `Protocol`, `RunID`, `Role`, `RequestDigest`, `Sequence`, and `Payload`.
Payload is **base64 of the exact response bytes**, not a nested JSON object.
This preserves the mandatory terminal newline on the strict Describe adapter
record. Go decodes <=96KiB framing and separately enforces <=64KiB decoded replies.
A JSON RawMessage wrapper is incorrect here: its encoding can discard that newline.

For Describe, use the existing strict adapter/semantic grammar, not a prose answer.
For GroupDecision and Reviewer use the existing NativeDecisionV3 schema. The
Reviewer receives the actual proposal and must name its predecessor correctly.
No hand-authored request/manifest/config/cursor files are required. Go performs
setup, registration, waits, submissions, and verification through the existing
controller interfaces rather than implementing another semantic coordinator.

The original frame and grant deadlines continue while the host thinks or delegates.
There is no pause, refresh, replay, refund, or resume authorization. Missing,
malformed, stale, disconnected, and withheld caller replies refuse. A bare shell
without the channel reports CALLER_AGENT_UNAVAILABLE (currently after bounded
folder acquisition, not a promise that no acquisition was spent).

Go sends informational `TERMINAL` only after publication readback verification.
The host must still wait for successful child exit: native owner cleanup and final
normal CLI output occur after the controller returns. A terminal message alone
is not a cleanup-success proof. Abnormal/forced termination never qualifies cleanup.

## Optional Pi mapping

`.pi/extensions/census-caller.ts` uses documented `registerTool`, sequential tool
execution, session shutdown, tree transition, and aborted-agent lifecycle hooks.
It registers `census_inventory` with start/next/respond/page/cancel actions.
`scripts/census-caller-process.mjs` only owns/spawns/joins the child and transports
bounded bytes; it does not call models, decide groups, schedule roles, validate
publication proofs, or persist native custody. Go has no Pi dependency.

Start returns activation; next returns the pending task; respond submits caller-
authored exact bytes and returns the next task. Large frames have bounded pages;
read every page before answering. The Go child remains attached/owned between
those tool calls. Cancel/session shutdown closes the channel, sends TERM, waits
for close, and escalates to KILL after two seconds if necessary; forced shutdown
is failure, not evidence of normal native cleanup. No detached/unref child is used.

The extension is a repository asset, not proof it is loaded in an already-running
host. Tests load it through the installed Pi-compatible module loader with a mock
extension API. No Pi reload/install or live model run was performed here. Loading
and registration checks do not qualify full Pi lifecycle behavior.

## Admission, producer identity and timing

The normal path derives the child application grant from the current planned call
count, the shared coordinator quote, V3's own inventory/retention allowances,
two bootstrap admissions (create/setup and open/run), API overhead derived as
16 fixed plus three per planned exchange, and a bounded local IPC/readback lane. It no longer copies
example grant numbers. `WithLocalFolderControllerV1` binds that exact vector to
the completed acquisition, reserves it plus the prelude in a fresh kernel chain,
independently reopens the pending head, then lends a one-shot callback permit.
An acquisition cannot obtain another controller grant by choosing a new directory.
The request builder rejects even a one-byte-short or changed-deadline child grant.
Preparation, acquisition, selected native ownership and controller accounting are
separate; parent prepayment is not added to child expenditure as measured cost.
The parent and child envelopes remain within the prior ceilings.

Only after reservation/readback does Go read its `os.Executable()` file, using a
32KiB buffer, a 64MiB finite hash/read bound, cancellation checks and before/after
file identity/size/time checks. `ProducerExecutableSHA256` now contains that
self-observed executable digest, not `WorkerSHA256`. Oversized/unreadable/changed
executables refuse. This is local self-observation, **not authentication of the
calling agent, model, loaded memory image, or an independently trusted producer**.

The semantic window is now
`min(request-timeout, 30s, 120s/planned-calls, remaining-owner-time)`;
there is no hard-coded one-second response limit. V3 retains its 120s cumulative
semantic allowance and the normal grant stays below the 180s application bound.
Crucially, the current native acquisition still has its original stricter **15s
absolute lifetime** (or a shorter caller timeout). Once acquisition starts, that
clock is not reset after selected preparation or between responses. Cancellation closes a pending IPC read and joins
the controller. Expired/failed reservations stay spent; no retry directory,
serialized request, cursor or transcript can renew the original owner.

## Supported and unqualified hosts

- **Controlled and tested:** macOS normal-folder command entry, driven by a Go
  owned subprocess/UNIX-stream client, both review modes, delayed reply beyond
  one second, missing/disconnected/malformed/stale/mismatched/withheld replies,
  cancellation, actual publication/reopen and corrupt/withheld report rejection.
  The provider wire and semantic answers are scripted. The client uses the Go
  test executable to enter the actual command path, not a released live-provider
  binary or a fabricated alternate controller.
- **Required but not universally available:** a host must keep the same Go child
  owned while returning a task to the agent, accept a later response, and cancel/
  join that child when its session ends. A Go host can use `exec.Cmd.ExtraFiles`
  and an owned UNIX stream, as the integration client demonstrates. An arbitrary
  harness lacking those capabilities is unsupported; no imaginary tool API is
  assumed. A synchronous shell call cannot service its own blocked agent.
- **Optional/unqualified:** the Pi convenience extension is not required for
  discovery, provisioning or authority. It was not changed or live-tested in this
  phase. No statement here qualifies all Pi versions or any other installed host.
- **Still unqualified:** real agent/subagent response latency within the remaining
  15s native lifetime, live semantic correctness, whole-application completeness,
  operating-system ports, exhaustive physical resource containment and all
  provider/language combinations. A caller needing more time must fail honestly;
  this work does not enlarge the native lifetime to 120s or 180s.
- The extension binds host-declared subagent identities but cannot establish that
  the agent actually delegated work or obtained context-independent review.
- The tests cover a controlled two-file application and scripted semantic answers.
  They do not establish whole-application coverage, real model correctness,
  independent-context assurance, all-language/provider support, or live completion.

## Offline checks

The primary portability checks require only Go; the process client starts and
joins its own test executable and exercises the actual normal-folder route.

```sh
GOPROXY=off GOSUMDB=off GOTOOLCHAIN=local GOTELEMETRY=off \
  go test -tags=lsp_trace_controlled_describe_fixture ./cmd/lsp-trace \
  -run '^TestCensusFolder(GoHost|HostNeutralBuild|AdmissionRequest|CrossFileInventory)$' -count=1
GOPROXY=off GOSUMDB=off GOTOOLCHAIN=local GOTELEMETRY=off \
  go test ./internal/inventorysuccessor -run '^TestFolderController' -count=1
```

The older Node adapter tests remain optional convenience checks, not build or
production prerequisites. Their `CENSUS_GO_TEST_BINARY` and `PI_MODULE_ROOT`
variables are test locators, never model/provider or product configuration.

The Go process tests cover owned return-of-control, exact newline transport,
cancellation joining, actual normal-folder SAME_THREAD/SUBAGENTS fixtures,
missing/malformed/stale/mismatched/disconnected/withheld replies, and delayed
responses beyond the old one-second window. Separate unit guards cover the exact
admitted grant, executable byte hash/bounds/cancellation and one-shot admission. `TestCensusFolderCrossFileInventory` additionally verifies real
normal controller publication, grouped+unresolved conservation, exact retained
originals, publication reopen, withheld/corrupt report rejection and restoration.
