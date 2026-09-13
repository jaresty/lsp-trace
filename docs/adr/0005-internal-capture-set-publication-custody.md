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
capture-sets/v1/sha256/<64 lowercase hexadecimal logical-digest digits>.bundle
```

Selectors are relative names beneath an already pinned `publication.Root`; they are not caller filesystem paths.

### Canonical bundle, publication, and verification

The sole private visibility unit is one regular bundle file. Its canonical binary encoding is the fixed `LSPCSB01` magic, a big-endian length-delimited canonical manifest, a bounded constituent count, and manifest-order constituent frames. Each frame contains a length-delimited immutable selector, a big-endian byte length, the raw 32-byte SHA-256, and the exact admitted V5 bytes. The complete bundle is bounded to 64 MiB and 1024 constituents. Decoding rejects truncation, trailing bytes, malformed lengths, duplicate selectors or native identities, digest mismatch, non-canonical manifest bytes, association/order mismatch, and metadata/admission mismatch.

`captureset.Publisher` creates the final bundle inode no-replace beneath the pinned root, retains one open file capability, writes all bytes with no-progress rejection, fsyncs, seeks and rereads through that same handle, and performs canonical bundle, manifest/constituent bijection, digest, V5 admission, and ceiling verification before activation. It does not publish from a replaceable temporary pathname.

Darwin does not provide the Linux `linkat(AT_EMPTY_PATH)` unnamed-file installation contract used by an exact-handle link design. This implementation therefore uses the permitted capability-bound fallback on all supported platforms: the final bundle name is reserved first, but is non-valid and non-resolvable through the private API until one no-replace symlink activation entry is created atomically. The activation payload binds the verified file identity, digest, and length. Same-owner replacement of the reserved pathname after verification causes activation to fail; replacement after activation causes resolution to reject the binding. Raw directory enumeration can observe the inactive reserved filename, so the guarantee is API selector non-resolution before commit, not namespace-name secrecy.

The activation-entry creation is the atomic namespace commit point. An existing bundle or activation entry returns `TARGET_EXISTS`; neither is overwritten. Concurrent same-selector publication therefore has exactly one create winner. Post-commit activation cleanup is unnecessary, and directory-sync/close failures cannot reverse committed namespace success. A successful post-commit directory sync records only that the final directory was sync-requested; it is not a whole-filesystem, device, or crash-recovery guarantee.

Verification accepts only canonical bundle selectors, requires the bound activation entry, reads bounded exact bytes beneath the pinned root, decodes constituents only from those bundle bytes, strictly validates the manifest and constituent metadata, recomputes logical identity, and requires selector/identity equality. It never derives a constituent from sibling filesystem child names. This is integrity and local custody verification, not authentication or source truth.

The returned `PublicationReceipt` is `PRIVATE` by default. It records selector, exact bundle SHA-256, bundle byte length, constituent count, namespace atomicity, and `capability_bound_file_with_atomic_activation`; it is not a public receipt schema and conveys no additional authority.

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
- staged directory replacement: rejected because an opened directory handle cannot guarantee exact child names against same-owner concurrent child replacement.
- pathname-based hard-link or rename of a verified temporary file: rejected because the source pathname can be replaced after verification.
- rename-over-existing or check-then-write: rejected because they are not race-safe immutable publication.
- idempotent success for an existing equal artifact: rejected because it obscures which operation created custody; retries receive `TARGET_EXISTS`.
- preserving private identity on redaction: rejected because identity would falsely denote different bytes/semantics.
- registering the internal schema now: rejected because internal implementation existence is not sufficient justification for a public compatibility surface.

## Private inspection

The existing `inspect` CLI may compose this private verification API only through:

```text
lsp-trace inspect CAPTURE_SET_SELECTOR --private-capture-set-root ABSOLUTE_ROOT [--json]
```

Capture-set inspection is disabled unless the root flag is explicitly present. The root must be a cleaned absolute no-follow directory and the selector must be the canonical relative capture-set publication selector; the command opens one pinned `publication.Root` and delegates immutable verification to `captureset.Publisher.Verify`. It never accepts a manifest artifact path as capture-set custody.

Default output is a bounded human summary. `--json` emits the separate internal `lsp-trace.capture-set-inspection.v1` projection, not the manifest. Both expose only capture-set logical identity, disclosure, target/batch/constituent counts, closed file/symbol denominator and disposition counts, and the fixed authority/completeness/custody/CALLS/Leiden ceilings. They omit canonical seeds, the private root, constituent selectors and native identities, and ledger resource identities. This projection remains internal and is not registered with public schema-get, validation, or MCP surfaces.

## Consequences

Hosts receive a small in-process exact-byte API and must provision the pinned publication root and native V5 verifier independently. Private read-only CLI inspection is the sole added command composition. No MCP command, language-server acquisition, seed-file lane, arbitrary-path reader, cross-capture edge construction, or Leiden pipeline is added.
