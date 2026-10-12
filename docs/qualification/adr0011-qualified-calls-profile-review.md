# ADR 0011 qualified CALLS profile amendment review

- **Review type:** independent amendment-governance and framework review
- **Repository HEAD:** `9d261a5765a9bec5a5aaf9a8964d61c188e1e01a`
- **Review scope:** documentation-only; no implementation, dependency, registration, activation, migration, source edit, or production-profile qualification
- **Terminal disposition:** `BLOCKED`

## Frozen author-source pins

| Source | SHA-256 |
|---|---|
| `docs/adr/0011-qualified-calls-profile-amendment.md` | `2fe2fb46385f92f2efea2e5ce613f34f25e166f74e71605028290fece0415219` |
| `docs/qualification/adr0011-qualified-calls-profile-causal-tests.md` | `6816d350eba760405a05d360168ffc1eb0cd9931e3507efa8924b9e5dd8621dd` |
| `docs/qualification/adr0011-qualified-calls-profile-evidence-manifest.json` | `09dec0d9c448fbd36bd0f39a03409fc353a5aace11bd71e296a1447e46141d4b` |
| `docs/qualification/adr0011-qualified-calls-profile-plan.md` | `d659b5cbdf51e293f2c81750cedd326212246e109f2dddde5c806ef5439f7575` |

## Authority inputs consulted

| Authority | Status used in this review | SHA-256 |
|---|---|---|
| `docs/adr/0011-versioned-definition-reference-grouping-evidence.md` | accepted ADR 0011 | `9318a706bdddc264531823992b8e66dc61e09132826b956f2578ed5a293bc2b4` |
| `docs/qualification/adr0011-production-admission-contract-freeze-v2.proposed.md` | proposed/unaccepted contract-v2 evidence; informative constraint only, not accepted authority | `015a83a8ed1cad60121e9a9ec466550d0af033a6dece1f9dfe4fa3e75091133d` |

## Methods

1. Verified the exact worktree HEAD before review.
2. Computed SHA-256 over each of the four frozen author files without editing them.
3. Read the complete amendment, plan, causal-test specification, evidence manifest, accepted ADR 0011, and contract-v2 candidate.
4. Compared the amendment against ADR 0011's historical omitted-selector `CALLS_ONLY`, additive-family, claim-ceiling, authority, qualification, and fail-closed requirements.
5. Mechanically counted manifest roles and statuses with `jq`; checked for `PRESENT` roles and non-null evidence fields.
6. Reviewed causal coverage for occurrence multiplicity, provider/method custody, authority separation, Stages 3–7, accounting, privacy, resources, canonical identity, historical compatibility, absence/staleness/substitution/refusal, self-minting, independent review, and semantic-vector confinement.
7. Applied separate decisions for amendment-governance acceptance and concrete production-profile qualification.

## Findings

### Framework completeness and historical compatibility

The proposed framework is broad and generally aligned with accepted ADR 0011. It preserves omitted-selector `CALLS_ONLY`, forbids retrofit and migration, keeps historical canonical bytes and identities unchanged, uses an explicit additive selection, retains `authority=0`, `accepted=false`, and `completeness=UNKNOWN`, and separates method/provider custody, occurrence multiplicity, relation-family admission, grouping input, independent Stage 4 policy qualification, readiness, representative selection, source context, replay, coupling/instability, accounting, resources, privacy, canonical identity, and independent review.

The causal specification includes direct compatibility tests for historical bytes, explicit-selection isolation, and no retrofit/migration. The plan keeps historical failure isolated from the additive path. These portions are suitable amendment-governance material.

### Fail-closed admission and authority separation

The four files consistently reject inference from names, flags, prose, tests, hashes without payloads, producer assertions, self-review, and test-only receipts. They separate provider/method qualification from grouping-policy qualification and later composition/readiness. Missing, stale, malformed, substituted, duplicate, unbalanced, or unavailable evidence is intended to stop admission without qualified fallback.

Amendment-governance acceptance must not require any role to be `PRESENT`. The manifest's absence of production evidence is honest and is not itself an amendment defect. It must continue to preserve production refusal.

### 42-role manifest inventory and honest status

Mechanical result:

- required roles: **42**;
- `MISSING`: **28**;
- `PLANNED`: **14**;
- `PRESENT`: **0**;
- roles with any non-null locator, digest, byte length, or authority: **0**.

The manifest is honestly `PLANNED`, with `authority=0`, `accepted=false`, `completeness=UNKNOWN`, missing GO, and:

```text
production_admission = REFUSED_MISSING_EVIDENCE
```

That refusal is mandatory and correct. This review does not qualify the profile, does not convert any role to `PRESENT`, and does not issue `ADR0011_QUALIFIED_CALLS_PROFILE_GO`.

### Causal-test adequacy

Q01–Q30 provide substantial planned causal coverage, including exact positive test custody, production-authority absence, one-field mutations, direction and multiplicity, malformed and bounded outcomes, relation-family isolation, Stage 3 exact input, Stage 4 independence and held-out integrity, Stages 5–7, accounting equations, finite ceilings, privacy non-disclosure, canonical parsing and identity, historical bytes, explicit selection, no retrofit, test-only receipt rejection, self-minting rejection, reviewer independence, staleness, absence, refusal replay, and public/default-off inspection.

The tests are planned and unexecuted, so they establish design coverage rather than qualification evidence. Their statuses are honest. They do not repair the blocking identity rule below.

### Semantic-vector boundary

The plan, manifest, and causal specification consistently limit `/Users/schwa/dev/semantic-vector` to a future test-only runner over expectations frozen independently by domain and qualification owners. It may not generate expectations, grouping truth, thresholds, authority, evidence, review findings, manifest roles, or GO. It is not added, executed, or made a dependency by these files. This boundary is adequate as a test-only tool boundary, subject to the same permanent identity separation required below.

## Material blocking defect: the synthetic profile identity is promotable

The requested invariant is not merely that test evidence is currently inadmissible. `CALLS_SYNTHCALLS_EXACT_V1` must be permanently test-only and must be incapable of promotion, conversion, renaming, aliasing, inheritance, or successor reuse into a production profile. Any production profile must have a distinct profile identity and must obtain independently qualified production evidence under that distinct identity.

The frozen amendment does not prove that invariant. Its decision says `CALLS_SYNTHCALLS_EXACT_V1` is production-ineligible **unless later** authoritative content-addressed evidence establishes the required roles and an independent reviewer issues GO. Its production-admission section likewise says test fixtures and test-only receipts are inadmissible, but does not reserve the concrete profile identity itself permanently to test custody. The plan describes the same concrete profile as production-ineligible **unless later evidence establishes authority**. Those clauses permit the same named synthetic profile to cross from proposed/test use into production after later evidence.

Q23 prevents copying, upgrading, resigning, publishing, or registering a `TEST_ONLY` receipt, but it does not require a production candidate to use a different profile identity. Q21 specifically routes the additive verifier through the exact `CALLS_SYNTHCALLS_EXACT_V1` identity. Q25 permits a complete candidate chain to receive a later independent GO without first asserting that production uses a distinct non-synthetic profile identity. Consistently reauthoring evidence around the same profile name therefore remains allowed by the written framework.

The manifest reinforces current refusal but not permanent non-promotability: it names `CALLS_SYNTHCALLS_EXACT_V1` as the concrete profile, permits role status `PRESENT`, and defines a GO slot for that manifest. Its `PROPOSED_SYNTHETIC` provider status and test-only receipt policy prohibit current evidence admission; they do not make the profile identifier an immutable test-only namespace that production parsers must reject forever.

This is material because renaming or wrapping can otherwise launder test lineage. A sufficient amendment must state and causally test all of the following:

1. `CALLS_SYNTHCALLS_EXACT_V1` is permanently `TEST_ONLY` by identity, not merely by current evidence status.
2. Every production admission parser rejects that identity regardless of evidence completeness, authority hooks, signatures, reviewer identity, build tags, flags, configuration, aliases, wrappers, or GO-shaped tokens.
3. No production identity may alias, rename, convert, inherit from, supersede-in-place, reuse the canonical domain/preimage of, or treat the synthetic identity or its receipts as a production predecessor.
4. A production profile requires a separately selected name, schema identity, canonical domain, provider/method authority, manifest, predecessor closure, evidence set, and independent qualification; similarity or derivation from synthetic fixtures grants no production fact.
5. Tests mutate every identity bridge—alias, rename, wrapper, inheritance, conversion, successor marker, schema substitution, domain substitution, and rehashed descendant closure—and require typed refusal with zero production-visible effects.
6. Any later production GO is scoped only to that distinct production profile and its independently qualified evidence. It cannot target, upgrade, or transitively authorize `CALLS_SYNTHCALLS_EXACT_V1`.

Until those rules are normative across the amendment, plan, manifest, and causal tests, amendment-governance acceptance would leave an identity-level promotion path open.

## Decision boundaries

### Amendment-governance decision

The framework is otherwise comprehensive, historically additive, fail-closed in the presence of missing evidence, explicit about authority separation, honest about its 42 absent/planned roles, and appropriately restrictive about semantic-vector. However, the permanent test-only identity requirement is a material governance invariant, and the frozen sources currently contradict it by allowing later qualification of the same synthetic profile identity. Amendment acceptance is therefore blocked pending correction of that invariant and its causal tests.

### Concrete profile qualification decision

Independently of the amendment defect, the manifest remains `PLANNED`; all 42 roles are `MISSING` or `PLANNED`; no evidence locator/digest/length/authority is populated; no independent production review exists; and GO is missing. Therefore:

```text
production_admission = REFUSED_MISSING_EVIDENCE
```

This review is not a profile qualification, not production authority, and not a GO artifact.

BLOCKED
