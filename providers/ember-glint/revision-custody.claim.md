# Revision-custody labeling repair

Baseline: `217fbb8`; coordinated documentation-only successor: `dff1d8f`.

## Claim and boundary

Reading source bytes and hashing them does not authenticate a supplied revision. Strict Ember responses now carry `CALLER_ASSERTED` only for an explicit supplied caller revision, otherwise `UNKNOWN`. Generic host adaptation does not trust a provider's self-described proof: matching caller assertions remain caller assertions under permissive policy, and only successful existing strict Git verification upgrades that document to `PROVIDER_PROVED`. Additional unverified documents remain `UNKNOWN`. No schema vocabulary or semantic provider identity (`ember-glint@1`) changed. npm runtime patch: `1.0.2` → `1.0.3`.

The host verifier still requires an exact managed-session generation, caller-asserted Git request, matching authenticated commit, safe in-repository path, and matching content/blob SHA-256 against pinned Git bytes. It accepts an honest caller-labeled document rather than demanding a self-asserted proof before verification. Unknown provider assertions remain rejected in that strict path.

## RED / GREEN witnesses

Before runtime changes, fresh npm pack/offline installation plus actual MCP incoming/slice produced 15 field-level failures: strict and published custody were `PROVIDER_PROVED`, expected `CALLER_ASSERTED`. Direct observations, endpoint/anchor comparisons and hashes passed. Host RED separately exposed unverified label preservation and the old verifier's premature-proof-label precondition. Session logs: `/tmp/revision-custody-red.log`, `/tmp/revision-host-red.log`.

GREEN regressions cover generic caller-owned non-Git JavaScript, direct → framed → packed strict → adapter → actual MCP publication; verified Git positives; mismatched Git rejection; unknown/no-revision and mocked self-asserted proof negatives; invalid custody rejection; and proof confinement to the verified document. No-revision JavaScript remains a domain failure under the existing analyzer requirement, not an authenticated graph. Original source anchors and endpoint IDs remain unchanged for identical materialized inputs; authority-bearing receipt/logical/artifact digests may change as required by corrected custody.

## Changed files

- `internal/provider/semantic_binding.go`
- `internal/provider/revision_verifier.go`
- `internal/provider/revision_bound_custody_test.go`
- `providers/ember-glint/src/protocol.mjs`
- `providers/ember-glint/package.json`
- `providers/ember-glint/package-lock.json`
- `providers/ember-glint/test/package.test.mjs`
- `providers/ember-glint/test/status-propagation.test.mjs`
- `providers/ember-glint/revision-custody.claim.md`

## Validation

Provider tests, fresh packed installed status/custody tests, fresh caller-JavaScript 48-attempt qualification (without `--retain`), focused Go suites, CI guard, retained caller48 guard and release check passed. Full/race initially failed only because the concurrently modified PRD was outside an old dirty-worktree ownership allowlist; the coordinated documentation commit removed that dirty path. Final reruns passed: `go test ./...`, `go test -race ./...`, `go test ./... -run 'Archive|Omission|Omitted|Parity|Custody' -count=1`, `go vet ./...`, `go build ./...`, `git diff --check`, `scripts/check-ci.sh`, `scripts/release-check.sh`, and `scripts/test-release-evidence.sh`. Provider: 123/123; fresh installed: 56/56; fresh caller qualification: 48/48. Logs: `/tmp/revision-{provider,installed,full,race,ci,release}-final.log`, `/tmp/revision-caller48-fresh.log`, `/tmp/revision-custody-parity-omission-archive.log`, `/tmp/revision-release-evidence.log`.

Historical evidence, consumer repositories, live configuration, and deployment were not changed. No staging or commits were performed by this implementation agent.
