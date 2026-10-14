# Private bidirectional CALLS composition contract/spec

Schema: `lsp-trace.adr0011.private-bidirectional-composition.qualification.v1`

Authority: `0`.
Accepted: `false`.
Production admission: `REFUSED_NOT_REQUESTED_NOT_GRANTED`.
Producer disposition: `ADR0011_PRIVATE_BIDIRECTIONAL_COMPOSITION_AUTHORED_NOT_GO`.
Producer may issue GO: `false`.
Private: `true`.
Default-on: `false`.

## 1. Purpose

This contract creates the smallest private evidence-only qualification package that lets reviewers address incoming and outgoing CALLS evaluation evidence together without merging either direction's custody, authority, completeness, ledgers, raw evidence, numeric bounds, frontier, profile state, or failure state.

## 2. Canonical top-level representation

The canonical composition artifact is a duplicate-free UTF-8 JSON metadata object with exactly one trailing LF and exactly two top-level keys in this order:

```json
{"incoming":<directional-envelope>,"outgoing":<directional-envelope>}
```

No other top-level keys are valid. The canonical serializer preserves this order. Every nested object is metadata only. It may carry immutable path/hash/count references, but it may not carry raw provider bytes or a reconstruction of provider bytes.

## 3. Directional envelope sum type

Each top-level value is a closed directional envelope with common fields: `direction`, `status`, authority `0`, accepted `false`, production refusal, target-set-relative completeness `UNKNOWN`, frontier `UNKNOWN`, raw-provider-byte policy, per-direction `33554432` ceiling, opaque custody, CALLS-only, immutable-reference-only, and peer-failure effect `PEER_UNCHANGED_NOT_ERASED_NOT_RELABELED`.

### A. Valid success directional result

A `SUCCESS` envelope contains an `accepted_directional_success_reference` payload. It preserves, for its own direction only:

- profile id and generic profile id;
- accepted dependency token, commit, package path, package hash, manifest hash, and result hash;
- producer disposition and GO evidence reference;
- accepted profile qualification state as dependency state, without producer self-GO;
- directional counts, failure reference, final-summary reference, ledger reference, numeric-bounds reference, raw-provenance reference, and custody references;
- no inverse synthesis, no dedup/union graph claim, no ledger concatenation, no shared authority, and no shared completeness.

### B. Valid failure directional result

A `FAILURE` envelope contains a bounded private fixture-native failure payload. It is evidence-only and exists solely for composition evaluation when no accepted production failure schema is available. It must carry:

- same direction as its top-level slot;
- `failure_disposition: FAILURE`;
- direction-specific stable `failure_code` beginning with `INCOMING_` or `OUTGOING_`;
- stable `failure_stage`;
- `message_digest` rather than raw message text when text would become an evidence payload; it must be a string that exactly matches lowercase `sha256:[0-9a-f]{64}` with no extra prefix, suffix, or whitespace;
- private evidence reference as available;
- authority `0`, completeness `UNKNOWN`, frontier `UNKNOWN`, production refusal;
- `counts_claimed: false`, `success_claims: false`, `inverse_inference: false`, and `peer_evidence_used: false`.

A failure envelope must not fabricate successful counts/claims, infer inverse evidence, relabel itself as the opposite direction, copy peer evidence, merge failures, or change the peer envelope.

### C. Invalid composition input

Invalid inputs are rejected and are not represented as directional failures. Invalid inputs include pin/hash/profile/direction/schema drift, cross-source or cross-direction references, missing or swapped directions, unknown keys, semantic aliases, public/default-on exposure, ceiling changes, CALLS-only drift, raw-copy/remarshal attempts, authority/completeness escalation, and failure-to-success promotion.

## 4. Peer preservation

The composer accepts independently valid envelopes and outputs an ordered pair. The canonical subtree serialization of each accepted directional envelope is byte-stable under composition. Incoming failure plus outgoing success preserves the outgoing success subtree hash. Outgoing failure plus incoming success preserves the incoming success subtree hash. Both failures are retained as two separate failure envelopes.

## 5. Forbidden composition effects

The composition must not:

- create shared authority or shared completeness;
- synthesize inverse evidence;
- deduplicate, union, or assert any graph claim;
- concatenate, merge, or reinterpret ledgers;
- transfer GO, evidence, custody, profile state, or failure state between directions;
- erase or relabel one direction when the other fails;
- read, copy, decode, remarshal, reconstruct, or embed raw provider bytes;
- change CALLS-only semantics, ceilings, custody owners, public surfaces, defaults, production admission, ADR0011->ADR0007 composition, Stage 8, runtime code, CLI, MCP, API, schemas, publication, release, or deployment.

## 6. Evidence boundary

The verifier is allowed to read only dependency metadata files required to recompute package hashes and exact result fields: `CHECKSUMS.sha256`, `MANIFEST.json`, and `RESULT.json` for the four accepted packages. It never opens raw request/response evidence paths; those paths remain opaque references.
