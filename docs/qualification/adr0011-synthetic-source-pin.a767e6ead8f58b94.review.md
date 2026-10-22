# ADR0011 synthetic source pin review — a767e6ead8f58b94

- Source HEAD: `bc382c61e6314d9d9a96f62783396a1b1813cec0`
- Review disposition: independently accepted with no findings
- Selection: 157 exact paths, 1504500 selected source bytes
- Aggregate digest: `sha256:ed1b6e44521ad5d0dd64b3575c35f67ba1bcba542208646a14fa8caf80a66e0b`
- Canonical manifest: `docs/qualification/adr0011-synthetic-source-pin.a767e6ead8f58b94.manifest.json`; 25040 bytes; SHA-256 `a767e6ead8f58b947e9f8ebf3618d8724616ab14f46fb3554c40a466ced65d96`
- Predecessor lineage: 154 unchanged, 3 changed, 0 added, and 0 removed paths
- Changed paths: `cmd/lsp-trace/census_private_discovery_causes.go`, `cmd/lsp-trace/census_discovery_adapter.go`, and `internal/censusacquisition/discovery.go`
- Entry identity: every selected entry is an exact mode `100644` regular non-symlink path with its source byte length and source-byte SHA-256
- Preserved bounds: selection ceilings and qualification history are unchanged

## Checks

- The manifest identity, manifest byte count, manifest SHA-256, selected path count, selected source-byte count, aggregate digest, and predecessor-lineage counts matched the independently reviewed values.
- The changed-path set matched the three independently reviewed paths.
- Every manifest entry satisfied exact mode `100644`, regular-file, and non-symlink constraints.
- Selection ceilings and qualification history remained unchanged.
- The independent review reported no findings.

## Limitations

- Authority is limited to the selected source bytes and the exact manifest mechanics recorded above.
- This review does not establish binary identity, dependency closure, runtime execution, semantic completeness, producer authenticity, provider authentication, production behavior, production admission, publication authority, or any broader qualification.
