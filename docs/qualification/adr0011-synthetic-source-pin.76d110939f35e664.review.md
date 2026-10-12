# ADR0011 synthetic source pin review — 76d110939f35e664

- Verifier: `REPIN_VERIFY_GO`
- Integration receipt: `20261007035526-3101`
- Selection: the eleven authoritative `syntheticSourceDirectories`, direct regular non-symlink `.go` files only, excluding `_test.go`
- Result: 118 files, 928,413 source bytes, no omitted or extra paths, no selection errors
- Aggregate encoding: `ADR0011_SYNTHETIC_SOURCE_SET_V1\x00`, followed per sorted path by uint64-be path length, UTF-8 path bytes, uint64-be source byte length, and raw SHA-256 digest
- Aggregate digest: `sha256:276e23928fc178234cf9d699d8b5fa1c63f7e835a961764e035d19f7e4372ecd`
- Canonical manifest: compact sorted-key JSON plus one newline; 18,915 bytes; SHA-256 `76d110939f35e66450128e4986870954c66d64c3d8f05bb7fe9e89f793603f10`
- Mechanical identities: every entry records the exact slash-relative path, byte length, and source-byte SHA-256; paths are unique and bytewise sorted
- Predecessor: `adr0011-synthetic-source-pin.25166c996fd6ade.manifest.json` remains byte-identical
- Predecessor delta: no additions or removals; exactly `sessionruntime/readiness_history_private.go` changed from 5,490 bytes / `sha256:3224441e1e496ec6349cba090e855ebaac06ebda154a166a56f6590f96b39c02` to 5,839 bytes / `sha256:d9d32d417ebd17d8e50a8f0c4e7f35fe931ff0f53cb2c144339cea02dc6ba1e0`
- Authority boundary: test-only source-byte evidence. It is not binary identity, dependency closure, production admission, provider authentication, publication authority, or live qualification.
