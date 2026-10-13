# Immutable external reviewer-result provenance

Status: `AUTHORED_NOT_GO`

The verdicts below are externally produced review results. The task handles are UI locators only; authority is bound to the copied immutable `.output` bytes, their SHA-256 digests and byte lengths, and the exact final GO text verified inside each artifact. Source artifacts were copied byte-for-byte from `/var/folders/bc/qbdy42vs4jn56zwrb2y09l9m0000gn/T/pi-subagents-501/Users-schwa-dev-lsp-trace/01a1006f-45e5-720d-b422-2d6ae9e3367b/tasks/` into `evidence/reviewers/`.

## Existing test write-set GO

- reviewer task: `78463f8e-b9b9-4ee`
- result: `GO`
- token: `ADR0011_DIRECTIONAL_RAW_CUSTODY_32MIB_TEST_WRITESET_GO`
- commit: `2faac2671987f6e6f7c680fe4f072064cfe48136`
- manifest digest: `52df9064796d71710ebc2c81c314e1f5c4b4dbd9aff531d483a475a71e4e2d13`
- applicability: exact six-production/four-new-test write-set only

## Generic amendment GO

- reviewer task: `17622b36-cbd9-48b`
- result: `GO`
- token: `ADR0011_GENERIC_DIRECTIONAL_CALLS_PROFILE_AMENDMENT_GO`
- commit: `f03672416bbe1342c06e7cbb33bf51127909ce8a`
- manifest digest: `df44a8befc36ff69b7e903be2b78b5cf9fbb508538b8cf04ed5bbb7fd35b2df7`
- applicability: provider-neutral directional architecture amendment only

## C15 lease-source V2 GO

- reviewer handle (UI locator only): `73ef3d2e-c865-43f`
- source artifact: `/var/folders/bc/qbdy42vs4jn56zwrb2y09l9m0000gn/T/pi-subagents-501/Users-schwa-dev-lsp-trace/01a1006f-45e5-720d-b422-2d6ae9e3367b/tasks/73ef3d2e-c865-43f.output`
- copied artifact: `evidence/reviewers/c15-lease-source-v2-go.output`
- artifact bytes: `977346`
- artifact SHA-256: `aa5a9066918eef445d6d46d73c118743fef3aba88a2e837dd67224c34661fc20`
- exact final GO text: `ADR0011_DIRECTIONAL_RAW_CUSTODY_C15_LEASE_SOURCE_V2_GO`
- verdict-context verification: exact GO text is the final assistant record; zero records follow it, so no later `BLOCKED` or `REVISE` exists
- commit: `a1cd77950ffa45ebed52310878f772f3c7344756`
- package-manifest digest: `6d764eb05029c5663c740ac0bfa452552eeb7f07e5a74f68b45e144ffe15d4a6`
- applicability: lease-source architecture bytes only; no semantic C15 classification or implementation authority

## Two-stage C15 contract GO

- reviewer handle (UI locator only): `1f290c50-b945-4fe`
- source artifact: `/var/folders/bc/qbdy42vs4jn56zwrb2y09l9m0000gn/T/pi-subagents-501/Users-schwa-dev-lsp-trace/01a1006f-45e5-720d-b422-2d6ae9e3367b/tasks/1f290c50-b945-4fe.output`
- copied artifact: `evidence/reviewers/c15-two-stage-contract-go.output`
- artifact bytes: `673755`
- artifact SHA-256: `fa2c94fa9f9b0c65564720de48c4f714e5da0c3a0d76fa8aff168f6525c25f0c`
- exact final GO text: `ADR0011_DIRECTIONAL_RAW_CUSTODY_C15_TWO_STAGE_CONTRACT_GO`
- verdict-context verification: exact GO text is the final assistant record; zero records follow it, so no later `BLOCKED` or `REVISE` exists
- commit: `acf563e7708095d8a32c809b49d97c7432d49b03`
- v2 run-manifest digest: `12fb6b1f1fecb7eda022b0ffe84750cca5bcefbb75e87fcaa48d24016228e8e8`
- applicability: two-stage ordering/schema and Stage A architecture-obligation completeness only; Stage B actual enumeration/adjudication remains required after implementation

## Private interface V3 GO

- reviewer handle (UI locator only): `a856f367-42c9-454`
- source artifact: `/var/folders/bc/qbdy42vs4jn56zwrb2y09l9m0000gn/T/pi-subagents-501/Users-schwa-dev-lsp-trace/01a1006f-45e5-720d-b422-2d6ae9e3367b/tasks/a856f367-42c9-454.output`
- copied artifact: `evidence/reviewers/private-interface-v3-go.output`
- artifact bytes: `736979`
- artifact SHA-256: `6fa84fe0fa45bc9cc9b2183d652a20d27629924c93d47984925bf066f1123718`
- exact final GO text: `ADR0011_DIRECTIONAL_RAW_CUSTODY_32MIB_PRIVATE_INTERFACE_V3_GO`
- verdict-context verification: exact GO text is the final assistant record; zero records follow it, so no later `BLOCKED` or `REVISE` exists
- commit: `be771d5627f9a2588781ac67707d7eb3c7b8d314`
- package-manifest digest: `02153887105c30cc6eaa3b976ec49e7f32b97afcf35ff6b8aaf185529e029704`
- applicability: exact private V3 declaration/interface bytes only

## Reconciliation boundary

External GO makes only the reviewed dependency applicable and reconciles its historical target pin against the verified clean successor chain. Producer `AUTHORED_NOT_GO` status remains immutable. No verdict above authorizes production/test mutation, RED execution, implementation, Stage B fabrication, semantic C15 classification, commit, activation, deployment, or production admission.

The three formerly unresolved reviewer-artifact provenance blockers are closed by the copied exact bytes and pins above. The handles remain locators, not authority; any mismatch in copied byte length, digest, final GO text, final-record position, or absence of later records is a hard stop.

AUTHORED_NOT_GO
