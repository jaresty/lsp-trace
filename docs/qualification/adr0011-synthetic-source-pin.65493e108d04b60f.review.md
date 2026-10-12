# ADR0011 synthetic source pin review — 65493e108d04b60f

- Verdict: `REPIN_VERIFY_GO`
- Integration receipt: `20261007181024-2602`
- Predecessor: `adr0011-synthetic-source-pin.83e9c0e3de15f522.manifest.json`; preserved byte-for-byte at SHA-256 `83e9c0e3de15f52272ec37ebdd81b595afe93a525e566188557d9cbb406cf8b5`.
- Selection: twelve authoritative directories; direct regular non-symlink `.go` files excluding `_test.go`.
- Manifest: 19,060 bytes; SHA-256 `65493e108d04b60f2304fa298da019a0fece435b6eea02740581141f1b661118`.
- Source set: 119 files; 940,751 bytes; no missing, extra, duplicate, or symlink entries.
- Aggregate digest: `sha256:c2437937f99e1497c247d20c4c428d8bfc08cf842bfcf6bd4f9c63038cb3b242`.
- Exact predecessor production delta: `internal/adr0011work/account.go` and `sessionruntime/b4_definition_private.go`; no production paths added or removed.
- Dedicated manager boundary: `sessionruntime/b4_definition_private.go`, 47,437 bytes, SHA-256 `7ffabc3e9389da44e300ea95139ce77bbad980afe8976a4ff8e716755d731147`; semantic-token denominator unchanged.
- Current `_test.go` changes are excluded by the fixed selection rule.
- Authority ceiling: source-byte evidence only; not build, binary, runtime, dependency closure, producer authentication, public issuance, qualification, or matrix closure.
