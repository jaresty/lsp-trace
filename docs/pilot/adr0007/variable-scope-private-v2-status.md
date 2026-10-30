# Flexible private pipeline: status and sequence

## What “V2” means

V2 is the additive private extension that removes the demonstration's fixed four
nomination files, two context files, and 25 targets. It is not a new public API,
a replacement for historical evidence, or an accepted product feature catalog.

The earlier real-25 run remains a separate completed milestone: reviewed technical
interpretations and 25 provisional leads with verified paired publication. No V1
carrier is rewritten as V2 and no historical exact-25 qualification is widened.

## Connected sequence

The following path is implemented and exercised using owned source fixtures,
real Manager leases, and fake pipe providers:

1. Declare explicit finite source members, roles, hashes, lengths and limits.
2. Freeze source bytes and identities with no-follow reads.
3. Prepare and bind the exact Manager's source supply under an active lease.
4. Discover nominations, derive batches, and spend each batch's original authority
   once. Context files are not nominations.
5. Acquire and capture bounded private evidence. Preserve original native bytes,
   process metadata, direction, duplicate occurrences, and missing-source facts.
6. Retain a self-contained packet covering every batch in two verified copies.
7. Independently reopen and qualify that packet; mint the distinct derived seal.
8. Project and compute each batch separately, retaining every requested seed run
   and comparison. Do not merge graphs or interpret instability as confidence.
9. Prepare bounded own-source AST interpretation packets from sealed evidence.
10. Account prospective caller submissions and reviews against exact packet pins.
11. Publish exact preparation and internally derived ledger in two verified copies.

Interpretation is deliberately a caller-controlled step between preparation and
publication. Mechanical citation, quote and review-pin checks do not establish
semantic correctness or authenticated reviewer independence.

## Implementation milestones

| Commit | Checkpoint |
|---|---|
| `3bc8f29` | Restrict-only variable query membership |
| `502cdcb` | Versioned Manager equality guard |
| `9225d10` | Private variable-scope source capture and parser |
| `151730b`, `cee2762` | Frozen scope, actual discovery, single-spend producer and lease/deadline regressions |
| `43d2823` | Complete-origin paired retention, qualification and derived seal |
| `cb776f6` | Per-batch computation and sealed instability assessment |
| `d6e44dc` | Preparation, caller accounting and final paired output |
| `07de8de` | Constructor-owned validated snapshot reuse; unchanged aggregate race gate passes |

These commits are local; no push is implied.

## Evidence and limits

Owned connected cases include 1/0, 2/1 and 5/2 nomination/context scopes,
multiple batches, matched declarations, independent origins, explicit source
and resource gaps, and paired output verification. A normal 72-target aggregate
case exercises over-64 preparation accounting without dropping index entries.

Parent verification for the final connection passed 348 preparation/publication
cases and 19 owned normal cases. Separately selected connected, refusal and
multi-origin race cases passed. The original aggregate race case exceeded 240
seconds, and an earlier all-V2 race invocation exceeded 360 seconds. These failed
attempts remain history. After profiling and constructor-owned snapshot reuse,
the unchanged aggregate passed `-race -count=1 -timeout=240s` in writer and parent
verification; the writer measured 171.87 seconds. The normal profiled workload
fell from 45.73 to 14.84 seconds. This resolves that named race gate, not the
entire all-V2 suite or general scalability. Repository-wide CI, exhaustive
mutation/interleaving adequacy and Linux runtime qualification remain unestablished.

Limits remain finite private policies, not comprehensive heap/CPU meters.
Source artifacts have a 4 MiB ceiling; producer packets and final publication
have distinct 16 MiB storage profiles. Separate work/encoding budgets may refuse
smaller objects. No automatic expansion, acquisition replay or publication retry
is authorized. Borrowed roots require nonconcurrent use and explicit Close.
Paired integrity is not atomic pair publication, attacker-proof custody,
continuous monitoring or a power-loss guarantee.

## What is not completed

- A new real varied-source execution through this flexible path.
- Actual semantic interpretation and domain/stakeholder adjudication for new
  workloads. Owned caller claims exercise mechanics, not real feature discovery.
- Authenticated review independence, accepted feature identity, public admission,
  production enablement, or repository-scale qualification.
- General revision/merge/split lifecycle and expiration/deletion interfaces.

Every stage retains authority=0, accepted=false, feature identity UNRESOLVED,
source completeness UNKNOWN, and absence inference PROHIBITED. AST association
is not native full-definition or MethodResult custody.

## Detailed contracts and executable examples

- [Producer and custody](../../../internal/censuslineagepilot/PRIVATE_V2_PRODUCER.md)
- [Complete packet retention and qualification](../../../internal/censuslineagepilot/producer_packet_private_v2.md)
- [Preparation, accounting and final publication](../../../internal/privatedescribeprep/PRIVATE_V2.md)
- [Connected owned example](../../../internal/censuslineagepilot/postpreparation_connected_private_v2_test.go)
- [Independent origins](../../../internal/censuslineagepilot/postpreparation_multiorigin_private_v2_test.go)

The current continuation worktree is separate from the earlier temporary
worktree whose unexplained tracked deletions were preserved untouched. Their
actor and intent remain unknown; creating the continuation did not restore or
overwrite those changes.

## Current contract and continuation — 2026-10-30

**Current edge: automate the demonstrated host workflow, not replace its architecture.**
This section is the current continuation at code HEAD
`8a772025f2e6b708e4dfaf047c850519f8c5be19`. Earlier checkpoints above and in linked
implementation notes are historical observations, not retroactive qualification
of this version. This documentation adds no execution permission.

The existing caller/host owns inference. lsp-trace prepares evidence, validates
returned typed data, and retains results. **Host independence is explicitly
decided:** ordinary tools, immutable artifacts and typed receipts are the chosen
free workflow; no required Pi extension, SDK, session API or runtime-specific
controller. Pi may be a development/execution harness, never a product dependency.
No HTTP model backend, new inference account/credential, lsp-trace-owned model or
harness invocation, service, or public CLI/MCP surface is selected.

Every result retains **authority=0, accepted=false, feature identity UNRESOLVED,
source completeness UNKNOWN, absence inference PROHIBITED**. V2 AST is not
qualified Describe, FULL_DEFINITION, MethodResult, ordinary Native or workflow
admission. No historical qualification transfers across operations/input families.
Stored evidence may support read-only semantics; it cannot restore acquisition
authority. Do not decode bytes into a sealed owner or recover a spent attempt.

Governing decisions: [caller-provided inference amendment](../../adr/0007-caller-provided-inference-amendment.md),
[normative consolidation](decisions/2026-10-08-normative-consolidation.md), and the
[separately scoped broader bridge](decisions/2026-10-08-host-llm-bridge-contract.md),
which remains execution-disabled. The development
[feature-inventory skill](../../../.pi/skills/lsp-trace-feature-inventory/SKILL.md)
and its preparation/adjudication references constrain provisional identity,
review, correction and acceptance; their location imposes no Pi dependency.

### Evidence register: read facts, not proposed success

Local archive aliases below identify immutable evidence, not new truth documents:

- **P**: `/Users/schwa/Library/Application Support/lsp-trace-postpublication-real-v1-2m9cp5a5/`.
- **S**: `/Users/schwa/Library/Application Support/lsp-trace-private-target-interpretation-n4wbs2pu/`.
- **G**: `/Users/schwa/Library/Application Support/lsp-trace-v2-interpretation-wh_5bnj9/`.
- **F**: `/Users/schwa/.local/state/lsp-trace-evidence/host-real-plan-4mjpjl6h/evidence/`.

| Evidence | Demonstrated result | Boundary / current disposition |
|---|---|---|
| Successful six-file POC, code `b483f21aceb7df866c8d4dfdc81ce8c3676dc81d`; P `reports/delivery-v2.json`, `inventory-v2.md`, `operation-v1.json`, `independent-verification-v1.json` | 4 nomination + 2 context files; 219 symbols; 25 selected targets and independently reviewed provisional leads; 6 navigation hypotheses, 2 ungrouped helpers, 3 unresolved capability questions. Native preparation/ledger pair plus reviewed human surface. | Completed bounded success. Two copies each 4,114,463 bytes, `sha256:8d000fa752fef89e6f97065a3a85bbd45d289469fe54823595231c42f239757e`; parent separately rehashed both with distinct inodes. Historical source-before agreement concerns its fixed source/workload, not today's implementation. Its remaining work explicitly included durable automatic orchestration, varied scopes and scale. |
| Generalized manual V2; G `INVENTORY.md` | Actual one-file/13-target operation, 27 own-AST claims, independent semantic review with limits, 6 navigation arrangements. | Successful partial generalization, not erased by later failures. Companion semantic archive was external/manual, not a native post-ledger update; candidate/AST/native namespaces are not equivalent to real-25. |
| Current V2 implementation; `AUTOMATIC_PRIVATE_V2.md` and host handoff runbook | Actual private worker, local file adapter, generic interpretation/review/correction/navigation and paired native `Inventory` / `HumanInventory`; separate owned emulator/worker/supervisor processes exercised one/three batches. | Owned fixtures establish bounded mechanics, not actual host-model role success or semantic qualification. Earlier 70-assertion records remain historical/incomplete; this continuation is not a replacement adequacy campaign. |
| Latest real trial; F `RESULT.md`, `parent-outcome.json` | `identity_policy.go`: 24 discovered symbols / 2 targets, one batch, 371,594-byte acquisition artifact, 2 prepared / 0 gaps. Stop/Shutdown/terminal Operation once; shutdown matched, resources closed, exited/reaped. | `FAILED_SPENT`: first INTERPRET frame 229,327 bytes expired at 60 seconds after truncated terminal display; **0 actual host-model roles, 0 responses, no inventory**. Joined=true, ExitCode=1, Stopped=false. Host execution failed; model incompatibility and need for a new controller were not established. Never replay or answer this mailbox. |
| Qualified Describe; amendment and consolidation above | Bounded DECLARATION-focused 24-case corpus, 22/24 replay agreement; all 96 attempts accounted. | Exact Describe successor only, not general algorithm explanations or this complete inventory workload. Search/Group/Location/Source Text Search retain their separate qualified scopes. No blanket qualification or disqualification of V2 follows. |

The evidence for this map is static artifacts and inspected source, not a new
server-reported CALLS traversal, fresh semantic rereview, or live execution.

### Stage compatibility: six-file POC to current private V2

The original nomination set and all 25 targets remain fixed historical evidence.
Its four nomination paths are `cmd/lsp-trace/filter_command.go`,
`cmd/lsp-trace/inspect_command.go`, `internal/filter/filter.go`, and
`internal/inspection/projection.go` (exact target coordinates are retained in
`retained_partial_exact25_private.go`). The two context-only supplies are not
nominations. Do not turn those counts, paths or coordinates into V2 defaults.
The following table identifies observed host coordination, not an assertion that
the end user manually performed every step.

| Stage | Actual POC input → operation → output/receipt | Reusable current V2 / exact difference | Fixed assumptions and demonstrated coordination |
|---|---|---|---|
| Scope and census | Fixed six-member source policy and original discovery → retained bounded capture → 219 symbols / 25 callable targets / 194 exclusions (P inventory census section; S `provisional-candidates-v1/nomination-census-coverage-v2.json`). | `DiscoverScopePrivateV2`, `AcquireScopeBatchPrivateV2` in current caller consume frozen finite members/roles and actual discovery; batches preserve one origin, context is not nomination. | POC exact-25 coordinates/policy are not a generic scope API. Current finite scope/batch accounting already removes fixed 4/2/25 cardinality. Formal historical nomination independence remains UNRESOLVED, not fixed by this map. |
| Retained evidence and computation | P `inputs/config.json` pins original three roots, partial pair, qualification pins and instability pair → `TestReal25PrivatePostPublication` at `b483f21` runs `NewExactRetainedPartialQualificationInput`, `QualifyRetainedPartialNativePrivate`, closes five borrowed roots, derives/projects and explicitly computes seeds 42..47 → full retained-assessment equality. P operation phases and invocation receipt record offline success, acquisition replay NOT_ATTEMPTED. | Current caller: `RetainCompleteScopePrivateV2` → `QualifyProducerPacketPrivateV2` → `DeriveProducerPacketPrivateV2` → per-batch `ProjectPrivateV2` / `InstabilityComputePrivateV2`. Distinct producer/profile/policy/batch/origin pins; no graph merging or seed cherry-picking. | POC host supplied exact paired artifacts and config; old private qualification admitted that exact retained family. This is evidence that acquisition need not stay live for its read-only continuation, **not** evidence that arbitrary saved V2 bytes can reconstruct current owners. |
| Preparation and opaque ownership | Sealed POC projection/assessment → `NewPostPreparation` with original limits → exact preparation `sha256:5d6af15eb96c4c33a00b8aea70705dba85f34d1c42ad040e5abb319db4c26f5a` (3,166,174 bytes), checked equal to original. | `NewPostPreparationPrivateV2` accepts actual `ProjectionPrivateV2` / `AssessmentPrivateV2` handles, not V1/JSON DTOs. V2 namespace binds packet, qualification, batch, original V3, derived V5, policy/profile and assessment. `NewAutomaticHandoffPrivateV2` binds exact `h.state`, scope and fresh nonce. | Both constructors own validated state. Old POC requalification and current live original-worker lifetime are different contracts; neither proves a permanent worker is inherently required. Current adapter requires original in-memory preparation owner and open roots during this invocation. Any alternative retained-owner admission needs explicit compatibility investigation/decision, never bytes-to-seal reconstruction. |
| Interpreter delivery | S `item-10/assignment.json` binds target, packet, input, original preparation and ordered parts; nine `evidence-*.txt` chunks (eight 20,000-byte chunks + 62 bytes) → own-packet interpretation → `producer.json`, exact producer digest and mechanical-check receipt. | `AutomaticRequestPrivateV2` carries own Bodies, Packet, PostLimits and correction context; `AutomaticInterpretationPrivateV2` uses exact UTF-8 offsets/quotes. `PRIVATE_V2_AST_LOCAL_HANDOFF.v1` frames bind invocation/nonce/sequence/stage/role/request/preparation/input/target/deadline/allowance. | POC evidence chunking and separate producer artifact show host-side packet delivery/collection. Current Python `request` validates a file-backed frame but `main` prints the full JSON; printing it is not complete model delivery. User copying is unnecessary and prohibited as the intended workflow. |
| Technical review and corrections | S `item-10/review.json` records reading all nine chunks and challenges attribution, opaque helpers, partial directions and source limits; `producer.mechanical-check.json` binds 5,066 producer bytes with no issues. Retained batch/item revisions preserve failed/wrong-pin/oversized outputs. | `AutomatePrivateV2` requests REVIEW_TECHNICAL against exact RevisionDigest with complete selected own packet/bodies, not only chosen citations; `AutomaticReviewPrivateV2` records APPROVE/REJECT/ABSTAIN. Semantic corrections alone are bounded and predecessor-linked. Transport errors remain terminal. | POC role assignment, output collection and revision selection were host-coordinated. Current worker already owns stage sequencing and bounded correction; host must dispatch actual isolated roles and capture exact bytes. Distinct tags remain assertions, not authenticated independence. No all-25 semantic rereview was performed for this map. |
| Candidates and navigation | S pinned `candidates.a803c5ec633d58ff03bd0db70e41972f90195fb9abe86b02eeb3eb92c6b02339.json` → grouping assignment with 32 candidate chunks → reviewed `grouping-v4.json`, finite `grouping-v4.correction.json`, `grouping-review-v3.json`; 25 IDs unchanged, 23 assignments + 2 ungrouped. | `private-provisional-inventory.v2` uses origin-based stable candidates, NAVIGATE / REVIEW_NAVIGATION, withdrawal conditions and retained revisions. Namespaces remain separate: candidate IDs, native node/relation IDs, packet IDs, handles, content digests and storage selectors are not interchangeable. | POC grouping wording correction changed two propositions, no candidate/assignment or accepted identity; later independent recheck is separate from the correction receipt's historical PENDING field. Keep questions, ungrouped items, alternatives and unresolved acceptance visible. |
| Typed ledger and admission | P fixed candidate digest + 50 producer/review records on both bare-hex CAS roots → historical `realPostConvert` / `CALLER_CONVERSION_V1`, `Reconcile` submission digest, imported review findings → 25-entry Crosswalk and ledger, 0 conversion rejections. | Current automation emits exact typed results prospectively; `automaticLegacyClaims` preserves long quotes via lossless 240-byte ledger chunks; `RetainAutomaticPrivateV2` requires the same opaque owner. V1 conversion is an exact historical importer, not a general V2 admission path. | POC driver has hard-coded pins/counts, string/object review ceiling compatibility and exact caller-import attribution. Do not import/coerce old ad-hoc JSON, reuse old admission bits, or normalize role output into validity. Mechanical citation/review binding is not semantic entailment. |
| Retention and human finish | `RetainPostPreparationPrivate` → P native exact preparation/ledger pair and per-copy receipts; reviewed human `inventory-v2.md` remains a separate surface (delivery revision 2 links it). | Current caller `variableRunWithAutomaticPinned` → `AutomatePrivateV2` → `RetainAutomaticPrivateV2` → exact reopen of both copies → `FinishAutomaticHandoffPrivateV2`; final native envelope already includes machine `Inventory` and `HumanInventory`. | POC human-view assembly and G's external companion were host work. Do not replicate parent artifact assembly: current pair is the native successor. Verified pair alone is insufficient; strict error-free final report/close plus Joined=true, ExitCode=0, Stopped=false must finish the whole caller. |

Source contracts: [V2 preparation/retention](../../../internal/privatedescribeprep/PRIVATE_V2.md),
[V1 post-preparation](../../../internal/privatedescribeprep/POST_PREPARATION_PRIVATE.md),
[V1 retained producer](../../../internal/censuslineagepilot/RETAINED_PARTIAL_NATIVE_PRIVATE.md),
[current automation](../../../internal/privatedescribeprep/AUTOMATIC_PRIVATE_V2.md),
[current handoff runbook](private-variable-v2-host-handoff.md).
The current source seams are
`internal/censuslineagepilot/variable_scope_real_v2_caller_test.go`,
`internal/privatedescribeprep/automatic_handoff_private_v2.go`,
`internal/privatedescribeprep/automatic_private_v2.go`, and
`scripts/private-variable-v2-host-handoff.py` (`request`, `respond`, `main`,
`supervise`). These are static implementation observations, not runtime proof.

### Single next checkpoint — portable file-backed HOST completion

**Proposed, not implemented or authorized to run here:** close the already
identified request-delivery/role-dispatch edge through the existing helper and
runbook. One bounded user request must reach a useful reviewed provisional
inventory with its native paired human/machine delivery; the user must not copy
payloads, calculate hashes, coordinate roles or assemble final artifacts.

Presumed: improve the existing helper/runbook and ordinary host tool recipe,
rather than introduce a framework or runtime controller. Revised if: a bounded
compatibility review/rehearsal demonstrates that the existing caller cannot
consume complete file-backed requests within its configured limits. Any required
material schema, ownership/admission, deadline or budget change is deferred for
parent decision; it is not an implicit fallback or bypass. The cross-component
interaction between retained-owner admission and restart/resumption is deferred;
this checkpoint uses the existing original-owner lifetime, not recovered authority.
Withdrawn by: remove that optional compatibility proposal without changing the
historical artifacts or the working original-owner contract.

Implementation handoff, in dependency order:

1. **Bind the portable host recipe.** Use the existing reviewed configuration,
   source/code/binary pins, finite limits, separate fresh roots and one supervisor
   invocation only when separately authorized. Prefer runbook improvements and
   private helper affordances for full bounded file consumption, rather than
   terminal payload display. Any helper change prepares/verifies artifacts only;
   it must not invoke a model/harness. No new public CLI/MCP operation is proposed.
2. **Complete host-owned delivery and exact return.** Route INTERPRET/NAVIGATE to
   the existing interpreter and REVIEW_TECHNICAL/REVIEW_NAVIGATION to an isolated
   reviewer through the host's ordinary capabilities. Feed complete pinned files
   (or lossless ordered bounded chunks with completion accounting), never a
   clipped terminal dump or summary. Capture the role's exact output to a private
   payload file and use existing `respond` framing/digest computation. Continue
   the worker-directed sequence without user intervention. No required Pi API,
   custom controller, second backend or new credential; no output repair,
   whitespace/newline normalization or hand-built response envelope.
3. **Finish through existing native retention.** Let `AutomatePrivateV2` own
   corrections/reconciliation/navigation and `RetainAutomaticPrivateV2` own the
   pair. Present its native human inventory and audit links alongside machine
   identity and full terminal accounting. Do not synthesize a parent companion
   archive as a substitute for native completion.

The following observable gates define this one checkpoint; fixture correctness
and actual host success must be reported separately:

| Gate | Required observation before claiming completion |
|---|---|
| Full delivery | File-backed consumption at the observed **229,327-byte shape** and at the **configured request ceiling**, with exact byte length/digest and all chunks accounted. Diagnostic inspection may use inert copies only, without responding to the spent mailbox or recreating authority. No terminal display limit may silently narrow role evidence. |
| Actual roles / exact capture | Attributable actual existing-host role invocation, complete-input observation, and exact output bytes/pins for each attempted stage. Owned emulator success is explicitly labeled emulator; it does not satisfy this gate. No lsp-trace inference invocation. |
| Review usefulness | Reviewer receives the complete own selected packet, source bodies, revision, limits and proposed output. A supported description explains meaningful visible algorithm/control/data behavior; otherwise retain explicit unsupported/abstained/rejected/unresolved disposition. Mechanical validity alone cannot make a vacuous description useful. |
| Finite corrections | Only configured semantic correction attempts, immutable predecessor/digest/feedback linkage and exact failed outputs. Frame errors, deadline, cancellation and missing/late responses terminate; no transport retry, provider fallback, hidden scope increase or semantic substitution. |
| Source accounting | Every selected target has an outcome: requested = prepared + source gaps + resource gaps; prepared submissions/reviews/dispositions reconcile. Preserve own attribution, direction, duplicates, unavailable/withheld source, origin/batch counts and all declared runs. No absence or confidence inference. |
| Custody and lifetime | Original preparation owner, nonce, invocation and open protected/output-root capabilities remain bound; no JSON-to-owner constructor or spent authority recovery. Source/provider cleanup and caller/supervisor completion remain separate observations. Retained-owner compatibility changes stop for explicit review. |
| Positive user-visible finish | Both native copies verify identical machine `Inventory` and `HumanInventory`; strict terminal verified-pair receipt and error-free final report/close agree with supervisor **Joined=true, ExitCode=0, Stopped=false**. Display the useful provisional inventory with review limits and unresolved questions. Pair-only status, emulator-only completion or joined nonzero failure is not success. |

Existing exchange/overall/operation deadlines remain unchanged, including the
failed trial's 60-second exchange bound. Measure the complete file-to-role-to-file
path before proposing a separately reviewed change; do not extend it by assumption.
No new live run, preflight, provider/model call, spent response, acquisition replay
or acceptance campaign is authorized by this handoff. Passive delivery inspection
cannot complete F's failed invocation.

### Continuity controls and unresolved decisions

Carry this compact packet into every later assignment and review:

- **Decisions:** caller-owned inference; host-neutral ordinary tools/artifacts/
  receipts; broader bridge disabled; no public surface or second backend.
- **Successful baseline:** P's completed 4+2 / 219 / 25 reviewed provisional leads,
  6 navigation hypotheses, 2 ungrouped helpers and 3 unresolved questions; G's
  successful manual 13-target generalization remains separate evidence.
- **Exact family:** current v4 caller / `PRIVATE_VARIABLE_SCOPE_V2_AUTOMATIC_PROVISIONAL`,
  `PRIVATE_V2_AST_LOCAL_HANDOFF.v1` transport and `private-provisional-inventory.v2`
  semantic protocol; not V1 importer or qualified Describe/Native/workflow.
- **Qualification scope:** retain the named Describe 24/96 threshold and each
  deterministic stage's own scope; no cross-family transfer or fresh general
  qualification prerequisite for this mapping.
- **Current edge / permitted delta:** full file-backed host-role delivery and exact
  capture using existing seams; no acquisition repair, owner bypass or architecture
  replacement. F is permanently FAILED_SPENT, not a pending stage to resume.
- **Finish:** useful reviewed provisional native human/machine pair plus strict
  joined-zero-not-stopped completion, without manual user transport or assembly.

Review **architecture/outcome separately from test correctness**: passing helper,
worker and emulator checks cannot establish actual host invocation or a useful
semantic inventory. Conversely, F's host failure does not invalidate P's success
or prove a model/backend defect. Preserve predecessor checkpoints and link later
measured results from this section rather than creating competing status documents.

Still unresolved: exact retained V2 ownership compatibility outside the current
original-worker lifetime; actual host-role throughput under unchanged deadlines;
varied real scopes/scale beyond the cited evidence; authenticated independence;
product/domain meaning, granularity and stakeholder acceptance. None may be
presumed from source, grouping, publication or test counts. The technical finish
may be PROVISIONAL_WITH_UNRESOLVED; accepted feature identity still requires an
explicit external acceptance contract and attributable correction lineage.
