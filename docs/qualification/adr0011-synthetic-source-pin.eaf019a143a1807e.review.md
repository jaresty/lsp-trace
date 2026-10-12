# ADR0011 synthetic source pin review — eaf019a143a1807e

- Verdict: `REPIN_VERIFY_GO`
- Integration receipt: `20261007134152-3493`
- Selection: twelve authoritative directories; direct regular non-symlink `.go` files excluding `_test.go`
- Result: 119 files, 936,058 source bytes, no omitted or extra paths, no selection errors
- Aggregate digest: `sha256:9e0e991e22863daeb36a209ae2e2e16c331d01f7e23197a463f81dc616f5b57f`
- Canonical manifest: compact sorted-key JSON plus one LF; 19,060 bytes; SHA-256 `eaf019a143a1807e3be86912877b6ca63001b102e347a155b55d4e10d1f16037`
- Predecessor `adr0011-synthetic-source-pin.74f597ab58e10985.manifest.json` remains byte-identical.
- Exact predecessor delta: only `internal/adr0011work/account.go`; no additions, removals, or other changed entries.
- The exported Go surface remains confined by the repository's `internal/` import boundary and carries no public product or schema authority.
- Authority ceiling: source-byte evidence only; not build, binary, runtime, dependency closure, provider authentication, or live qualification.
