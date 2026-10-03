# ADR 0007 prerequisite bundle

- **Bundle status:** `PREREQUISITES_DRAFT`
- **Pilot status:** `PILOT_DISABLED`
- **Authority:** [Accepted ADR 0007](../../adr/0007-optional-local-semantic-feature-index.md) authorizes preparation of prerequisites only.

This directory is a documentation-only scaffold. It does not freeze or approve a prerequisite, assign an owner, enable a pilot, integrate or select Yzma, or authorize implementation, qualification, execution, shipment, public availability, a public schema registry, a core `go.mod` dependency, CLI/MCP surfaces, runtime code, or a worker.

## Current delivery direction: caller-provided inference

The [caller-provided inference amendment and minimal plan](../../adr/0007-caller-provided-inference-amendment.md) makes the existing host LLM the first interpretation path over bounded lsp-trace evidence. No second model, additional API key, or new service is required for that path. Inference location follows the host; remote source disclosure still requires permission.

See the [bounded host-assisted caller inference example](host-assisted-example.md) for one evidence-limited answer recipe.

This directory remains the standalone worker/pilot scaffold. Its `PILOT_DISABLED` state remains unchanged, and prior records are not reclassified. Its worker-specific prerequisites do not block ordinary host-assisted explanation through existing tools. Local-model support remains optional and deferred; do not restart local calibration to deliver the host-assisted example.

## Earlier key-free local inference and optional GPT amendment

The [accepted scope amendment](../../adr/0007-single-user-gpt-amendment.md) preserves key-free local-model execution as the delivery goal and names @jaresty (GitHub handle) as operator and human approval authority. GPT may support development/evaluation or a separately enabled optional backend. It is not a required runtime dependency, and no local request may silently fall back to remote inference.

The drafts and digest manifest indexed below have **not** been migrated or approved by that amendment. Local-model verification, staged provisioning, network-denied inference, and containment remain applicable to the local backend. Optional GPT uses separate provider, disclosure, and controlled-egress requirements. TARGET Describe evaluation and single-user approval are scoped by the amendment; no backend is qualified by another's results. Prepare separately identified backend-profile bundles and preserve historical draft identities. `PREREQUISITES_DRAFT` and `PILOT_DISABLED` remain unchanged. No new model execution or source transmission is enabled here.

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

The four-packet `TARGET` Describe diagnostic completed under network denial with all packets returning `COMPLETE / SUPPORTED`. This result is accepted as an approved local operational pilot. It does not create hosted/service or public authorization, or authorize CLI/MCP/core integration, shipment, census behavior, or feature-identity claims. The fail-closed runtime preflight is `experiment/preflight-final-four-packet.sh`; immutable v2 outputs and hashes are under `experiment/final-four-packet-v2/`.

Every document is intentionally draft. Later approval requires named accountable identities, recorded decisions, and immutable digests; a path, commit, review, or process boundary alone is insufficient.
