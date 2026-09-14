# Prerequisite gates

Status: `PREREQUISITES_DRAFT`; pilot: `PILOT_DISABLED`.

All gates are conjunctive and fail closed. Each eventual gate record must contain a stable gate ID, artifact IDs, SHA-256 digests and byte lengths, decision, named accountable approver identity, timestamp, scope, exceptions (normally none), and supersession/revocation references. Drafting or passing one gate does not imply another passed.

| Gate | Required frozen evidence | Blocking condition |
|---|---|---|
| G1 Protocol | Versioned schemas, NDJSON framing, enums, denominator equations, conformance vectors | Any ambiguity, schema drift, or unbalanced accounting |
| G2 Admission/lineage | Typed corpora, TARGET/NEIGHBORHOOD/CENSUS, identity, correction, invalidation | Mutable identity or unaccounted admission |
| G3 Artifact governance | Exhaustive class inventory and privacy/access/encryption/retention/deletion/backups policy | Unclassified artifact or missing disposition |
| G4 Supply chain | Exact runtime/model/native digests, SBOM, license, vulnerability and revocation decisions | Download enabled, missing pin, quarantine, or revocation |
| G5 Runtime containment | Network-denied process, no-download proof, process-tree CPU/memory/time/file controls | Escape, external retrieval, traversal, or uncontrolled descendant |
| G6 Evaluation F0–F8 | Digest-bound plan, labels, splits, metrics, calibrated numeric thresholds, stops | Test unlocked before threshold freeze or failed gate |
| G7 Ownership | Every accountable and approval cell names a real identity and records conflicts | Placeholder, self-approval conflict, or missing separation |
| G8 Bundle | Manifest binds all approved artifacts and decisions by digest | Missing/mismatched/revoked component |

Pilot enablement requires a later explicit enablement record referencing one exact G1–G8 bundle digest. There is no implicit enablement from ADR acceptance, document presence, code review, process separation, or evaluation success.

## Rejection and revocation

Any mismatch, policy violation, authority escalation, incomplete denominator, leakage, network access, download, stale dependency, missing owner, failed threshold, or conformance failure keeps `PILOT_DISABLED`. Revocation invalidates dependent records and requires a new bundle or a closed rejection; it never mutates prior evidence.
