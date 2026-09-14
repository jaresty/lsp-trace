# Yzma candidate supply-chain dossier

Status: `PREREQUISITES_DRAFT`; pilot: `PILOT_DISABLED`.

This dossier records one observed candidate tuple and a proposed pre-freeze checklist. It is not owner assignment, approval, frozen evidence, qualification, permission to retrieve or execute artifacts, public availability, or selection of Yzma. Mutable upstream retrieval is not frozen evidence. Yzma remains an unfrozen, replaceable candidate behind the backend-neutral subprocess protocol.

## Observed candidate tuple

Keep this observation distinct from proposals and unknowns:

| Component | Observed candidate value | Disposition |
|---|---|---|
| Yzma release | `v1.26.1` | candidate, unfrozen |
| Yzma commit | `347c6ee0b893cc0bcac50ff7fb12ece0611fd01a` | candidate source identity, unfrozen |
| Required Go version | `1.26.0` | candidate build prerequisite, unfrozen |
| Default runtime manifest | `v0.4.0` | candidate, unfrozen |
| Default runtime manifest digest | `sha256:b95e8680b4d30761492bbc2d4a6fed656f124c5756dd4387cf02c30d27c13d90` | observed candidate digest, not an approved artifact |
| Runtime mapping | `llama.cpp` tag `b10809` | candidate transitive mapping, unfrozen |

These values must be re-acquired into controlled custody and independently verified before they can support a freeze record. A release label, commit, mutable manifest URL, package-manager result, local cache, successful build, or successful process launch is not a complete or approved tuple.

## Proposals and unknowns

The following remain proposals or unknowns: Yzma selection; wrapper source and module graph; runtime archive and extracted-tree identities; builder/toolchain identities; target OS/architecture/accelerator; native ABI; model, tokenizer, chat template and quantization; model source and license; inference parameters; resource limits; vulnerability disposition; redistribution terms; signing/notarization policy; protocol framing details; and alternate backend.

No in-process Go `LocalModel` interface is proposed for protocol-visible requests. Yzma and native runtime types must remain behind the separately built worker; they never enter the core process, core `go.mod`, or backend-neutral records.

## Pre-freeze checklist

Every item is required evidence for a later gate decision; an unchecked or passing item does not imply approval or pilot enablement.

### Complete tuple and custody

- [ ] Bind wrapper release, commit, source archive, module graph and exact Go requirement.
- [ ] Bind runtime manifest bytes, manifest digest, mapped runtime revision, source archive, builder and toolchain.
- [ ] Bind model, tokenizer, chat template, quantization, model card, inference parameters and all transitive native libraries.
- [ ] Record source URL, acquisition time, custody transition, byte length and SHA-256 for every acquired artifact; mutable retrieval alone is not evidence of frozen bytes.
- [ ] Reconcile upstream labels against acquired bytes and record every mismatch, substitution, patch and locally produced artifact.

### SBOM, licenses and hashes

- [ ] Produce complete Go-module and native-library SBOMs, including build-only and dynamically loaded dependencies.
- [ ] Close wrapper, runtime, builder, native library and model licenses, notices, redistribution duties and compatibility; unresolved or incompatible terms block the gate.
- [ ] Record separately the builder-input, source-archive, binary-archive and extracted-file/tree hashes; never treat an archive hash as the extracted payload identity.
- [ ] Bind reproducibility evidence or record that reproducibility is unavailable; a local build result alone does not establish upstream provenance.

### macOS native inspection

- [ ] Inventory every Mach-O binary and library, architecture slice, minimum OS, load command, rpath and linked dependency.
- [ ] Capture `otool` dependency and load-command results and reject undeclared, writable-path, ambient or missing libraries.
- [ ] Record `codesign` identity, entitlements, hardened-runtime/notarization state and verification result without equating a valid signature with approval.
- [ ] Record quarantine attributes and Gatekeeper disposition for archives and extracted artifacts; do not silently clear quarantine as evidence.
- [ ] Compare exported/imported ABI symbols, C/C++ runtime expectations and wrapper-required symbols against the exact runtime build.

### Model and GGUF closure

- [ ] Bind exact GGUF/model bytes, digest, size, source revision, conversion recipe/tool identity and original-model provenance.
- [ ] Verify model, dataset and conversion licenses and required notices or use restrictions.
- [ ] Verify declared architecture, tokenizer, chat template, quantization and runtime compatibility with the candidate runtime and backend-neutral protocol capabilities.
- [ ] Reject automatic model replacement, format conversion, quantization or download not represented in the frozen tuple.

### Vulnerability and revocation

- [ ] Record vulnerability scans and manual review for Go, native, builder and model artifacts with dated database identities and explicit dispositions.
- [ ] Define revocation triggers for compromised source, signature, license, vulnerability, model provenance, runtime ABI and protocol incompatibility.
- [ ] Demonstrate that revocation invalidates dependent worker, index, cache and generated-product identities and produces closed rebuild, deletion, unavailable or externally-retained receipts.

### Offline, containment and failure behavior

- [ ] Start from empty external caches with network denied; prove build or installation preparation used only separately admitted frozen inputs.
- [ ] With network denied, prove worker startup, negotiation, every operation and control interaction, retry, failure, restart and termination performs no DNS, connect, listen, fetch, update or download; all labels and lifecycle details remain subject to the G1 freeze.
- [ ] Prove no fallback to a different model, runtime, backend, remote service, tool, source reread or filesystem traversal occurs when an artifact is absent or invalid.
- [ ] Apply CPU, memory, wall-time, process/thread, file-descriptor, output and disk limits to the worker and descendants; verify forced termination and cleanup.
- [ ] Verify cooperative and forced cancellation close accounting, kill descendants when required, bound output, remove or disposition temporary data, and do not resurrect revoked or deleted inputs.

### Alternate-backend contract

- [ ] Run the same backend-neutral protocol and conformance vectors against a non-Yzma test backend without changing request or response schemas.
- [ ] Prove backend/runtime/model identities keep caches and products distinct while source authority and generated `authority=0`, `accepted=false` semantics remain unchanged.
- [ ] Reject backend-specific fields, Yzma types, implicit capabilities and behavior that cannot be represented by the neutral contract.

## Freeze blockers

The supply-chain gate remains blocked until the complete tuple, custody, hashes, SBOMs, licenses, native inspection, ABI, model provenance, vulnerability/revocation, offline behavior, resource/cancellation behavior and alternate-backend contract all have immutable evidence and separate recorded decisions. This draft names no owner or approver and creates no approved artifact.
