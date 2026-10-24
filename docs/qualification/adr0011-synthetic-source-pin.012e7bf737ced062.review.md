# ADR0011 synthetic source pin review — 012e7bf737ced062

**Verdict: `REPIN_VERIFY_GO` — independently reproduced within the source-pin boundary; authority 0.**

- Base source HEAD: `22aa9c694dc99d305894b1573d50e0b457ee6f6a`
- Candidate manifest: `adr0011-synthetic-source-pin.012e7bf737ced062.manifest.json`
- Manifest SHA-256: `012e7bf737ced062e586acf6d8d69692c3e4d2f9163081235e1679fa9543b7cf`
- Aggregate: `sha256:fa74cd343e0de7f067f000718592a2e09b3f25a1ad1e4f94c277a37425d16dd8`
- Selected source: 179 paths, 1,795,316 bytes
- Canonical segments: 128 directory-selected Go files + 45 ordered additional Go files + 6 ordered schemas
- Predecessor: `adr0011-synthetic-source-pin.d39be47a532db79e.manifest.json`
- Delta: 175 unchanged, 2 added, 2 changed, 0 removed
- Changed paths:
  - `internal/adr0011acquisition/synthetic_source_pin.go`
  - `internal/schema/schema.go`
- Added paths:
  - `internal/schema/schemas/lsp-trace.community-instability.v1.schema.json`
  - `internal/schema/schemas/lsp-trace.community-instability.v2.schema.json`

Independent reconstruction hashed every selected exact regular non-symlink source file and reproduced the production aggregate framing. It reproduced canonical compact JSON with exactly one trailing newline byte-for-byte. The full manifest digest and filename prefix bind correctly. Exact policy path order contains no missing, unexpected, or duplicate paths; the community-instability schemas occur in canonical v1-then-v2 order.

The v1 schema remains byte-identical at `sha256:8c581786a7b3371e42623ab4b8b6bd6594f1d0458b5b7092fcffdc23d5dbcef6`. The v2 schema is `sha256:f400d19d9b561cbad8bb0573640114492e12ce38ce8ea9fee9fbe3e5f7adb71b`, and its strict delta from v1 is exactly `/$id`, `/properties/claim_ceiling/const`, and `/properties/schema_version/const`. The predecessor manifest remains immutable and matches its filename identity. File, aggregate, and count ceilings remain 2 MiB, 16 MiB, and 256.

Policy/order, canonical encoding, aggregate framing, source-byte, ceiling, predecessor-immutability, and delta tests passed. `git diff --check` passed. The selector-bearing tests remain intentionally bound to predecessor `d39be47a…` and therefore fail in this intermediate state; selector advancement is a separate next transition.

This review accepts only bounded source-byte reproducibility. Authority is 0. It does not establish semantic correctness, build or binary identity, runtime behavior, dependency closure, provider authentication, feature identity, completeness, production qualification, deployment suitability, publication, release acceptance, or permission for any authority escalation.
