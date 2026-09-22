# ADR 0007 governance authorization proposal

## Status

`APPROVED — LOCAL OPERATIONAL PILOT ONLY`

This document records the project owner's approval for repeatable local operational execution of the ADR 0007 pilot. It is not hosted/service, public, CLI, MCP, core, shipment, census, or feature-identity authorization.

## Requested scope

The requested authorization is limited to local execution of the bounded diagnostic pilot over revision-bound code evidence.

- Local four-packet `TARGET` Describe execution: in scope.
- Local preflight, conformance, replay, and failure testing: in scope.
- Internal service deployment, public or external-facing surfaces, CLI, MCP, core integration, production use, shipment, and repository census: out of scope and prohibited.

No broader surface is requested or effective from this proposal.

## Recorded approval statement

The project owner stated approval for “the whole thing,” understood here as approval to prepare and seek authorization across the full scope above. The statement is preserved as an approval request and does not override repository, deployment, security, privacy, or release controls.

## Proposed governance defaults

The project owner proposes the following defaults for the decision record:

- **Accountability:** the project owner holds project, security/privacy, acceptance, and release roles.
- **Review:** the documented G7 self-review exception waives independent review; this is an explicit exception, not evidence of independent assurance.
- **Deployment:** local/internal first, deny-by-default, with no public, CLI, MCP, core, or production exposure unless separately switched on by a recorded decision.
- **Evidence:** immutable input/output digests, pinned runtime identities, replayable records, and retained terminal accounting.
- **Safety:** fail closed on missing or mismatched artifacts; network denial and process/resource containment remain mandatory.
- **Rollback:** revocation and rollback must be tested before any broader activation.
- **Claims:** no repository census, feature identity, semantic authority, completion, or acceptance claims from this pilot alone.
- **Data:** only revision-bound code evidence from the four admitted packets; no external service, user data, or unclassified sensitive data.

## Owner endorsement

The project owner endorsed the proposed governance defaults. This endorsement confirms the intended control posture; it does not activate any broader surface.

## Activation checklist

| Control | Status |
|---|---|
| Local four-packet diagnostic evidence | `COMPLETE` |
| Pinned model, adapter, worker, and runtime digests | `COMPLETE` |
| Network denial and fail-closed preflight | `COMPLETE` |
| Immutable outputs and replay records | `COMPLETE` |
| G7 self-review exception recorded | `COMPLETE` |
| Privacy, retention, and sensitive-data policy | `OWNER-APPROVED REQUIREMENT; NOT VERIFIED` |
| Resource limits and production monitoring | `OWNER-APPROVED REQUIREMENT; NOT VERIFIED` |
| Rollback and revocation drill | `OWNER-APPROVED REQUIREMENT; NOT VERIFIED` |
| Exact enabled surface: local diagnostic only | `OWNER-APPROVED; ACTIVE WITHIN PILOT SCOPE` |
| Production/public release authorization | `OUT OF SCOPE / PROHIBITED` |

## Verification result

Verified from retained local evidence:

- local four-packet execution and terminal accounting;
- pinned model, adapter, worker, and runtime identities;
- network-denied execution;
- fail-closed model preflight;
- immutable request/response hashes and replay records;
- explicit local-only scope and prohibitions.

Not verified by the retained evidence:

- privacy, retention, and sensitive-data policy;
- production monitoring and operational resource controls;
- rollback or revocation drill;
- activation of any internal service, public, CLI, MCP, core, or production surface.

Accordingly, the local diagnostic scope is active only within the already executed pilot boundary; no broader activation exists. Requirements for any future scope remain non-applicable unless that scope is separately introduced.

## Required decision record before activation

Before any broader scope becomes effective, an accountable governance decision must record:

1. the exact authorized surfaces and environments;
2. named accountable owner(s), security/privacy owner(s), and acceptance authority;
3. frozen protocol/schema and compatibility decisions;
4. evaluation labels, thresholds, replay, failure, latency, and resource results;
5. model/runtime supply-chain verification and network/resource containment;
6. data handling, privacy, retention, and incident response controls;
7. revocation, rollback, monitoring, and audit procedures;
8. explicit approval or rejection of production, public, CLI, MCP, core, shipment, census, and feature-identity claims.

Until that record exists, ADR 0007 remains accepted only as bounded local diagnostic evidence and the existing prohibitions remain in force.
