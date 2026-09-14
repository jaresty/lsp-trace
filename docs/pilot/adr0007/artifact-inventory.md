# Artifact inventory and lifecycle fields

Status: `PREREQUISITES_DRAFT`; pilot: `PILOT_DISABLED`. Every raw and derived artifact is treated as potentially source-sensitive. This inventory defines draft classes only; it contains no frozen, approved, qualified, executable, or public artifact.

## Required artifact classes

- submitted source bytes; canonicalized representations; selectors; admission attempts and rejections;
- structural evidence and provenance references;
- backend-neutral protocol artifacts, interaction identities, lifecycle/control evidence and accounting closures, using only vocabulary and representations later frozen by G1;
- prompts, system instructions, templates, grammars, sampling and seeds;
- descriptions, embeddings, indexes, queries, rankings, groups, rationales, work-context packets and caches, all generated with `authority=0` and `accepted=false`;
- correction, supersession, invalidation, tombstone, rebuild and deletion receipts;
- protocol/schema/enum/policy/evaluation manifests and approval records;
- observed candidate tuples kept distinct from proposals, unknowns, acquired bytes and later gate decisions;
- model/GGUF bytes, tokenizer, chat template, quantization, model card, provenance, conversion recipe, compatibility and license metadata;
- worker executable, wrapper/backend source, Go module graph, runtime manifest, builder/toolchain, source and binary archives, extracted files/trees, native libraries and ABI-symbol inventories;
- per-stage hashes, Mach-O inventories, `otool` results, `codesign`/notarization results, quarantine/Gatekeeper records, environment/runtime manifests, SBOMs, licenses, signatures and vulnerability reports;
- logs, metrics, traces, crash dumps, temporary/spill files and process-control records;
- test fixtures, labels, splits, calibration outputs, baselines and immutable reports;
- archives, snapshots, replicas, backups and externally retained copies.

## Fields required per class

Each class definition must declare: stable class ID and version; producer/consumer; content/media/schema identity; sensitivity and corpus; source/derived status; authority and acceptance; storage locations and custody boundaries; encryption in transit/at rest where applicable and key owner/reference; read/write/delete access roles and audit trail; retention trigger and duration; deletion trigger, mechanism, verification evidence and deadline; cache/temp handling; backup/snapshot/replica coverage and expiry; tombstone propagation; rebuild dependencies and receipt outcome; legal/license constraints; incident/revocation procedure; and named accountable owner and approver.

Draft schema artifacts, if later authored, remain under `docs/pilot/adr0007/schemas/`. This bundle contains no actual approved artifact. Only after every gate may later approved immutable bytes be placed under `qualification/adr0007/approved/`; path placement alone never supplies approval or authority.

Deletion receipts enumerate each affected artifact as `DELETED`, `REBUILT`, `UNAVAILABLE`, or `EXTERNALLY_RETAINED`, with identity, owner, reason, dependencies and timestamp. Tombstones prevent stale reappearance; rebuilds exclude deleted admissions and bind new identities. This bundle makes no secure-erasure claim without independent storage-layer evidence.
