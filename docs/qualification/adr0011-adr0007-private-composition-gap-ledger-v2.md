# ADR 0011 → ADR 0007 private composition v2 gap ledger

- **Schema:** `lsp-trace.adr0011-adr0007-private-composition-gap-ledger.v2`
- **Status:** corrected documentation candidate; not accepted; not GO
- **Base contract authority:** `docs/qualification/adr0011-adr0007-private-composition-contract.effe3f74.md`
- **Causal specification authority:** `docs/qualification/adr0011-adr0007-private-composition-causal-tests.1105f75e.md`
- **ADR0011 authority:** `docs/adr/0011-versioned-definition-reference-grouping-evidence.md`
- **ADR0007 authority:** `docs/adr/0007-optional-local-semantic-feature-index.md`
- **Positive-world scope:** `CAUSAL_TEST_ONLY`; never production-admissible

## Reading rule

“Derived” means uniquely reproducible from admitted predecessor bytes and an explicit formula. “Selected” means this test-only v2 fixture chooses a literal value where v1 requires a fact but does not choose its fixture value. “Authority” names the exact file and heading that requires or bounds the fact; line numbers are intentionally not used because they are unstable. No fixture selection is historical or production authority.

## Exhaustive ledger

| ID | Required positive-world fact | Exact v1 locator | Exact ADR authority locator | Required bytes/schema | Derived or selected | Identity/custody rule | Success effect | Refusal effect |
|---|---|---|---|---|---|---|---|---|
| G01 | One normative contract identity | v1 §2, §3, §4.11, §7, §10 | ADR0011 “Stage 5: identity and logical digest” | appendix `contract-definition` | selected bytes; identity derived | every dependent object carries exactly its identity | one identity closure | mismatch: `IDENTITY_CONFLICT`, zero stages |
| G02 | Closed ordered request roles | v1 §3 “Required request” | ADR0011 “Stage 7: additive artifacts and interfaces” | `PrivateCompositionRequestV2`, 47 roles | selected role order | request commits role/schema/id/SHA/length | request admitted | missing/duplicate/reorder: `PREDECESSOR_SUBSTITUTED` |
| G03 | Canonical LF JSON and ExactCarrier bytes | v1 §2 | ADR0011 “Stage 3: immutable grouping-input admission and composition” | appendix artifact/carrier base64 | selected spelling; checks derived | one LF; byte SHA distinct from domain identity | exact reconstruction | malformed/collision refuses before allocation/effect |
| G04 | Stage3 CALLS-only input | v1 §3.1, §4.5 | ADR0011 “Stage 3: immutable grouping-input admission and composition” | `reference-grouping-input.v2` | selected minimal occurrence | exact revision, occurrence, range, accounting | one admitted CALLS edge | substitution/imbalance refuses |
| G05 | Stage4 grouping policy | v1 §3.1–2, §4.5–6 | ADR0011 “Stage 4: versioned grouping policy” | `grouping-policy.v2` | selected `CAUSAL_TEST_ONLY` policy | exact policy identity and scope | fixture policy eligible only inside C01 world | never production-admissible |
| G06 | Frozen plan and corpus | v1 §3.2, §12 | ADR0011 “Qualification before thresholds” items 1–5 | evaluation-plan and corpus-manifest bytes | selected test-only corpus | bind plan/corpus identities and predecessor bytes | fixture criteria evaluable | absent artifact yields `GROUPING_POLICY_UNQUALIFIED` |
| G07 | Fixture/held-out partitions | v1 §3.2, §12 | ADR0011 “Qualification before thresholds” | partitions bytes | selected | exact partition identity | complete denominator | missing partition yields `INCOMPLETE` |
| G08 | Implementation and environment identities | v1 §3.2, §12 | ADR0011 “Qualification before thresholds”; “Stage 5” | implementation-environment bytes | selected non-executable fixture | explicitly `non-executable-fixture`, OS `none` | test reconstruction only | cannot qualify production |
| G09 | Complete metrics/results | v1 §3.2, §9, §12 | ADR0011 “Qualification before thresholds” items 4–6 | metrics bytes | arithmetic derived | expected=completed+missing; every criterion named | fixture criteria satisfied | imbalance yields `INCOMPLETE` |
| G10 | Report, timestamp, predecessor lengths/digests, maintainer acceptance | v1 §3.2, §12 | ADR0011 “Qualification before thresholds” paragraphs defining receipt | report and qualification-receipt bytes | selected fixed timestamp/reviewers; predecessor metadata derived | closed decision `QUALIFIED`, scope `CAUSAL_TEST_ONLY`, `production_admissible=false` | satisfies only causal C01 “Given” | production consumer must reject scope |
| G11 | Evidence, trust anchors, validity epoch | v1 §3.2, §4.6 | ADR0011 “Qualification before thresholds” | three exact carriers | selected test-only authority class | bind receipt and epoch exactly | fixture chain complete | stale/missing refuses |
| G12 | Stage5 composition/readiness | v1 §3.3–4 | ADR0011 “Stage 5: identity and logical digest” | composition/readiness bytes | selected | bind Stage3/4 and test scope | fixture `AVAILABLE`/`READY` | production remains unavailable |
| G13 | Stage5 grouping/successor/seed/algorithm | v1 §3.4, §4.10 | ADR0011 “Stage 5”; “Successor instability identity” | grouping, successor, seed, algorithm bytes | partition selected; identities/accounting derived | exact input/policy/composition/seed | one community | missing successor gives `GROUPING_UNSTABLE` |
| G14 | Stage6 representative/disclosures | v1 §3.5, §6 | ADR0011 “Stage 6: CALLS-only consumers and no-CALLS communities” | representative/disclosure bytes | selected CALLS representative | exact community/grouping custody | representative selected | no D/R promotion |
| G15 | Stage7A method receipts | v1 §3.6, §4.4, §5 | ADR0011 “Bounded source-bearing context” | separate query and target method receipts | selected fixture calls | role, method, provider, request, ordinal, session/generation exact | method custody present | missing/mismatched receipt refuses |
| G16 | Stage7A projection receipts | v1 §3.6, §4.4, §5 | ADR0011 “Bounded source-bearing context”; “Source-efficient many-to-one representation” | separate query and target projection receipts | selected policy | each binds method receipt and document digest | projection custody present | no ambient fallback |
| G17 | Distinct evidence/item/selection/display/context ranges | v1 §5 | ADR0011 “Bounded source-bearing context” | Stage7A source-context bytes | selected ranges; UTF arithmetic derived | five separately named ranges per role; no collapsed role | exact context selected | invalid containment/custody refuses |
| G18 | Independent query/reference and target custody | v1 §5 | ADR0011 “Bounded source-bearing context” | Stage7A roles and workspace binding | selected synthetic workspace | URI/version/revision/session/generation/encoding/digest/length retained independently | two terminal selected roles | mismatch: `SOURCE_CONTEXT_INCOMPLETE` |
| G19 | Stage7B snapshot/packet/checkpoint/replay | v1 §3.7, §4.7, §10 | ADR0011 “Snapshot, packet, and checkpoint”; “Replay” | five exact objects | selected limits; links derived | every object binds exact upstream Stage7A bytes | byte-identical replay | stale/foreign link refuses |
| G20 | Stage7C coupling | v1 §3.8, §4.9 | ADR0011 “Relation-scoped design coupling” | coupling bytes | Ca/Ce/I derived | exact CALLS scope and Stage7A/7B commitments | `Ca=1,Ce=1,I=1/2` | scope/family mismatch refuses |
| G21 | Stage7C instability | v1 §3.9, §4.10 | ADR0011 “Successor instability identity” | instability bytes | pair arithmetic derived | exact successor, seed, Stage7A/7B identities | stable fixture | missing/incomplete run refuses |
| G22 | Source/workspace binding | v1 §3 paragraph after inventory, §4.4 | ADR0011 “Bounded capability-aware acquisition”; “Bounded source-bearing context” | source-workspace-binding bytes | selected synthetic identities; hashes derived | truthful source bytes/digests remain separate from claimed Stage7A fields | custody verifies | mismatch refuses at admission precedence 4 |
| G23 | Full private policy inventory | v1 §3 final paragraph | ADR0007 “Operations and item-independent Describe”; “Closed terminal states and denominator accounting” | 13 policy carriers | selected | request commits exact bytes | deterministic policy closure | policy substitution refuses |
| G24 | Source-bearing packet | v1 §5 | ADR0007 “Assembled projection admission boundary” | packet bytes | derived from Stage3–7 and request | sole Describe predecessor; no reacquisition | packet admitted | missing mandatory source refuses |
| G25 | Describe semantics | v1 §7.1 | ADR0007 “Operations and item-independent Describe”; causal C25 | exact prompt/backend/response/citation bytes | prompt/response selected; SHAs derived | cites Stage7A target display custody | `DESCRIBED` | uncited/context-invalid result refuses |
| G26 | Search semantics | v1 §7.2 | ADR0007 “Operations and item-independent Describe”; causal C26 | exact LF query, deterministic tokenization policy, corpus tokens, score derivation, order bytes | query tokens derived as `caller,ok,returns,to`; exact score `4/4` derived | ASCII lowercase; maximal `[a-z0-9]+` runs; discard empty; deduplicate; lexical sort; corpus is exact Describe result | `MATCHED` | incomplete member remains explicit |
| G27 | Group semantics | v1 §7.3 | ADR0007 “Operations and item-independent Describe”; causal C27 | membership rule/groups/unmatched bytes | membership derived from Search | labels excluded from identity authority | one deterministic group | unmatched remains explicit |
| G28 | Location semantics | v1 §7.4 | ADR0011 “Bounded source-bearing context”; causal C28 | location request/result with exact target carrier/custody | derived from Group + Stage7A | no source selection or repair | one `LOCATED` carrier | unavailable/ambiguous explicit |
| G29 | Source Text Search semantics | v1 §7.5 | ADR0007 “Assembled projection admission boundary”; causal C29 | exact pattern/source/match ordinal/byte+UTF16 ranges | one literal match derived | searches only admitted target bytes | multiplicity 1, `MATCHED` | cannot repair Location/Stage7A |
| G30 | Candidate assembly and citations | v1 §7.6 | causal C30; ADR0007 claim ceiling | Candidate request/result bytes | derived from Group+Location+STS | citation names Location and STS identities, match/range/digest | one private candidate | no direct Stage7A-only citation manufacture |
| G31 | Six stage accounting and checkpoints | v1 §7, §9, §10 | ADR0007 “Closed terminal states and denominator accounting” | six results and six checkpoints | equations/links derived | each success `1/1`, prior checkpoint hash-linked | six evaluations/checkpoints | refused stage has no completion checkpoint |
| G32 | Final generation/effects | v1 §8, §11 | ADR0007 “Immutable admission, identity, cache, and invalidation” | final generation bytes | identity/effects derived | binds contract/request/results/final checkpoint | 6 evaluations, 6 checkpoints, 1 private commit, 0 publication | no generation on refusal |
| G33 | Exactly one semantic mutation | causal C03, C08, C12, C20 | ADR0011 “Bounded source-bearing context” | target Stage7A `source_digest` only | selected one-field change | every Stage7B/7C/request/packet commitment is mechanically resealed; truthful workspace binding unchanged | none | earliest admission custody check returns `SOURCE_CONTEXT_INCOMPLETE` |
| G34 | Mutation accounting/effects | v1 §4.4, §7–§9 | ADR0007 denominator rules | refusal bytes | derived from precedence 4 | no stage member started; all six expected members missing | none | stage evaluations/checkpoints/candidates/private commits/publication all zero |
| G35 | Reconstruction and freeze | v1 §2, §10 | ADR0011 “Replay” | appendix plus content-addressed copies/manifest | derived after checks | decode base64; verify LF/length/SHA/domain identity/link graph | reviewable bytes | mismatch blocks candidate; never repaired |
| G36 | Claim ceiling | v1 §13 | ADR0011 “Authority and claim ceiling”; ADR0007 “Authority and independent adjudication” | literal ceiling | selected unchanged | applies to positive and refusal worlds | authority remains 0 | forged elevation is semantic forgery |

## Test-only qualification boundary

No repository artifact supplies an independently qualified Stage4 grouping receipt for this composition. `docs/qualification/adr0011-definition-reference-grouping-plan.md`, under “Closed decision gate,” explicitly says the draft supplies none. The causal specification permits a test-only builder to produce structurally valid exact carriers but forbids production qualification minting. Accordingly, this v2 fixture includes every ADR0011-required receipt field and uses closed decision `QUALIFIED` only within `scope=CAUSAL_TEST_ONLY`, with `production_admissible=false`. It satisfies the assumed predecessor in causal case C01 only. Production admission must reject this scope and remains blocked.

## Mutation boundary

The one semantic change is `stage7a-source-context.roles[1].source_digest`. Mutated snapshot, target packet, Stage7 checkpoint, replay request/result, coupling, instability, composition request, generation key, and source-bearing packet are derived reseals, not additional semantic changes. The truthful source/workspace digest remains unchanged, so v1 admission §4 step 4 is the earliest applicable guard. No private stage starts.

## Normative and handoff boundary

Historical omitted-selector `CALLS_ONLY` behavior and canonical bytes remain unchanged. This v2 is normatively limited to `CAUSAL_TEST_ONLY`, is non-production-admissible, and neither mints nor recovers historical production qualification.

Non-normative handoff only: future production composition requires a new additive versioned CALLS qualification/custody regime covering method/provider custody, admitted occurrences, relation-family policy, grouping qualification, source/replay custody, instability, coupling, accounting, and the unchanged ceiling. This sentence is not a carrier, fixture input, implementation authorization, or selected regime.

## Exclusion boundary

This ledger does not amend the accepted ADRs, frozen v1 contract, causal specification, historical bytes/readers, implementation, runner/library dependencies, CLI, MCP, catalog, capability discovery, public schemas, publication, release, deployment, acceptance, or GO.
