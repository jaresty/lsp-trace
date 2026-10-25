# ADR 0012: Add a private Census-to-candidate orchestration bridge

- **Status:** Accepted additive bridge; private implementation and qualification only
- **Date:** 2026-10-25
- **Accepted:** 2026-10-25
- **Decision authority:** @jaresty (GitHub handle)
- **Decision owners:** LSP Trace maintainers
- **Accepted proposal SHA-256:** `1381d7d6f29049a23f5da8d54a432466bbd09aa0f27ba2833c3594ba0ed9c0ca`
- **Independent review receipt:** `20261025064133-6639` (`REVISE_BEFORE_ACCEPTANCE`; all blocking and clarification findings applied before acceptance)
- **Depends on:** [ADR 0007](0007-optional-local-semantic-feature-index.md), [ADR 0008](0008-dehydrated-structural-evidence-and-deterministic-source-projection.md), and [ADR 0011](0011-versioned-definition-reference-grouping-evidence.md)
- **Extends without modifying:** `docs/qualification/adr0011-adr0007-private-composition-contract.effe3f74.md`
- **Test-only predecessor:** `docs/qualification/adr0011-adr0007-private-composition-contract-v2.md`

## Context

ADR 0011 records the intended feature-discovery experience but explicitly leaves inventory planning, cache assessment, incremental orchestration transactions, consolidated results, and public CLI/MCP dispatch to a separately versioned bridge or follow-on ADR. The existing ADR 0011 → ADR 0007 private composition contract freezes the downstream stage semantics, while its v2 successor proves only a synthetic `CAUSAL_TEST_ONLY` world with `production_admissible=false`.

Repository qualification now supplies verified frozen Census terminal custody and an additive one-transition Program C continuation primitive. It does not yet supply a production-admissible end-to-end composition, representative nomination denominator, complete representative lineage, complete retained source closure, `FULL_DEFINITION` bindings, caller-occurrence V3 custody, stakeholder correction authority, or feature-acceptance authority.

A private orchestrator is needed to resume verified custody, advance qualified work without caller-authored internal state, and stop truthfully at the first missing authority or evidence boundary. Modifying historical Census behavior or treating the synthetic v2 world as production evidence would violate the predecessor contracts.

The accepted ADR 0007 caller-provided-inference direction remains controlling. The existing host LLM recognizes user intent and invokes bounded operations; this bridge is an optional deterministic private execution kernel beneath that caller-driven workflow. It does not replace host interpretation, introduce an autonomous semantic service, or qualify an isolated Describe worker.

## Decision

Add a transport-neutral, private Census-to-candidate successor workflow. It begins only from verified frozen Census terminal custody, reuses the qualified one-transition Program C continuation, and advances through the existing private ADR 0011 → ADR 0007 stage contracts only when each stage's exact predecessor and qualification gates pass.

The intended complete chain remains:

```text
verified frozen Census terminal custody
→ Program C and instability per Native V5 constituent
→ Describe preparation
→ Describe
→ Search
→ provisional Group
→ optional Location
→ optional Source Text Search
→ mechanical admission
→ immutable provisional candidates
→ reversible grouping hypotheses
→ independent semantic challenge
→ stakeholder correction
→ optional explicit acceptance
```

The workflow may stop before any later stage. A truthful stop is a terminal workflow result, not an orchestration failure and not evidence that omitted stages are complete.

## First authorized profile

The first profile is:

```text
CALLS_ONLY_FROZEN_CENSUS_V1
```

It is limited to exact verified frozen Census custody and independently qualified CALLS-family artifacts. It does not admit definition/reference-only or mixed grouping, does not alter omitted-selector `CALLS_ONLY` behavior, and does not imply a repository-wide aggregate graph.

Definition-only, reference-only, no-CALLS, and mixed profiles remain unavailable until their own ADR 0011 acquisition, admission, grouping-policy, composition, replay, and qualification gates pass. Program C remains per Native V5 constituent. The workflow cannot compose capture sets, reacquire custody, synthesize cross-partition `CALLS`, compare constituents to infer absent edges, or relabel communities as feature candidates.

## Workflow state and transaction boundary

A workflow identity binds:

- exact frozen Census receipt, availability ledger, source pin, plan, and their verified bytes;
- profile and workflow-contract identities;
- every admitted predecessor artifact and policy identity;
- privacy, retention, resource, replay, and qualification identities;
- the immutable successor-head lineage.

The durable head records the workflow identity, current stage, exact committed-prefix or stage result, predecessor head, cumulative accounting, and one of these workflow outcomes:

```text
ADVANCED
COMPLETE_PROVISIONAL
WAITING_EVIDENCE
WAITING_EXTERNAL
RETAINED_FAILURE
CONFLICT
INDETERMINATE_EXECUTION
CANCELLED
```

Every invocation performs at most one durable transition. A private top-level runner may repeatedly invoke that primitive, but between transitions it must close, reopen, and verify durable state. It cannot retain private terminal custody across transitions as public result bytes.

```text
top-level private run
  → one transition invocation
  → durable commit or typed stop
  → close/reopen and verify
  → next transition invocation
```

The coordinator selects only the next state implied by the verified durable head. Callers cannot supply ordinals, selectors, processed sets, raw plans, CAS heads, retry or skip policy, semantic outcomes, correction decisions, or acceptance decisions.

## Program C progression

For `PROGRAM_C`, the coordinator selects `plan[committed_prefix]`. Only valid `COMPLETE`/`COMPLETE_NONEMPTY` and `EMPTY`/`SUCCESSFUL_EMPTY` results advance the committed prefix. `EMPTY` is structurally valid but nominates no candidate or representative.

A retained terminal failure does not advance the prefix and is never retried unchanged. Unreceipted `CLAIMED` state or an unverifiable write-ahead claim yields `INDETERMINATE_EXECUTION`; it cannot be replayed, skipped, replaced, or treated as failure. `SessionPoisoned` and `LifecycleConflict` cannot trigger restart, replacement, retry, substitution, or unsafe continuation.

Manager/sessionruntime remains terminal lifecycle owner. Valid JSON-RPC request errors remain non-poisoning while Manager is `READY`; malformed post-write protocol responses poison Manager. Completion sequence remains Manager-owned, unique, monotonic, and non-wrapping.

## Downstream stage gates

The private ADR 0011 → ADR 0007 composition contract remains the semantic baseline for Describe, Search, Group, Location, Source Text Search, candidate generation, accounting, replay, and claim ceilings. This ADR does not rewrite its v1 bytes or promote its v2 test fixture.

A stage advances only from retained matching bytes and qualified lineage. Missing, malformed, stale, mixed, synthetic-as-authentic, privacy-invalid, incoherent, sequence-colliding, sequence-exhausted, nondeterministic, or unverifiable evidence fails closed.

Describe preparation cannot use checkout fallback, LSP reacquisition, mutable selectors, historical Describe output, invented nomination denominators, or community-as-candidate relabeling. Until representative nomination denominator, community-to-representative lineage, retained source closure, `FULL_DEFINITION` display bindings, and caller-occurrence V3 custody are qualified, the workflow returns `WAITING_EVIDENCE` before Describe packet construction.

Historical provisional candidates, grouping hypotheses, Describe packets, caller results, and admission results are design evidence only. They cannot seed a new generation.

## Structural, semantic, and authority separation

Structural computation, semantic generation, independent challenge, stakeholder correction, and acceptance are separate authority layers.

Program C, Leiden, instability, adjacency, centrality, matching names, capture-set co-membership, structural `STABLE`/`UNSTABLE`, Search rank, or Group membership cannot establish feature identity, purpose, ownership, product value, runtime use, completeness, acceptance, or production admission.

Every generated request, result, checkpoint, candidate, group, challenge, and correction retains:

```text
authority=0
accepted=false
featureIdentity=UNRESOLVED
completeness=UNKNOWN
effects=NONE
implementationDecision=UNISSUED
production_admission=REFUSED_MISSING_EVIDENCE
source_graph_complete=UNKNOWN
absence_inference=PROHIBITED
```

Independent challenge requires a separately identified review relationship and immutable evidence packet. A different process, model, or person alone does not establish independence.

Stakeholder correction is external to automatic stage execution. A correction creates an immutable successor revision with finite typed deltas and never mutates predecessors. If required correction has not been supplied, the workflow returns `WAITING_EXTERNAL`.

`COMPLETE_PROVISIONAL` means the provisional generation is mechanically complete under its exact bounded inputs. It does not change `accepted=false`, `featureIdentity=UNRESOLVED`, or `completeness=UNKNOWN`.

Optional explicit acceptance requires a separate authenticated external contract identifying authority, scope, accepted proposition types, unresolved handling, coverage ceiling, and reopening procedure. This ADR does not define or authorize that acceptance transition.

## Result contract

A canonical private workflow result discloses:

- workflow, profile, contract, head, and predecessor identities;
- reused and freshly computed immutable artifact identities;
- current and last-completed stages;
- exact outcome and typed stop reason;
- denominators, evaluated counts, terminal counts, omissions, and untouched ambiguity;
- reached limits and unavailable, stale, withheld, failed, or indeterminate evidence;
- candidate and grouping counts only when actually computed;
- exact resumability or external-evidence requirement;
- unchanged authority and completeness ceilings.

`FAILED`, `PARTIAL`, `REFUSED`, `UNKNOWN`, bounded zero, successful-empty, and untouched ambiguity remain distinct. Exit `0` or a mechanically complete result is not semantic success.

Private terminal custody, raw source bytes, prompts, hidden diagnostics, CAS heads, and internal scheduling state do not serialize into a public completion result.

## CLI and MCP boundary

One transport-neutral workflow owns sequencing, durable state, and result construction. Future CLI and MCP adapters must call that same workflow and cannot implement stage selection, retries, skips, correction, acceptance, or head management.

The provisional ordinary CLI intent remains:

```text
lsp-trace census --catalog
```

An equivalent future MCP operation must expose the same canonical request and result semantics. Neither spelling is enabled or registered by this ADR. Existing Census commands, MCP operations, schemas, defaults, selectors, readers, result bytes, help, completion, skills, advertisements, capability discovery, and exit behavior remain unchanged.

## Compatibility and implementation authorization

The frozen v1 composition contract remains unchanged, including its original write-set and implementation gate. This accepted ADR is the additive successor authorization for the bridge-only namespaces and behaviors listed below; it does not retroactively broaden v1 or qualify v2.

This ADR authorizes private implementation and qualification of:

1. the repeated Program C driver over the existing one-transition primitive;
2. a successor workflow head and one-transition coordinator;
3. transport-neutral private result construction;
4. fail-closed downstream stage gates that may return `WAITING_EVIDENCE` or `WAITING_EXTERNAL` without fabricating artifacts;
5. private replay, conflict, cancellation, indeterminate-execution, retained-failure, and completed-state no-op tests.

Implementation must be additive and use separate successor namespaces. It cannot modify frozen Census behavior or the immutable v1/v2 composition artifacts. Every implementation phase requires assertion-specific RED evidence, repository-wide compatibility checks appropriate to the changed packages, independent review, and exact-head qualification.

This ADR does not authorize:

- public CLI or MCP exposure or operation registration;
- definition/reference or mixed-profile enablement;
- Describe packet construction before its lineage gate passes;
- historical artifact migration or reuse as current evidence;
- candidate, group, challenge, correction, or acceptance fabrication;
- main merge, release, deployment, production admission, or public acceptance.

Public adapters require a separate accepted enablement decision after private end-to-end qualification. Explicit feature acceptance requires the separate external authority contract described above.

## Required qualification

Before any public-enablement proposal, qualification must demonstrate:

- exact ordinal progression across at least three Program C records with close/reopen between transitions, or a truthful retained failure at the third record without replay;
- no republication of committed work;
- cancellation, CAS conflict, lifecycle conflict, poisoned Manager, retained failure, unreceipted claim, receipt-linked recovery, and completed-state no-op behavior;
- byte-stable workflow identity, head lineage, result accounting, and workspace-free replay;
- unchanged historical Census and private composition bytes, defaults, selectors, readers, and exit behavior;
- identical transport-neutral semantics under private CLI- and MCP-shaped adapter tests;
- fail-closed `WAITING_EVIDENCE` at the first absent downstream qualification;
- no elevation of authority, acceptance, feature identity, completeness, production admission, or absence inference.

Passing these checks qualifies only the private profile and implementation claims actually tested. It does not authorize public exposure or later semantic stages whose evidence remains absent.

## Consequences

### Positive

- One private trigger can make bounded progress without caller-authored orchestration state.
- Existing Census and composition contracts remain immutable.
- The workflow can stop honestly at missing evidence rather than manufacturing continuity.
- CLI and MCP can eventually share one canonical engine.
- Provisional completion remains distinct from stakeholder correction and acceptance.

### Costs and risks

- Durable one-transition orchestration and reopen verification add state-machine complexity.
- The first profile is intentionally narrower than ADR 0011's eventual relation-family goal.
- The real workflow remains blocked before Describe until its retained lineage is qualified.
- Public usability remains deferred until a separate enablement decision.

## Alternatives considered

### Modify legacy Census orchestration

Rejected. It risks changing historical bytes, defaults, selectors, lifecycle behavior, and public compatibility.

### Promote the v2 synthetic composition world

Rejected. It is explicitly `CAUSAL_TEST_ONLY` and `production_admissible=false`.

### Let CLI and MCP own loops independently

Rejected. It would duplicate sequencing authority and permit transport drift.

### Run all stages in one durable transition

Rejected. It breaks one-transition atomicity, complicates custody recovery, and can republish completed work.

### Reuse historical candidates and groups

Rejected. Their bounded provenance cannot establish the successor generation's custody or denominator.

### Require acceptance for provisional completion

Rejected. Mechanical completion and external semantic acceptance are different authority layers.

## Acceptance record and follow-up

On 2026-10-25, @jaresty explicitly accepted the independently reviewed proposal whose SHA-256 is recorded in this ADR header. The review disposition was `REVISE_BEFORE_ACCEPTANCE`; its blocking status/authorization finding and both required clarifications were applied before acceptance. This acceptance authorizes only the private implementation and qualification scope stated here. It does not authorize public enablement, Describe construction without qualified lineage, production admission, release, deployment, or semantic feature acceptance.

1. Implement and qualify the smallest private repeated Program C driver.
2. Prove ordinal `0 → 1`, `1 → 2`, and `2 → 3` durable progression or retain a truthful ordinal-2 stop.
3. Implement the successor head and one-transition coordinator.
4. Produce a read-only Describe-preparation gap ledger from retained matching bytes.
5. Do not construct Describe packets or public adapters until their independent gates pass.
