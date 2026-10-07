# ADR0011 synthetic source pin review — 17e4a67fbebd0227

- Verdict: `REPIN_VERIFY_GO`
- Integration receipt: `20261007223225-1205`
- Predecessor: `adr0011-synthetic-source-pin.6f5b75c7019e862d.manifest.json`; preserved byte-for-byte at SHA-256 `6f5b75c7019e862d9627a94205eceffdfedfddbfd5e55ece453474e00b2581f5`.
- Selection: twelve authoritative directories; direct regular non-symlink `.go` files excluding `_test.go`.
- Canonical manifest: compact sorted-key JSON plus one LF; 19,061 bytes; SHA-256 `17e4a67fbebd0227488515f90562d05b2981ccf77a3978ac5a0a860d3efce80f`.
- Source set: 119 files; 950,476 bytes; no added, removed, duplicate, omitted, extra, or symlink entries relative to the fixed selection.
- Aggregate digest: `sha256:0f605406e07d2f70e6765f9820a2c5677c6087744a60e6b318ce4631bd73d098`.
- Exact predecessor production delta: `internal/adr0011work/account.go`, `sessionruntime/b4_definition_private.go`, and `sessionruntime/sessionruntime.go`; no production paths added or removed.
- `sessionruntime/sessionruntime.go` wires the decoded-message clone allocation owner to the exact transaction work account and identity, the required second successor-owner construction site.
- Dedicated manager source pin matches `sessionruntime/b4_definition_private.go`: `sha256:6eefd0a89f278844bb9a15806d89b59ff50145b02f7789938182a46a8fb56712`; semantic-token checks remain intact.
- No other dedicated source-byte pin required adjustment; no token check was removed or weakened.
- Current `_test.go` changes are excluded by the fixed selection rule.
- Authority ceiling: source-byte evidence only; not build, binary, runtime, dependency closure, producer authentication, public issuance, qualification, stage, or matrix closure.
