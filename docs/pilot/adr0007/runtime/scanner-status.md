# G4 scanner status

- **Status:** `OPEN_WITH_GAPS`
- **Pilot:** `PILOT_DISABLED`

## Completed local inventory

- Runtime inventory: `SHA256SUMS`, 62 entries.
- Go module inventory: `go-modules.txt`, 83 lines.
- Go module inventory SHA-256: `d368444f934b978769e8931300f1bdd166e13c703c03ca60cfd289c7488d288b`.
- Native bundle remains bound to the Yzma installer manifest and individual file hashes.

## Scanner availability

The following tools were not installed in the environment:

- `syft`
- `grype`
- `trivy`
- `osv-scanner`

Available checks were limited to `codesign` and `otool`; those results are recorded in `runtime-verification.md`.

## Decision

No vulnerability-clean claim is made. A vulnerability scan and SBOM generation remain required before G4 can close. The absence of scanners is a blocking evidence gap, not a pass.
