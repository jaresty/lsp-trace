# Releasing

A release candidate must satisfy:

1. `./scripts/release-check.sh`
2. `go test ./...`
3. `go vet ./...`
4. `go build ./...`
5. retained PASS evidence for TypeScript, C#, and ElixirLS qualification, including server versions, capability results, exact caller/range assertions, and graph output
6. review of security, semantic, and schema-policy changes
7. a clean GoReleaser snapshot and checksums

A BLOCKED or FAIL language qualification—including unavailable ElixirLS or unusable Call Hierarchy—blocks release; it is not a pass and must be attached to the release decision. Support claims require retained PASS evidence; fixture presence is not support evidence.

`./scripts/release-check.sh` is the hermetic release dry-run. It validates required documentation, fixtures, normalized retained PASS evidence, and GoReleaser v2 configuration, then runs `go build -trimpath` into a temporary directory and requires a nonempty binary. It does not publish, create a tag, modify retained evidence, or invoke GoReleaser. It does not start or install external language servers. CI runs this dry-run plus formatting, Python/shell syntax, `go test ./...`, `go vet ./...`, `go build ./...`, and clean-tree checks.

Qualification runs remain separate and opt-in because servers and SDKs may access the network and execute project logic. Preserve `qualification/retained/` byte-for-byte during release closure; regenerate it only from reviewed all-PASS raw evidence using `./scripts/retain-qualification.py`.

Create an annotated `vX.Y.Z` tag only after the checklist passes. The release workflow builds Linux, macOS, and Windows archives from that tag and publishes checksums. Do not claim support for a platform without a produced archive and successful smoke test on that platform.
