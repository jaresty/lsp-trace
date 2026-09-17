# ADR 0007 / ADR 0008 source-projection qualification review

## Review identity

- Reviewed revision: `5d392f9710225bd1761af67fd9d851c2ff4b2f82`
- Branch/worktree: `pi-qualification-review` at `/private/tmp/lsp-trace-qualification-review`
- Execution record: `qualification/adr0008-source-projection-matrix.execution.5d392f9710225bd1761af67fd9d851c2ff4b2f82.json`
- Governing matrix: `qualification/adr0008-source-projection-matrix.v1.json` (unchanged historical input)
- Governing plan: `docs/qualification/adr0007-adr0008-source-projection-plan.md`
- Fixture plan: `docs/qualification/adr0008-cross-mode-fixture-plan.md`
- Result: **not qualified**; `implementation_qualified=false`

## Environment and custody

The child-local MCP process listed no exact child-worktree session. It derived exactly once from READY alias `project`, generation 1, for `file:///private/tmp/lsp-trace-qualification-review`. The resulting exact session was:

- session: `sk1:71207826e6edd64af821c6e9076553df80494aca21d4b6ad0d464c04ea48d5e6`
- generation: `1`
- state: `READY`
- server profile: `gopls`
- position encoding observed in projection: `utf-16`

Managed exact-position structural context over `TestCrossModeV2Fixture` completed as request `sc_b1c0ade21faf3fcaf037a68c171cbe8a`, preserving `authority=0`, `source_graph_complete=UNKNOWN`, and `graph_facts_added=0`.

A fresh CLI was built from the reviewed worktree with SHA-256 `15bf0f115cfb76ade8a496969e635135add543be00bf3987b94b116aa000a272`. The installed provider was operator-identified at the same revision with SHA-256 `a12aff35061ddafceff9a0704d83208fd095595d8571f84dcf7285cd0561d665`. These are distinct binaries and are not conflated.

## Commands executed

| Purpose | Command | Outcome |
|---|---|---|
| Fixture/runtime assertions | `go test -count=1 -v ./internal/liveprojection -run '^TestCrossModeV2Fixture$'` | PASS, eight named subtests |
| Retained oracle/adversarial cases | `go test -count=1 -v ./internal/retainedoperation -run '^TestOperation41CrossMode'` | PASS |
| Retained direct/gateway parity | `go test -count=1 -v ./internal/mcp -run '^TestOperation41RealFixtureDirectGatewayExactParity$'` | PASS |
| Full repository suite | `go test -count=1 ./...` | PASS |
| Release guard | `./scripts/release-check.sh` | PASS, ended `RELEASE CHECK PASS` |
| Fresh build | `go build -trimpath -o /tmp/lsp-trace-qualification-5d392f97 ./cmd/lsp-trace` | PASS; SHA above |
| Exact child target projection | direct managed `lsp_trace_v2_structural_context` exact-position TARGET | COMPLETE |
| Host renderer domain-failure trial | direct managed `lsp_trace_v2_structural_context` for missing symbol `QualificationDefinitelyMissingSymbol_5d392f97` | canonical typed failure plus improper `Expected parameters` decoration |

Historical operation-41 smoke at prior revision `b02d23ab` was used only as a locator and was not credited to any PASS or FAIL verdict.

## Verdict summary

| Verdict | Count | Cells |
|---|---:|---|
| PASS | 0 | none |
| FAIL | 1 | L04 |
| BLOCKED | 22 | C01–C06, R01–R04, L01–L03, I01–I07, X01–X02 |
| NOT_RUN | 0 | none |

Partial current execution moves cells from mere absence to attributable review, but it does not satisfy every clause of any cell's `pass_evidence`. Missing prerequisites therefore remain BLOCKED rather than being promoted from partial tests.

## Cell-by-cell rationale

| Cell | Verdict | Current evidence | Why this is the ceiling |
|---|---|---|---|
| C01 | BLOCKED | Runtime exact-artifact and permutation subtests pass. | Permutation checks decoded JSON equality, not byte-identical canonical output; identity-field sensitivity is incomplete. |
| C02 | BLOCKED | Retained oracle covers UTF-16 and distinct evidence/item/selection/display ranges. | UTF-8/32, BOM, newline families, empty/cross-line, non-BMP and complete citation vectors are not all qualified. |
| C03 | BLOCKED | Selected object/range/source/work/response limit cases pass. | Object/node/page/page-count and exact observed/limit accounting are incomplete. |
| C04 | BLOCKED | Fixture and managed projection preserve zero-authority invariants. | No complete before/after graph-byte mutation oracle exists. |
| C05 | BLOCKED | Privacy-withheld variant and retained body accounting exist. | The full body-eligibility/privacy/non-disclosure corpus is absent. |
| C06 | BLOCKED | Mixed/malformed carrier, invalid UTF-16 boundary, and missing binding cases pass. | The required exhaustive named RED mutation corpus does not exist. |
| R01 | BLOCKED | Retained V2 fixture oracle passes. | Exact admitted V5 manifest binding and predecessor digest inventory are incomplete. |
| R02 | BLOCKED | Exact source-object lookup executes once. | Commit/tree/blob, dirty, non-Git, digest-only, and alternate-class cases are incomplete. |
| R03 | BLOCKED | Missing and corrupt source typed failures pass. | Withheld/collected/shallow/rewritten/GC and all no-fallback clauses are incomplete. |
| R04 | BLOCKED | General release publication guards pass. | No source-projection race receipt and cold replay packet covers every clause. |
| L01 | BLOCKED | Exact managed session/generation projected admitted target bytes. | Mutation/substitution/ambient-checkout/retained-fallback negatives are absent. |
| L02 | BLOCKED | Fixture enumerates document/projection/response limits. | Enumeration is not execution of every independent budget and frontier assertion. |
| L03 | BLOCKED | Exact-position managed TARGET projection completed. | No full symbol/position/snippet trial adjudication and cross-track requalification exists. |
| L04 | **FAIL** | Valid missing-symbol request reached canonical typed domain failure; host appended `Expected parameters`. | This exactly matches L04 fail evidence. Canonical server envelope correctness remains distinct and is not disproved. |
| I01 | BLOCKED | None qualifying. | Typed ADR0007 admissions do not exist and execute. |
| I02 | BLOCKED | None qualifying. | Cache identity/invalidation contract and vectors are absent. |
| I03 | BLOCKED | None qualifying. | Executing ADR0007 citation schema is absent. |
| I04 | BLOCKED | None qualifying. | Cross-corpus coexistence mapping/execution is absent. |
| I05 | BLOCKED | None qualifying. | Denominator/terminal accounting execution is absent. |
| I06 | BLOCKED | None qualifying. | Semantic authority/graph-fact guard does not exist and execute. |
| I07 | BLOCKED | None qualifying. | Privacy/deletion ownership and dependent-artifact invalidation are absent. |
| X01 | BLOCKED | Current 41/full and 12/compact guards, broad suite, release schema-byte guards pass. | One current packet does not prove every predecessor hash and all operation-boundary/no-integration clauses. |
| X02 | BLOCKED | Retained operation-41 direct/gateway exact parity passes. | Unified operation-36 direct/gateway/CLI projection and typed-failure parity is incomplete; host rendering separately fails L04. |

## Track statuses

- `COMMON_PROJECTION`: **BLOCKED** — every common cell has partial or missing clauses.
- `RETAINED_OBJECT_RESOLVER`: **BLOCKED** — no cell satisfies the complete custody/manifest/object-store predicate.
- `LIVE_SESSION_RESOLVER`: **FAIL** — L04 is a qualifying host-renderer failure; L01–L03 remain blocked.
- `ADR0007_INTEROPERABILITY`: **BLOCKED** — typed admission, cache, privacy, deletion, and authority prerequisites do not exist and execute.
- `COMPATIBILITY`: **BLOCKED** — retained parity and count guards are partial evidence only.

## Fixture identity

The execution record contains SHA-256 digests for all eleven files under `internal/sourceprojection/testdata/crossmode-v2`. The principal expected artifacts are:

- `expected.json`: `bf0a23db5c3d7a5f5b12724fedacda5d153d772788090aa167730fe541bb544b`
- `retained-expected.json`: `8a6b70b0cfd4736062fe9dbc02a17e82e2750638804c2a134f3eb44d888be66d`
- `retained-snapshot-v2.json`: `c3c50f9a4a4e342240497d273b7717332523347edfa45e075add85f299b34a4d`
- `structural.json`: `256e71a16fc45e525aaa3178d3f59b42423db4c5a8c99005edd927fe917e6564`

## Failures and blockers

The only qualifying FAIL is L04. It belongs to host rendering because L04 explicitly owns host parameter-help behavior. It must not be relabeled external or confused with canonical server-envelope correctness.

All other cells remain BLOCKED because current evidence is partial or a prerequisite artifact/policy does not exist. No setup failure was labeled FAIL. No historical receipt or mixed-revision report was used as current qualification evidence.

## Claim ceiling

This review does not promote `source_graph_complete`, authority, acceptance, retained status, replayability, publication eligibility, semantic admission, or implementation qualification. It does not authorize ADR0007 inference/indexing, shipment, deployment, or production behavior. `implementation_qualified` remains `false`.
