# ADR-0011 Call-Hierarchy Layer-1 Generic V3 Attachment-Enable Corpus Bytes — Independent Review V2

## Disposition

```text
ADR0011_CALLHIERARCHY_LAYER1_GENERIC_V3_ATTACHMENT_ENABLE_CORPUS_BYTES_BLOCKED
```

Reviewed independently in exact checkout `/Users/schwa/dev/lsp-trace-adr0011-c17` at repository HEAD `78b652537e7a4727977fb89ced1f45ccf1f246a4`, including the modified v3 manifest and 25 modified world files. This review created only this document.

The corrected corpus remains **blocked**. Its aggregate counts are `22 false / 63 true`, but all 22 W25 standalone execution bits are inverted relative to the corrected occurrence mapping and their physically retained controls. This review grants no corpus, RED, source, test, implementation, runtime, production, activation, generated-output, commit, semantic, or feature authority.

## Independent method

The review used a fresh strict duplicate-key JSON decoder and did not invoke or trust a rematerializer. It independently:

1. decoded and canonically rebuilt the manifest, registry, and every world as recursively key-sorted compact UTF-8 JSON with exactly one terminal LF;
2. verified every manifest world byte length and SHA-256, exact world-file membership, lexical aggregate order, aggregate length and digest, and registry pin;
3. traversed the physical corpus structure and enumerated all root, W24 worker, W25 standalone execution, and W25 pair-member envelopes;
4. read `driver.request.attachment_enabled` directly from every occurrence;
5. checked the corrected category mapping and compared each bit with the three physical `expected.control` booleans only where that object exists;
6. compared every modified world with exact `HEAD` after recursively removing only the new attachment-enable driver field;
7. checked that the manifest changed only world and aggregate lengths and hashes;
8. checked attachment-enable key placement for expected-data feed; and
9. checked the immutable v1 and v2 corpus trees against their accepted predecessor pins.

## Canonical bytes and manifest accounting

All reviewed JSON inputs are canonical under the stated encoding rule. The 25 manifest paths are exactly the lexical sequence `worlds/world-01.json` through `worlds/world-25.json`, and they exactly equal the regular files in `worlds/`. Every world length and digest matches its manifest entry. Ordered concatenation reproduces the aggregate.

```text
manifest byte_length  4733
manifest sha256       46c822ef9728988449cd3e7d9d491caed39ba84dd4a37b8c36bcf21cff68a8cf
registry byte_length  51098
registry sha256       8c660fa8450ddaf79ba6237fb6cb82695d28f4ea0c929420f5c80022def50933
aggregate byte_length 582734
aggregate sha256      616614ac15907b55afde074aede769ec22fe50a0c9efb2745986652642663897
```

The registry bytes match the manifest pin. Relative to `HEAD`, the manifest changed only per-world and aggregate byte lengths and hashes; schema, source pins, names, paths, order, and every other manifest value are unchanged.

## Structural inventory and aggregate bit counts

Independent traversal reproduced:

```text
root envelopes                    25
world-24 inline worker envelopes  16
world-25 inline executions        22
world-25 ordered pairs            11
pair-member envelopes             22
all envelope occurrences          85
```

Every occurrence has exactly one non-null boolean `driver.request.attachment_enabled`. The stored aggregate is:

```text
false  22
true   63
 total 85
```

The physical control domain is also exact:

```text
controls carrying false driver bits  22
controls carrying true driver bits   47
physical-control occurrences          69
W24 true/control-absent occurrences   16
all occurrences                       85
```

All 16 physical omissions are exactly the W24 inline workers. Each carries `true`; none carries or borrows `expected.control`.

## Blocking W25 standalone mapping inversion

The corrected mapping requires odd-numbered W25 standalone executions to be `true` and even-numbered standalone executions to be `false`. The reviewed bytes encode the opposite mapping for every one of the 22 standalone executions:

```text
w25-execution/01, /03, ..., /21  stored false; physical controls all true
w25-execution/02, /04, ..., /22  stored true;  physical controls all false
```

Thus all 22 W25 standalone occurrences disagree with the corrected occurrence rule and with each of their three physically retained controls:

```text
expected.control.enabled
expected.control.attachment_available
expected.control.observation_available
```

The aggregate `22 false / 63 true` and physical-domain `22 false / 47 true = 69` arithmetic therefore pass only as totals. They do not prove category membership. The required category partition fails exactly on all 22 standalone executions. Roots, W24 workers, and W25 pair members account for the remaining 63 occurrences, but their correctness cannot cure this exact-set failure.

## Unchanged expectations, other drivers, and expected accounting

For every world, recursively removing only `driver.request.attachment_enabled` from every root and embedded envelope makes the complete decoded world exactly equal to `HEAD`. Therefore:

- every other driver field and value is unchanged;
- every expected-owned field and value is unchanged;
- W24 and W25 embedding, ordering, pair metadata, and all other structure are unchanged; and
- no expected value was rewritten to force agreement.

The accepted expected accounting consequently remains unchanged:

```text
A production generic owner/view fields  116
B RED-derived assertions                 20
C fixture/control-only paths              9
retained expected paths                 145
removed/deferred paths                   17
```

The existing occurrence-sensitive totals remain `9,825 / 1,735 / 765`, union `12,325 = 85 × 145`. Every other driver value is unchanged. These preservation checks do not cure the 22-member category inversion.

## No expected-data feed

Across all 85 envelopes, `attachment_enabled` appears only at the envelope-local path:

```text
driver.request.attachment_enabled
```

It does not occur in any expected-owned object. Exact decoded equality with `HEAD` after removing that driver field also proves that no expected field was changed or added. The reviewed corpus therefore introduces no expected-data feed in its stored shape. This structural fact does not excuse the independently materialized bit from agreeing with the corrected mapping and downstream physical controls.

## Immutable v1/v2 predecessors

The predecessor trees remain unchanged and equal their accepted pins:

```text
v1 tree at HEAD, ea049c2, and 12d9af1
  7d278859a92b2f4438cce3c1f80f1591cd9a8c1a

v2 tree at HEAD and abef88564d02c240f354f3392f809407744b72d0
  f728e3afa7713b8df8717a2e99c982fd95036741
```

No v1 or v2 corpus byte is modified in the reviewed worktree.

## Required correction and final status

A successor must rematerialize the W25 standalone bits so odd IDs are true and even IDs are false, update only the authorized v3 world and manifest bytes/hashes, and receive another fresh independent exact-byte review. Totals alone are insufficient; the re-review must prove exact category membership and physical-control agreement.

Until then:

```text
ADR0011_CALLHIERARCHY_LAYER1_GENERIC_V3_ATTACHMENT_ENABLE_CORPUS_BYTES_BLOCKED
CORPUS / RED / SOURCE / TEST / PROTOTYPE / PRODUCTION AUTHORITY — NOT ISSUED
```
