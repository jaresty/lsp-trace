# ADR-0011 call-hierarchy Layer-1 generic corpus v2 bytes review v3

## Disposition

**BLOCKED.** The request-ID update does not satisfy the amended derivation contract for all 85 self-contained envelopes. Twenty-two world-25 envelope occurrences carry the enabled partner's odd request ID while retaining the disabled record's even request-identity key.

```text
ADR0011_CALLHIERARCHY_LAYER1_GENERIC_CORPUS_V2_BYTES_BLOCKED
```

This disposition authorizes no corpus correction, RED change, implementation, activation, generated output, or commit.

## Reviewed state and scope

The independent review used exact checkout `/Users/schwa/dev/lsp-trace-adr0011-c17` at:

```text
1a9a71cb2e2b35bb99800cafeb21c12076a550c6
```

The pre-existing modified amendment, modified v2 corpus, untracked amendment review, and untracked RED were preserved. Only this review file was created. No corpus, RED, implementation, prior review, or other file was edited.

## Blocking request-ID mismatch

Recursive traversal found the required 85 envelope occurrences. Every `driver.request.request_id` is present as an exact positive JSON integer representable as `uint64`. Every inserted ID equals the JSON-RPC ID in its complete request frame; every inserted ID also equals the ID in its complete response frame where that frame is available. World 7 has the one unavailable response frame and remains governed by its transport-error control.

The retained expected-source comparison fails for 22 occurrences:

```text
all envelope occurrences                         85
request IDs present/positive/uint64              85
request-frame ID matches                         85
available response-frame ID matches              84
unavailable response frames                       1
retained expected request-key matches            63
retained expected request-key mismatches         22
```

The mismatches are the 11 disabled executions in world 25 and the same 11 disabled records repeated as pair members:

```text
execution-302  request_id 301  expected request_key 302
execution-304  request_id 303  expected request_key 304
execution-306  request_id 305  expected request_key 306
execution-308  request_id 307  expected request_key 308
execution-310  request_id 309  expected request_key 310
execution-312  request_id 311  expected request_key 312
execution-314  request_id 313  expected request_key 314
execution-316  request_id 315  expected request_key 316
execution-318  request_id 317  expected request_key 318
execution-320  request_id 319  expected request_key 320
execution-322  request_id 321  expected request_key 322
```

Each mismatch occurs once in `comparison.executions` and once again as the corresponding `comparison.pairs[*].disabled` envelope. The request and response frames contain the same odd ID as the inserted driver field, so this is not a frame/corpus transcription disagreement. It is a failure to derive each disabled envelope's request ID from that envelope's own retained v1 `expected.identity.request_identity.request_key`, as required by the amendment. The disabled records instead inherit their enabled partner's ID.

## Change isolation

After removing only `driver.request.request_id` from every updated envelope, all 85 drivers decode equal to the committed v2 corpus at HEAD. No other driver field changed.

All retained expected oracle leaves remain equal to the committed v2 corpus. Worlds 24 and 25 necessarily show byte-level differences within their root `expected` objects because those objects contain complete nested envelopes and the nested drivers gained `request_id`; no retained expected value changed.

Therefore the candidate change is isolated to:

- one `driver.request.request_id` field in each of 85 envelope occurrences; and
- the manifest's resulting per-world and aggregate byte/hash metadata.

## Canonical files and aggregate

The manifest and all 25 world files strictly decode without duplicate keys and are canonical compact recursively key-sorted UTF-8 JSON with exactly one terminal LF. The manifest lists exactly the secure lexical sequence `worlds/world-01.json` through `worlds/world-25.json`; the world directory has no missing or extra file. Every complete world envelope reproduces its manifest byte length and SHA-256.

Concatenating the 25 complete world files in manifest order independently reproduces the reported aggregate:

```text
aggregate byte_length 506782
aggregate sha256      aaf4d4c0519e5a692f7b190bfce3073397add2608371f5702508107118468c6c
```

The manifest hashes whole `{driver, expected}` envelopes, not `expected` alone.

## Prior corpus criteria

The unchanged portions retain the previously reviewed criteria:

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
forbidden expected containers                0
other driver changes                          0
retained expected-value changes               0
```

Every occurrence remains a self-contained `{driver, expected}` envelope. The finite driver schema remains unchanged except for the amended required `request_id`; external byte fields, params states/bytes, transport events, complete frame bytes, availability distinctions, event kinds, and forbidden-semantic exclusions remain unchanged. The strict expected schema remains the 145-path v1-minus-17 projection, with no request/response body or payload/result subspan containers and no `result.form`.

## Historical pins and immutability

The required commits remain reachable ancestors of the reviewed HEAD. Independent Git-object verification reproduced:

```text
v1 corpus tree at ea049c2  7d278859a92b2f4438cce3c1f80f1591cd9a8c1a
v1 corpus tree at 12d9af1  7d278859a92b2f4438cce3c1f80f1591cd9a8c1a
v1 RED blob at 751b0276    daea113100184745998cde6961d3164c0a3431ce
v1 manifest SHA-256        c761601bf78195634163b98676e133bac2dff610c521be149b3445c9f620492a
v1 RED fixture SHA-256     e766074f72b2ff7d43a59d9532b52b3d5c150e4d30f3b61f431c9fab6a540dc2
```

Historical internal `source_head` `4f2b5504c1e4f4e559f48db7ab8e0c5b8ca70fa9` remains reachable and still does not contain the v1 corpus path. It remains metadata only.

## Required correction and boundary

A corrected candidate must assign each disabled world-25 envelope its own exact even request ID (`302` through `322`), derived from that envelope's retained request-identity key, while maintaining equality with that envelope's already fixed complete request and response frames. It must then regenerate canonical world-25 and manifest bytes and undergo another independent review of all 85 occurrences and the new aggregate.

This review establishes bounded corpus-byte, schema, cardinality, change-isolation, and historical-pin evidence only. It does not establish runtime execution, production behavior, source completeness, or implementation authority.

```text
ADR0011_CALLHIERARCHY_LAYER1_GENERIC_CORPUS_V2_BYTES_BLOCKED
```
