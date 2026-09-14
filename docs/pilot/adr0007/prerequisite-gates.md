# Prerequisite gates

Status: `PREREQUISITES_DRAFT`; pilot: `PILOT_DISABLED`.

All gates are conjunctive and fail closed. Each eventual gate record must contain a stable gate ID, artifact IDs, SHA-256 digests and byte lengths, decision, named accountable approver identity, timestamp, scope, exceptions (normally none), and supersession/revocation references. Drafting or passing one gate does not imply another passed.

| Gate | Required frozen evidence | Blocking condition |
|---|---|---|
| G1 Protocol | Versioned backend-neutral schemas, framing/state machine, accepted enums and denominator equations by immutable reference, conformance vectors | Any ambiguity, backend-specific type, schema drift, invalid transition, or unbalanced accounting |
| G2 Admission/lineage | Typed corpora, TARGET/NEIGHBORHOOD/CENSUS, identity, correction, invalidation | Mutable identity or unaccounted admission |
| G3 Artifact governance | Exhaustive class inventory and privacy/access/encryption/retention/deletion/backups policy | Unclassified artifact or missing disposition |
| G4 Supply chain | Complete wrapper/runtime/model tuple; builder/archive/extracted hashes; Go/native SBOM and license closure; Mach-O, `otool`, `codesign`, quarantine and ABI evidence; GGUF provenance; vulnerability/revocation decisions | Mutable retrieval, download enabled, missing pin, unresolved license, incompatible ABI/model, quarantine, or revocation |
| G5 Runtime containment | Separate network-denied subprocess; caller-supplied admitted bytes/relationships; no-reread/no-traversal/no-tool/no-fallback proof; process-tree resource and cancellation controls | In-process backend, escape, external retrieval, traversal, execution, fallback, or uncontrolled descendant |
| G6 Evaluation F0–F8 | Digest-bound plan, labels, splits, metrics, calibrated numeric thresholds, stops | Test unlocked before threshold freeze or failed gate |
| G7 Ownership | Every accountable and approval cell names a real identity and records conflicts | Placeholder, self-approval conflict, or missing separation |
| G8 Bundle | Manifest binds all approved artifacts and decisions by digest | Missing/mismatched/revoked component |

Pilot enablement requires a later explicit enablement record referencing one exact G1–G8 bundle digest. There is no implicit enablement from ADR acceptance, document presence, code review, process separation, candidate tuple observation, or evaluation success.

## Unresolved freeze decisions

G1 remains blocked until later decisions freeze protocol/schema versions; JSON versus NDJSON transport scope; the final LF/line-ending rule; canonical JSON and digest algorithms; state transitions; correlation, ordering, field, duplicate and unknown-field policy; limits; cancellation/shutdown/retry semantics; accepted-list references; corpus/item schema details; identity/cache encoding; lifecycle dispositions; backend capability negotiation; and conformance vectors.

G4–G6 remain blocked until later decisions freeze the complete acquired supply-chain tuple, model choice, metric definitions, calibrated numeric thresholds, resource limits, environment and alternate-backend contract. The [Yzma candidate supply-chain dossier](supply-chain-candidate.md) is a checklist and candidate observation only, not gate evidence or a decision.

## Rejection and revocation

Any mismatch, policy violation, authority escalation, incomplete denominator, leakage, network access, download, stale dependency, missing owner, failed threshold, or conformance failure keeps `PILOT_DISABLED`. Revocation invalidates dependent records and requires a new bundle or a closed rejection; it never mutates prior evidence.
