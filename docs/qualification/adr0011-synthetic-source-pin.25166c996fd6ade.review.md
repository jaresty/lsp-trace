# ADR0011 synthetic source pin review — 25166c996fd6ade

- Source HEAD: `42fe29db5cfc07f53617d273985c5a3e8bc9c6b3`
- Verifier: `REPIN_VERIFY_GO`
- Selection: the eleven authoritative `syntheticSourceDirectories`, direct regular non-symlink `.go` files only, excluding `_test.go`
- Result: 118 files, 928064 source bytes, no selection extras or errors
- Predecessor delta: exactly 2 additions, 5 modifications, and no removals
- Aggregate encoding: `ADR0011_SYNTHETIC_SOURCE_SET_V1\x00`, followed per sorted path by uint64-be path length, UTF-8 path bytes, uint64-be source byte length, and raw SHA-256 digest
- Aggregate digest: `sha256:28ece3dff6ff2cbdb5f502c4e97d27b220bf8ffaed2ba24932fa5dcabff4081a`
- Canonical manifest: compact sorted-key JSON plus one newline; 18915 bytes; SHA-256 `25166c996fd6adecc4cf21e15b7e7321f9172568b755e6e19616d500220bc505`
- Mechanical identities: every entry records the exact slash-relative path, byte length, and source-byte SHA-256; paths are unique and bytewise sorted
- Authority boundary: this is test-only source-byte evidence. It is not a binary identity, dependency closure, production pin, schema/public admission, provider authentication, publication authority, or live qualification.
- Source basis: accepted C17 capability/event accounting and accepted C18 composed-definition source changes at the stated HEAD.
