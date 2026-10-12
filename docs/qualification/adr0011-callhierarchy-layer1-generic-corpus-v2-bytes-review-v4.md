# ADR-0011 call-hierarchy Layer-1 generic corpus v2 bytes review v4

## Disposition

**BLOCKED.** The corrected request IDs equal their own retained expected request-identity keys in all 85 self-contained envelopes, but the 22 disabled world-25 occurrences do not equal the JSON-RPC IDs embedded in their fixed complete request and response frames.

```text
ADR0011_CALLHIERARCHY_LAYER1_GENERIC_CORPUS_V2_BYTES_BLOCKED
```

This disposition authorizes no corpus correction, RED change, implementation, activation, generated output, or commit.

## Reviewed state and scope

The independent review used exact checkout `/Users/schwa/dev/lsp-trace-adr0011-c17` at:

```text
1a9a71cb2e2b35bb99800cafeb21c12076a550c6
```

The pre-existing modified amendment, modified v2 corpus, untracked prior reviews, and untracked RED were preserved. Only this review file was created. No corpus, RED, implementation, amendment, prior review, or other file was edited.

## Request-ID equality

Recursive traversal found exactly 85 envelope occurrences. Every `driver.request.request_id` is present as an exact positive JSON integer representable as `uint64`, and every one equals that occurrence's retained `expected.identity.request_identity.request_key`.

The complete-frame equality requirement nevertheless fails:

```text
all envelope occurrences                         85
request IDs present/positive/uint64              85
retained expected request-key matches            85
retained expected request-key mismatches          0
request-frame ID matches                         63
request-frame ID mismatches                      22
available response-frame ID matches              62
available response-frame ID mismatches           22
unavailable response frames                       1
```

The 22 mismatches are the 11 disabled executions in `comparison.executions` and the same 11 disabled records repeated in `comparison.pairs[*].disabled`:

```text
record         driver/expected ID   request/response frame ID
execution-302          302                    301
execution-304          304                    303
execution-306          306                    305
execution-308          308                    307
execution-310          310                    309
execution-312          312                    311
execution-314          314                    313
execution-316          316                    315
execution-318          318                    317
execution-320          320                    319
execution-322          322                    321
```

Each row occurs once in `comparison.executions` and once as the corresponding pair's disabled member. The correction changed each disabled occurrence's driver ID from the enabled partner's odd ID to its own even expected key, but its request and response frames still embed the enabled partner's preceding odd ID.

Therefore none of those 22 occurrences satisfies the required three-way equality:

```text
driver.request.request_id
  == expected.identity.request_identity.request_key
  == complete request-frame JSON-RPC id
  == available complete response-frame JSON-RPC id
```

## World-25 records and pairs

The world-25 structural accounting and ordering are otherwise exact:

```text
execution records                  22
execution IDs                 301..322
ordered pairs                       11
pair disabled IDs      even 302..322
pair enabled IDs         odd 301..321
pair member envelopes               22
```

Every pair's disabled and enabled envelope equals the correspondingly named envelope in the execution array. The pair ordering is disabled-even followed by enabled-preceding-odd. The defect is confined to the disabled records' complete frames, not the execution/pair association or retained expected request keys.

## Canonical files and aggregate

The manifest and all 25 world files strictly decode without duplicate keys and are canonical compact recursively key-sorted UTF-8 JSON with exactly one terminal LF. The manifest lists exactly the secure lexical sequence `worlds/world-01.json` through `worlds/world-25.json`; the world directory has no missing or extra file. Every complete world envelope reproduces its manifest byte length and SHA-256.

Concatenating the 25 complete world files in manifest order independently reproduces:

```text
aggregate byte_length 506782
aggregate sha256      5f4e37e14b61cc5756c45e24735aeeed0ea2587a26fb5aec0ee9cc5b1e7a4760
```

The manifest hashes whole `{driver, expected}` envelopes, not `expected` alone. Correct canonical accounting does not cure the request-ID/frame contradiction.

## Prior corpus criteria

The independently rechecked and preserved accounting is:

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
```

Every occurrence remains a self-contained `{driver, expected}` envelope. Relative to the committed corpus at HEAD, recursively deleting only the newly added `driver.request.request_id` from every occurrence reproduces each complete prior world object exactly. Thus no other driver field or retained expected value changed.

The finite driver schema, external byte fields, params states and bytes, transport events, complete-frame bytes, availability distinctions, event kinds, and forbidden-semantic exclusions remain unchanged. The strict expected schema remains the 145-path v1-minus-17 projection, with no request/response body or payload/result subspan containers and no `result.form`.

## Required correction and boundary

A corrected candidate must make each of the 11 disabled world-25 records' fixed complete request and response frame IDs equal that record's own even request ID (`302` through `322`), in both its execution-array occurrence and repeated pair-member occurrence, while preserving exact frame length/SHA metadata and all other accepted criteria. It must regenerate canonical world-25 and manifest bytes and undergo another independent review of all 85 occurrences and the resulting aggregate.

This review establishes bounded corpus-byte, schema, cardinality, request-ID, frame-consistency, change-isolation, and manifest evidence only. It does not establish runtime execution, production behavior, source completeness, or implementation authority.

```text
ADR0011_CALLHIERARCHY_LAYER1_GENERIC_CORPUS_V2_BYTES_BLOCKED
```
