# ADR0011 synthetic source pin review — 7afa01cb27caf911

Status: INDEPENDENTLY REPRODUCED; ACCEPTED WITHIN SOURCE-PIN BOUNDARY; AUTHORITY 0

## Reviewed source pin

- Review checkout: `6cd0c90022b74c42e2ae3c7417862e6030805190`
- Review checkout tree: `f45b451d2d50c206547986f4f2939882948ac00e`
- Source-bearing revision: `b2492e5e35186dcf2c1db67bb27979f06e615bfc`
- Source-bearing tree: `61b24bd54628059a76bc06bca9a5204adc79b9b9`
- Manifest: `adr0011-synthetic-source-pin.7afa01cb27caf911.manifest.json`
- Manifest SHA-256: `7afa01cb27caf91114a2f76f7996c59c395dc98f41ce50769996969c412e0b93`
- Manifest Git blob: `6baf1f03f4f1416d96365c16c1002941625529a4`
- Manifest size: `26,119` bytes
- Aggregate digest: `sha256:f613563993c80b5358c6f3a82eca8e0a3517c42dbd2cc31c1580ccfcd23c541a`
- Schema: `ADR0011_SYNTHETIC_SOURCE_PIN_V1`
- Ordered selection: `164` paths
- Selected source bytes: `1,648,707`
- Comparison result: zero discrepancies
- Candidate commit scope: one added manifest file

## Independent reproduction

The reviewer independently parsed the checked-in source policy, enumerated its eligible non-test Go files and schema paths, and reread every selected file from the exact checkout.

The review reproduced:

- the exact path set and prescribed order;
- every selected byte length and SHA-256;
- tracked stage-0 mode `100644` for every selected path;
- equality between index blobs and filesystem bytes;
- zero selected `_test.go` paths;
- inclusion of the source-pin policy source;
- the length-prefixed V1 aggregate digest;
- canonical compact JSON with sorted keys and one final LF;
- the canonical filename prefix derived from the manifest SHA-256.

## Lineage

The immutable predecessor is `adr0011-synthetic-source-pin.035360f4979f6f3f.manifest.json`, SHA-256 `035360f4979f6f3feb9ab54af8e59fda0dfd65bbb452476a33e78359e5dc215d`. Its Git blob remained unchanged.

Exact lineage:

- unchanged: `159` paths;
- changed: `4` paths;
- added: `1` path;
- removed: `0` paths.

Changed paths:

1. `cmd/lsp-trace/acquisition_v2.go`
2. `cmd/lsp-trace/census_command.go`
3. `internal/adr0011acquisition/synthetic_source_pin.go`
4. `sessionruntime/sessionruntime.go`

Added path:

- `cmd/lsp-trace/private_provider_probe.go`

## Guard status

At the review checkout, candidate/source-policy/ceiling guards passed. The two predecessor selector guards remained expected RED because they still selected the immutable predecessor. Selector advancement and post-selector GREEN verification are separate steps.

## Authority limits

This review accepts only the manifest's exact source bytes, ordering, lengths, hashes, aggregate, and lineage. It grants no binary, dependency, provider, runtime, Census-result, inventory, selector, CI, release, deployment, feature-identity, completeness, acceptance, or production authority.

The predecessor, rejected candidates, prior reviews, and all historical source-pin artifacts remain immutable. This review does not rewrite or supersede them.

## Disposition

Within the exact manifest boundary above, independent reproduction found zero discrepancies across `164` paths totaling `1,648,707` bytes, with lineage `159` unchanged, `4` changed, `1` added, and `0` removed.

Selector advancement is permitted only as a separate test-only commit followed by exact-head verification. Authority remains `0`.
