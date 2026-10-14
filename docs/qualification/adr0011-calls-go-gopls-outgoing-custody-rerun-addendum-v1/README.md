# ADR0011 Go/gopls outgoing custody rerun addendum v1

Status: `OUTGOING_CUSTODY_RERUN_ADDENDUM_AUTHORED_NOT_GO`.

This private package authors a fail-closed addendum for a future fresh outgoing-only custody rerun. It does **not** run the full 5,900-target campaign, does **not** modify historical workload evidence, does **not** read or authorize any untracked separate provider candidate, does **not** modify the separate provider candidate, and grants no profile, production, public, runtime, composition, custody-owner, incoming-provider, or ceiling authority.

The existing `adr0011-calls-go-gopls-outgoing-profile-qualification-v1` package remains non-authoritative and untouched. The committed incoming custody addendum is used only as implementation precedent for fail-closed custody mechanics; it supplies no outgoing authority.

## Safety corrections included at v1

- Public prelaunch summary is allowlist-only and contains no raw environment-derived values, environment key names, raw paths, `go_env_json`, argv/cwd/rootUri/workspaceFolders. Private custody retains exact values.
- Run namespace reservation is fresh-only: an existing run dir or same run-id dir is refused, with symlink/confinement checks and atomic `mkdir(exist_ok=False)`.
- Full-run finalization is package-owned: `--finalize-run` and `--verify-full-run` seal and recheck generated artifacts. Synthetic TEST_ONLY tamper cases prove changed, missing, extra, and symlink/FIFO/socket/device artifacts fail. Repository-context tamper cases prove unrelated descendants, dirty tracked files, staged drift, checksum drift, and base-not-ancestor states fail, while package-only descendants are accepted.
- Exact HEAD self-reference is forbidden: execution requires immutable dependency base `17057a8c53af8e8fe66be9c37788bbf59d10c5cd` as an ancestor, tracked descendant changes confined to this package, no staged/dirty tracked files, and package checksums matching `CHECKSUMS.sha256`.

## Verification

```bash
PYTHONDONTWRITEBYTECODE=1 python3 docs/qualification/adr0011-calls-go-gopls-outgoing-custody-rerun-addendum-v1/verify_outgoing_custody_rerun_addendum.py
PYTHONDONTWRITEBYTECODE=1 python3 docs/qualification/adr0011-calls-go-gopls-outgoing-custody-rerun-addendum-v1/verify_outgoing_custody_rerun_addendum.py --tamper-tests
```

## Full rerun after independent review only

```bash
python3 docs/qualification/adr0011-calls-go-gopls-outgoing-custody-rerun-addendum-v1/capture_outgoing_custody_rerun.py --run-full --direction outgoing --workspace <exact-reviewed-workspace> --gopls gopls --run-id <fresh-never-used-reviewed-run-id>
python3 docs/qualification/adr0011-calls-go-gopls-outgoing-custody-rerun-addendum-v1/capture_outgoing_custody_rerun.py --finalize-run <fresh-run-dir>
python3 docs/qualification/adr0011-calls-go-gopls-outgoing-custody-rerun-addendum-v1/capture_outgoing_custody_rerun.py --verify-full-run <fresh-run-dir>
```

## Estimate

The contractual request-timeout-only worst case is about 49.17h if every target consumed 30s. No estimate in this addendum depends on reading an untracked provider-candidate directory. This is not a guarantee or GO evidence.
