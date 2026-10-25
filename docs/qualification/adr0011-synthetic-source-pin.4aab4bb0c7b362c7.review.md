# ADR0011 synthetic source pin review — 4aab4bb0c7b362c7

**Verdict: `REPIN_VERIFY_GO` — independently reproduced within the source-pin boundary; authority 0.**

- Exact pilot commit: `0da37247d3d3daefe213454f6a4639aa5f650fe1`
- Exact pilot tree: `12fcfe0b0d1c298eed40b1f161df4c16cb377bea`
- Candidate manifest: `adr0011-synthetic-source-pin.4aab4bb0c7b362c7.manifest.json`
- Manifest SHA-256: `4aab4bb0c7b362c779098f395be86d1027a36d74cfbc0cf52c4a0abc488d6a89`
- Manifest size: 29,282 bytes
- Aggregate: `sha256:7b0fe344ac7d34fee7101d998b1f958f22b71d08f9060a104066732c6017d4a6`
- Selected source: 184 paths, 1,872,841 bytes
- Canonical segments: 128 directory-selected Go files + 50 ordered additional Go files + 6 ordered schemas
- Predecessor: `adr0011-synthetic-source-pin.012e7bf737ced062.manifest.json`, unchanged at SHA-256 `012e7bf737ced062e586acf6d8d69692c3e4d2f9163081235e1679fa9543b7cf`
- Delta: 177 unchanged, 5 added, 2 changed, 0 removed
- Changed paths:
  - `internal/adr0011acquisition/synthetic_source_pin.go`
  - `internal/censussourceassembly/stores.go`
- Added paths, in required order:
  - `cmd/lsp-trace/programc_transition_private.go`
  - `internal/censuscontinuation/programc_authoritative.go`
  - `internal/censuscontinuation/programc_transition.go`
  - `internal/programctransitionhost/host.go`
  - `internal/programc/instability.go`

Independent reconstruction enumerated every direct non-test Go file in the twelve authoritative directories, then appended the exact ordered additional Go and schema paths from current `synthetic_source_pin.go`. It rejected tests, aliases, traversal, duplicates, omissions, non-regular files, symlink leaves, and symlink ancestors. It hashed the exact current bytes of every selected path, reproduced the `ADR0011_SYNTHETIC_SOURCE_SET_V1\x00` aggregate with big-endian uint64 path and byte-length framing, and reproduced compact sorted-key JSON with exactly one trailing LF byte-for-byte. The full manifest digest, filename prefix, aggregate, count, and selected-byte total all match.

The five added production paths are the complete new Program C transition chain within the established source-pin policy: private CLI seam → production host → authoritative retained-input plan and transition coordinator → Program C v2 instability implementation. Their corresponding tests and near-name aliases are not production inputs and remain excluded. No additional implementation path required by the established explicit directory/additional-path/schema scope is omitted.

This review is additive. The candidate manifest and this review are new artifacts; no predecessor manifest or review changed. The producer-owned changes to `synthetic_source_pin.go` and `synthetic_source_pin_test.go` were not authored or altered by this reviewer. The exact pilot commit and tree identify the checked-out base, while the candidate manifest binds the selected current working-tree bytes, including the two changed predecessor paths and five added production paths.

Focused policy/candidate tests passed before selector advancement. The selected review test, complete `internal/adr0011acquisition` package, final issuance/owner tests, formatting, and diff checks are acceptance gates after the test-owned selector is bound to this review.

This review accepts only bounded source-byte reproducibility. Authority is 0. It does not establish semantic correctness, build or binary identity, runtime execution, transitive dependency closure beyond the established source-pin policy, source-graph completeness, producer authentication, feature identity, ownership, permission, production qualification, deployment suitability, publication, release acceptance, or authority escalation.
