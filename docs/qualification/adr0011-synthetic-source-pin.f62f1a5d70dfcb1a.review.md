# ADR0011 synthetic source pin review — f62f1a5d70dfcb1a

- Verdict: `REPIN_VERIFY_GO`
- Integration receipt: `20261007200547-7439`
- Predecessor: `adr0011-synthetic-source-pin.65493e108d04b60f.manifest.json`; preserved byte-for-byte at SHA-256 `65493e108d04b60f2304fa298da019a0fece435b6eea02740581141f1b661118`.
- Selection: twelve authoritative directories; direct regular non-symlink `.go` files excluding `_test.go`.
- Canonical manifest: compact sorted-key JSON plus one LF; 19,061 bytes; SHA-256 `f62f1a5d70dfcb1ae8573fb07b744ad0c9e0a0ba0278cd74c3cc11e5bd6b23f1`.
- Source set: 119 files; 943,930 bytes; no added, removed, duplicate, omitted, extra, or symlink entries relative to the fixed selection.
- Aggregate digest: `sha256:8c2ee1b38ad2d80c30492623cb258885efa89dd50bec6bad27b253290879981e`.
- Exact predecessor production delta: `internal/adr0011work/account.go`, `sessionruntime/b4_capability_event_c17_private.go`, `sessionruntime/b4_definition_private.go`, and `sessionruntime/sessionruntime.go`; no production paths added or removed.
- Dedicated manager source pin matches `sessionruntime/b4_definition_private.go`: `sha256:c0fbc909be8d082a672a1e7193cddc6da6f043fe88b8fded7746bce992961d6d`; semantic-token checks remain intact.
- No dedicated capability-event or session-runtime source-byte pin required adjustment; no token check was removed or weakened.
- Current `_test.go` changes are excluded by the fixed selection rule.
- Authority ceiling: source-byte evidence only; not build, binary, runtime, dependency closure, producer authentication, public issuance, qualification, or matrix closure.
