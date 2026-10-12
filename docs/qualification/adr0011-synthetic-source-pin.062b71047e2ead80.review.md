# ADR0011 synthetic source pin review — 062b71047e2ead80

- Verdict: `REPIN_VERIFY_GO`
- Integration receipt: `20261007051714-2645`
- Selection: eleven authoritative directories; direct regular non-symlink `.go` files excluding `_test.go`
- Result: 118 files, 928,503 source bytes, no omitted or extra paths, no selection errors
- Aggregate digest: `sha256:743de2f8a0ebdc91349e96578661c81688f186df60bfc860918603e53642d54b`
- Canonical manifest: compact sorted-key JSON plus one LF; 18,915 bytes; SHA-256 `062b71047e2ead804516bc5d4baf019899c3a714426a5bd0d0d908d85931dd1c`
- Predecessor `adr0011-synthetic-source-pin.76d110939f35e664.manifest.json` remains byte-identical.
- Exact predecessor delta: no additions/removals; 116 unchanged entries; exactly `sessionruntime/sessionruntime.go` and `sessionruntime/startup_attempt.go` changed.
- `sessionruntime/sessionruntime.go`: 125,260 / `sha256:fbf899113983a9728516cad538e9087e04982e1c0b96909f1696d254758081c0` → 125,274 / `sha256:f4d2877d9e3612a8894f795450416e6607b99223eb0a607979e444e635398718`.
- `sessionruntime/startup_attempt.go`: 3,906 / `sha256:afc8b56b0c03e95f1a7c4485101f293ece54e0b307cb4ea7f9be2c1d82033383` → 3,982 / `sha256:9c35d9e507189257bcd25605c65f00f5ddcaa3529371729383f55bcbd0f57ec6`.
- Authority ceiling: source-byte evidence only; not build, binary, runtime, dependency closure, provider authentication, or live qualification.
