# Automatic provisional inventory — private V2 AST

Experimental, default OFF. This is neither ordinary Native, qualified Describe,
MethodResult, FULL_DEFINITION nor the disabled broader ADR0007 bridge. All outputs
remain authority=0, accepted=false, featureIdentity=UNRESOLVED,
sourceCompleteness=UNKNOWN, absence=PROHIBITED. Local implementation is not live
execution permission, qualification, shipment or stakeholder acceptance.

## Architectural correction

The primary inference path is the **existing HOST session**, not another model
backend. Go prepares evidence and validates/retains typed results only. The HTTP
adapter, model/endpoint configuration, credential reads and TLS fixtures have been
removed. No model client, new provider dependency, public API/MCP/CLI operation,
Go agent/harness/process launcher or permanent server replaces them.

The real private caller now connects:

`variableRunWithAutomatic -> NewPostPreparationPrivateV2 ->
variableRealAutomatic -> NewAutomaticHandoffPrivateV2 -> AutomatePrivateV2 ->
RetainAutomaticPrivateV2 -> exact paired readback -> terminal receipt`.

The original worker keeps the opaque owner and root capabilities alive. The
mailbox exports evidence, not acquisition authority. There is no from-JSON owner
constructor or restart recovery. New owner nonces prevent same-byte mailbox
replay; the adapter additionally checks the exact in-memory preparation owner.

**[Host-owned launch/read/respond/status workflow](../../docs/pilot/adr0007/private-variable-v2-host-handoff.md)**
is the actual entry recipe. A single user request lets the host perform all tool
calls and isolated roles automatically. Neither a manual user handoff nor parent
assembly of report artifacts is required. The helper never invokes those roles.

## Configuration and custody

- Legacy `private-variable-scope-v2-caller.v2` is preparation-only, unchanged.
- HTTP `private-variable-scope-v2-caller.v3` is retired and refused before claim
  or process start. No migration, inference fallback or automatic retry.
- Local handoff uses `private-variable-scope-v2-caller.v4` and purpose
  `PRIVATE_VARIABLE_SCOPE_V2_AUTOMATIC_PROVISIONAL`.
- `automation` is `AutomaticHandoffConfigPrivateV2`: Enabled, Directory,
  Invocation, InterpreterRole, ReviewerRole, ScopeSHA256, TimeoutMS,
  OverallTimeoutMS, PollMS, unchanged Limits, ApprovalSHA256. Approval is the
  SHA256 of exact Go canonical config bytes with ApprovalSHA256 blanked.
- Role/session tags must differ; the same host model is allowed. This is a caller
  assertion of isolation, not authenticated independence or expertise.
- Existing exact code/config/source/server pins, Darwin supervision, explicit
  acquisition environment, scope, shutdown and single-spend guards stay intact.
  Real execution requires a fresh reviewed configuration and separate approval;
  the dirty working diff cannot pass by reusing an old code pin.
- Mailbox is a fresh canonical current-owner 0700 directory outside `/tmp`,
  nonoverlapping source, producer, operation/journal, protected and final roots.
  Knowable path errors are refused before claim. Files are direct-child,
  current-owner 0600 single-link regular objects, no-follow, create-only and
  descriptor-rooted. A matching temporary hard link is an unfinished atomic
  publication, never accepted as a response. No arbitrary same-user attacker or
  crash-recovery resistance is claimed.

## Finite protocol

The distinct custody label is `PRIVATE_V2_AST_LOCAL_HANDOFF.v1`.
`preparation.json` binds exact prepared bytes/length/digest, including the selected
packet manifests, scope, invocation, random owner nonce, limits and overall
deadline. Each `request-NNNNNN.json` contains a canonical request and binding:
protocol, invocation, nonce, monotonic sequence, stage, role ID, request digest,
preparation/input/target, attempt, predecessor, response allowance and deadline.

Each `response-NNNNNN.json` echoes that binding and binds exact payload bytes by
length and SHA256. Payload is base64, so even unacceptable model bytes can be
preserved without normalization. The host helper calculates framing/digests; it
does not repair semantic output. A duplicate cannot overwrite an earlier file.
`terminal.json` records verified-pair digest/length or accountable failure and
closes the mailbox. Report-write/close failures can still invalidate overall caller
success; both the joined host exit status and verified terminal report matter.

Stages accept only the existing exact types: INTERPRET uses
AutomaticInterpretationPrivateV2; REVIEW_TECHNICAL and REVIEW_NAVIGATION use
AutomaticReviewPrivateV2; NAVIGATE uses nonnull []NavigationPrivateV2. Reviewers
receive the complete selected own packet and bodies, not merely chosen citations.
Source, prompts, prior revisions and model responses are inert data.

## Preserved pipeline and limits

Every planned batch still completes original acquisition and one Stop/Shutdown/
terminal cleanup observation before source preparation. Requests preserve order,
direction, duplicates, cache/source attribution and null versus empty arrays.
Quotes retain exact UTF-8 bytes; the existing lossless 240-byte ledger chunk
adapter, bounded explicit semantic corrections, predecessor IDs, review binding,
stable origin-based candidate IDs, reversible navigation, reconciliation and
paired human/machine publication remain unchanged. No old Describe admission bits
or selectors are borrowed. No historical packet/campaign is modified or replayed.

Request frames and consumed response-frame prefixes are charged by the existing
aggregate budget. Request sizing conservatively reserves the maximum allowance's
encoded digit width; a narrowed aggregate allowance can overcharge a few reserved
digit bytes, never undercharge the emitted request. No response capacity means no send/count/charge. Reads consume
at most allowance+1; only the possible final aggregate lookahead is excluded from
the saturating counter. This is logical IPC-byte accounting, not heap, syscall,
OS buffer or host-inference metering. No automatic limit is increased.
Malformed **frames**, misbindings, unavailable/unsafe/closed mailboxes, late or
missing responses and cancellation are terminal, not semantic retry. Malformed
or rejected **payloads** retain their exact mailbox bytes and may use only the
existing explicit bounded correction sequence. Terminal failure accounts for
remaining selected targets and publishes no successful candidate inventory.

## Verification and incomplete qualification

New owned tests exercise the real file adapter and actual host helper in separate
original-worker, host-emulator and supervisor processes, for one and three
batches, with paired machine/human readback. The emulator is a fixture, not a model
or production fallback. Stop/join behavior is separately exercised. Binding,
foreign-owner, unsafe-file, cancellation/timeout and charged-prefix guards are
bounded regression tests. Generic backend and citation/correction/navigation/
publication tests are retained.

The predecessor 2026-10-30 correspondence review recorded 70 assertion identities
without qualifying current-version failure observations and additional historical
derivation/isolation limitations. Those records remain historical and incomplete;
this correction does not retro-certify source hashes or claim full all-assertion
falsification, universal adequacy, semantic quality or provider compatibility.
Only actual new command/witness records support this working diff. Independent
parent review and checks are required before a corrective commit; no commit/push
or live provider/model execution is performed by this change.
