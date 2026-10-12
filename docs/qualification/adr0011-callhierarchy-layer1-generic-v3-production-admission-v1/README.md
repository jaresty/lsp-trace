# ADR-0011 call-hierarchy Layer 1 generic v3 production-admission evidence

**Status: `REVIEW_REQUIRED`**

This is a durable, fail-closed evidence package. It has **no production, merge, release, deployment, acceptance, approval, or implementation authority**. It does not establish runtime frequency, production execution, allocator behavior, or authority-qualified behavior.

The package preserves the accepted four-file patch, GREEN verification evidence, exhaustive C15 enumeration/deltas, final semantic adjudication, and cleanup receipt. All source evidence is copied into this directory; `/tmp` provenance locators are historical provenance only and are not required to verify the package.

See [`ADMISSION.md`](ADMISSION.md) for gate interpretation and [`MANIFEST.json`](MANIFEST.json) for machine-readable provenance, counts, hashes, and authority ceilings.

## Integrity

Run from this directory:

```sh
sha256sum -c SHA256SUMS
```

`SHA256SUMS` covers every regular package file except `SHA256SUMS` itself, using paths relative to this directory, sorted bytewise (`LC_ALL=C`). This avoids checksum self-reference. `MANIFEST.json` inventories evidence and authored package documents; its own digest is supplied by `SHA256SUMS`.
