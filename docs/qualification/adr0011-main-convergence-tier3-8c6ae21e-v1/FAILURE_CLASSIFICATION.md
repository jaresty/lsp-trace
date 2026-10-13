# Failure classification

| selector | result | class | exact comparison/basis |
|---|---:|---|---|
| `scripts/check-ci.sh` | 0 | PASS | current candidate |
| `scripts/check-go-format.sh` | 1 | inherited | reports the pre-existing versioned `enumerator_test.go`; blob exists at parent `320edd24` and HEAD does not modify it |
| `go build ./...` | 1 | inherited | versioned evidence `.go` files under prior qualification packages reference private `sessionruntime` identifiers; those blobs exist at parent `320edd24` |
| `go vet ./...` | 1 | inherited | same prior qualification-package compile boundary |
| `go test ./...` | 1 | mixed inherited / intentional RED / platform | prior qualification-package compile failures are inherited; named `*_red_*` and prospective policy tests are intentional RED; process/provider failures include unavailable local external package `@glint/core`; top-level command timed out at 10m after reporting these failures |
| `scripts/release-check.sh` | 1 | platform unavailable | external Ember/Glint production qualifier cannot import `@glint/core`; exact stderr retained |
| Python Program C selectors | 0 | PASS | current candidate |
| shell syntax selector | 0 | PASS | current candidate |
| focused integrated ownership/unowned/adjacent/fixture package | 0 | PASS | rerun at exact candidate |
| census continuation package | 0 | PASS | rerun at exact candidate |

No complete-selector failure is promoted to acceptance. The hard-block decision does not depend on classifying a failure as harmless.
