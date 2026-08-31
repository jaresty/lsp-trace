# Releasing

A release candidate must satisfy:

1. `./scripts/release-check.sh`
2. `go test ./...`
3. `go vet ./...`
4. `go build ./...`
5. retained PASS evidence for TypeScript, C#, and ElixirLS qualification, including server versions, capability results, exact caller/range assertions, and graph output
6. review of security, semantic, and schema-policy changes
7. a clean GoReleaser snapshot and checksums

A BLOCKED language qualification—including unavailable ElixirLS or unusable Call Hierarchy—blocks release; it is not a pass and must be attached to the release decision. CI intentionally validates hermetic repository checks without installing external language servers. Qualification runs are retained separately because servers and SDKs may access the network and execute project logic.

Create an annotated `vX.Y.Z` tag only after the checklist passes. The release workflow builds Linux, macOS, and Windows archives from that tag and publishes checksums. Do not claim support for a platform without a produced archive and successful smoke test on that platform.
