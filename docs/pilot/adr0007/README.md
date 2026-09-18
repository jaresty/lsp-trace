# ADR 0007 prerequisite bundle

- **Bundle status:** `PREREQUISITES_DRAFT`
- **Pilot status:** `PILOT_DISABLED`
- **Authority:** [Accepted ADR 0007](../../adr/0007-optional-local-semantic-feature-index.md) authorizes preparation of prerequisites only.

This directory is a documentation-only scaffold. It does not freeze or approve a prerequisite, assign an owner, enable a pilot, integrate or select Yzma, or authorize implementation, qualification, execution, shipment, public availability, a public schema registry, a core `go.mod` dependency, CLI/MCP surfaces, runtime code, or a worker.

## Boundary and future layout

Canonical draft schemas, if later authored, belong under `docs/pilot/adr0007/schemas/`. No approved artifacts exist in this scaffold. Later approved immutable bytes belong under `qualification/adr0007/approved/` only after every gate is independently satisfied and recorded, and the approved bundle must bind every component by digest.

The prerequisite architecture direction is a separate backend-neutral, network-denied subprocess behind a serialization-neutral boundary. Complete-JSON versus NDJSON framing remains an unresolved G1 freeze decision; neither term names a frozen wire contract here. It must never enter the core process, public registry, core `go.mod`, CLI, or MCP surface. Backend-specific types never cross the protocol boundary. Yzma remains an unfrozen, replaceable candidate backend, not an authority; process isolation itself confers no authority.

## Bundle index

1. [Prerequisite gates](prerequisite-gates.md)
2. [Artifact inventory](artifact-inventory.md)
3. [Ownership and approvals](ownership-and-approvals.md)
4. [Evaluation plan](evaluation-plan.md)
5. [Protocol outline](protocol-outline.md)
6. [Yzma candidate supply-chain dossier](supply-chain-candidate.md)
7. [Threat model and conformance tests](threat-model.md)
8. [G1 protocol decision draft](protocol-decision.md)
9. [G1 lifecycle and cancellation draft](lifecycle-and-cancellation.md)
10. [G1 conformance vectors](conformance-vectors.md)
11. [G2 admission and lineage](admission-and-lineage.md)
12. [G3 artifact governance](artifact-governance.md)
13. [G4 supply-chain controls](supply-chain-controls.md)
14. [G5 runtime containment](runtime-containment.md)
15. [G6 evaluation thresholds](evaluation-thresholds.md)
16. [G7 ownership decision worksheet](ownership-decision-worksheet.md)
17. [G8 draft bundle manifest](g8-bundle-manifest.draft.json)
18. [Recovered relative-consumer experiment package](experiment/README.md)
19. [Final bounded local pilot report](FINAL-PILOT-REPORT.md)

## Bounded local diagnostic result

The four-packet `TARGET` Describe diagnostic completed under network denial with all packets returning `COMPLETE / SUPPORTED`. This result is retained as local evidence only. It does not change the prerequisite bundle status, create production or public authorization, or authorize CLI/MCP/core integration, shipment, census behavior, or feature-identity claims. The fail-closed runtime preflight is `experiment/preflight-final-four-packet.sh`; immutable v2 outputs and hashes are under `experiment/final-four-packet-v2/`.

Every document is intentionally draft. Later approval requires named accountable identities, recorded decisions, and immutable digests; a path, commit, review, or process boundary alone is insufficient.
