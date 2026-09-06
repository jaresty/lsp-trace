# Analyzer status propagation — package 1.0.2

Base: clean active HEAD `ec56a17a19843c9d7a226fef06260758b89d2ccd`, rechecked before mutation. No branch created.

## Exact claim

The default analyzer explicitly normalizes compiler `outcome`/`blocker`, legacy extractor `status`/`reason`, and script coverage-only results. Compiler BLOCKED, UNAVAILABLE, FAILED, and partial evidence no longer become checked EMPTY. Checked EMPTY remains distinct. Mixed safe task / unresolved reload requests retain safe observations and relation-specific unresolved coverage, independent of relation order. Unresolved unsafe calls in the lower analyzer remain partial when safe observations coexist; compiler-proved value-flow origins are not newly classified as unresolved.

The strict envelope and graph provenance add optional `diagnostics` using the existing generic `graph.Diagnostic` shape. Diagnostic messages preserve upstream outcome, reason, and coverage; `method` identifies the unresolved selected relation. No new lifecycle, failure, relation, or coverage enums. Go changes are provider-neutral transport/adaptation only. Strict no-observation domain errors retain upstream diagnostics; their existing public error codes and checked-empty behavior are unchanged. Accepted partial graphs retain PARTIAL coverage and diagnostics.

Original anchors, endpoint and observation identities, logical relation digests, package-identity admission, historical evidence, CALLS authority, and omitted-relations behavior are preserved. Runtime package version changes from 1.0.1 to 1.0.2; semantic provider identity remains `ember-glint@1`.

## Verification

- Assertion-specific RED on the base: unsafe and missing-dependency wrappers returned EMPTY instead of BLOCKED; both mixed relation orders returned COMPLETE instead of PARTIAL. Checked EMPTY and task/reload positives passed.
- Fresh npm pack / offline isolated install, actual MCP incoming and slice RED: unsafe/missing errors contained only `typed provider result contains no accepted observations`; mixed requests reported complete coverage.
- Focused installed GREEN: **46/46** tests, including generic caller-owned JS projects with modeled package declarations, explicit-any reload, missing dependency, checked EMPTY, checked task and typed reload positives, both mixed relation orders, and strict BLOCKED/UNAVAILABLE/FAILED/PARTIAL diagnostics. Text/structured MCP parity checked.
- Before/after observations compared at identical caller paths against the independently installed pre-fix 1.0.1 lower analyzer: endpoint identities and original anchors match.
- Provider suite: **123/123**.
- Caller JavaScript qualification: **48/48**, freshly packed/offline-installed actual MCP; no retained historical evidence rewritten.
- `go test ./...`: pass (3344 tests reported); `go test -race ./...`: pass.
- `go vet ./...`, `go build ./...`: pass.
- `scripts/check-ci.sh`, `scripts/release-check.sh`, `scripts/test-release-evidence.sh`: pass. Release validation includes historical B05 and caller evidence guards, not a new native campaign.
- Additive graph diagnostic schema acceptance, historical omission, unknown-field rejection, partial provenance and identity tests: pass. Existing custody, omission, and parity tests included in full/race suites.
- `scripts/assert-core-archive-boundary.sh` against a freshly built core binary tarball: pass.

Reproduce installed status tests: `sh providers/ember-glint/test-installed-status.sh`. This creates temporary build/pack/install/bootstrap fixtures and removes them on exit. Optional `STATUS_BASELINE` points at a previously installed lower analyzer for same-path identity comparison.

## Boundaries

No live bootstrap, installed provider assets, consumer repository, or native campaign modified. No deployment or native replay success claimed. Deployment and native replay remain separate outstanding work. Isolated offline test installation is not a live installation.
