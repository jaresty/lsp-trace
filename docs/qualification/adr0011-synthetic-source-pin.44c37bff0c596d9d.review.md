# ADR0011 synthetic source pin review — 44c37bff0c596d9d

- Source HEAD: `dfe85d90d178c3bc76181a1fdcbe23da2b48369f`
- Review disposition: independently accepted with no discrepancies
- Selection: 158 exact paths, 1515894 selected source bytes
- Aggregate digest: `sha256:4fbd9685d34036ede16cddc29dbf0e6d32c94ae950953b232a31dc264ff607fb`
- Canonical manifest: `docs/qualification/adr0011-synthetic-source-pin.44c37bff0c596d9d.manifest.json`; 25206 bytes; SHA-256 `44c37bff0c596d9dea1ef9e31bf8c22531511277413336aaf3b810943a0ddbc9`
- Predecessor: `docs/qualification/adr0011-synthetic-source-pin.a767e6ead8f58b94.manifest.json`, unchanged
- Predecessor lineage: 150 unchanged, 7 changed, 1 added, and 0 removed paths
- Changed paths: `cmd/lsp-trace/census_assembly.go`, `cmd/lsp-trace/census_core.go`, `cmd/lsp-trace/census_discovery_adapter.go`, `cmd/lsp-trace/census_runner.go`, `internal/adr0011acquisition/synthetic_source_pin.go`, `internal/censusacquisition/discovery.go`, and `sessionruntime/sessionruntime.go`
- Added path: `cmd/lsp-trace/private_census_terminal_observation.go`
- Entry identity: every selected entry exactly reproduced the accepted path, policy membership, order, source-byte SHA-256, source byte length, mode `100644`, regular-file status, and non-symlink status
- Candidate commit scope: `dfe85d90d178c3bc76181a1fdcbe23da2b48369f` changes only the canonical manifest

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
