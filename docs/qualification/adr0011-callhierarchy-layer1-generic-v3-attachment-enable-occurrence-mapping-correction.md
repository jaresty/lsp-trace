# ADR-0011 Call-Hierarchy Layer-1 Generic V3 Attachment-Enable Occurrence-Mapping Correction

Status: **REVIEW_REQUIRED / CORPUS CORRECTION BLOCKED**

- Repository: `/Users/schwa/dev/lsp-trace-adr0011-c17`
- Pinned starting HEAD: `78b652537e7a4727977fb89ced1f45ccf1f246a4`
- Starting state: uncommitted v3 corpus rematerialization plus the blocked corpus-bytes review are present.
- Mutation by this correction: this document only.
- Corpus, RED, source, test, prototype, generated-artifact, execution, writer, commit, activation, carrier-acceptance, semantic, feature, and production authority: **not issued**.

## 1. Correction disposition

The prior attachment-enable occurrence mapping is incorrect. The exhaustive corrected mapping over the same 85 structural occurrences is:

```text
false = 11 even-numbered W25 standalone executions
      + 11 W25 disabled pair members
      = 22

true  = 11 odd-numbered W25 standalone executions
      + 11 W25 enabled pair members
      + 25 roots, including root/25
      + 16 W24 workers
      = 63

 total = 22 + 63 = 85
```

This correction supersedes the prior `11 false / 74 true` mapping. It does not authorize mutation of the currently rematerialized corpus or any other artifact.

## 2. Exhaustive occurrence partition

The categories below are disjoint and exhaustive.

| Occurrence class | Exact members | `false` | `true` | Total | Physical `expected.control` |
|---|---|---:|---:|---:|---|
| Roots | `root/01` through `root/25` | 0 | 25 | 25 | Present on all 25 |
| W24 workers | `w24-worker/01` through `w24-worker/16` | 0 | 16 | 16 | Omitted on all 16 |
| W25 standalone executions | `w25-execution/01` through `w25-execution/22` | 11 even IDs | 11 odd IDs | 22 | Present on all 22 |
| W25 pair members | `w25-pair/01/disabled`, `/enabled` through pair 11 | 11 disabled | 11 enabled | 22 | Present on all 22 |
| **Total** | exact structural occurrences | **22** | **63** | **85** | **69 present / 16 omitted** |

The exact false standalone set is:

```text
w25-execution/02
w25-execution/04
w25-execution/06
w25-execution/08
w25-execution/10
w25-execution/12
w25-execution/14
w25-execution/16
w25-execution/18
w25-execution/20
w25-execution/22
```

The other 11 standalone executions, with odd IDs `01` through `21`, are true. The exact false pair-member set is `w25-pair/01/disabled` through `w25-pair/11/disabled`; the corresponding 11 `/enabled` members are true.

## 3. Physical-control domain arithmetic

Exactly 69 occurrences physically carry `expected.control`:

```text
25 roots
22 W25 standalone executions
22 W25 pair members
--
69 physical-control occurrences
```

Their corrected enable-bit partition is:

```text
false = 11 even W25 standalone executions
      + 11 disabled W25 pair members
      = 22

true  = 25 roots
      + 11 odd W25 standalone executions
      + 11 enabled W25 pair members
      = 47

 total = 22 + 47 = 69
```

The 16 W24 workers omit `expected.control` and are independently mapped true. Their omission remains physical absence: it is not false, unavailable, a default, or a value inherited from `root/24`. They participate in the all-85 driver-bit census but not in physical C-value agreement.

The complete cross-domain arithmetic is therefore:

```text
physical controls:  false 22 + true 47 = 69
W24 omissions:      false  0 + true 16 = 16
all occurrences:    false 22 + true 63 = 85
```

## 4. Independent occurrence-registry rule

The offline occurrence registry must assign the bit solely by exact structural occurrence kind:

1. assign `false` to a W25 standalone execution iff its execution ordinal is even;
2. assign `false` to a W25 pair member iff its member kind is `disabled`;
3. assign `true` to every other occurrence: every root, every W24 worker, every odd W25 standalone execution, and every W25 `enabled` pair member.

The registry rule is an offline authoring rule only. Occurrence IDs, world numbers, execution ordinals, pair ordinals, and member labels may identify the record while authoring and auditing corpus bytes, but they are not runtime inputs and must not be consulted by RED execution or production code.

Each final envelope must be self-contained. It carries its own required, non-null boolean:

```text
driver.request.attachment_enabled
```

After authoring, that driver bit is the sole proposed enablement operand. Runtime must read the bit directly from the occurrence's driver data. It must not recover, infer, validate, repair, or override the bit from an occurrence ID, registry lookup, world/name/pair label, `expected`, commitment availability or content, attachment zero value, transport, result, error, terminal, owner state, sibling actual, or hidden side table.

The mapping requires no expected-data feed. For the 69 physical-control occurrences, any comparison to `expected.control.enabled`, `expected.control.attachment_available`, or `expected.control.observation_available` is downstream agreement checking only, after the driver bit has been independently materialized and structurally validated. The 16 W24 workers receive no such comparison and must preserve control omission without borrowing or manufacture.

## 5. Supersession and lineage correction

The prior contract stating:

```text
false = 11 W25 disabled pair members
true  = every other occurrence = 74
```

is superseded and invalid for future corpus, RED, prototype, test, source, or production reasoning. It incorrectly classified all 22 W25 standalone executions as true and thereby disagreed with the retained physical controls on the 11 even standalone executions.

Any replacement lineage whose branch construction, attempt-builder creation, owner publication, observation publication, or validation matrix relies on that `74 true / 11 false` partition is likewise superseded and invalid at that mapping boundary. This correction does not otherwise accept or reject the proposed private boolean field, setter, attempt builder, transfer seam, finalization seam, or publication design. Those remain proposals without authority.

Historical documents and reviews must remain immutable. They may continue to record the prior proposal and its blocked review as historical evidence, but they must not be cited as the current mapping contract after this correction.

## 6. Required lineage arithmetic and decision-matrix correction

Before any corpus correction is reviewed or performed, a successor documentation amendment must correct every affected finite statement and table, including:

- all-85 mapping arithmetic from `11/74` to `22/63`;
- physical-control arithmetic to `22 false / 47 true` over exactly 69 occurrences;
- explicit true treatment of all 16 W24 workers while preserving control omission;
- W25 standalone decision rows so even IDs are disabled and odd IDs are enabled;
- W25 pair-member decision rows so disabled members are false and enabled members are true;
- any owner/observation publication decision matrix that previously treated every standalone execution as enabled;
- every lineage edge or branch condition whose expected outcome depends on the superseded partition;
- corpus-rematerialization counts, hashes, and aggregate expectations that would change under corrected driver bytes; and
- review checklists and exact-set assertions so they prove both membership and cardinality, not totals alone.

The corrected decision matrix must include at least:

| Occurrence predicate | Driver bit | Physical-control action | Proposed runtime consequence |
|---|---:|---|---|
| Any root | `true` | Compare three present controls downstream | Eligible for enabled publication path |
| Any W24 worker | `true` | Verify control omission only; no C comparison | Eligible for enabled publication path |
| W25 standalone, odd ordinal | `true` | Compare three present controls downstream | Eligible for enabled publication path |
| W25 standalone, even ordinal | `false` | Compare three present controls downstream | Disabled publication path |
| W25 pair member `enabled` | `true` | Compare three present controls downstream | Eligible for enabled publication path |
| W25 pair member `disabled` | `false` | Compare three present controls downstream | Disabled publication path |

“Eligible” does not assert successful acquisition, observation, callback, release, semantic validity, or production behavior. Those remain actual outcomes to be established under separately accepted lineage and authority.

## 7. Re-review gate before corpus correction

A fresh independent documentation review must occur before any corpus correction. It must verify:

1. the exact 85-occurrence partition is complete, disjoint, and counted once;
2. the false set is exactly the 11 even W25 standalone executions plus the 11 disabled pair members;
3. the true set is exactly the other 63 occurrences;
4. the 69 physical-control domain partitions as 22 false and 47 true;
5. all 16 W24 workers omit controls, independently carry true, and have no C comparison or borrowed value;
6. the registry rule is offline-only and has no expected feed;
7. final driver envelopes are self-contained and runtime uses only the materialized bit;
8. the prior `74/11` contract and dependent replacement-lineage mapping are marked superseded rather than silently reinterpreted;
9. all affected lineage arithmetic and decision matrices are corrected consistently; and
10. no historical review, corpus, RED, source, test, prototype, generated artifact, or production file is mutated under this document.

Only after that review issues an explicit disposition may a separately authorized corpus correction be proposed. That later correction must itself receive a fresh exact-byte review. This document grants neither step.

## 8. Final status

```text
CORRECTED OCCURRENCE MAPPING: 22 FALSE / 63 TRUE
PHYSICAL-CONTROL DOMAIN:       22 FALSE / 47 TRUE = 69
W24 CONTROL-OMISSION DOMAIN:    0 FALSE / 16 TRUE = 16
PRIOR 11 FALSE / 74 TRUE CONTRACT: SUPERSEDED / INVALID
DEPENDENT REPLACEMENT-LINEAGE MAPPING: SUPERSEDED / INVALID
CORPUS CORRECTION: BLOCKED PENDING LINEAGE/DECISION-MATRIX CORRECTION AND RE-REVIEW
AUTHORITY: NOT ISSUED
```

This is a documentation-only correction. It establishes no corpus truth, runtime result, semantic commitment validity, feature identity, carrier acceptance, implementation approval, writer authority, activation, or production authority.
