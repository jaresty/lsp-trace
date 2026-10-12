# ADR0011 synthetic source pin review — 83e9c0e3de15f522

- Verdict: `REPIN_VERIFY_GO`
- Integration receipt: `20261007171209-0471`
- Committed predecessor: `adr0011-synthetic-source-pin.c835ea603a2fadda.manifest.json` (immutable)
- Selection: twelve authoritative directories; direct regular non-symlink `.go` files excluding `_test.go`
- Result: 119 files, 938,218 source bytes, no omitted, extra, duplicate, or symlink paths
- Aggregate digest: `sha256:ef367fe53147e145adc5173122dad8455209dbdf049927b131ddec83b8c611c0`
- Canonical manifest: compact sorted-key JSON plus one LF; 19,060 bytes; SHA-256 `83e9c0e3de15f52272ec37ebdd81b595afe93a525e566188557d9cbb406cf8b5`
- Production delta is bounded to `internal/adr0011acquisition/owner.go`, `proposal_context.go`, `proposal_record.go`, `raw_scanner.go`, and `scanner_record.go`; there are no production path additions or removals.
- Dedicated scanner implementation pin matches `raw_scanner.go`: `sha256:f8d62768798154c372e35fc2d3d43752fbd928aa7a247ec665868df2998adc4e`.
- Authority ceiling: source-byte evidence only; not build, binary, runtime, dependency closure, provider authentication, or live qualification.
