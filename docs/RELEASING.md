# Releasing

A release candidate must satisfy:

1. `./scripts/release-check.sh`
2. `go test ./...`
3. `go vet ./...`
4. `go build ./...`
5. retained PASS evidence for every provider the release claims to support, including server versions, capability results, exact caller/range assertions, and graph output
6. review of security, semantic, schema-policy, and [provider protocol](PROVIDERS.md) changes, including named-profile precedence and environment-value non-persistence
7. a clean GoReleaser snapshot that packages both `lsp-trace` and `lsp-trace-mcp`, plus checksums
8. `./scripts/test-b05-qualification.sh`, requiring the exact nine-family ledger, real managed incoming/slice acceptance, retained B05 evidence, and documented non-entailments

A BLOCKED or FAIL qualification cannot support a provider claim. It blocks release only when that provider is advertised as supported by the release. Retain the result and state its qualification boundary; fixture presence is not support evidence. The retained ElixirLS result is BLOCKED and is not a release support claim.

`./scripts/release-check.sh` is the hermetic release dry-run. It validates required documentation, fixtures, normalized retained PASS evidence for claimed providers, GoReleaser v2 configuration, and the embedded Stage 1 MCP manifest/schema contract. It directly requires production bootstrap coverage, exactly thirteen canonical tools, the trusted-local warning on stderr and protocol-clean MCP stdout, and then runs the broader focused MCP contract suite. It then uses `go build -trimpath` to require nonempty `lsp-trace` and `lsp-trace-mcp` binaries in a temporary directory. It does not publish, create a tag, modify retained evidence, or invoke GoReleaser. It does not start or install external language servers. CI runs this dry-run plus formatting, Python/shell syntax, `go test ./...`, `go vet ./...`, `go build ./...`, and clean-tree checks.

Qualification runs remain separate and opt-in because servers and SDKs may access the network and execute project logic. Preserve `qualification/retained/` byte-for-byte during release closure; regenerate it only from reviewed all-PASS raw evidence using `./scripts/retain-qualification.py`.

The hermetic B05 gate does not start providers or establish external adapter support. Its PASS covers the committed fixture ledger, managed executor composition, red validation, and evidence/documentation integrity. Any external provider outcome remains PASS, BLOCKED, or FAIL according to its separately retained evidence; BLOCKED and unknown outcomes cannot satisfy a support claim.

Create an annotated `vX.Y.Z` tag only after the checklist passes. The release workflow builds Linux, macOS, and Windows archives containing both `lsp-trace` and `lsp-trace-mcp`, whose normative Stage 1 manifest, schemas, and transcripts are embedded in the MCP binary, then publishes checksums. Do not claim support for a platform without a produced archive and successful smoke test on that platform.
