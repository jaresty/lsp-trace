# ADR0011 Go/gopls incoming custody rerun addendum v1

Status: `INCOMING_CUSTODY_RERUN_ADDENDUM_AUTHORED_NOT_GO`.

This private package authors a fail-closed addendum for a future fresh incoming-only custody rerun. It does **not** run the full 5,900-target campaign, does **not** modify historical workload evidence, does **not** read or authorize any untracked separate provider candidate, does **not** modify the separate provider candidate, and grants no profile, production, public, runtime, outgoing, composition, custody-owner, or ceiling authority.

## NO-GO blocker corrections

- Public prelaunch summary is allowlist-only and contains no raw environment-derived values, environment key names, raw paths, `go_env_json`, argv/cwd/rootUri/workspaceFolders. Private custody retains exact values.
- Run namespace reservation is fresh-only: existing run dir or same run-id dir is refused, with symlink/confinement checks and atomic `mkdir(exist_ok=False)`.
- Full-run finalization is package-owned: `--finalize-run` and `--verify-full-run` seal and recheck generated artifacts. Synthetic TEST_ONLY tamper cases prove changed, missing, extra, and symlink artifacts fail; repository-context tamper cases prove unrelated descendants, dirty tracked files, staged drift, checksum drift, and base-not-ancestor states fail, while an addendum-only descendant is accepted.

## Verification

```bash
PYTHONDONTWRITEBYTECODE=1 python3 docs/qualification/adr0011-calls-go-gopls-incoming-custody-rerun-addendum-v1/verify_custody_rerun_addendum.py
PYTHONDONTWRITEBYTECODE=1 python3 docs/qualification/adr0011-calls-go-gopls-incoming-custody-rerun-addendum-v1/verify_custody_rerun_addendum.py --tamper-tests
```

## Full rerun after independent review only

```bash
python3 docs/qualification/adr0011-calls-go-gopls-incoming-custody-rerun-addendum-v1/capture_incoming_custody_rerun.py --run-full --direction incoming --workspace <exact-reviewed-workspace> --gopls gopls --run-id <fresh-never-used-reviewed-run-id>
python3 docs/qualification/adr0011-calls-go-gopls-incoming-custody-rerun-addendum-v1/capture_incoming_custody_rerun.py --finalize-run <fresh-run-dir>
python3 docs/qualification/adr0011-calls-go-gopls-incoming-custody-rerun-addendum-v1/capture_incoming_custody_rerun.py --verify-full-run <fresh-run-dir>
```

## Estimate

The contractual request-timeout-only worst case is about 49.17h if every target consumed 30s. No estimate in this addendum depends on reading an untracked provider-candidate directory. This is not a guarantee or GO evidence.

## Self-reference correction

The execution wrapper no longer requires an exact self-referential HEAD. It requires immutable dependency base `c3d77247f5a2e06c0d6ac5b1b55a3df7dbea588a` to be an ancestor of HEAD, all tracked changes from that base to HEAD to be confined to this addendum package, no staged or dirty tracked files, and current package files matching `CHECKSUMS.sha256`. Public provenance records only head token/digest; private provenance records the full execution HEAD.
