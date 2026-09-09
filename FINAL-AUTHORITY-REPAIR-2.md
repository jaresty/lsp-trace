# Final authority repair 2

## Authority boundary repaired

- The production CLI no longer accepts `--seed-custody-trust-config`; CLI and MCP request data cannot select, add, or replace the seed-custody root.
- Official source exposes only an internal host-bootstrap seam. The default official build supplies no seed authority and therefore fails closed for seeded admission until an embedding host provides an already-authenticated authority. No private key is embedded.
- Tests inject roots directly into internal package functions. That test-only path is absent from the external production API and CLI.
- `HostRootSet` selects an explicit root-set version and root key ID. Unknown IDs and root-version mismatch reject, permitting deliberate rotation without accepting arbitrary self-signed roots.
- A selected custody receipt and prepared policy must both verify under the independently selected public host root. Key self-digests or keys transported beside a request are not authority.

## Exact prepared policy and opaque admission context

The signed version-1 host policy binds `prepared=true`, policy ID, assessment ID, context ID, repository, source commit, required adaptation IDs, the precise ordered allowed-change set, target path, unchanged target digest, and validity interval. Canonical policy, prepared-manifest, and custody-receipt signatures use distinct domains with big-endian domain-length and payload-length framing.

`VerifyPrepared` is called while authenticated host bundles are admitted. It requires an exact match between policy, custody receipt, and canonical prepared manifest. It rejects prepared=false downgrade, source substitution, adaptation substitution, allowed-change substitution, target substitution, policy/assessment/context substitution, expiry, and cross-context replay. Its output is an opaque `VerifiedPreparedContext`; prepared `HostReceiptAuthority.Verify` fails closed unless that context is carried into admission.

Official binary/provider source identity remains independently bound by the existing provider identity checks: class, authority, name, version, executable-path digest, executable payload digest, and canonical process-config digest.

## Authorization and reviewer boundary

No live D01 or real provider execution was performed. Repository qualification now states only that no D01 execution occurred here. It distinguishes that fact from an external operator authorization record and references such records generically; no temporary-path digest or authorization artifact is committed. The preserved external authorization records were not executed or modified.

Reviewer materialization of a real host bootstrap adapter, signed policy, and signed receipt bundle is permitted later. It must preserve the inaccessible-root boundary and exact policy bindings above.

## Focused evidence

Persistent RED mutation guards cover caller root selection, arbitrary self-signing, unknown/rotated root IDs, prepared=false downgrade, source, allowed-change, policy, and context substitution, expired replay, opaque context carriage, prepared-manifest mutation, target modification, and provider identity substitutions.

Executed focused evidence:

- Normal affected packages: `go test ./internal/seedbinding ./cmd/lsp-trace-mcp -count=1` — 243 passed in 2 packages.
- Focused authority/mutation selection: 15 passed in 2 packages.
- Focused race authority/mutation selection: 9 passed in 2 packages.
- Fake-LSP process lifecycle: `TestBuiltFakeLSPSeedAdmissionLifecycle` — 6 passed in 1 package; no real provider was used.
- Focused vet: `go vet ./internal/seedbinding ./cmd/lsp-trace-mcp` — exit 0.
- Focused build: `go build ./cmd/lsp-trace ./cmd/lsp-trace-mcp` — exit 0.
- Compatibility and exact-25 selection: public V3 contract, normalized provider request/capabilities/publication, registry contract, and retained-calls V2 registration/current count — 5 passed in 1 package.

No full suite or full race was used.

## Readiness

Source-level technical repair is implemented. Technical readiness is still **not established** until independent review accepts this repair and a real host supplies an authenticated root set plus a correctly signed, currently valid host policy and matching receipt/manifest bundle. Focused fake-LSP evidence does not establish real-host configuration and does not itself authorize live D01.
