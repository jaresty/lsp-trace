# Accepted expanded write-set

The proposed patch may name only these seven production paths:

1. `sessionruntime/call_hierarchy_directional_raw_v1_private.go` — new private partition/lease/Stage A hooks.
2. `sessionruntime/sessionruntime.go` — Manager initialization, accepted-owner binding, Shutdown terminal release.
3. `sessionruntime/b4_explicit_bytes_c15_private.go` — real account-owned directional partition state; legacy general layout and cap unchanged.
4. `sessionruntime/b4_owned_method_pair_lease.go` — refused-owner binding and existing owner terminal splice.
5. `sessionruntime/b4_c15_ownership_registry_private.go` — permitted but byte-identical; Stage B only after separate authority.
6. `internal/lspwire/read_frame.go` — permitted but byte-identical.
7. `internal/lspwire/successor_ingress.go` — permitted but byte-identical.

The exact patch changes only paths 1–4. No production test path is changed. Focused probes are generated only in the verifier’s disposable checkout and are not part of `PROPOSED.patch`.

Any additional production, test, public API, dependency, schema, profile, registry, CLI/MCP, config, build-tag, history, predecessor, or lineage path is `BLOCKED_WRITESET_EXPANSION_REQUIRED`.

AUTHORED_NOT_GO
