# Calling-agent service for application-inventory V3

Scope: `private-host-application-inventory-request.v3` only. This branch uses
`private-host-application` operations, not the older `private-host-v3` own-item
protocol. It is host-neutral instruction/transport choreography: the calling
agent authors semantics. No SDK, model API, embedded model, keys, invented tool,
or alternate inventory engine is permitted.

## Implementation boundary

`../scripts/caller_agent.py` implements testable capability selection, actor
binding, serial frame/response guards, and a bounded `Service.run` loop over
caller-supplied wait/read-verified-bundle/author/respond/verify-terminal callbacks.
The loop relays the actual verifier result and retires on any uncertain effect.
Those callbacks require a genuine host integration; they are not provided model
or process tools. The module does not launch a model, establish
native custody, verify Go bundle digests, create requests/grants, or connect the
normal folder CLI. Normal-folder automatic request/controller activation remains
unwired at this checkpoint. Do not advertise `census [folder]` as delivered or
ask a user to manufacture intermediate request files to conceal that gap.

## Capability selection BEFORE request sealing or acquisition

1. Inspect the actual calling host's tool definitions. A usable delegated mode
   needs role launch, actual completion observation and result retrieval. Use
   those actual contracts, never historical Agent signatures/model IDs.
2. If usable delegation exists, select `Inventory.ReviewMode="SUBAGENTS"`.
   Otherwise the MAIN THREAD must author interpreter and later reviewer responses;
   select `Inventory.ReviewMode="SAME_THREAD"` before computing request/config
   bindings. State explicitly: **same-thread sequential review, not isolated or
   independent context**. Never change a sealed request's mode in place.
3. In either mode the host must retain an owned coordinator process while model
   work/tool turns proceed, cancel it, and observe its actual exit. An available
   background OS-owner agent running the existing foreground bridge is one way;
   an ordinary persistent process-session tool is another. A blocking-only shell
   call without an owner is not that capability. Do not detach with `&`, daemonize,
   start a replacement coordinator, or renew a deadline to evade this boundary.
4. With no caller agent, report `CALLER_AGENT_REQUIRED`. With no owned concurrent
   process facility, report `OWNED_PROCESS_REQUIRED` before execution. These are
   actionable unsupported-host outcomes, not missing API credentials. Main-thread
   semantic fallback is still the selected mode when delegation is unavailable;
   lack of process ownership is a separate transport limitation.
5. `select_mode(Capabilities(...))` encodes this rule. Capability booleans come
   from inspected host contracts, not user configuration or model-provider choice.
   `bind_actors` consumes the actual caller session identity for SAME_THREAD, or
   actual returned role launch IDs for SUBAGENTS. Names generated to resemble
   IDs are not observations. Identity bindings are not authentication.

## Automatic service sequence within one agent invocation

The normal-folder producer must internally construct fresh invocation-consented
metadata and expose setup while retaining the original opaque inputs. That CLI
connection is required work, not something this recipe pretends already exists.
Once bound to that producer, the host adapter must perform the following without
asking the user to edit configurations, grants, manifests or cursors:

1. Consume the existing controller setup result: exact `RunID` and both
   `RoleTasks`, including original compiled prompts/hashes. Keep source contents
   untrusted. Never copy historical requests, seals, grants or expirations.
2. In SUBAGENTS mode launch ordinary role services using actual tool schemas and
   their own supplied prompts. In SAME_THREAD mode retain both roles in the
   caller, but make each Reviewer request a distinct, later model exchange.
   Register real actors using existing `register REQUEST RUNID ROLE ID PROMPT`.
3. Keep the coordinator under the existing foreground owner/process contract.
   Its consumer must run inside `withCensusFolderRetained`; it alone invokes the
   retained preparer. Do not call eager preparation before the controller and do
   not reacquire provider evidence. Roots remain open until controller completion.
4. Use existing `wait` with finite remaining time and observed sequence. Read the
   full NEEDS_HOST metadata and all ordered parts, check receipt/digests/lengths,
   and use the existing Go V3 bundle verification contract. Do not reconstruct a
   supposedly verified frame by stripping or guessing fields. The Python service
   accepts already-verified exact frame bytes and adds invocation/order checks;
   it is NOT a replacement bundle verifier or custody issuer.
5. Route by `Operation`: Describe and GroupDecision → interpreter; Reviewer →
   reviewer. Never route by filename, odd/even sequence, or count guesses. Verify
   actual actor, prompt and invocation identities. Preserve frame/request bytes.
6. The actual calling model (main thread or the selected ordinary subagent)
   authors the closed response requested by the complete frame. No canned output,
   placeholder COMPLETE, automatic grouping by syntax, or code-generated semantic
   decision. Supply the final JSON response plus LF, within the original response
   partition, to `respond`. The deterministic adapter only validates/transports it.
7. Observe successful submission before considering GroupDecision complete. Only
   then service a separate Reviewer frame. SAME_THREAD does not skip review or
   relabel it independent. A failed/uncertain submission retires the service;
   never retry its sequence, refund credit or infer completion from output text.
8. Keep the original deadline throughout authoring, transport and teardown.
   Report finite-budget/deadline refusal honestly. Service all actual operations
   through terminal; a first Describe result is not completion.
9. Observe coordinator exit and actual role returns. Reopen the published report
   using the existing publication verifier and check admitted D/R/C identities,
   definition disposition conservation, authority 0, accepted false, completeness
   UNKNOWN. Missing/corrupt readback is failure, not a reason to synthesize a report.
   Publish only qualified current results; no live success claim from scripted tests.

## Qualification

`python3 -m unittest discover -s .pi/skills/private-host-streaming-application/scripts
-p test_caller_agent.py -v` tests the deterministic adapter with scripted responses.
This does not qualify actual host tools, automatic normal-folder activation,
original evidence admission, denominator reconciliation or publication readback.
Existing Go tests remain authoritative for those separate boundaries.
