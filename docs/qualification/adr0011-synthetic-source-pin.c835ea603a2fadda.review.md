# ADR0011 synthetic source pin review — c835ea603a2fadda

- Verdict: `REPIN_VERIFY_GO`
- Integration receipt: `20261007144633-1116`
- Selection: twelve authoritative directories; direct regular non-symlink `.go` files excluding `_test.go`
- Result: 119 files, 936,354 source bytes, no omitted or extra paths, no selection errors
- Aggregate digest: `sha256:aaa87d849ff4235a7d73797f3d9dadb5d2cd87888d5583b498938e927d4416e1`
- Canonical manifest: compact sorted-key JSON plus one LF; 19,060 bytes; SHA-256 `c835ea603a2faddac4e2bd5e9726884927ef645de368d4b6a67f334c8de2e335`
- Predecessor `adr0011-synthetic-source-pin.eaf019a143a1807e.manifest.json` remains byte-identical.
- Exact predecessor delta: only `sessionruntime/sessionruntime.go`; no additions, removals, or other changed production entries.
- Test, verifier, selector, packet, and historical snapshot files are outside the production-source denominator.
- Authority ceiling: source-byte evidence only; not build, binary, runtime, dependency closure, provider authentication, or live qualification.
