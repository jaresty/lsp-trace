# Provisional census-to-candidate workflow decision

- **Date:** 2026-10-10
- **Status:** provisional interface decision; implementation and exposure disabled
- **Authority:** 0
- **Accepted:** false
- **Completeness:** `UNKNOWN`
- **Feature identity:** `UNRESOLVED`
- **Provisional CLI spelling:** `lsp-trace census --catalog`
- **Revisit trigger:** retained LLM workflow evidence demonstrates material misrouting, ambiguity, or inability to complete or explain the candidate workflow

## Decision

Use the existing `census --catalog` shape as the provisional ordinary LLM-facing entry point for the eventual idempotent census-to-candidate workflow. Do not create a separate CLI redesign or require ordinary callers to orchestrate Program C directly before feature-inventory evidence exists.

The intended private composition is:

```text
accountable census
-> typed ADR 0011 relation admission
-> qualified grouping and policy composition
-> cross-seed instability and relation-scoped coupling
-> representative and source-context custody
-> ADR 0007 Describe
-> Search
-> provisional Group
-> Location
-> Source Text Search
-> immutable candidate generation
```

Program C remains a qualified computation and artifact boundary. Direct Program C commands may remain available for qualification, replay, diagnosis, and advanced inspection, but they are not the recommended ordinary route for an LLM asked to find feature candidates.

## Existing behavior boundary

Plain `lsp-trace census` remains unchanged. The existing `--catalog` mode and its currently qualified behavior are not redefined by this prose decision. Any implementation that extends the mode to terminal candidate generation requires a separately accepted versioned request/result contract, compatibility analysis, qualification, and execution/public-surface authority.

This decision does not choose whether the eventual implementation extends the current catalog contract in place or introduces a new version behind the same CLI spelling. Historical requests, artifacts, selectors, readers, bytes, defaults, and exit behavior remain unchanged unless a separately authorized migration says otherwise.

## LLM-facing result

The eventual terminal workflow result should make internal stages transparent while retaining enough identity and disclosure for inspection. It should expose, when available:

- exact census or capture-set selector;
- immutable candidate-generation selector and publication receipt;
- frozen workflow/request identity and predecessor identities;
- stage terminal outcomes and resumability;
- candidate count and omission/failure accounting;
- policy, privacy, resource, and qualification identities;
- exact next inspection or verification commands;
- `authority=0`;
- `accepted=false`;
- `completeness=UNKNOWN`;
- `featureIdentity=UNRESOLVED`.

A candidate generation is a review input. It is not an accepted feature inventory and cannot establish feature identity, completeness, ownership, purpose, runtime use, product value, or publication authority.

## Idempotency and resume

The workflow identity must bind the exact source/workspace revision, admitted source set, census policy, relation families, grouping and composition policies, seeds and instability policy, coupling policy, representative/source-context policy, ADR 0007 operation versions, privacy/resource policies, and every predecessor artifact identity.

Required behavior:

1. An identical request with an existing complete qualified generation verifies and returns that generation without duplicate semantic or publication effects.
2. An identical request with a resumable committed checkpoint continues only from the exact next admitted stage.
3. A same-identity byte, policy, receipt, or predecessor conflict fails closed.
4. Any identity-bearing input or policy change creates a distinct generation identity.
5. Concurrent equivalent requests converge through qualified create-only/CAS ownership and return one verified generation or a typed refusal; arrival order is not semantic order.
6. Missing qualification, unstable or incomplete mandatory evidence, failed accounting, or publication refusal produces no candidate generation.

These requirements reuse but do not weaken ADR 0011 continuation/custody rules, ADR 0007 candidate-publication custody, or the host-bridge concurrency gates.

## Skill and help routing

The embedded LLM skill should eventually route:

- “find or review feature candidates” to the census/catalog workflow;
- “inspect why this candidate exists” to retained inspection and source-context evidence;
- “replay or diagnose grouping” to advanced Program C operations.

The skill must disclose the candidate claim ceiling and must not teach direct Program C orchestration as the ordinary feature-candidate workflow unless retained usability evidence later requires it.

## Revisit policy

Revisit the spelling only after the end-to-end feature-inventory workflow exists privately and retained LLM-as-user evaluation shows material problems, such as:

- repeated selection of plain census when candidate generation was intended;
- interpretation of “catalog” as accepted feature identity;
- inability to discover, resume, inspect, or explain the workflow;
- substantially better success with another intent-level spelling;
- a compatibility conflict that cannot be resolved by versioning behind the current spelling.

Absence of implementation, preference for symmetry with MCP, or the existence of internal Program C operations is not evidence requiring a new CLI command.

## Authorization boundary

This decision authorizes documentation and qualification planning only. It does not authorize implementation, changes to current `census --catalog`, public CLI/MCP schemas, operation registration, automatic host execution, migration, candidate publication, feature acceptance, release, or deployment.
