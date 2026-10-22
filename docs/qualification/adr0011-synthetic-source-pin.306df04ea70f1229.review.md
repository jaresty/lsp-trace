# ADR0011 synthetic source pin review — 306df04ea70f1229

- Source HEAD: `7614ac1fb60c919d2375cba59bef1b3f127760e2`
- Review disposition: independently accepted with no discrepancies
- Selection: 162 exact paths, 1555529 selected source bytes
- Aggregate digest: `sha256:5d014e76e7cc315533266510b616d859c9f74f66b68bd7470f5c50ddc7bf2d54`
- Canonical manifest: `docs/qualification/adr0011-synthetic-source-pin.306df04ea70f1229.manifest.json`; 25800 bytes; SHA-256 `306df04ea70f12298b33ee5e841bde606bbf0e9d8127e34f0d9c0c0cc1aeef59`
- Predecessor: `docs/qualification/adr0011-synthetic-source-pin.44c37bff0c596d9d.manifest.json`, unchanged
- Predecessor lineage: 157 unchanged, 1 changed, 4 added, and 0 removed paths
- Changed path: `internal/adr0011acquisition/synthetic_source_pin.go`
- Added paths: `internal/describeworker/config.go`, `internal/describeworker/runner.go`, `internal/describeworker/runner_v2.go`, and `internal/managedprocess/process.go`
- Entry identity: every selected entry exactly reproduced the accepted path, policy membership, order, source-byte SHA-256, source byte length, mode `100644`, regular-file status, and non-symlink status
- Candidate commit scope: `7614ac1fb60c919d2375cba59bef1b3f127760e2` changes only the canonical manifest

## Checks

- The accepted independent review result reported exact reproduction of manifest entry paths, policy membership, order, hashes, lengths, modes, regular-file status, and non-symlink status, with zero discrepancies.
- The manifest identity, manifest byte count, manifest SHA-256, selected path count, selected source-byte count, aggregate digest, predecessor-lineage counts, changed-path set, and added-path set matched the accepted independent review result.
- The predecessor manifest remained unchanged.
- Selectors pinned to stale manifest identities failed as expected.
- The candidate satisfies source-pin selector eligibility requirements.
- The independent review accepted the candidate; this artifact records that accepted result and does not claim that its recorder performed the independent review.

## Limitations

- Selector eligibility does not select or advance this candidate, mutate any selector, or grant publication, production, or broader qualification authority.
- Authority is limited to the selected source bytes, exact manifest mechanics, lineage, candidate commit scope, expected stale-selector failures, and selector eligibility recorded above.
- This review does not establish binary identity, dependency closure, runtime execution, semantic completeness, producer authenticity, provider authentication, production behavior, production admission, publication authority, selector advancement, or any broader qualification.
