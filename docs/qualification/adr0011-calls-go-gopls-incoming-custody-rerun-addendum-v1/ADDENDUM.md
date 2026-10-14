# Incoming custody rerun addendum v1

This package finalizes and corrects an interrupted addendum for a future fresh incoming-only Go/gopls exact-5900 correction campaign. It replaces untrusted partial producer scripts with reviewed, fail-closed scripts and package evidence.

## Disposition

`INCOMING_CUSTODY_RERUN_ADDENDUM_AUTHORED_NOT_GO` — authored, not GO. No full campaign was run. No historical evidence was overwritten.

## Corrections after independent NO-GO review

1. **Public summary leakage fixed.** `prelaunch-custody.summary.json` is now produced by allowlist construction only. It carries structural counts and digests, not raw environment values, raw environment key names, raw paths, `go_env_json`, argv, cwd, rootUri, or workspace folders. The private custody artifact retains exact values.
2. **Fresh namespace reservation fixed.** The wrapper refuses pre-existing run dirs and same run-id dirs, checks symlink/confinement ancestry, and atomically reserves with `mkdir(exist_ok=False)` before writing. There is no custody-only resume path.
3. **Post-full-run sealing added.** `--finalize-run` inventories every generated run artifact, writes outside-run checksums and a seal without self-reference, binds contract/source/corpus/custody digests and final counts, and rejects missing/extra/symlink/nonregular/unfinalized outputs. `--verify-full-run` independently recomputes the seal. Only a synthetic TEST_ONLY fixture was sealed now.

## Required fresh-run properties

- profile: `CALLS_GO_GOPLS_INCOMING_EXACT_V1` bound to `CALLS_LSP_INCOMING_EXACT_V1` without authority merge
- direction: incoming only
- ordered corpus: source revision `4b910fc0c517cb7801fb51d17d295897230d11fa`, source tree `761474bf5d90bbe3468df62b1a68a1fa26dc2c82`, target TSV sha256 `8aa361546ab5245ae0ac2f945e3d19b36a48b745bfe7187e5baef6d6d9a14bd4`
- denominator: 5,900 targets, batch size 50, batch count 118
- retry policy: retry-free fresh run directory only; existing run dir/run-id dir fails closed
- raw response ceiling: 33,554,432 bytes unchanged
- namespace: `lsp-trace.adr0011.calls.go.gopls.incoming.custody_rerun.v1`

## Exact-HEAD self-reference correction

The failed pre-effect run `incoming-custody-correction-v1` produced no successful run directory or evidence because the wrapper pinned the addendum precommit parent as an exact `EXPECTED_HEAD`. This correction keeps `c3d77247f5a2e06c0d6ac5b1b55a3df7dbea588a` as an immutable dependency base, but allows descendant HEADs only when the base is an ancestor, the tracked diff from base to HEAD is confined to this addendum package, there are no staged or dirty tracked files, and every current package file is regular/non-symlink and covered by matching package checksums. Untracked provider-candidate material is neither read nor authorized.
