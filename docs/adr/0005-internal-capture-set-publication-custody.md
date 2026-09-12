# ADR 0005: Keep capture-set publication private and content-addressed

- **Status:** Accepted
- **Date:** 2026-09-12
- **Decision owners:** LSP Trace maintainers
- **Scope:** `internal/captureset` publication and custody

## Context

The existing `lsp-trace.capture-set.v1` manifest composes independently custodied Graph Provenance V5 captures. It has a deterministic logical identity but previously left selector authority, publication races, receipt visibility, and redaction policy implicit. This increment must not enter CLI/MCP acquisition lanes or accept request-supplied paths.

## Decision

### Canonical selectors and authority

`ExactBytesAuthority` is the constituent metadata authority. It first invokes the supplied native Graph Provenance V5 structural/semantic admission, which derives the native V5 identity from the exact admitted bytes, then derives the remaining exact-byte metadata. Callers cannot choose or override the identity, selector, digest, length, or schema ID.

Canonical constituent selector syntax is:

```text
graph-provenance-v5/sha256/<64 lowercase hexadecimal digits>
```

The suffix is SHA-256 of the exact verified constituent bytes. `sha256`, byte length, schema ID, native identity, and selector must all match on verification. Private manifest validation rejects a selector not derived from its declared exact-byte digest.

Canonical capture-set publication selector syntax is:

```text
capture-sets/v1/sha256/<64 lowercase hexadecimal logical-digest digits>/manifest.json
```

Selectors are relative names beneath an already pinned `publication.Root`; they are not caller filesystem paths.

### Publication and verification

`captureset.Publisher` canonicalizes and validates the manifest, then delegates to `publication.Publisher`'s component-relative temporary-file, sync, hard-link, no-replace installation. The hard link is the atomic commit point. An existing selector returns `TARGET_EXISTS`; it is never overwritten, even by an equivalent retry. Concurrent same-selector publication therefore has exactly one create winner.

Verification accepts only canonical capture-set selectors, reads bounded exact bytes beneath the pinned root, strictly decodes and semantically validates the manifest, recomputes logical identity, and requires selector/identity equality. This is integrity and local custody verification, not authentication or source truth.

The returned `PublicationReceipt` is `PRIVATE` by default. It records selector, exact artifact SHA-256, byte length, and `atomic_no_replace`; it is not a public receipt schema and conveys no additional authority.

### Exact redaction policy

`Redact` changes exactly these fields:

- `disclosure`: `PRIVATE` to `REDACTED`;
- each file-ledger and symbol-ledger `identity` to deterministic `!redacted:<zero-padded ordinal>`;
- each target `canonical_seed_v2` to the same ordinal form and its SHA-256 to the digest of that replacement;
- each constituent `immutable_selector` and `native_v5_identity` to the ordinal form;
- `logical_digest` and capture-set `immutable_selector`, which are recomputed over the redacted view.

It retains schema version, authority ceiling, source completeness, custody/CALLS/Leiden exclusions, census and duplicate policies, ledger denominators/dispositions/ordinals, target ordinals, constituent schema/digest/byte length, and batch accounting. Ordinal-qualified replacements preserve canonical ordering and uniqueness without disclosing original identities. A redacted view must have a distinct logical digest and selector and is never represented as the private artifact's identity.

### Claim ceiling

Both private and redacted manifests preserve:

- `authority: 0`;
- `source_graph_complete: "UNKNOWN"`;
- `native_single_capture_custody: false`;
- empty `cross_capture_calls` (no cross-capture `CALLS`);
- `leiden_admissible: false` (no direct Leiden admission).

Constituent custody remains independently native; capture-set publication does not inherit or aggregate it.

### Public schema registry

`lsp-trace.capture-set.v1` does **not** join `internal/schema` in this increment. Registry admission would expose a public compatibility family through schema-get/validation surfaces, while this contract is intentionally internal, private by default, and has no public producer/consumer commitment. The embedded package schema remains an internal guard. A later ADR may admit a new public family only with a stable public transport, semantic validator, compatibility owner, privacy review, and demonstrated multi-consumer need.

## Rejected alternatives

- Caller-provided constituent selectors: rejected because metadata could claim custody over different bytes.
- URI or absolute-path selectors: rejected because they introduce ambient/arbitrary path ingress.
- rename-over-existing or check-then-write: rejected because they are not race-safe immutable publication.
- idempotent success for an existing equal artifact: rejected because it obscures which operation created custody; retries receive `TARGET_EXISTS`.
- preserving private identity on redaction: rejected because identity would falsely denote different bytes/semantics.
- registering the internal schema now: rejected because internal implementation existence is not sufficient justification for a public compatibility surface.

## Consequences

Hosts receive a small in-process exact-byte API and must provision the pinned publication root and native V5 verifier independently. No CLI/MCP command, language-server acquisition, seed-file lane, arbitrary-path reader, cross-capture edge construction, or Leiden pipeline is added.
