# ADR0011 synthetic source pin review — d894fac416e29a01

**Verdict: ACCEPT — independently reproduced within source-pin boundary; authority 0.**

## Identity

- HEAD: `68b314784e2177673e699bfbbb3317b374b86eb4`
- Parent: `7c1898f2d7859eb9ec60facbf699db2ea841d1ee`
- Tree: `7aaecace7c3179f115b5f3736f401597d03dd9f6`
- Manifest: `docs/qualification/adr0011-synthetic-source-pin.d894fac416e29a01.manifest.json`
- Manifest SHA-256: `d894fac416e29a01ae87c2f4fbdf70ffc4cd99470cd9b18a2e6e186410ef2e53`
- Manifest bytes: `26,119`
- Manifest Git blob: `d0af0aa374ae64ddb97c8a2006385f11b9540cfa`
- Aggregate: `sha256:71f4d81cd3b6cc3ecd46cd37df98a711acb757077c8b1fc218d39dafdf0cd664`
- Schema: `ADR0011_SYNTHETIC_SOURCE_PIN_V1`

## Independent reproduction

The authoritative selection was independently transcribed from `internal/adr0011acquisition/synthetic_source_pin.go`:

- 128 globally sorted direct, regular, non-symlink `.go` files excluding `_test.go` from the twelve authoritative directories;
- 32 additional Go paths in declared order;
- four additional schema paths in declared order.

Result: `164` unique paths and `1,659,060` selected bytes. Exact path order, coverage, byte lengths, and every per-file SHA-256 matched the candidate manifest. There were no omissions, extras, duplicates, test files, symlink leaves, or symlink ancestors. All selected index entries were mode `100644`; all index blobs matched filesystem bytes.

The aggregate was rebuilt from `ADR0011_SYNTHETIC_SOURCE_SET_V1\x00`, followed for each ordered entry by big-endian uint64 path-byte length, UTF-8 path bytes, big-endian uint64 file-byte length, and raw 32-byte file SHA-256.

Compact recursively sorted-key UTF-8 JSON plus exactly one final LF reproduced the candidate byte-for-byte. Its SHA-256 matches the filename prefix.

## Limits and safety

- Count: `164 / 256`
- Total bytes: `1,659,060 / 16,777,216`
- Largest file: `154,095 / 2,097,152`
- Actual selected symlinks: none
- The verifier rejects unclean, absolute, traversal, backslash, NUL, duplicate/order-invalid, non-regular, oversized, and symlink-leaf/ancestor inputs.
- Focused ceiling and safety tests passed.

## Lineage

Against immutable predecessor `adr0011-synthetic-source-pin.6c13fe3322a965fa.manifest.json`:

- unchanged: `163`
- changed: `1`
- added: `0`
- removed: `0`

Changed path:

- `cmd/lsp-trace/census_assembly.go`
  - before: `12,702` bytes, `sha256:7cbe9b6dd47f5cd5dcfceb0f71a306a70326260e9844e5078d38853bcec40a66`
  - after: `14,759` bytes, `sha256:280663b1d2a4caaa385e9fdafe2c74e29d6ba06d7ddeafe6bab0cc16f303fbc0`

HEAD adds only the candidate manifest. The predecessor manifest remains SHA-256 `6c13fe3322a965faa6eb70a155a54344e276c5eee6ca2937709f2acfb9cfd492`.

## Checks

- Independent Python enumeration, hashing, aggregate framing, canonical serialization, and lineage comparison
- `git ls-files -s` / `git hash-object` equality for all selected paths
- `git diff --check`
- clean `git status --porcelain=v1`
- `shasum -a 256` over candidate and predecessor artifacts
- `go test ./internal/adr0011acquisition -run '^TestSyntheticSourcePin($|AdditionalProductionPaths$|CeilingsUnchanged$)' -count=1` — PASS

The broader `^TestSyntheticSourcePin` run also executed the still-predecessor-pinned `TestSyntheticSourcePinProducerCandidate`, which failed as expected because selection had not advanced from `6c13fe…`. This does not contradict the independently reproduced candidate; it confirms the candidate was unselected at review time.

## Authority ceiling

Unchanged:

```text
authority=0
accepted=false
featureIdentity=UNRESOLVED
completeness=UNKNOWN
effects=NONE
reviews=NOT_REQUESTED
implementationDecision=UNISSUED
production_admission=REFUSED_MISSING_EVIDENCE
```

Acceptance is limited to exact source selection, bytes, hashes, aggregate framing, canonical manifest identity, limits, symlink safety, and stated lineage. It establishes no build, binary, dependency closure, runtime behavior, provider authentication, production behavior, feature identity, completeness, deployment, or production admission.
