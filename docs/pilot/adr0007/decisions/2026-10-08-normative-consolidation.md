# ADR0007 normative consolidation decision

- **Date:** 2026-10-08
- **Status:** additive normative consolidation
- **Authority:** 0
- **Accepted:** false
- **Completeness:** `UNKNOWN`
- **Feature identity:** `UNRESOLVED`

## Decision

ADR0007's qualified local chain is recorded as:

```text
Describe -> Search -> Group -> Location -> Source Text Search
```

This is a documentation-only consolidation of existing repository evidence. It does not mutate frozen historical records, campaign artifacts, schemas, code, ADR0011, Program C, or public surfaces. It does not authorize production use, a release, a push, public CLI/MCP/API exposure, semantic feature acceptance, or stakeholder feature identity.

All entries below retain the common ceiling: `authority=0`, `accepted=false`, `completeness=UNKNOWN`, and `featureIdentity=UNRESOLVED`.

## Consolidated current chain

| Step | Qualified version and bounded role | Design/freeze identity | Execution/seal identity | Local integration commit(s) | Claim ceiling | Qualified current status |
| --- | --- | --- | --- | --- | --- | --- |
| Describe | `describe-custody-successor-2026-10-07`; bounded semantic usefulness over the exact 24-case / 96-attempt Describe campaign. | Prepare generation `g-730b55e6a0c4d080d6c371425e7fabcdda3f30f3060fbf20a6cfbf4e901c3743`; `PREPARE_FREEZE.json` files include `producer-request-records.json` `sha256:730b55e6a0c4d080d6c371425e7fabcdda3f30f3060fbf20a6cfbf4e901c3743`; `REVIEW_POLICY2_FREEZE.json` supersedes review freeze `sha256:4f43ed1a381cc07dab11141a7dd50f18d081c18d8225985aa423e431ae78950a`. | `THRESHOLD_EVALUATION.json` qualified `true`; policy `sha256:96fccb903445ddb6b55bf2090861e14e20e26f441348fa5726d1d97e62ac1e8d`; `QUALIFIED_REPORT.md` reports custody `96/96`, critical checks `48/48`, case agreement `22/24`. | Amendment records main `40440412`, source `8ae7f308`. | Bounded Describe semantic usefulness only; not feature identity, completeness, authority, public enablement, Search qualification, or Group qualification. | Current qualified Describe successor. The two replay-only limitations `item-09` and `item-14` remain historical limitations, not erased by the threshold amendment. |
| Search | Search v11 bounded deterministic custody. | Freeze identity `sha256:b1207bdd52463dcc81fd0c47eccae0452630cf0e96d2c4d72991d99a4c4fe301`. | `FINAL_SEAL.json` verdict `SEARCH_CUSTODY_GO`; execution manifest digest `sha256:bda8404feca6bad41b971dc8b951a8f19352002eed5a9e7f784d6af7c31236bf`; final audit digest `sha256:9bb9c9d3b5dfac0ad780a23fa6e7d3810a2516ab68f91e2b2eb61b58605d5b24`; terminal report digest `sha256:cefa9168920ba4da91bc8756cb1186efee28c9f47052249b1fede27b8d4c58cd`. | Main `4bc75e97`, source `9c44d561`. | Bounded deterministic Search custody only; no search inference was invoked; no production or public surface authorization. | Current Search successor for the exact frozen deterministic campaign. |
| Group | Group v1 bounded deterministic custody over mechanical candidate groups. | Freeze identity `group-freeze-f66b64da93d0ec4797d821fb788ec7afabeaca96c68375d9689bc88ad0ce6771`. | Existing decisions record `GROUP_CUSTODY_GO` and `INTEGRATION_GO`; infrastructure repair `1`; semantic retry / repair / substitution `0 / 0 / 0`. | Main `7f96fae9`. | Mechanical candidate grouping only. Group candidates are not features; the result does not establish semantic usefulness, navigation usefulness, representative usefulness, feature identity, or stakeholder acceptance. | Current Group successor for the exact frozen deterministic campaign. |
| Location | Location Intersection v5 successor custody. | Freeze root identity `sha256:1195a420cc2ae215ff1627dbf23b606caaa243aa9fc0ae234242acceddafb48d`; `FINAL_CUSTODY.json` status `IMPLEMENTATION_AGREEMENT_GO`; 26 cases; boundaries `W`, `W-1`, `B`, `B-1`; source chain records spec `637680d2`, input corrections `64afd44b` and `1546da22`, evaluator source `fccc0a03`, oracle source `1cff6a76`, integration `a9c82f1f`, merge base `d78e54d3`, skipped patch-equivalent `7354f53c`. | Final seal digest `sha256:f4981045d3489f5ef0633eb4ce6b4a17ab6de1ddc73c4b729f9dd524106b1fd6`; final seal commit `16f40dcb03a234b00db80059a7eef400495e9d97`; recommendation `LOCATION_CUSTODY_GO`. Retained chain evidence: successor head `f0f8b49aa368bea9b3e6d105eef5cb2614221067`; gate `sha256:8c40f5067796e5d5d5d31fb52b8b62e94208f8a7c94a21bbf80ad31ba3654908`; final audit `sha256:f87f9074cdf10eebfa141a1bf64d72a8f5c8ec4354ea76b9d7535949056278e0`; terminal report `sha256:60dd0003d7341130a20cf0fe8df0dd2ef5f28cfd22c71e7215470cbb9e7eacd0`. | `FINAL_AUDIT.json` records frozen tree `844d106b5270d94c49c0742472c42e76912bb228`, predecessor tree `bfc90d65ad8133c5a3ed3bea23f55f3ccfdc3332`, gate head `827978ee44e26929412da9d287649dcbb6f262cc`, and successor head `f0f8b49aa368bea9b3e6d105eef5cb2614221067`; main merge `dbebb5a2079c35bd8fe2ce8eac2aaab47b325958`. | Bounded location-relation custody only; Location relations remain bounded to the frozen v5 cases, boundaries, source bindings, and exact successor custody. | Current Location successor custody. It does not authorize broad location truth, production location services, or unbounded relation inference. |
| Source Text Search | Source Text Search v4 successor2 plus correction-generation custody. Exact literal mechanical search only. | Design successor2 commit `34ed9915`; successor2 root `sha256:f885c60275246dc660f07dffa53e3a929abd2105cf79c8b6a7597a8924decdbf`; correction envelope `sha256:46c4b3d140cb18e891b8e5471c2dc4d6a8ee0e7c84d60adebf96bc63089f4952`. | Sequence: correction execution/seal `cef7e0b4` is earlier evidence, not final custody; independent-review correction `80b7107d` namespace `source-text-search-v4-independent-review-correction-generation-8f6b2c4d` disposition `REVISE`; final-binding correction `5e5e32fc` namespace `source-text-search-v4-final-binding-correction-generation-80b7107d` with artifact pending external review and `go:false`; later independent terminal verdict `SOURCE_TEXT_SEARCH_QUALIFICATION_CUSTODY_GO`; main merge `90ac78d7` has second parent `5e5e32fc`. No earlier seal was final custody. | Main merge `90ac78d7` through second parent `5e5e32fc`; retained correction chain evidence includes `02ca9324`, `91f756c7`, `e75fc133`, `b10a8681`, and `cef7e0b4`. | Exact literal mechanical source text search only. It does not provide fuzzy search, semantic search, regex semantics, feature identity, production search, public API/CLI/MCP, or release authority. | Current Source Text Search custody only at the later independent terminal `SOURCE_TEXT_SEARCH_QUALIFICATION_CUSTODY_GO`. Production dispatch remained disallowed in earlier bindings, production artifacts were preserved by digest-reference, and no earlier seal is treated as final custody. |

## Superseded, blocked, or rejected predecessors that remain historical

| Current step | Historical predecessor identity | Current disposition | Why it remains historical |
| --- | --- | --- | --- |
| Describe | Generation `g-9a5fd445d96fbea3b1c37e9a33fb609b09dda81f765e216bd9ec0108dcb4ac0f` and review freeze `sha256:4f43ed1a381cc07dab11141a7dd50f18d081c18d8225985aa423e431ae78950a`. | Superseded by `describe-custody-successor-2026-10-07`. | The successor states the exact declared parameter count semantic change and a policy2 review change. Prior records remain evidence of the earlier attempt and are not rewritten. |
| Search | Search `v1` through `v10` in `experiment/search-custody-prospective-v11/PREDECESSORS.json`, bound into freeze `sha256:b1207bdd52463dcc81fd0c47eccae0452630cf0e96d2c4d72991d99a4c4fe301`. | Blocked predecessors. | Search v11 reached `SEARCH_CUSTODY_GO` only for the exact frozen deterministic campaign; blocked v1-v10 records remain unchanged inside the predecessor manifest. |
| Group | Blocked predecessors in `experiment/group-custody-prospective-v1/PREDECESSORS.json`: `Group execution`, `public CLI/MCP`, `ADR0011`, and `candidate-group`; candidate-group revision `448a1f4f`. | Blocked; candidate-group `448a1f4f` remains wholesale blocked pending explicit migration. | Group v1 supersedes only mechanical guard and custody concerns. The legacy candidate-group implementation is not partially admitted; selectors, receipts, `Get`, and `Put` remain unchanged. |
| Group | Prior `NOT_USEFUL` results and the rejected corpus. | Historical, not erased. | Group custody does not supersede semantic/navigation usefulness failures; representatives' usefulness remains unresolved. |
| Location | Location v1, v2, v3, and v4. | Immutable predecessors. | `FINAL_CUSTODY.json` records v1-v4 as immutable predecessors; v5 custody is bounded to the exact v5 cases and does not rewrite earlier design-audit history. |
| Location | Initial `location-intersection-v5-qualification-2026-10-07` blocked execution artifacts including `FINAL_SEAL_BLOCKED.json` and `FINAL_AUDIT_BLOCKED.json`. | Blocked historical execution. | The successor carries the corrected custody path; blocked pre-dispatch / zero-effect repair records remain the history of the failed path. |
| Source Text Search | `source-text-search-v4-qualification-34ed9915`. | Blocked predecessor campaign. | `AUTHORIZED_BINDINGS.json` for correction-generation names it as `blocked_predecessor_campaign_id`; it remains the blocked campaign that motivated correction. |
| Source Text Search | `source-text-search-v4-zero-effect-successor-02ca9324` and commit `02ca9324`. | Blocked / production-reference predecessor. | The later correction-generation campaign names `source-text-search-v4-zero-effect-successor-02ca9324` as `production_reference_campaign_id`; its artifacts are preserved by digest-reference and not used as a qualification oracle. |
| Source Text Search | `source-text-search-v4-correction-generation-91f756c7` at head `91f756c72317f09ba8ffac7c0b8933d46494bf0c`. | Historical correction-generation authorization / predecessor in the custody chain. | It records stakeholder repair provenance and blocked predecessor binding; the final custody is the correction chain through seal/merge, not a mutation of that history. |

## Non-authorizations

This consolidation authorizes none of the following:

- code, schema, frozen campaign, or frozen evidence byte changes;
- ADR0011, Program C, public CLI, MCP, API, registry, or release-surface changes;
- production execution or production enablement;
- release, push, publication, or external distribution;
- generated feature identity, stakeholder acceptance, ownership, completeness, design correctness, safety, or operational authorization;
- semantic feature grouping from Group candidates;
- unbounded Location relation inference;
- fuzzy, regex, semantic, or feature-level interpretation from Source Text Search.

Representatives' usefulness remains unresolved. Group outputs are mechanical candidates, not features. Source Text Search is exact literal mechanical search only. Location relations are bounded to the frozen campaign and successor custody artifacts. All generated products and qualifications retain `authority=0`, `accepted=false`, `completeness=UNKNOWN`, and `featureIdentity=UNRESOLVED`.

## Unknowns left explicit

No repository artifact inspected here established public/production/release authorization, accepted stakeholder feature identity, or representative usefulness. The consolidation therefore leaves those as unresolved rather than inferred.
