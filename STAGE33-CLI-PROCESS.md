# Stage 3.3 CLI process finalization

## Scope

Stage 3 replaces the Stage 2 zero-length graph-provenance binding with the exact finalized public V3 byte sequence. The private lifecycle document records the public schema, byte length, SHA-256 digest, and an optional publication digest. Empty public bytes are rejected.

This work is CLI-private. It adds no MCP operation, public schema-registry entry, collector input, public opt-in field, deployment behavior, product behavior, or live D01 claim. D01 was not run.

## Ordering and failure precedence

The CLI first obtains and validates the public graph-provenance V3 artifact, applies optional pretty rendering, and writes or immutably publishes those final bytes. Only after that succeeds does it ask the manager for an exact-attempt, exact-generation certified diagnostic snapshot set, project the private lifecycle document, validate it against the same final public bytes, and publish through the hardened sink.

Private projection or publication failure is secondary and emits only `private request diagnostics unavailable`; it does not alter public stdout, public bytes, or the public exit status.

## Private selector and sink

`--private-request-diagnostic-root` and `--private-request-diagnostic-selector` are accepted only as a complete pair on acquisition V3. V1 and V2 reject the pair. The root must be absolute and private. The selector must be relative and remain beneath the root. Publication reuses `manageddiagnostic.PublishHardened`: root-descriptor-relative access, private parent directories, an exclusive 0600 temporary file, content revalidation, no-replace linking, file sync, and directory sync. Startup diagnostics remain independently available and unchanged.

The runtime adapter retains only opaque manager-issued diagnostic handles. It does not retain caller-created event arrays or build a second request collector. `DiagnosticSnapshotSetFor` rejects forged, stale, cross-attempt, duplicate, open, and over-bound sources.

## Bounds and omission

The lifecycle contract remains exactly bounded at 64 retained records, 65,536 encoded bytes, and 4,096 bytes per retained string. Projection accounts for manager omissions and projection-only dropped detail. Over-bound source sets fail closed; details are omitted rather than truncated into ambiguous values.

## Offline validation

`lsp-trace validate-private-request-diagnostics PRIVATE_PATH PUBLIC_V3_PATH` validates the private lifecycle document only after checking its exact public V3 length and SHA-256 binding. The historical one-path private diagnostic validator remains accepted for compatibility. This command is local and does not expose a public schema registry.

## Focused process evidence

The committed real-CLI/fake-LSP process guard builds both executables, establishes successful initialize and document-symbol readiness, forces a prepare-call-hierarchy deadline, and checks the resulting private lifecycle correlation and timeout terminal. It also verifies unchanged valid public V3 stdout, generic stderr, absence of the raw document URI, 0600 no-replace publication, and no artifact without the private selector pair.

Existing focused runtime guards retain matched and unmatched correlation, late-after-deadline handling, malformed framing, process exit, write failure, startup diagnostics, seed binding, MCP25 compatibility, and FR22 qualification coverage. No full suite, full race, public MCP call, network access, installation, deployment, product action, or live D01 run is claimed.
