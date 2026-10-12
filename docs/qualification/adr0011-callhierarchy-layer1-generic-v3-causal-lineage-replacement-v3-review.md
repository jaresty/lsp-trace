# ADR-0011 Call-Hierarchy Layer-1 Generic V3 Causal-Lineage Replacement V3 — Independent Review

Status: **DOCUMENTATION-LINEAGE GO / PRODUCTION BLOCKED**

- Reviewed repository: `/Users/schwa/dev/lsp-trace-adr0011-c17`
- Reviewed HEAD: `78b652537e7a4727977fb89ced1f45ccf1f246a4`
- Reviewed mapping correction: `docs/qualification/adr0011-callhierarchy-layer1-generic-v3-attachment-enable-occurrence-mapping-correction.md`
- Reviewed replacement: `docs/qualification/adr0011-callhierarchy-layer1-generic-v3-causal-lineage-replacement-v3.md`
- Review mode: independent documentation-only structural, arithmetic, anti-feed, and carrier-completeness review
- Mutation by this review: this document only
- Tests run: none
- Uncommitted corpus state: read-only and non-authoritative; all corpus findings below were derived independently from committed `HEAD` blobs
- Corpus, RED, source, test, prototype, generated artifact, write set, writer, implementation, activation, carrier acceptance, semantic authority, and production authority: **not issued**

## Review boundary

A documentation-lineage GO requires a finite current-or-proposed producer, explicit transfer, finalizer, and private publication carrier for every retained production-owned conditional. Mechanical totals are necessary but insufficient. Proposed symbols remain `PROPOSED_NORMALIZATION`; RED-only and fixture-only rows retain production seam `NONE`. This review does not treat structural evidence as runtime execution, implementation, carrier acceptance, semantic validity, or production authority.

The modified working-tree corpus was not used as authority. The authoritative corpus basis was the 25 files at:

```text
HEAD:sessionruntime/testdata/callhierarchy-layer1-red-v3/worlds/world-01.json
...
HEAD:sessionruntime/testdata/callhierarchy-layer1-red-v3/worlds/world-25.json
```

## Independent exact occurrence mapping

Independent traversal of committed corpus structure found this disjoint and exhaustive set:

| Occurrence class | Exact structural members | Count | False | True | Physical `expected.control` |
|---|---|---:|---:|---:|---|
| Roots | `root/01..25` | 25 | 0 | 25 | present on all 25 |
| W24 workers | `w24-worker/01..16` | 16 | 0 | 16 | omitted on all 16 |
| W25 standalone executions | `w25-execution/01..22` | 22 | 11 even | 11 odd | present on all 22 |
| W25 pair members | `w25-pair/01..11/{disabled,enabled}` | 22 | 11 disabled | 11 enabled | present on all 22 |
| **Total** | exact structural occurrences | **85** | **22** | **63** | **69 present / 16 omitted** |

The exact false standalone set is `/02`, `/04`, `/06`, `/08`, `/10`, `/12`, `/14`, `/16`, `/18`, `/20`, and `/22`. The exact false pair-member set is every `/disabled` member of pairs 01 through 11. Every root, every W24 worker, every odd standalone execution, and every `/enabled` pair member is true.

Committed physical controls independently enumerate as:

```text
false = 11 even W25 standalone controls
      + 11 disabled W25 pair-member controls
      = 22

true  = 25 root controls
      + 11 odd W25 standalone controls
      + 11 enabled W25 pair-member controls
      = 47

total = 22 + 47 = 69 physical controls
```

Exactly the 16 W24 worker records omit `expected.control`. Their omission is structural absence, not false, unavailable, inherited, or defaulted. Applying the correction's offline structural occurrence rule assigns those 16 workers true without manufacturing or borrowing C values:

```text
physical controls: false 22 + true 47 = 69
W24 omissions:     false  0 + true 16 = 16
all occurrences:   false 22 + true 63 = 85
```

Conclusion: replacement-v3's `22 false / 63 true`, controls `22 false / 47 true = 69`, and W24 `16 true / controls absent` statements exactly match committed corpus structure.

## Retained ownership and assertion accounting

Replacement-v3 preserves the carrier-complete v2 ownership model while correcting only the occurrence-sensitive enablement partition:

```text
ordinary occurrence: A=116, B=20, C=9, total=145
World-24 root:       A=81,  B=55, C=9, total=145

A = 84×116 + 81 = 9,825
B = 84×20  + 55 = 1,735
C = 85×9        =   765
union                 12,325 = 85×145
```

Only the 35 World-24 root ledger members migrate from A to B. The exact 17 removed/deferred paths remain absent: four request-body paths, four request-payload paths, four response-body paths, four response-result paths, and `result.form`. An independent scan of all 85 committed occurrence records found none of those deferred families.

Categories A, B, and C remain explicitly separated. B, C, World-24 aggregation, and World-25 equality retain production seam `NONE`; replacement-v3 does not invent production carriers for RED-only or fixture-only values.

## Refusal and wrapper-transfer preservation

Pinned current source establishes two entry routes:

```text
RoundTrip -> roundTrip -> roundTripWithPrivate -> roundTripWithPrivateTransaction
RoundTrip -> roundTripWithPrivate -> roundTripWithPrivateTransaction
```

It also establishes:

- the Manager private-accounting `ToolNotImplemented` branch as the first current branch in `RoundTrip`;
- the remaining pre-admission returns before `key := r.pending.Begin(req.Generation)`;
- key allocation as the admission boundary; and
- the common `finishRoundTrip` funnel for post-key terminal returns.

Replacement-v3 preserves v2's complete proposed chain:

1. read only the explicit attachment bit at the first executable `RoundTrip` line;
2. create exactly one enabled attempt builder, or nil when disabled;
3. for the Manager private-accounting branch and every other pre-admission refusal, call `recordRefusal` and then `finishRefusal` immediately before return;
4. preserve the public Manager `ToolNotImplemented` result while using the exact private refusal enum and `session.Failure("private-accounting-not-implemented")` reason;
5. publish exactly one immutable refusal owner for enabled refusal and no owner for disabled refusal;
6. pass the same attempt pointer explicitly through every ordinary and direct-private wrapper call site;
7. call `admit` immediately after key allocation on that same pointer; and
8. finalize accepted/post-key outcomes through `finishAccepted` and private owner publication.

The replacement continues to prohibit package globals, Manager fields, request-hidden state, context values, side tables, builder reconstruction, and replacement builders. Refusal terminal/admitted state remains unavailable. Known-zero refusal values are limited to effects proven untouched by refusal-before-effect; unavailable implementation values are not converted to zero.

No pre-key current return is left without the proposed refusal finalizer. No post-key current return bypasses the accepted terminal funnel. No ordinary or direct-private wrapper edge is missing the proposed explicit transfer. **No carrier gap was identified.**

## Anti-feed result

Replacement-v3 keeps the driver envelope self-contained and makes `driver.request.attachment_enabled` the sole proposed runtime enablement operand. Occurrence IDs, world labels, execution ordinals, pair labels, and the registry are restricted to offline corpus authoring and audit.

Expected data and C controls are downstream comparison operands only on the 69 occurrences where controls physically exist. They cannot construct or repair the driver bit, select execution, create a builder, choose refusal, populate actual observations, or produce B values. The 16 W24 workers receive no C comparison and borrow no root control. Commitment availability/content, zero attachment values, transport, result, error, terminal, failure text, sibling actuals, and hidden state likewise cannot enable or reconstruct lineage.

Thus the corrected mapping does not introduce an expected-data feed. The committed corpus currently supplies structural evidence for the mapping and physical-control agreement domain; the proposed driver field and runtime carrier remain unimplemented and unaccepted.

## Findings

1. **Exact mapping passes:** `22 false / 63 true` over exactly 85 occurrences.
2. **Physical-control domain passes:** `22 false / 47 true = 69`.
3. **W24 omission handling passes:** all 16 workers omit controls, are independently mapped true, and receive no C comparison or borrowed value.
4. **Ownership accounting passes:** `85×145 = 12,325`, with A/B/C ownership and the World-24 migration preserved.
5. **Deferred-path boundary passes:** the exact 17 removed/deferred paths remain absent.
6. **Refusal completeness passes:** the Manager private-accounting branch and every other pre-admission branch retain finite producer, transfer, refusal finalizer, and publication treatment.
7. **Wrapper transfer passes:** both current entry routes carry one proposed pointer explicitly through every wrapper to post-key admission.
8. **Anti-feed passes:** no expected, control, commitment, name, occurrence identity, sibling, or hidden-state runtime feed is authorized.
9. **Carrier-gap result:** none identified in the finite documentation lineage.

## Authority disposition

This review accepts only replacement-v3 as a complete and internally consistent documentation lineage at the pinned HEAD. It does not accept or authorize corpus rematerialization, RED execution, source changes, tests, prototypes, generated artifacts, write sets, writers, implementation, activation, carrier use, semantic conclusions, public API, release, deployment, or production behavior.

```text
ADR0011_CALLHIERARCHY_LAYER1_GENERIC_V3_CAUSAL_LINEAGE_REPLACEMENT_V3_GO
```

**GO documentation only. Production and all mutation remain blocked pending separate authority and subsequent exact-byte/runtime review.**
