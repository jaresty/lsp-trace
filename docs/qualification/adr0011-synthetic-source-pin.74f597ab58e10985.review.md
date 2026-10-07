# ADR0011 synthetic source pin review — 74f597ab58e10985

- Verdict: `REPIN_VERIFY_GO`
- Integration receipt: `20261007131717-8674`
- Selection: twelve authoritative directories; direct regular non-symlink `.go` files excluding `_test.go`
- Result: 119 files, 932,185 source bytes, no omitted or extra paths, no selection errors
- Aggregate digest: `sha256:32086207636424490095a6c3cdebd88176b3d858eb8e50e7058ea6d526457b62`
- Canonical manifest: compact sorted-key JSON plus one LF; 19,060 bytes; SHA-256 `74f597ab58e10985191874e138e09010492f9506a0d5b79bab8c3874a18189ce`
- Predecessor `adr0011-synthetic-source-pin.062b71047e2ead80.manifest.json` remains byte-identical.
- Exact predecessor delta: one added file, `internal/adr0011work/account.go`; exactly three modified files, `internal/adr0011acquisition/raw_scanner.go`, `scanner_record.go`, and `synthetic_source_pin.go`; no other changes.
- The dedicated scanner implementation pin equals the current `raw_scanner.go` digest `sha256:da7e8e90fbfc176c92bb9e7a7bc4469461cb14b5cbaffa911f05fb005349120e`.
- Adding `internal/adr0011work` to the selected directories expands the fail-closed denominator; it does not weaken source validation.
- Authority ceiling: source-byte evidence only; not build, binary, runtime, dependency closure, provider authentication, or live qualification.
