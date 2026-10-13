# ADR0011 directional raw-custody 32 MiB test write-set amendment v1

Status: `AUTHORED_NOT_GO`

Exact review token: `ADR0011_DIRECTIONAL_RAW_CUSTODY_32MIB_TEST_WRITESET_GO`

Target repository HEAD: `9166c224e167a4a5df6f7c9a6d3319e2da15a904`.

This package is a narrow authorship amendment to the accepted raw-custody design. It freezes test ownership and the complete future production-plus-test write-set. It does not authorize RED execution, implementation, activation, production admission, Phase 0 work, behavior changes, commits, or reopening the accepted design.

## Decision boundary

The six accepted production targets remain exactly unchanged from the accepted design. This amendment adds exact new private test paths and assigns non-overlapping selector prefixes. Historical tests and qualification packages are immutable pinned dependencies only.

No unlisted production or test path may change. If any obligation cannot be met inside the listed paths, if a listed new test belongs to a different Go package, or if implementation requires modification of a historical test, stop `BLOCKED_OWNERSHIP_OR_PACKAGE_MISMATCH`; do not widen the write-set.

Read in order:

1. `WRITESET.md`
2. `DEPENDENCIES.sha256`
3. `REVIEW_REQUEST.md`
4. `SHA256SUMS`

## Result

`AUTHORED_NOT_GO`
