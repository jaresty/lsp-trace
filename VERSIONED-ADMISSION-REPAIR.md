# Versioned Admission Repair

Baseline authority: `c47032f`
Reviewed evaluator repair: `ca86c50`

## RED boundary

Committed guard `TestProgramAAdmissionV1ReceiptCountFrozen` at `7bae740` produced:

`ASSERT_PROGRAM_A_V1_RECEIPT_COUNT_FROZEN: got=7 want=6`

This established the HOLD defect before implementation: the reviewed six-receipt Program A v1 admission was silently widened under its historical identity.

## Frozen Program A / Program B v1

The repair restores the reviewed v1 surface and behavior:

- `VerifiedProgramASubstrate` and `ProgramAAdmission` contain exactly the original six ordered opaque receipts: custody, effective configuration, identity, relation normalization, support accounting, and projection.
- `AdmitVerifiedProgramA` remains the v1 API and accepts only that six-receipt type.
- `ProgramBDecisionPolicy` remains `lsp-trace.program-b-admission-decision.v1`.
- `programBEvidenceDomain` remains `lsp-trace.program-b-program-a-evidence.v1\x00`.
- `VerifyProgramBAdmission`, `ProgramBAdmission`, its binding/expectation types, and its exact six-receipt validation remain the reviewed v1 API.
- The frozen canonical v1 receipt payload golden is 357 bytes with digest `sha256:b5fc328b3d3e2da7d57887642c838c896503490472bb3733ac8bff7589218271`.
- The frozen six-receipt evidence-set vector digest is `sha256:a404fa13fc730ec476227c1ef58f8c34635c9128360bdacc9ba076f9313339e7`.

Existing dormant normative analytics therefore remains on its reviewed Program B v1 path. Nothing silently rebinds it to v2.

## Explicit Program A v2

`VerifiedProgramASubstrateV2`, `ProgramAAdmissionV2`, and `AdmitVerifiedProgramAV2` form a separate static API. Its ordered opaque receipt set is the six v1 axes followed by the A4 `qualification_matrix` receipt. It requires:

- family `lsp-trace.program-a-admission`;
- version `lsp-trace.program-a-admission.v2`;
- policy `lsp-trace.program-a-admission-policy.v2`, policy version `v2`;
- evidence domain `lsp-trace.program-a-evidence-set.v2\x00`;
- exactly seven unique receipt digests.

A2 canonical provisioning output, A3 minimum-dependence support accounting, and A4 exact canonical qualification-profile-v2/result/provider-version semantics remain wired through the evaluator. Qualification v1 remains historically readable by `qualificationmatrix` but cannot issue A4 evidence.

The frozen seven-receipt evidence-set vector digest is `sha256:d5634130ad6ff2fbc37a8ef57bf9a2eff75e1b40feb402097f5002dc9ef22048`.

## Explicit Program B v2

`ProgramBAdmissionV2`, its v2 binding/expectation types, and `VerifyProgramBAdmissionV2` consume only `ProgramAAdmissionV2`. They bind:

- admission version `lsp-trace.program-b-admission.v2`;
- decision policy `lsp-trace.program-b-admission-decision.v2`;
- Program A admission version `lsp-trace.program-a-admission.v2`;
- Program A evidence domain `lsp-trace.program-a-evidence-set.v2\x00`;
- Program B evidence domain `lsp-trace.program-b-program-a-evidence.v2\x00`;
- qualification-matrix domain `lsp-trace.program-b-qualification-matrix.v2\x00` and exact profile-v2 canonical bytes;
- exact Program A v2 revision, operation, scope, substrate, authority, provisioning, assessment, policy, and seven-receipt evidence-set digest.

Distinct Go types prevent v1 APIs from consuming seven receipts and v2 APIs from consuming six. Runtime mutations independently reject omitted, reordered, or substituted A4 receipts and v1 policy/domain/version downgrades. No digest helper accepts both cardinalities.

## Actual status and scope

Actual Program A remains unadmitted: no production evaluator authority/key is provisioned and retained inputs still fail the normative requirements documented by the evaluator review. Actual Program B remains unadmitted. No executor or public registry entry was added or activated.

No delegation, network access, installation, deployment, product/live-D01 action, provider qualification, full suite, full race, or public registry mutation occurred.

## Focused verification

At the repair state:

- `go test ./internal/qualificationpolicy -run 'TestProgramA|TestProgramB|TestExternalCaller' -count=1` — 65 passed.
- `go test ./internal/qualificationmatrix ./internal/qualificationpolicy -run 'ProgramB|ProgramA|ExternalCaller' -count=1` — 66 passed across two packages.
- The same two-package selection with `-race` — 66 passed.
- `go vet ./internal/qualificationmatrix ./internal/qualificationpolicy` — no diagnostics.
- `go build ./internal/qualificationmatrix ./internal/qualificationpolicy` — passed.
