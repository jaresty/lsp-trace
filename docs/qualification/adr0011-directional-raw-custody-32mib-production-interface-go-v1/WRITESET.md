# Frozen write set

The proposed patch may name only these six production paths:

1. `sessionruntime/call_hierarchy_directional_raw_v1_private.go` (new)
2. `sessionruntime/sessionruntime.go`
3. `internal/lspwire/read_frame.go` (permitted, byte-identical)
4. `internal/lspwire/successor_ingress.go` (permitted, byte-identical)
5. `sessionruntime/b4_owned_method_pair_lease.go`
6. `sessionruntime/b4_c15_ownership_registry_private.go` (permitted, byte-identical pending postimplementation enumeration)

The exact patch changes only paths 1, 2, and 5. No test path was needed. Any additional production/test path is `BLOCKED_WRITESET_EXPANSION_REQUIRED`; any public/dependency expansion is `BLOCKED_PUBLIC_DEPENDENCY`.

The package itself is the only worktree change. Proposal generation and verification use fresh disposable exact-HEAD clones.

Disposition: `AUTHORED_NOT_GO`.
