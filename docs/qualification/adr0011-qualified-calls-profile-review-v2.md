# ADR 0011 qualified CALLS profile amendment review v2

- **Review type:** independent corrected-source amendment-governance and relation-family-symmetry rereview
- **Repository HEAD:** `9d261a5765a9bec5a5aaf9a8964d61c188e1e01a`
- **Review scope:** documentation-only; no implementation, evidence production, source mutation, registration, activation, migration, production-profile qualification, or commit
- **Amendment decision:** `ADR0011_QUALIFIED_CALLS_AMENDMENT_GO`
- **Concrete profile admission:** `REFUSED_MISSING_EVIDENCE`
- **Production-profile GO:** not issued

## Supersession and preserved review boundary

This v2 review supersedes only the `BLOCKED` amendment decision in `docs/qualification/adr0011-qualified-calls-profile-review.md`. It does not overwrite, amend, reinterpret as evidence, or replace that original reviewer artifact. The original remains byte-identical at SHA-256 `c79df498d162620e023d3cb7ce42326bf2985662bcadd624c58fd4f2700a0296` and remains the historical record of the defect it identified.

The prior review's source and authority boundaries remain explicit. This rereview adjudicates the corrected four-file author set against accepted ADR 0011 and consults the proposed contract-v2 artifact only as informative, unaccepted constraint material.

## Exact source pins

### Corrected author sources

| Source | SHA-256 |
|---|---|
| `docs/adr/0011-qualified-calls-profile-amendment.md` | `e59338d5c525b555cfee94ed446cfc6f50811bb950e690ddf34eec68e4be939a` |
| `docs/qualification/adr0011-qualified-calls-profile-plan.md` | `0becb74c634ae54b51e47371fe0a0842d1b4502de0a2a797277750025c77c8cf` |
| `docs/qualification/adr0011-qualified-calls-profile-evidence-manifest.json` | `884a7b802d5386c798854d54dbce8b5801a23c4596544ff44cba65db0f130d1d` |
| `docs/qualification/adr0011-qualified-calls-profile-causal-tests.md` | `e7e97a68d9fa081fb0f56c1a796dc8888087015ad96fcf946f57ea8b2192cc49` |

### Preserved reviewer and authority sources

| Source | Status in this review | SHA-256 |
|---|---|---|
| `docs/qualification/adr0011-qualified-calls-profile-review.md` | unchanged historical independent review; decision superseded only by this v2 artifact | `c79df498d162620e023d3cb7ce42326bf2985662bcadd624c58fd4f2700a0296` |
| `docs/adr/0011-versioned-definition-reference-grouping-evidence.md` | accepted ADR 0011 authority | `9318a706bdddc264531823992b8e66dc61e09132826b956f2578ed5a293bc2b4` |
| `docs/qualification/adr0011-production-admission-contract-freeze-v2.proposed.md` | proposed/unaccepted; informative only | `015a83a8ed1cad60121e9a9ec466550d0af033a6dece1f9dfe4fa3e75091133d` |

## Methods

1. Verified exact worktree HEAD and computed SHA-256 over every pinned source without modifying it.
2. Read the corrected amendment, plan, manifest, and causal specification and compared them with the original review's exact blocking defect.
3. Mechanically counted all manifest roles and statuses and checked the production-admission field.
4. Traced every prohibited synthetic-to-production bridge from normative amendment language through plan, manifest, and Q31 causal vectors, including required refusal timing and zero-effect accounting.
5. Compared CALLS, `REFERENCES_SYMBOL`, and `RESOLVES_TO_DEFINITION` treatment in accepted ADR 0011 across authority ceiling, custody, qualification, composition, readiness, replay, instability, coupling, accounting, resource ceilings, and shared implementation concepts.
6. Separately adjudicated amendment-governance acceptance from concrete production-profile qualification. No `PRESENT` role was required for the amendment decision.

## Findings

### 1. Original blocking defect is closed

`CALLS_SYNTHCALLS_EXACT_V1` is now permanently and irreversibly `TEST_ONLY` by exact identity and canonical namespace, rather than merely inadmissible because current evidence is incomplete. Completion of every role, review, receipt, signature, flag, configuration, build tag, privileged caller, or GO-shaped token cannot promote it. The manifest records `TEST_ONLY_IRREVERSIBLE`, `PERMANENTLY_FORBIDDEN`, `UNCONDITIONALLY_REJECTED`, no permitted override, and zero inherited authority.

All 13 prohibited bridge classes are normative and causally represented:

1. aliasing;
2. renaming;
3. conversion;
4. wrapping or embedding;
5. inheritance;
6. successor reuse;
7. schema-ID reuse;
8. canonical-domain reuse;
9. receipt reuse;
10. lineage or predecessor authority;
11. evidence copying, rehashing, resigning, or resealing;
12. profile-ID substitution; and
13. production GO naming or transitively relying on the synthetic identity.

For each class, amendment, plan, manifest, and Q31 require typed `SYNTHETIC_PROFILE_PRODUCTION_FORBIDDEN` before all production effects. The required effect count is exactly zero across writes, publications, registrations, admissions, dispatches, checkpoints, receipts, and authority changes. No fallback, repair, laundering, alternate success, partial effect, or successor authority is permitted. Nothing in the corrected set promotes the synthetic identity.

### 2. Forbidden inheritance and permitted independent qualification are distinct

The prohibition is against carrying synthetic identity, facts, bytes, receipts, lineage, predecessor status, review, or authority into production. It is not a prohibition on independently qualifying a different production CALLS profile.

A production candidate is eligible for evaluation only under a distinct profile ID, schema identity, and canonical domain, with independently selected provider/method authority and independently produced manifest, predecessor closure, evidence, receipts, review, and exact-profile-and-domain-scoped GO. It inherits exactly zero authority from `CALLS_SYNTHCALLS_EXACT_V1`. Reuse of general framework concepts or schema patterns does not transfer identity, custody, evidence, qualification, acceptance, or authority.

Thus forbidden synthetic authority inheritance and permitted independent production qualification are not conflicting paths: the former is categorically closed; the latter starts a separate authority chain.

### 3. Relation-family symmetry is preserved

Permanent test-only status attaches only to `CALLS_SYNTHCALLS_EXACT_V1`; it does not attach to the CALLS relation family, `QUALIFIED_CALLS_V1` as an abstract framework, or a distinct independently qualified production CALLS profile.

Accepted ADR 0011 establishes the common architecture rather than a hierarchy in which CALLS is categorically weaker. Production CALLS, references, and definitions remain governed by the same abstract requirements where applicable: authority and claim ceiling; exact method/provider and workspace/session/revision custody; immutable admitted occurrences; independent policy qualification; composition and readiness; canonical identities; deterministic replay; cross-seed instability; relation-scoped coupling; closed accounting; privacy; finite resource ceilings; substitution resistance; and independent review. It explicitly compares CALLS-only, no-CALLS D/R, and mixed modes and requires relation-scoped `Ca`, `Ce`, and instability treatment for CALLS, references, and definitions over shared admission, composition, identity, and replay concepts.

The legitimate differences are relation-specific: acquisition methods and capabilities, canonical direction, query-target requirements, occurrence shapes and multiplicity, inverse-lookup limits, family-specific policy projections, and CALLS-only representative/outward-consumer semantics. Those differences neither grant nor remove production eligibility.

Common framework concepts and schema patterns may therefore be reused as design structure, provided every concrete production identity and evidentiary chain is independently bound. Such reuse never implies authority inheritance. No corrected clause makes CALLS categorically weak, permanently test-only, or outside the accepted model.

### 4. Manifest and admission remain honestly incomplete

Mechanical manifest result:

- required roles: **42**;
- `MISSING`: **28**;
- `PLANNED`: **14**;
- `PRESENT`: **0**;
- `production_admission`: **`REFUSED_MISSING_EVIDENCE`**.

No role carries production evidence. This is correct for a planned synthetic test-only profile. Amendment GO assesses whether the governance correction is coherent; it does not require any role to become `PRESENT`, does not convert planned material into evidence, and does not qualify a production profile.

### 5. Historical compatibility remains closed

Omitted selection remains historical `CALLS_ONLY`. Its schemas, defaults, canonical bytes, snapshots, packets, checkpoints, replay, representative, coupling/instability, CLI/MCP behavior, errors, and exit behavior remain unchanged. Retrofit, migration, wrapping, reissue, or silent widening is forbidden. Additive failure cannot mutate the historical path or fall back while labeled qualified.

### 6. Decision boundary

The corrected documents close the exact identity-promotion defect identified by the original reviewer and preserve relation-family symmetry. The amendment framework may proceed as governance and planning material.

This decision does **not** admit `CALLS_SYNTHCALLS_EXACT_V1`, does not accept any manifest role as evidence, does not authorize implementation or a public surface, and does not issue `ADR0011_QUALIFIED_CALLS_PROFILE_GO`. Any distinct production profile remains subject to its own complete independent evidence chain and review.

ADR0011_QUALIFIED_CALLS_AMENDMENT_GO
