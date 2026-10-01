# ADR 0011 references — first-tranche preflight freeze (not row execution)

**STOP / INCOMPLETE.** This freezes a proposed first tranche, not production-query admission or a matrix pass. No row is credited until its exact fixture, guard and independently observed result are retained. The prior private synthetic checkpoint is implementation falsification only.

## Exact inputs (byte pins)

| Input | SHA-256 / identity |
| --- | --- |
| Accepted-for-implementation references contract | `d71ec03c9244eb70fe9d93023410e0dc5cedc9932c992b7ff1ef45f2fc42f7f9` |
| Normative 162-row matrix | `c370fd6ae76bb3b74a15da12a7a9c41d9730c68662830294a50dfe89177546a2` |
| Draft row index (`adr0011-references-row-evidence-index.draft.json`) | `80710f51c86f00fd6508b232c13829c8367967fe26470b23460133750306211d`; 162 entries, every case INCOMPLETE, `implementation_schema_policy_digests=null` |
| Independently byte-reviewed non-test implementation set | `adr0011-synthetic-source-pin.f8cf96e649a5fc05.manifest.json`, manifest `f8cf96e649a5fc051fa6475f8b68fe38bcc346e52fd459bb946d99951f57269a`, 90 files/606162 bytes, aggregate `sha256:b06eca5f8b40b96eee624e932e39c215144a7e2c14162e05cd5896eb83bf475c` |
| Admission policy | `483a0db3ac8f56f043aacd00a7cb7e3be0361dbaf9e595ba0a6d942a245de4f3` |
| Method policy | `2fb6c0174945a92b34ccffaa751d15eaa737b7349e2178899d8ed2df00a4710f` |
| Privacy policy | `8d5f9bfba89bab65ad4a44bf89e95f4fd427b7fc2227966ef2be6468e363770f` |
| Retention policy | `5e4a817e8dc8aed4bc6053e18e1f4dfda00a159cf909fed37e0ae39f4d1baa68` |
| Diagnostic schema | `b469b2becabc5b7657648c32a29324e4390f745dab3fd5721527e2c99a1f21bf` |
| Issuance-record schema | `bf49de9460fee5a68ba13204f633dc72d2dca566fa5f92d704bc19b5514618ba` |
| Request-key successor schema | `86bb214e419f7c69b80ff5dfbdf0f0760c495c9bf0f88d06bdc227b77920fc8e` |
| Private-raw schema | `3b1f36e240c7136b977a4f9a18ed6d0f3bb220e47f84241429d3b9fc55845f28` |

These schema and policy bytes are **proposals**, not accepted production-query admission pins. The 90-file manifest pins local source bytes but not a build, executable, Git cleanliness, provider identity, or transitive dependencies. The checkout is dirty. No claim is made that this list exhausts all eventual successor schema/policy roles; a complete separately reviewed role-to-file inventory is a prerequisite for a production freeze.

## Proposed first tranche — exact row identity and expected outcome

| Row-index case | Fixture to prepare before invocation | Frozen expected result | Current executable evidence |
| --- | --- | --- | --- |
| `R11-TERMINAL-015-base` (matrix line 15) | Exact registered clean Go/gopls session and predeclared query; capability absent; independent observation of zero method invocation | `UNSUPPORTED` / `NONE`, 0/0; `U/0/0/0/0`; no receipt/publication attempted, inactive | No admitted production owner or row fixture; **NOT EXECUTED** |
| `R11-TERMINAL-016-base` (line 16) | Same profile with owner-keyed server error whose signed code **0 is present**; raw error-member evidence retained only under authorized per-run disclosure | `PROVIDER_FAILURE` / `NONE`, 0/0; `U/0/0/0/0`; separately verified diagnostic D, inactive | No authorized disclosure or production diagnostic seam; **NOT EXECUTED** |
| `R11-TERMINAL-017-base` (line 17) | Same error message and profile, code member **absent**; raw member bytes separately retained | `PROVIDER_FAILURE` / `NONE`, 0/0; `U/0/0/0/0`; verified D records absence, never code zero | No admitted production owner or row fixture; **NOT EXECUTED** |

The `U/0/0/0/0` notation and expected publication states are copied from the pinned index; they are expectations, not observations. Before a run each row needs exact input bytes/URI/position, independently selected expected source/version/revision, workspace Git custody, managed session generation/key, provider/adapter/capability as reported, fixture process/config identity, expected N/B/T/E/E_B/E_T/P/A, privacy selection, retained selectors/digests, executable guard and reviewer. A source or policy change invalidates this freeze.

## Authorization and admission stop gates

1. Durable live raw capture is **default off**. Obtain explicit per-run user opt-in for the exact local mode, run, 0700 root outside tracked/synchronized workspaces, 0600 single-link no-replace files, selected 1,048,576 bytes/object, 16 objects/16 MiB/run and 128 MiB/root, bounded errors and redacted public outputs. The user has not supplied such authorization here. Signed server-error code disclosure needs its own explicit authorization; without it row 016 is WITHHELD under the selected privacy proposal, not a supposedly replayable code-erased D. Synthetic secret-free controls do not substitute for original provider evidence.
2. Independently establish a production document-symbol query-target receipt and references owner with exact ready Go/gopls registered clean worktree, managed keyed method write/read, negotiated encoding, prepared source/version and before/after `HOST_GIT_PROBE_V1`, independent role/policy/schema/implementation digest selection and full retained terminal replay. `PublishReferences` currently fails closed and three historical public-success tests remain RED. The test-owned synthetic private final is not production admission.
3. Historical omitted `CALLS_ONLY` process baseline: 43,541 bytes, SHA-256 `d9630f80e4b8df33f345c8dfb52659b9813adeca5f884935d6dae680e39e95c5`; `TestADR0011HistoricalOmittedCallsOnlyProcessByteBaseline` passed offline in this checkpoint. Explicit `input_family:"CALLS_ONLY"` is rejected and omitted/explicit parity **cannot** be claimed; `relations:["CALLS"]` is not a substitute. No new selector is authorized.

**Tranche disposition:** preflight STOP before invoking any matrix row. Count remains **0/162**. Do not manufacture blocked-row verdicts, weaken historical RED assertions, or enable index, definitions, grouping, Leiden, or public surfaces. Resume only after the missing admission and per-run privacy gates are independently established; then run precisely these three rows and stop for separate review.
