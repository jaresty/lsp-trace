# ADR0011 synthetic source pin review — d39be47a532db79e

**Verdict: `REPIN_VERIFY_GO` — independently reproduced within the source-pin boundary; authority 0.**

- Source HEAD: `016f6feea8f669b2176bf31d2c2eb6369af9753e`
- Candidate manifest: `adr0011-synthetic-source-pin.d39be47a532db79e.manifest.json`
- Manifest SHA-256: `d39be47a532db79e8ff126333b41ce7ca61537f7cc28cc948cda01d89efa534b`
- Aggregate: `sha256:a9daffb0e453239cf5a56e3c86840f46f14fc8d327d5161fe6ffec5442ac3832`
- Selected source: 177 paths, 1,783,479 bytes
- Canonical segments: 128 directory-selected Go files + 45 ordered additional Go files + 4 ordered schemas
- Predecessor: `adr0011-synthetic-source-pin.38487f72b1a9111e.manifest.json`
- Delta: 175 unchanged, 0 added, 2 changed, 0 removed
- Changed paths:
  - `internal/censusacquisition/core.go`
  - `internal/censussourceassembly/terminal.go`

Independent reconstruction hashed exact regular non-symlink source bytes, reproduced the production aggregate framing, and reproduced compact sorted-key JSON with one trailing newline byte-for-byte. The full manifest digest and filename prefix bind correctly. The predecessor content hash matches its filename, and its working-tree Git blob remains identical to the indexed blob.

Focused non-selector source-pin tests passed: 7 tests. Scoped `go vet` and `git diff --check` passed. The broader selector-bearing run reported 7 passes and exactly two expected failures: `TestReviewedSyntheticSourcePin` and `TestSyntheticSourcePinProducerCandidate`. Both remain explicitly bound to predecessor manifest `38487f72…` and aggregate `7f6f268f…`; this confirms the successor remains unselected rather than identifying a candidate defect.

This review accepts only bounded source-byte reproducibility. Authority is 0. It does not establish semantic correctness, build or binary identity, runtime behavior, dependency closure, producer authentication, live or production qualification, deployment suitability, selector advancement, publication, release acceptance, or permission to commit or push.
