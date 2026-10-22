# ADR0011 synthetic source pin review — 6c13fe3322a965fa

Status: INDEPENDENTLY REPRODUCED; ACCEPTED WITHIN SOURCE-PIN BOUNDARY; AUTHORITY 0

## Reviewed source pin

- Review checkout: `e78f5c6ea6106024bd3a362cdcca6364186432ad`
- Review checkout tree: `7de76f376127599dfeee1393feb71879bbd1ead9`
- Source-bearing parent revision: `f769c0872a2701b780aa4b2d35ba598ba5be75ab`
- Source-bearing parent tree: `116511054ea310faad6cd9cba29be192167fd270`
- Manifest: `adr0011-synthetic-source-pin.6c13fe3322a965fa.manifest.json`
- Manifest SHA-256: `6c13fe3322a965faa6eb70a155a54344e276c5eee6ca2937709f2acfb9cfd492`
- Manifest Git blob: `3e945a0acc9c18e72d19416777eedf8b1cb2b052`
- Manifest size: `26,119` bytes
- Aggregate digest: `sha256:93d1ed2e06a41da0b1eff066190d31004566f2e9c4e62006c8daa15e770d3ad5`
- Schema: `ADR0011_SYNTHETIC_SOURCE_PIN_V1`
- Ordered selection: `164` paths
- Selected source bytes: `1,657,003`
- Comparison result: zero discrepancies
- Candidate commit scope: one added manifest file

## Independent reproduction method

The reviewer independently parsed `internal/adr0011acquisition/synthetic_source_pin.go`, rather than accepting the candidate's file list. The selected set was reconstructed as the globally sorted non-test `.go` files directly within the twelve policy directories, followed in declared order by the thirty-two additional Go paths and four additional schema paths.

For every reconstructed path, the reviewer independently obtained the source-parent Git tree blob, stage-0 index blob and mode, and filesystem bytes. All `164` paths were regular stage-0 mode `100644`; source-parent, index, and filesystem bytes were equal for every path. No selected path was a test file.

The reviewer recomputed every byte length and SHA-256. The aggregate was independently rebuilt from `ADR0011_SYNTHETIC_SOURCE_SET_V1\x00`, then for each ordered entry an unsigned 64-bit big-endian path-byte length, UTF-8 path bytes, unsigned 64-bit big-endian file-byte length, and the raw 32-byte file SHA-256. The resulting aggregate exactly matched the manifest.

The reviewer independently serialized the reconstructed object as compact UTF-8 JSON with recursively sorted keys and exactly one final LF. Those bytes exactly matched the candidate. Their SHA-256 produced the filename's first sixteen hexadecimal characters, `6c13fe3322a965fa`.

## Lineage and immutability

The immutable predecessor is `adr0011-synthetic-source-pin.7afa01cb27caf911.manifest.json`, SHA-256 `7afa01cb27caf91114a2f76f7996c59c395dc98f41ce50769996969c412e0b93`. Its paired review SHA-256 is `d08f552f1fe7d06a04519200a685966c977c5b01a15f3d7404b3080fb708344c`. Both predecessor files are byte-identical between the source-bearing parent and the reviewed checkout.

Exact predecessor delta:

- unchanged: `161` paths;
- changed: `3` paths;
- added: `0` paths;
- removed: `0` paths.

Changed paths:

1. `cmd/lsp-trace/acquisition_v2.go`
2. `cmd/lsp-trace/census_command.go`
3. `cmd/lsp-trace/private_provider_probe.go`

## Commit-scope and exclusion checks

The clean reviewed manifest commit has exactly one changed path: the added candidate manifest. It contains no test-file, source-policy, implementation, selector, schema/policy, predecessor, or extra-path change. No candidate, predecessor, selector, implementation, policy, or schema file was modified during this review.

No hosted CI, push, provider/Census operation, runtime exercise, or production observation was performed. The review adds only this review record after acceptance.

## Authority limits and acceptance boundary

This review accepts only the exact policy-derived ordered source set, source-parent/index/filesystem byte equality, stage-0 modes, per-path lengths and hashes, aggregate framing and digest, canonical manifest bytes and filename, predecessor delta, and predecessor immutability stated above.

Authority remains `0`. This source pin makes no runtime, production, provider-authentication, identity, feature-identity, behavioral, dependency-closure, build, binary, inventory, Census-result, CI, release, deployment, or completeness claim. It does not advance selectors and does not authorize any production or externally consequential action.

## Disposition

Within that exact source-pin boundary, independent adversarial reproduction found zero discrepancies across `164` paths totaling `1,657,003` bytes, with lineage `161` unchanged, `3` changed, `0` added, and `0` removed. The manifest is accepted at authority `0`; all broader claims remain outside this review.
