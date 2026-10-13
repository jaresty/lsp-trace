# Failure classification

| selector/evidence | result | class | exact basis and acceptance treatment |
|---|---:|---|---|
| `verify_accounting.py` | 0 | PASS | Exact role/path/count/field/byte/hash, manifest, ID-set delta, exclusion-category, and current-output checks. |
| focused ADR0007 ownership registration | 0 | PASS | Fresh exact-candidate affected-scope rerun. |
| focused ADR0007 unowned exclusions | 0 | PASS | Fresh exact-candidate affected-scope rerun. |
| focused ADR0007 adjacent-path rejection | 0 | PASS | Fresh exact-candidate affected-scope rerun. |
| `go test ./internal/integratedconformance -count=1 -v` | 0 | PASS | Fresh exact-candidate package rerun; reference/test evidence only. |
| `go test ./internal/censuscontinuation -count=1 -v` | 0 | PASS | Fresh exact-candidate package rerun. |
| `scripts/check-ci.sh` | 0 | PASS | Fresh exact-candidate repository policy selector. |
| `TestPrivateB4C15V2SuccessorCatalog` | 1 | inherited frozen-contract/current-generator divergence | Exact failure: `got=4155 want=3847`. Test and frozen catalog paths are unchanged from immediate parent `320edd24`. Direct strict frozen-byte accounting passes. No regeneration, normalization, repair, or acceptance. |
| v1 `scripts/check-go-format.sh` | 1 | inherited | Pre-existing versioned `enumerator_test.go`; retained in immutable v1 evidence. |
| v1 `go build ./...` and `go vet ./...` | 1 | inherited | Prior qualification-package `.go` evidence references private `sessionruntime` identifiers; retained in immutable v1 evidence. |
| v1 `go test ./...` | 1 | mixed inherited / intentional RED / platform / timeout | Inherited compile failures, named RED/prospective policy tests, unavailable `@glint/core`, then ten-minute harness timeout. Retained, not rerun as an acceptance gate, and not accepted. |
| v1 `scripts/release-check.sh` | 1 | platform unavailable | External Ember/Glint qualifier cannot import `@glint/core`; retained unchanged and not accepted. |

No failing selector is promoted to acceptance. The dormant-integration GO is bounded to authored composition and affected-scope preservation. It is not production admission and not a whole-repository-green result.
