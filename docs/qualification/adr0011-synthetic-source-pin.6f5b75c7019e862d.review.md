# ADR0011 synthetic source pin review — 6f5b75c7019e862d

- Verdict: `REPIN_VERIFY_GO`
- Integration receipt: `20261007212118-4708`
- Predecessor: `adr0011-synthetic-source-pin.f62f1a5d70dfcb1a.manifest.json`; preserved byte-for-byte at SHA-256 `f62f1a5d70dfcb1ae8573fb07b744ad0c9e0a0ba0278cd74c3cc11e5bd6b23f1`.
- Selection: twelve authoritative directories; direct regular non-symlink `.go` files excluding `_test.go`.
- Canonical manifest: compact sorted-key JSON plus one LF; 19,061 bytes; SHA-256 `6f5b75c7019e862d9627a94205eceffdfedfddbfd5e55ece453474e00b2581f5`.
- Source set: 119 files; 947,945 bytes; no added, removed, duplicate, omitted, extra, or symlink entries relative to the fixed selection.
- Aggregate digest: `sha256:54a509f15bc0128e26bbcdaf5919e1a5214d9f0893081a936925ec7af8c5fa38`.
- Exact predecessor production delta: `internal/adr0011work/account.go` and `sessionruntime/sessionruntime.go`; no production paths added or removed.
- Dedicated manager source pin remains matched to unchanged `sessionruntime/b4_definition_private.go`: `sha256:c0fbc909be8d082a672a1e7193cddc6da6f043fe88b8fded7746bce992961d6d`; semantic-token checks remain intact.
- No dedicated session-runtime source-byte pin required adjustment; no token check was removed or weakened.
- Current `_test.go` changes, including document test ownership, are excluded by the fixed selection rule.
- Authority ceiling: source-byte evidence only; not build, binary, runtime, dependency closure, producer authentication, public issuance, qualification, stage, or matrix closure.
