# Artifact inventory and lifecycle fields

Status: `PREREQUISITES_DRAFT`. Every raw and derived artifact is treated as potentially source-sensitive.

## Required artifact classes

- submitted source bytes; canonicalized representations; selectors; admission attempts and rejections;
- structural evidence and provenance references;
- prompts, system instructions, templates, grammars, sampling and seeds;
- descriptions, embeddings, indexes, queries, rankings, groups, rationales, work-context packets, caches;
- correction, supersession, invalidation, tombstone, rebuild and deletion receipts;
- protocol/schema/enum/policy/evaluation manifests and approval records;
- model, tokenizer, chat template, quantization, model card and source metadata;
- worker executable, backend, native libraries, environment/runtime manifests, SBOMs, licenses, signatures and vulnerability reports;
- logs, metrics, traces, crash dumps, temporary/spill files and process-control records;
- test fixtures, labels, splits, calibration outputs, baselines and immutable reports;
- archives, snapshots, replicas, backups and externally retained copies.

## Fields required per class

Each class definition must declare: stable class ID and version; producer/consumer; content/media/schema identity; sensitivity and corpus; source/derived status; authority and acceptance; storage locations and custody boundaries; encryption in transit/at rest where applicable and key owner/reference; read/write/delete access roles and audit trail; retention trigger and duration; deletion trigger, mechanism, verification evidence and deadline; cache/temp handling; backup/snapshot/replica coverage and expiry; tombstone propagation; rebuild dependencies and receipt outcome; legal/license constraints; incident/revocation procedure; and named accountable owner and approver.

Deletion receipts enumerate each affected artifact as `DELETED`, `REBUILT`, `UNAVAILABLE`, or `EXTERNALLY_RETAINED`, with identity, owner, reason, dependencies and timestamp. Tombstones prevent stale reappearance; rebuilds exclude deleted admissions and bind new identities. This bundle makes no secure-erasure claim without independent storage-layer evidence.
