# Program B evidence binding

## Outcome

Canonical Program B admission is now issued and verified by `internal/qualificationpolicy`, the package that owns opaque Program A receipt provenance. `internal/qualificationmatrix` retains matrix generation and historical admission-policy validation, but no longer exposes a Program B minting path. Program A no longer exposes `ProgramABinding`, receipt digests, or a bool-returning projection.

A Program B admission binds the ordered digest of exactly six unique verified Program A receipts; common authority ID, key ID, provisioning digest, assessment ID, nonce, issuance epoch, evaluation scope, admission policy ID/version, operation, revision, and substrate; the exact canonical qualification-v2 matrix digest (which includes provider class/version); and `lsp-trace.program-b-admission-decision.v1`.

Execution supplies and verifies expected operation, scope, revision, substrate, matrix digest, and exact Program A evidence-set digest. Zero, omitted, reordered, substituted, duplicated, mixed-context, wrong-matrix, wrong-policy, and v1 constructions reject. Canonical Program B admission rejects waivers; the qualification validator continues to reject blocked or waived foundational cells.

Executor-owned units and descriptors remain internal, unregistered, and historical v1 behavior remains in `qualificationmatrix.ProgramBAdmitted`; only canonical Program B v2 token issuance moved. No bool is used for admission verification.

## Falsification evidence

With only the exact evidence-set comparison removed, the committed guard `TestProgramBAdmissionRejectsReceiptDigestSwapAndSubstitution` produced:

- `ASSERT_PROGRAM_B_RECEIPT_DIGEST_SWAP_REJECTED`
- `ASSERT_PROGRAM_B_RECEIPT_SUBSTITUTION_REJECTED`

Restoring that one comparison made the focused normal and race suites pass (`70 passed in 3 packages` each). The AST package-boundary guard rejects reintroduction of an exported `ProgramABinding` projection.

## Admission status

This change implements and tests the dormant admission capability. It does not provision real Program A authority, does not supply real qualification evidence, and does not activate an executor or registry entry. Actual Program A remains unadmitted. Actual Program B remains unadmitted. No deployment, installation, network access, live environment change, registry mutation, or product action occurred.
