# ADR-0011 Call-Hierarchy Layer-1 Generic V3 Attachment-Enable Corpus Bytes — Independent Review

## Disposition

```text
ADR0011_CALLHIERARCHY_LAYER1_GENERIC_V3_ATTACHMENT_ENABLE_CORPUS_BYTES_BLOCKED
```

Reviewed independently in exact checkout `/Users/schwa/dev/lsp-trace-adr0011-c17` at repository HEAD `78b652537e7a4727977fb89ced1f45ccf1f246a4`, with only the v3 manifest and its 25 world files modified before this review document was created.

The rematerialized corpus is **blocked** because 11 of the 69 occurrences physically carrying `expected.control` disagree with the newly materialized driver bit. This review grants no corpus, RED, implementation, runtime, production, activation, generated-output, commit, semantic, or feature authority.

## Independent method

The review used a fresh strict duplicate-key JSON decoder and did not invoke or trust a rematerializer. It independently:

1. decoded and canonically rebuilt the manifest and every world as recursively key-sorted compact UTF-8 JSON with exactly one terminal LF;
2. checked every manifest world length and SHA-256, the ordered aggregate, exact world-file membership, registry pin, and unchanged non-hash manifest fields;
3. traversed the physical corpus structure to enumerate roots, W24 workers, W25 executions, and W25 pair members;
4. read `driver.request.attachment_enabled` directly from every occurrence and classified it without using names to infer the stored value;
5. independently compared the stored mapping to the amendment’s occurrence-kind rule;
6. compared the driver bit to the three physical control values only where `expected.control` exists and separately checked worker omission;
7. compared every modified world to the exact `HEAD` version after recursively removing only the newly added driver field from every embedded envelope; and
8. checked immutable v1 and v2 Git trees against their accepted predecessor pins.

## Canonical bytes and manifest accounting

All 26 modified JSON files are canonical under the stated encoding rule. The 25 manifest paths are the exact lexical sequence `worlds/world-01.json` through `worlds/world-25.json`; the listed paths exactly equal the regular files in `worlds/`. Every per-world byte length and SHA-256 matches the manifest, and ordered concatenation reproduces the aggregate.

```text
manifest byte_length  4733
manifest sha256       2e95cf2f1c2e5af18978d3986b5e0b3d5d2e89b9807eef126b32355efa1d6b4a
registry byte_length  51098
registry sha256       8c660fa8450ddaf79ba6237fb6cb82695d28f4ea0c929420f5c80022def50933
aggregate byte_length 582723
aggregate sha256      509734db0f88e2ad0342a055ce577f00ba0ceef0e56fb8b90a26c25dece9791c
```

The registry bytes still match the manifest pin. Relative to `HEAD`, the manifest changed only per-world and aggregate lengths/hashes; schema, source pins, names, paths, order, and all other manifest values are unchanged.

## Structural inventory and stored mapping

Independent traversal reproduced:

```text
root envelopes                    25
world-24 inline worker envelopes  16
world-25 inline executions        22
world-25 ordered pairs            11
pair-member envelopes             22
all envelope occurrences          85
```

Every occurrence has exactly one non-null boolean `driver.request.attachment_enabled`. Stored values total:

```text
true   74
false  11
 total 85
```

The 11 stored false values are exactly:

```text
w25-pair/01/disabled through w25-pair/11/disabled
```

Every other stored value is true. Therefore the bytes satisfy the amendment’s literal `74 true / 11 false` occurrence-kind mapping.

## Blocking 69-control agreement failure

The physical domains are exact:

```text
occurrences physically carrying expected.control  69
W24 workers omitting expected.control              16
 total                                              85
```

All 16 omissions are exactly `w24-worker/01` through `w24-worker/16`. No control value was borrowed or projected for them.

However, the required agreement check fails for exactly 11 of the 69 physical-control occurrences:

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

For every listed execution:

```text
driver.request.attachment_enabled       = true
expected.control.enabled                 = false
expected.control.attachment_available    = false
expected.control.observation_available   = false
```

The amendment requires all three physical C values to agree with the independently materialized driver bit for all 69 physical-control occurrences and makes any disagreement a hard stop. The review cannot repair expected values, reinterpret physical false as omission, or exempt W25 executions. Consequently the corpus cannot receive a GO token even though its stored 74/11 mapping matches the amendment’s stated occurrence-kind list.

## Unchanged expectations, other drivers, and accounting

A recursive decoded comparison against exact `HEAD` removed only `driver.request.attachment_enabled` from every root and embedded envelope. After that removal, every complete world equals `HEAD` exactly. Therefore:

- every other driver field and value is unchanged;
- every expected-owned value is unchanged;
- W24 and W25 embedding, ordering, pair metadata, and all other structure are unchanged; and
- no expected value was rewritten to force agreement.

Because the sole added path is driver-owned and all prior expected-owned values and structures are unchanged, the accepted expected accounting remains unchanged:

```text
A production generic owner/view fields  116
B RED-derived assertions                 20
C fixture/control-only paths              9
retained expected paths                 145
removed/deferred paths                   17
```

The prior occurrence-sensitive totals likewise remain `9,825 / 1,735 / 765`, union `12,325 = 85 × 145`. This preservation does not cure the newly exposed 11-occurrence C disagreement.

## Immutable v1/v2 predecessors

The predecessor trees remain unchanged and equal their accepted pins:

```text
v1 tree at HEAD, ea049c2, and 12d9af1
  7d278859a92b2f4438cce3c1f80f1591cd9a8c1a

v2 tree at HEAD and abef88564d02c240f354f3392f809407744b72d0
  f728e3afa7713b8df8717a2e99c982fd95036741
```

No v1 or v2 corpus byte is modified in the reviewed worktree.

## Required correction and re-review

A successor must resolve the contradiction under explicit authority rather than silently choosing one side. At minimum it must specify whether the 11 even W25 executions belong to the enabled or disabled causal-input class, then rematerialize only authorized driver bytes and hashes or separately authorize an expected/control change. A fresh independent review must repeat exact-byte, 85-occurrence, 74/11 (or explicitly revised), 69/16, unchanged-content, 145/17, and v1/v2 checks.

Until that correction exists:

```text
ADR0011_CALLHIERARCHY_LAYER1_GENERIC_V3_ATTACHMENT_ENABLE_CORPUS_BYTES_BLOCKED
CORPUS / RED / SOURCE / TEST / PROTOTYPE / PRODUCTION AUTHORITY — NOT ISSUED
```
