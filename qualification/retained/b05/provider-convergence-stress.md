# Provider convergence integration stress

Base: `9c363fa203436e26b9002b65f7757604b369c78c`

## Integration ledger

| Commit | Role | Result |
|---|---|---|
| `6188247` | generalized Glint | applied cleanly |
| `bd78eec` | ingress | applied cleanly |
| `2d5e525` | template adapter | conflict in `default-analyzer.mjs`; composed ingress classification with subordinate adapter normalization |
| `8d3ad99` | script adapter | conflict in `default-analyzer.mjs`; retained template adapter and admitted six-relation set while keeping TypeScript-only script extractor |
| `49bcab9` | package convergence | conflicts in dispatch/package/reload guard; retained integrated dispatch, union package contents, and source-constrained TypeScript export |
| `03413b9` | qualification | applied cleanly |

## Perturbation matrix

| Category | Controlled perturbation | Fail observation | Restored/pass observation | Status |
|---|---|---|---|---|
| shared-dispatch conflicts | replayed three conflicting cherry-picks | Git content conflicts in shared dispatch and tests | architecture-composed resolutions; provider suite 68/68 | pass |
| Glint→adapter contract mismatch | ran inherited synchronous ingress tests after async package convergence | four results were `undefined` | callers await async analyzer; focused ingress/template tests advanced to semantic contract | pass |
| advertised-but-dead relations | required every positive attempt to resolve | `PASSES_CALLBACK` failed strict envelope decoding | not restored; relation must not be advertised until strict compiler-derived adapter exists | BLOCKED |
| spelling/marker false positives | existing positive/confusable-negative provider guards, including `perform`/`reload` same spelling | perturbation hooks reject name-only qualification | provider suite 68/68 | pass at package-test layer |
| source-semantic insensitivity | changed `this.itemCount` to `this.otherCount` and reran packed 24-attempt qualifier | old qualifier still reported 24 passes | source restored; persistent source/endpoint guard added | guard added; production matrix BLOCKED downstream |
| retired synthetic authority leakage | convergence guard removes selectable synthetic provider while retaining fixture payload | existing perturbation guard removes retirement invariant | provider convergence guard passes | pass |
| original/generated custody mismatch | removed invalid request fields so provider execution reached custody | macOS `/var` vs `/private/var` caused `RELATION_CUSTODY_FAILED` | canonicalized workspace path | pass |
| omitted-relations graph-v3/provider-start | existing repository compatibility tests exercised omitted selectors | no integration-specific failure observed | Go and race suites 3314/3314 | pass |
| package/core-archive regression | exact npm pack/offline install and release evidence guard | package allowlist conflict during integration | provider pack test and release core-archive guard pass | pass |
| BLOCKED/generated evidence overclaim | positives previously accepted deterministic domain errors as PASS | canonical request exposed provider/schema failures | positive attempts now must resolve; no false qualification admitted | guard pass; relation BLOCKED |

## Observations

1. The original Frame 6 request included unsupported `max_bytes` and `max_messages`; deterministic input rejection was counted as a passing attempt.
2. After removing invalid fields, macOS path aliasing caused custody rejection until the temporary workspace was canonicalized.
3. Once requests reached the provider, `BINDS_ARGUMENT` resolved, but `PASSES_CALLBACK` failed strict envelope decoding because analyzer output includes unsupported `confidence` and non-protocol endpoint structure.
4. The package suite passing does not establish production protocol qualification for all advertised relations.
5. `PASSES_CALLBACK` remains blocked; advertising/admission must be reduced or its strict compiler/Glint adapter repaired before release.
6. Final non-qualification checks: provider tests 68/68; Go 3314/3314; race 3314/3314; vet pass; docs pass; retained B05 checks pass; release check pass; GoReleaser configuration pass.
7. The final packed real-MCP run reached genuine BINDS success, then stopped at `PASSES_CALLBACK` with strict-envelope rejection of unsupported field `confidence`; no all-six PASS is claimed.

## Derivation

Dependency-ordered integration exposed an async Glint boundary before production qualification. Controlled source mutation then showed the old qualifier was not a semantic witness because it admitted domain errors. Tightening positive admission revealed request-schema and custody prerequisites, and after those repairs the first genuine provider-contract mismatch appeared at `PASSES_CALLBACK`. Therefore the honest terminal state is blocked rather than qualified: retain the failing assertion-specific guard, do not treat deterministic BLOCKED evidence as success, and do not publish all-six relation capability until each positive and confusable-negative pair traverses normal packed MCP execution successfully.
