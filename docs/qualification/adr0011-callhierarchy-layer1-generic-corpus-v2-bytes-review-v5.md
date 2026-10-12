# ADR-0011 call-hierarchy Layer-1 generic corpus v2 bytes review v5

## Disposition

```text
ADR0011_CALLHIERARCHY_LAYER1_GENERIC_CORPUS_V2_BYTES_GO
```

Reviewed independently in exact checkout `/Users/schwa/dev/lsp-trace-adr0011-c17` at repository HEAD `1a9a71cb2e2b35bb99800cafeb21c12076a550c6`, under the accepted availability-aware amendment review-v6. This disposition accepts only the bounded v2 corpus bytes and their manifest accounting. It grants no RED, implementation, runtime, production, activation, generated-output, or commit authority.

## Availability-aware request-ID result

Strict recursive traversal found exactly 85 self-contained `{driver, expected}` envelope occurrences. Every `driver.request.request_id` is present as a positive JSON integer representable as `uint64`.

```text
all envelope occurrences                         85
available occurrences                             63
W25 disabled/unavailable occurrences              22
request-frame ID matches                          85
response-frame ID matches                         85
```

For every one of the 63 occurrences whose own retained control/attachment observation is available, independent decoding of the complete fixed frames established:

```text
driver.request.request_id
  == expected.identity.request_identity.request_key
  == complete request-frame JSON-RPC id
  == complete response-frame JSON-RPC id
```

For all 22 occurrences of the 11 W25 disabled records—once in `comparison.executions` and once in `comparison.pairs[*].disabled`—the retained even expected request-identity scalar remains present but inert. It was not used as the causal equality criterion. Each disabled occurrence instead satisfies:

- `control.enabled=false`;
- `control.attachment_available=false`;
- `control.observation_available=false`;
- its request ID equals both of its own complete fixed frame IDs;
- its request ID equals the enabled pair member's request ID and retained enabled request key;
- its complete request and response frames equal the enabled pair member's corresponding frames;
- its retained expected request key is the even successor of the enabled member's odd key.

Thus the corrected disabled IDs use the pair's enabled/fixed-frame ID while preserving each disabled record's even retained expected scalar as non-causal data.

## Canonical manifest and aggregate pins

The manifest and all 25 listed world files strictly decode without duplicate keys and reproduce canonical compact recursively key-sorted UTF-8 JSON with exactly one terminal LF. Manifest order and names are exactly `worlds/world-01.json` through `worlds/world-25.json`; the world directory has no missing, extra, or symlink entry. Every complete world file reproduces its manifest byte length and SHA-256.

Independent ordered concatenation of the 25 exact complete world files reproduces:

```text
aggregate byte_length 506782
aggregate sha256      aaf4d4c0519e5a692f7b190bfce3073397add2608371f5702508107118468c6c
```

The manifest schema remains `adr0011.callhierarchy.layer1.red-oracle-corpus/v2`, and `source_head` remains the normative corpus-containing commit `ea049c2dad9be74e1b3e316fe042430be7f04c8e`.

## Prior corpus and source-pin invariants

The accepted immutable pins independently reproduce:

```text
v1 corpus tree at ea049c2  7d278859a92b2f4438cce3c1f80f1591cd9a8c1a
v1 corpus tree at 12d9af1  7d278859a92b2f4438cce3c1f80f1591cd9a8c1a
v1 manifest blob           80ebe165f3b92a9678a0dc6d891c46649e24885e
v1 manifest SHA-256        c761601bf78195634163b98676e133bac2dff610c521be149b3445c9f620492a
v1 worlds tree             2d11c2a58332d02c22ef27344a2f65a6c0165ca4
v1 boundary blobs          4c7bcfa1dc82401b65c58988102f75d97cefd971
                           e088a59c7e4203ab4db29102c8b2aa6129adc30c
                           e0c12dfc2daa0390f6ef9b76960a750fb0fac6dc
v1 RED blob                daea113100184745998cde6961d3164c0a3431ce
v1 RED SHA-256             e766074f72b2ff7d43a59d9532b52b3d5c150e4d30f3b61f431c9fab6a540dc2
```

The historical internal source-head commit `4f2b5504c1e4f4e559f48db7ab8e0c5b8ca70fa9` still does not contain the v1 corpus path.

The accepted inventory remains exact:

```text
v1 expected paths                         162
preserve generic                          133
preserve opaque                            12
remove/defer                               17
successor expected                         145
root envelopes                              25
world-24 inline worker envelopes            16
world-25 inline execution envelopes         22
world-25 ordered pairs                      11
pair member envelopes                       22
all envelope occurrences                    85
```

The amendment's 162-row table has exactly 145 PRESERVE rows and 17 REMOVE/DEFER rows. Per-envelope own expected values equal the v1 record projection after deleting only the 17 named rich paths and adding only the 16 already-fixed v1 RED derivations. The current expected subtrees are unchanged from the committed prior v2 corpus. Recursively deleting only `driver.request.request_id` from all 85 current occurrences reproduces every complete prior v2 world object exactly; therefore no other driver field, expected key, array order, or decoded expected value changed.

The finite driver shape, params states and bytes, derived external-byte null/string rules, event kinds and event payload shapes, complete-frame length/SHA commitments, strict expected schema, forbidden body/payload/result-subspan containers, and absence of `result.form` remain preserved. Driver request IDs remain outside the 145-path expected denominator.

## Boundary

This review establishes canonical corpus bytes, manifest pins, strict decoded structure, source-pin custody, request-ID/frame consistency, availability-aware W25 pair consistency, change isolation, expected projection, schema accounting, and cardinalities only. It does not establish runtime execution, source completeness, implementation correctness, production behavior, feature identity, or authority beyond the corpus-byte gate.

```text
ADR0011_CALLHIERARCHY_LAYER1_GENERIC_CORPUS_V2_BYTES_GO
```
