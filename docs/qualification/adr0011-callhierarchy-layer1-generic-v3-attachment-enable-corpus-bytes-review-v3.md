# ADR-0011 Call-Hierarchy Layer-1 Generic V3 Attachment-Enable Corpus Bytes — Final Superseding Independent Review V3

## Disposition

```text
ADR0011_CALLHIERARCHY_LAYER1_GENERIC_V3_ATTACHMENT_ENABLE_CORPUS_BYTES_GO
```

Reviewed independently in exact checkout `/Users/schwa/dev/lsp-trace-adr0011-c17` at pinned repository HEAD `78b652537e7a4727977fb89ced1f45ccf1f246a4`, including the modified V3 manifest and all 25 modified V3 world files. This review created only this document. It supersedes the earlier attachment-enable corpus-byte reviews.

The corrected bytes satisfy the attachment-enable occurrence mapping and preservation contract. This is corpus-byte qualification only; it grants no RED, source, test, implementation, runtime, activation, semantic, feature, or production authority.

## Independent method

A fresh strict duplicate-key JSON decoder independently:

1. decoded and canonically rebuilt the manifest and every world as recursively key-sorted compact UTF-8 JSON with exactly one terminal LF;
2. verified exact world membership and lexical order, every manifest world byte length and SHA-256, ordered aggregate length and SHA-256, and the registry pin;
3. physically enumerated all 85 root, W24-worker, W25-standalone, and W25-pair-member envelopes;
4. read each envelope-local `driver.request.attachment_enabled` without using expected data;
5. checked exact category membership, not merely aggregate totals;
6. compared the bit with all three physical control booleans only where `expected.control` exists;
7. recursively removed only the new driver field and compared every decoded world with exact `HEAD`;
8. verified that the manifest changed only world and aggregate lengths and hashes; and
9. verified that the immutable V1 and V2 corpus trees are unmodified and retain their accepted tree identities.

No rematerializer or prior review verdict was trusted as evidence.

## Canonical bytes, hashes, and pins

All 26 modified JSON files are strict-decode valid and canonical under the stated encoding rule. Manifest paths are exactly `worlds/world-01.json` through `worlds/world-25.json`, in lexical order, and exactly equal the regular-file membership of `worlds/`. Every per-world byte length and SHA-256 matches its manifest entry.

```text
manifest byte_length   4733
manifest sha256        7d381e2a81c98d9af678eb7f5a38ff42630928e5baf8b435c4ef964a904b056f
registry byte_length   51098
registry sha256        8c660fa8450ddaf79ba6237fb6cb82695d28f4ea0c929420f5c80022def50933
aggregate byte_length  582734
aggregate sha256       b195c00a1325340a3c400bccafc1dc641ce7858f310c1c2b23b7e6cb7bf9d3af
```

The registry bytes exactly match the manifest source pin. Concatenating the 25 world files in the manifest's stated order exactly reproduces the aggregate length and digest. Relative to `HEAD`, the manifest changes only per-world and aggregate byte lengths and hashes; schema, source pins, names, paths, order, and every other manifest value are unchanged.

## Physical occurrence inventory and exact assignments

Independent physical traversal reproduced:

```text
root envelopes                    25
world-24 inline worker envelopes  16
world-25 standalone executions    22
world-25 ordered pairs            11
pair-member envelopes             22
all envelope occurrences          85
```

Every occurrence carries exactly one non-null boolean at the envelope-local path `driver.request.attachment_enabled`.

The exact false set is:

```text
w25-execution/02, /04, /06, /08, /10, /12, /14, /16, /18, /20, /22
w25-pair/01/disabled through w25-pair/11/disabled
```

The exact true set is:

```text
root/01 through root/25
w24-worker/01 through w24-worker/16
w25-execution/01, /03, /05, /07, /09, /11, /13, /15, /17, /19, /21
w25-pair/01/enabled through w25-pair/11/enabled
```

Therefore the standalone assignment is exactly odd `true` and even `false`; every pair is exactly `disabled=false` and `enabled=true`.

```text
false  22
true   63
total  85
```

## Controls and W24 omissions

Exactly 69 occurrences physically retain `expected.control`. For every one, each of these values agrees with the independently stored driver bit:

```text
expected.control.enabled
expected.control.attachment_available
expected.control.observation_available
```

There are no control mismatches.

```text
physical controls with false bit  22
physical controls with true bit   47
physical-control domain            69
```

The remaining 16 occurrences are exactly `w24-worker/01..16`. Every worker independently carries `attachment_enabled=true`, and every worker physically omits `expected.control`. No root control is borrowed, no worker control is manufactured, and omission is not interpreted as false or unavailable.

```text
W24 true/control-absent occurrences  16
all occurrences                      85
```

## Preservation: everything else, 145/17, and no expected feed

For every world, recursively removing only `driver.request.attachment_enabled` from every root and embedded envelope makes the complete decoded world exactly equal to `HEAD`. The semantic diff against `HEAD` is therefore exactly 85 field additions. The remaining manifest differences are only the 25 world length/hash pairs and aggregate length/hash pair.

Consequently:

- every other driver field and value is unchanged;
- every expected-owned field and value is unchanged;
- W24 and W25 embedding, ordering, pair metadata, and all other structure are unchanged;
- no expected value was rewritten to force agreement; and
- `attachment_enabled` occurs only at `driver.request.attachment_enabled`, never in expected-owned data.

The accepted accounting remains unchanged:

```text
A production generic owner/view fields  116
B RED-derived assertions                 20
C fixture/control-only paths              9
retained expected paths                 145
removed/deferred paths                   17
```

The 17 removed/deferred paths remain absent: four request-body, four request-payload, four response-body, four response-result, and `result.form`. Existing occurrence-sensitive totals remain `9,825 / 1,735 / 765`, with union `12,325 = 85 × 145`.

## Immutable predecessors

The predecessor corpus trees have no worktree modifications and retain their accepted tree identities:

```text
v1  7d278859a92b2f4438cce3c1f80f1591cd9a8c1a
v2  f728e3afa7713b8df8717a2e99c982fd95036741
```

No V1 or V2 corpus byte changed.

## Final status

```text
ADR0011_CALLHIERARCHY_LAYER1_GENERIC_V3_ATTACHMENT_ENABLE_CORPUS_BYTES_GO
CORRECTED MAPPING: FALSE 22 / TRUE 63
W25 STANDALONE: ODD TRUE / EVEN FALSE
W25 PAIRS: DISABLED FALSE / ENABLED TRUE
PHYSICAL CONTROLS: FALSE 22 / TRUE 47 = 69; ALL AGREE
W24: 16 TRUE / CONTROL ABSENT
PRESERVATION: EVERYTHING ELSE UNCHANGED
EXPECTED ACCOUNTING: 145 RETAINED / 17 REMOVED-DEFERRED
V1/V2: UNCHANGED
```

The GO token is limited to these exact modified V3 corpus bytes and their manifest accounting at the pinned HEAD. It does not authorize execution or any source, RED, test, implementation, semantic, activation, feature, or production change.
