# Final authority repair

## Repaired blockers

1. **MCP seed custody trust** — bootstrap process JSON now carries only a canonical SHA-256 `seed_custody_selector`. Receipt bytes and Ed25519 public keys are loaded from the separate host startup option `--seed-custody-trust-config`. The unexported MCP trust store verifies key identity and the domain/length-framed receipt signature before constructing a seed revision authority. A v2 seed with absent host trust or an unresolved selector fails closed before server construction. Caller-supplied receipt/key fields remain rejected by strict bootstrap decoding.
2. **Prepared-copy custody** — a signed canonical prepared modification manifest binds Finder digest, source archive digest, source tree digest, prepared tree digest, allowed change set, target path and unchanged target digest. It separately names the source commit and adaptations and rejects equality. Verification checks exact canonical bytes, SHA-256 digest, Ed25519 signature, unchanged-target binding, and target exclusion from allowed changes before admitting the receipt.
3. **Selected provider identity** — v2 validator identity can bind provider class, authority, name, version, executable-path digest, executable payload digest, and canonical process-configuration digest. Bootstrap verifies host-observed executable/payload/config fields before launch. Readiness retains `initialize.serverInfo`; semantic admission compares the observed name/version with the retained exact identity before `didOpen` and `textDocument/documentSymbol`. Mismatch is terminal for admission even though initialization may already have begun.

Canonical signature inputs use explicit domains and big-endian length prefixes. Historical seed-manifest fields are additive with `omitempty`; existing CLI/MCP graph v1/v2/v3 output paths and the 25-tool surface were not changed.

## Focused evidence

- Normal: five affected packages, 571 passing tests.
- Mutation guards: 20 passing tests covering caller-minted bootstrap trust, prepared manifest swap, prepared target-change admission, self-minted prepared signature, provider class/version/executable/payload/config/authority substitution, and runtime provider-version mismatch before semantic requests.
- Process lifecycle: six passing cases; mismatch trace reaches provider initialization but emits no `didOpen`, `documentSymbol`, or prepare request.
- Focused race: 20 passing tests in three affected packages.
- Focused vet: five affected package groups passed.
- Focused build: `cmd/lsp-trace` and `cmd/lsp-trace-mcp` passed.
- Focused compatibility: canonical public parity, logical digest parity, retained-calls registration/count, and normalized provider contract guards passed.

No network, install, deploy, product access, live D01 execution, full suite, or full race was performed.

## Authorization and readiness boundary

D01 operator authorization exists. Technical readiness is **not yet established**: it awaits independent review of this repair and a real host configuration that provisions the trusted seed-custody key/receipt set and exact provider identity. Fake-LSP and focused local evidence do not establish that real-host configuration or authorize a live D01 run by themselves.
