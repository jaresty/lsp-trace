# ADR0011 synthetic source pin review — aeaa6398c681f1cb

- Source branch: `backup/main-2026-10-08-d1f8ffec`
- Verifier: focused mechanical regeneration and row-by-row DuckDB comparison
- Selection: the eleven authoritative `syntheticSourceDirectories`, direct regular non-symlink `.go` files only, excluding `_test.go`
- Result: 120 files, 938909 source bytes, no selection extras or errors
- Predecessor delta: exactly 2 additions, 5 modifications, 113 unchanged, and no removals
- Added: `internal/publication/boundfile_cas.go`, `internal/publication/boundfile_cas_other.go`
- Modified: `internal/publication/selector.go`, `internal/publication/verifiable.go`, `sessionruntime/b4_explicit_bytes_c15_private.go`, `sessionruntime/diagnostic_backing_c15_private.go`, `sessionruntime/diagnostic_history_storage_c15_private.go`
- Aggregate encoding: `ADR0011_SYNTHETIC_SOURCE_SET_V1\x00`, followed per sorted path by uint64-be path length, UTF-8 path bytes, uint64-be source byte length, and raw SHA-256 digest
- Aggregate digest: `sha256:2d255c2e575c26c615d5c3496ca0369c6fc59638c021ec3e5044a4a9afefc416`
- Canonical manifest: compact sorted-key JSON plus one newline; 19222 bytes; SHA-256 `aeaa6398c681f1cb4d83e33971278dc954ccd35b186d47c35c6ffcb770a7f348`
- Mechanical identities: every entry records the exact slash-relative path, byte length, and source-byte SHA-256; paths are unique and bytewise sorted
- Authority boundary: this is test-only source-byte evidence. It is not a binary identity, dependency closure, production pin, schema/public admission, provider authentication, publication authority, or live qualification.
