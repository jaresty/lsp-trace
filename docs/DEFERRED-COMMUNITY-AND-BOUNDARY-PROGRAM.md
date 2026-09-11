# Deferred community and boundary program decision package

## Decision

**Status: DEFERRED.** This package does not authorize implementation of community detection, community schemas, boundary reporting, or instability diagnostics. It authorizes only the bounded, offline investigation described below after the investigation-admission gate passes. Program C implementation requires a separate decision after every implementation-admission receipt passes.

Basis: FR12 defers community diagnostics until the substrate and deterministic-analysis programs qualify; Program C requires a library, licensing, portability, determinism, and resource spike before implementation is decided. FR13 requires a qualified community artifact before boundary reporting and limits crossings to structural observations rather than business-boundary evidence.

## Authority boundaries

- Community labels are run-local identifiers, never feature identities.
- Crossings are structural observations under an identified projection, never product, ownership, service, organizational, or business-boundary evidence.
- A qualified library or successful spike does not itself authorize implementation.
- A community artifact cannot upgrade graph custody, source completeness, provider coverage, projection authority, or domain meaning.
- D01 execution or deployment, network access, installation, product-repository work, and a full test suite are outside this package.

## Gate I: admit the bounded investigation

Set `INVESTIGATION_ADMITTED` only when every row is supported by an addressable, current local receipt. Missing, stale, partial, waived foundational, or semantically invalid evidence is a failing row. The machine-readable index is `qualification/program-c/gate-i-receipts.tsv`; it must contain exactly one row for each ID I-01 through I-08. A passing row names a repository-relative regular receipt file, its exact SHA-256 digest, and `PASS`.

Each passing receipt is a reproducibility record, not a cryptographic attestation. It must declare the matching `gate_id`, `result: PASS`, `current: true`, repository revision, exact command, policy/matrix identity, tool identity, run identity, one or more input SHA-256 digests, result SHA-256 digest, and explicit counts or inventories for `PASS`, `FAIL`, and `BLOCKED` outcomes. Gate I does not require an authority, key, signature, opaque Program A token, or cross-user trust. A receipt records what one local run established under named inputs and policy; another user may reproduce it independently.

The repository checker has two distinct modes. Its default mode validates the DEFERRED decision-package and receipt-index shape without treating intentionally missing receipts as a CI failure. `--admission` evaluates Gate I and exits unsuccessfully unless every indexed local receipt is addressable, digest-matching, current, structurally complete, and passing. Checker success in default mode never sets `INVESTIGATION_ADMITTED`; structural receipt validation does not replace review of whether the named run actually satisfies its row.

| ID | Required receipt | Pass condition | Failure disposition |
|---|---|---|---|
| I-01 | Substrate qualification | A reproducible local run records the exact substrate policy/matrix, inputs, results, and all pass/fail/blocked cells; every required cell passes and blocked cells remain visible. | Keep Program C deferred. |
| I-02 | Deterministic-analysis qualification | Reproducible local runs qualify the required projection, components, metrics, PageRank/PPR, replay, resources, and CLI/MCP parity obligations under named inputs and policy. | Keep Program C deferred. |
| I-03 | Candidate/version inventory | Exact Infomap and Leiden libraries, algorithm variants, versions, maintainers, and implementation surfaces are identified without installation. | Investigation not admitted. |
| I-04 | License decision inputs | Candidate licenses and intended distribution/linkage modes are recorded for later authorized review; no compatibility conclusion is inferred from package metadata alone. | Investigation not admitted. |
| I-05 | Portability/packaging questions | Required operating systems, architectures, toolchains, CGO/native dependencies, and packaging constraints are explicitly listed. | Investigation not admitted. |
| I-06 | Determinism questions | Seed handling, input ordering, label canonicalization, numeric/replay scope, and implementation-version identity are explicit. | Investigation not admitted. |
| I-07 | Resource envelope | Candidate node/edge sizes, memory/time ceilings, timeout behavior, and typed incomplete/failure policy are explicit. | Investigation not admitted. |
| I-08 | Neutrality rules | Outputs are prohibited from asserting feature or business-boundary identity, and negative examples are specified. | Investigation not admitted. |

Mechanical rule:

```text
INVESTIGATION_ADMITTED = PASS(I-01..I-08)
```

No partial admission exists. A receipt may close only its own row.

## Bounded investigation authorization

When Gate I passes, the investigation is limited to a decision spike with these bounds:

- **Questions:** candidate API fitness; license compatibility inputs; cross-platform build feasibility; deterministic replay; seed and label canonicalization; resource behavior; boundary-accounting definitions; repeated-seed instability definitions.
- **Inputs:** already retained, authenticated graph and projection fixtures plus locally available candidate source/package metadata. No reacquisition and no network fallback.
- **Candidate algorithms and implementations:** the proposed primary is Gonum's native-Go Leiden implementation only after it appears in an exact tagged Gonum release. Gonum Louvain from that same release is the comparator. Infomap is optional external-reference material for directed-flow diagnostics only; it is not a linked or production dependency and requires separate provisioning and license approval before any admitted execution. `vtraag/leidenalg` is rejected because its Python/C++/igraph packaging and GPL-3.0-or-later boundary conflict with the preferred native-Go, permissively licensed dependency profile. No unreleased Gonum commit is admitted. Changing these roles or adding an algorithm requires revising this package before work begins.
- **Fixture cap:** at most six retained fixtures covering directed, weighted, disconnected, singleton, high-degree-hub, and adversarial-order cases; one fixture may cover multiple categories.
- **Execution cap:** at most three deterministic runs per seed per candidate per fixture, plus one input-order permutation per fixture. Larger statistical studies require a new authorization.
- **Resource cap:** each run must declare wall-clock and memory ceilings before execution; ceiling exhaustion is typed failure/incomplete, never silent sampling, fallback, approximation, or algorithm substitution.
- **Outputs:** one candidate matrix, retained command/environment receipts, replay/ordering observations, resource observations, license-review inputs, boundary-definition proposal, instability-definition proposal, and an explicit recommendation of `REJECT`, `CONTINUE_INVESTIGATION`, or `IMPLEMENTATION_CANDIDATE`.
- **Stopping conditions:** stop immediately on unapproved license risk, unsupported required platform, non-replayable output within the declared scope, unbounded resource behavior, inability to canonicalize labels, missing admitted input/projection identity, or neutrality violation.

The spike must not add public schemas, CLI/MCP commands, operation-registry entries, production packages, deployment configuration, or community implementation code.

## Gate II: admit an implementation decision

`IMPLEMENTATION_CANDIDATE` is only a recommendation. A later authority may consider implementation only when all receipts below pass:

| ID | Required receipt | Pass condition |
|---|---|---|
| A-01 | Gate I snapshot | Exact Gate I receipts and their digests remain current and passing. |
| A-02 | Candidate selection | Library, algorithm variant, and implementation version are selected with rationale against both evaluated candidates. |
| A-03 | License approval | Authorized license review approves the selected distribution/linkage model. |
| A-04 | Portability/packaging approval | Required platform builds and packaging policy are feasible and reproducible under declared constraints. |
| A-05 | Determinism replay | Identical admitted bytes, projection, parameters, ceilings, seed, implementation, and declared runtime scope reproduce canonical labels and logical digest; input permutation produces the canonical equivalent. |
| A-06 | Resource trials | Every bounded fixture reports exact limits and outcomes; no silent fallback, truncation, substitution, or approximation occurs. |
| A-07 | Boundary accounting contract | Proposed boundaries define intra/crossing occurrences, conductance or named equivalent, high-centrality crossing nodes, bridges/articulation points, path witnesses, hub-crossing status, projection identity, and denominator accounting. |
| A-08 | Instability contract | Repeated-seed comparison defines canonical matching, variation metrics, run/seed accounting, acceptable thresholds, and an explicit unstable/incomplete result. |
| A-09 | Neutrality review | Positive and negative examples show labels and crossings remain structural and run-local, with no feature or business-boundary claims. |
| A-10 | Test-plan review | Planned guards cover seed replay, label canonicalization, instability diagnostics, boundary accounting, resources, validation, compatibility, and CLI/MCP parity without claiming those tests already pass. |

Mechanical rule:

```text
IMPLEMENTATION_DECISION_ALLOWED = INVESTIGATION_ADMITTED && PASS(A-01..A-10)
```

Even when true, this rule permits a new implementation decision; it does not itself authorize implementation. The approving authority, selected scope, schema/version boundary, and delivery program must be recorded separately.

## Boundary and instability definitions required by Gate II

A boundary proposal must bind every value to an exact admitted graph digest, projection digest, projection policy, community artifact digest, algorithm/version, parameters, seed, and resource policy. It must account for all admitted relation occurrences and distinguish unavailable, incomplete, and empty outcomes. Representative paths and hub crossings remain witnesses under the projection, not semantic explanations.

An instability proposal must compare repeated seeded runs without treating numeric labels as stable identities. It must define deterministic community matching, unmatched-community handling, variation metrics, denominator accounting, thresholds, seed inventory, failed/incomplete runs, and whether the result blocks publication. A stable result within the tested seeds is not a universal stability claim.

## Falsifiable decision assertions

The repository checker for this package must emit assertion-specific results for:

1. `ASSERT_PROGRAM_C_REMAINS_DEFERRED`: passes only while the status is `DEFERRED` and the package says it does not authorize implementation.
2. `ASSERT_DECISION_PACKAGE_SCOPE`: passes only while the package authorizes bounded investigation and a later decision, never implementation or public delivery surfaces.
3. `ASSERT_INVESTIGATION_GATE_EXACT`: passes only while Gate I contains exactly I-01 through I-08 and uses `PASS(I-01..I-08)`.
4. `ASSERT_GATE_I_RECEIPT_INDEX`: passes only while the receipt index contains exactly I-01 through I-08 with no duplicate or extra rows and each row uses the declared state/path/digest shape.
5. `ASSERT_GATE_I_RECEIPT_I_01` through `ASSERT_GATE_I_RECEIPT_I_08`: admission-mode assertions pass only when the corresponding indexed local receipt is addressable, digest-matching, current, structurally complete, and passing; the checker requires all reproducibility fields and explicit pass/fail/blocked outcomes without requiring cryptographic authority.
6. `ASSERT_BOUNDED_INVESTIGATION`: passes only while questions, inputs, candidate roles, fixture/execution/resource caps, outputs, stopping conditions, and excluded activities are present.
7. `ASSERT_IMPLEMENTATION_GATE_EXACT`: passes only while Gate II contains exactly A-01 through A-10, depends on `INVESTIGATION_ADMITTED`, and states that passing permits only a later decision.
8. `ASSERT_BOUNDARY_NEUTRALITY`: passes only while crossings are structural observations and business-boundary inference remains prohibited.
9. `ASSERT_EXCLUDED_EXECUTION`: passes only while D01, deployment, network, installation, product work, and full-suite execution remain outside authorization.

Any failure preserves `DEFERRED`. Default checker success proves only package and index shape, not that an admission receipt has passed; only a successful explicit `--admission` evaluation may support `INVESTIGATION_ADMITTED`.

## Decision outcomes

- **Current:** `DEFERRED` — Gate I receipts have not been supplied by this package.
- **After Gate I passes:** `BOUNDED_INVESTIGATION_ONLY` — run only the stated spike.
- **After Gate II passes:** `IMPLEMENTATION_DECISION_ALLOWED` — submit a separate decision; do not begin implementation automatically.
- **Any failed, missing, stale, partial, or invalid row:** return to `DEFERRED` and retain the failing receipt.
