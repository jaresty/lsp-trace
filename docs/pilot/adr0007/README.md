# ADR 0007 prerequisite bundle

- **Bundle status:** `PREREQUISITES_DRAFT`
- **Pilot status:** `PILOT_DISABLED`
- **Authority:** [Accepted ADR 0007](../../adr/0007-optional-local-semantic-feature-index.md) authorizes preparation of prerequisites only.

This directory is a documentation-only scaffold. It does not freeze or approve a prerequisite, assign an owner, enable a pilot, integrate Yzma, or authorize implementation, execution, shipment, a public schema registry, a core `go.mod` dependency, CLI/MCP surfaces, runtime code, or a worker.

## Boundary and future layout

Canonical draft schemas, if later authored, belong under `docs/pilot/adr0007/schemas/`. An approved immutable bundle, if every gate is independently approved, belongs under `qualification/adr0007/approved/` and must bind every component by digest. Only after that freeze may a separate nested worker module be considered. It must never enter the public registry, core `go.mod`, CLI, or MCP surface. Yzma remains a replaceable process backend, not an authority.

## Bundle index

1. [Prerequisite gates](prerequisite-gates.md)
2. [Artifact inventory](artifact-inventory.md)
3. [Ownership and approvals](ownership-and-approvals.md)
4. [Evaluation plan](evaluation-plan.md)
5. [Protocol outline](protocol-outline.md)
6. [Threat model and conformance tests](threat-model.md)

Every document is intentionally draft. Later approval requires named accountable identities, recorded decisions, and immutable digests; a path, commit, review, or process boundary alone is insufficient.
