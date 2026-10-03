# ADR-0011 P1 Unit 1 bounded acceptance

Status: `ACCEPTED_FOR_UNIT1_PACKAGING`

This record packages the already reviewed and user-accepted ADR-0011 P1 Unit 1 result. It records acceptance; it does not claim that a commit occurred, authorize broader work, or report any new execution.

## Accepted boundary

The bounded acceptance covers:

- 48 behavioral rows;
- six static source-order predicates, not runtime-allocation proof;
- one separately accounted focused compatibility invocation;
- repaired-baseline reconciliation, including the observation-only `BeforeUnread` repair in accepted runtime source SHA-256 `67ff7e5848de8d6a641678afedf3f5dc4aa8db1ee2b4e15191022cb7c4013bb4`;
- 155 qualification invocations, including preserved unsuccessful attempts; and
- 43 earlier-baseline triplets carried forward at their original bounded ceilings through unchanged relevant paths and focused compatibility, not rerun, plus five Q6 triplets covering the repaired baseline.

Original pre-enforcement same-frozen-test chronology is permanently missing. Prospective evidence does not restore it. Synthetic state checks do not establish general real-I/O completeness. History metadata validation does not establish history acquisition. Compatibility is focused and bounded, not repository-wide qualification.

Correct preserved chronology includes invocation 114 as `Q5-B01 MUTANT_SURVIVED` and invocation 116 as a Q5-B01 nil-pointer panic, `PANIC_NOT_RED`. These were spent invocations; no result was erased or retroactively reclassified as valid RED.

## Exact eight-path package

The separately approved Unit 1 source/acceptance package consists of these six source/test paths and two documentation paths. Exact byte sizes and SHA-256 values are recorded in the corrected local inventory packet `adr0011-p1-unit1-commit-inventory-v2`.

- `internal/lspwire/successor_ingress.go`
- `internal/lspwire/successor_ingress_test.go`
- `internal/lspwire/read_frame_successor_test.go`
- `internal/lspwire/successor_ingress_boundaries_test.go`
- `internal/lspwire/successor_ingress_observation_test.go`
- `internal/lspwire/successor_ingress_step_test.go`
- `docs/qualification/adr0011-p1-unit1-acceptance.md`
- `docs/qualification/artifacts/adr0011-p1-unit1-prospective-qualification_test.go.txt`

The frozen archive has SHA-256 `0b8c1d163b67e85f44474d9293d657cc3d97aeafe3ebfb2f1238bdf7c5c6f637`. Its final evidence manifest has SHA-256 `34aa79d848fb894e88dc7a0041324d4e89f25ce74552e7b24457176b4a4a490a` and remains referenced local evidence, not portable commit content.

This archive preserves only the exact frozen final qualification-test source. Historical failure variants, journals, manifests, and unsuccessful-attempt logs remain in local/external evidence packets and are not included in this commit. Recorded hashes provide references to those artifacts but do not make this commit a complete portable qualification record.

## Existing prerequisites excluded

`internal/lspwire/read_frame.go`, `internal/lspwire/wire.go`, and `internal/lspwire/read_frame_test.go` are existing clean HEAD prerequisites, not Unit 1 package members. The actual `read_frame_test.go` Git blob is `34e0937cd3c14cf3db4acde967e6721b28588f2a`; the prior v1 inventory's `f83fe5e...` value belonged to `read_frame.go` and was incorrect for this dependency.

## Exclusions and authority

Unit 2, full P1, broader C, D, and occurrence admission remain excluded. This eight-path source/acceptance package is not qualification enablement and does not install the archived `.txt` source as a runnable Go test. It grants no runtime expansion, test change beyond the six accepted source/test bytes, staging, commit, push, or merge authority. Any byte drift, scope expansion, or changed evidence identity requires renewed review and authorization.
