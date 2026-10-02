# ADR0011 synthetic source-pin successor — review packet

Status: proposed, synthetic-only. This is a source-byte expectation selected only by a private test; it grants no production, binary, runtime, dependency-closure, provider-authentication, live-qualification, or custody authority. Independent review remains outstanding.

## Immutable predecessor

- `docs/qualification/adr0011-synthetic-source-pin.f8cf96e649a5fc05.manifest.json`: SHA-256 `f8cf96e649a5fc051fa6475f8b68fe38bcc346e52fd459bb946d99951f57269a`, 90 entries, aggregate `sha256:b06eca5f8b40b96eee624e932e39c215144a7e2c14162e05cd5896eb83bf475c`. Preserved byte-for-byte; not superseded in place.

## New, separately selected expectation

- `docs/qualification/adr0011-synthetic-source-pin.7a5f3200f8b985cc.manifest.json`: 14,692 exact bytes, SHA-256 `7a5f3200f8b985cc6380b4d46e2bae4462e7bb55e2fd2c59964fb02c31490851`.
- Exactly 92 sorted path entries: all 90 original entries unchanged plus only `internal/adr0011acquisition/owner_private_builtin.go` (7,002 bytes, `sha256:cb43e865e4f28e9888fd7ac237eaea9e7a37d7fb0079b0f64bc438e49de501f5`) and `internal/adr0011acquisition/production_attempt_custody.go` (14,307 bytes, `sha256:3afa0b127599d0480d949367099d08f2ad73d85f4e5ae8a73da828aa05ed5459`).
- Aggregate `sha256:fff7a4b28496fa93a851d009293357893a61b7bfe8335ab82ae4f81b53ffed47` uses `ADR0011_SYNTHETIC_SOURCE_SET_V1\x00`, then for each sorted entry big-endian uint64 UTF-8 path-byte count, path bytes, big-endian uint64 file-byte count, raw 32-byte file SHA-256. `schema_version` stays `ADR0011_SYNTHETIC_SOURCE_PIN_V1`; the successor identity is its new digest-bearing filename and exact manifest bytes, not a new encoding.
- Independently checked each of 92 current file lengths and hashes against the new manifest, original entry equality, exact two-path set difference, sorted order, V1 aggregate, original and new manifest byte hashes. This is a checkout-time observation, not an executable or production custody claim.

## Test boundary

Only `internal/adr0011acquisition/synthetic_source_pin_review_test.go` changes its private selected filename, manifest SHA-256, aggregate and expected count (90 to 92). The supplied pre-change targeted RED was `TestReviewedSyntheticSourcePin` FAIL in `/Users/schwa/Library/Application Support/rtk/tee/1790639287_go_test.log`; this writer did not independently replay that historical log. Post-change targeted `GOPROXY=off GOSUMDB=off GOTOOLCHAIN=local go test ./internal/adr0011acquisition -run '^TestReviewedSyntheticSourcePin$' -count=1` passed (1 test, 1 package). Affected offline suite `GOPROXY=off GOSUMDB=off GOTOOLCHAIN=local go test ./internal/adr0011acquisition ./internal/adr0011builtinprofile ./internal/adr0011querytarget -count=1` reported 563 passed, 1 failed across 3 packages (retained log `/Users/schwa/Library/Application Support/rtk/tee/1790639848_go_test.log`): `TestADR0011OwnerManagedTwoEqualLocations` in `owner_darwin_test.go:387` reported `ASSERT_ADR0011_OWNER_MANAGED_CHAIN: receipt=<nil> err=ADR0011 managed acquisition not verified key_guard_reached=true owner_read_reached=true`. This is a distinct managed-chain assertion, not a source-pin failure; its cause has not been diagnosed or corrected here. No production `.go` files were edited by this successor task.
