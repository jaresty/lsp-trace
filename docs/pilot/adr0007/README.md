# ADR 0007 prerequisite bundle

- **Bundle status:** `PREREQUISITES_DRAFT`
- **Pilot status:** `PILOT_DISABLED`
- **Authority:** [Accepted ADR 0007](../../adr/0007-optional-local-semantic-feature-index.md) authorizes preparation of prerequisites only.

This directory is a documentation-only scaffold. It does not freeze or approve a prerequisite, assign an owner, enable a pilot, integrate or select Yzma, or authorize implementation, qualification, execution, shipment, public availability, a public schema registry, a core `go.mod` dependency, CLI/MCP surfaces, runtime code, or a worker.

## Boundary and future layout

Canonical draft schemas, if later authored, belong under `docs/pilot/adr0007/schemas/`. No approved artifacts exist in this scaffold. Later approved immutable bytes belong under `qualification/adr0007/approved/` only after every gate is independently satisfied and recorded, and the approved bundle must bind every component by digest.

The prerequisite architecture direction is a separate backend-neutral, network-denied subprocess behind a JSON/NDJSON boundary. It must never enter the core process, public registry, core `go.mod`, CLI, or MCP surface. Backend-specific types never cross the protocol boundary. Yzma remains an unfrozen, replaceable candidate backend, not an authority; process isolation itself confers no authority.

## Bundle index

1. [Prerequisite gates](prerequisite-gates.md)
2. [Artifact inventory](artifact-inventory.md)
3. [Ownership and approvals](ownership-and-approvals.md)
4. [Evaluation plan](evaluation-plan.md)
5. [Protocol outline](protocol-outline.md)
6. [Yzma candidate supply-chain dossier](supply-chain-candidate.md)
7. [Threat model and conformance tests](threat-model.md)

Every document is intentionally draft. Later approval requires named accountable identities, recorded decisions, and immutable digests; a path, commit, review, or process boundary alone is insufficient.
