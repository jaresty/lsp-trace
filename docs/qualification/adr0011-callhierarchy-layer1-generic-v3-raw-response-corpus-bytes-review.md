# ADR-0011 Call-Hierarchy Layer-1 Generic V3 Raw-Response Corpus Bytes Review

Status: **GO**

## Review boundary

This is an independent strict review of exact repository `/Users/schwa/dev/lsp-trace-adr0011-c17` at HEAD `c461ec3d07723328a0dcee4de7e60dd070976e44` plus exactly six modified v3 corpus files. This review created only this document and made no corpus or other edit.

The accepted driver is the sole causal derivation input. Existing affected expected values, RED output, implementation output, and runtime output were not used as oracle operands. This disposition accepts only the bounded corpus-byte correction; it grants no RED, implementation, runtime, production, activation, semantic, or feature authority.

## Exact write set and occurrence census

Before this review document was created, `git diff --name-only` was exactly:

```text
sessionruntime/testdata/callhierarchy-layer1-red-v3/manifest.json
sessionruntime/testdata/callhierarchy-layer1-red-v3/worlds/world-07.json
sessionruntime/testdata/callhierarchy-layer1-red-v3/worlds/world-08.json
sessionruntime/testdata/callhierarchy-layer1-red-v3/worlds/world-09.json
sessionruntime/testdata/callhierarchy-layer1-red-v3/worlds/world-10.json
sessionruntime/testdata/callhierarchy-layer1-red-v3/worlds/world-11.json
```

Independent recursive traversal reproduced exactly 85 self-contained occurrences: 25 roots, 16 World-24 workers, 22 World-25 executions, and 22 World-25 pair members. Decoded comparison to HEAD found changed expected values in exactly five occurrences: roots World 07 through World 11. Every driver decoded value is unchanged in all 85 occurrences. World 24 has 17 recursively visited occurrences including its root and World 25 has 45 including its root; neither contains an affected copy.

## Driver-only derivation and exact corrected values

For each occurrence, response custody was independently derived only from successfully emitted `driver.transport.events[kind=response-frame].frame_hex` values in event order.

World 07 has one emitted raw malformed frame:

```text
R hex  436f6e74656e742d4c656e6774683a206e6f70650d0a0d0a7b7d
N      1
L      26
SHA256 138a9db5f8d8eb54884d5df8d0ffba294626a607c70b7bae9191edae5226739d
```

The reviewed expected record retains those exact raw bytes as an available response frame, reports response bytes/messages `26/1`, and recomputes `accounting.bytes.value = 164 + 26 = 190`.

Worlds 08–11 contain only `cancel`, `deadline`, `shutdown`, and `unavailable`, respectively. Each independently yields `N=0`, `L=0`, unavailable frame custody (`false`, `null`, length zero, zero digest), available-empty `external.responses_hex=""`, and available-known-zero response byte/message and ledger accounting. Their aggregate accounting values exactly recompute from request frame length plus zero:

```text
World 08  170
World 09  162
World 10  162
World 11  188
```

Their existing error, failure, result, and terminal values remain unchanged and independently caused by their control events; no error serialization is treated as a response frame.

## Exact affected set, complement, and ownership

The authorized finite affected set is exactly 27 A paths:

```text
frames.response.available
frames.response.frame_hex
frames.response.frame_length
frames.response.frame_sha256
external.responses_hex
transport.bytes.available
transport.bytes.known_zero
transport.bytes.value
transport.messages.available
transport.messages.known_zero
transport.messages.value
accounting.bytes.value
round_trip_ledger.dimensions.external_bytes.after
round_trip_ledger.dimensions.external_bytes.available
round_trip_ledger.dimensions.external_bytes.before
round_trip_ledger.dimensions.external_bytes.delta
round_trip_ledger.dimensions.external_bytes.known_zero
round_trip_ledger.dimensions.transport_bytes.after
round_trip_ledger.dimensions.transport_bytes.available
round_trip_ledger.dimensions.transport_bytes.before
round_trip_ledger.dimensions.transport_bytes.delta
round_trip_ledger.dimensions.transport_bytes.known_zero
round_trip_ledger.dimensions.transport_messages.after
round_trip_ledger.dimensions.transport_messages.available
round_trip_ledger.dimensions.transport_messages.before
round_trip_ledger.dimensions.transport_messages.delta
round_trip_ledger.dimensions.transport_messages.known_zero
```

and exactly 3 B paths:

```text
derived.external.responses_bytes
derived.external.messages
derived.external.bytes
```

Decoded HEAD-to-candidate comparison found no changed value outside this 30-path set. It also proved every materialized path outside the set unchanged in each affected occurrence. Availability and ledger `before` operands in the finite set are explicitly revalidated even when their scalar values remain unchanged. The exact schema complement is therefore `145 - 27 - 3 = 115` unchanged paths.

The retained schema/accounting remains exact:

```text
A production generic owner/view fields       116
B RED-derived direct-actual assertions        20
C fixture/control-only assertions              9
                                             ---
retained expected paths                      145
removed/deferred paths absent                 17
```

All 17 removed paths remain absent from all 85 occurrences. No path changes A/B/C category.

## Commitments, other worlds, and predecessor immutability

For every occurrence, `driver.commitment_inputs` and `expected.commitments` are decoded-equal to HEAD. Every other driver value is also decoded-equal to HEAD. For affected roots, every expected value outside the authorized set is decoded-equal to HEAD.

Worlds 01–06 and 12–25 are byte-identical to HEAD. Their manifest entries are exactly unchanged. The manifest source object, schema, registry pin, v2 pin, and lexical world order are unchanged. Current immutable predecessor trees remain:

```text
v1  7d278859a92b2f4438cce3c1f80f1591cd9a8c1a
v2  f728e3afa7713b8df8717a2e99c982fd95036741
```

No v1 or v2 path is modified.

## Canonical bytes and hashes

Strict duplicate-key decoding and independent recursively key-sorted compact JSON serialization with exactly one terminal LF reproduced the manifest and all 25 world files byte-for-byte. The manifest lists exactly the secure lexical sequence `worlds/world-01.json` through `worlds/world-25.json`; there are no missing or extra world files. Every world length and SHA-256 matches its manifest entry.

Changed world values independently reproduce as:

| World | Byte length | SHA-256 |
|---|---:|---|
| 07 | 6415 | `96619af0be673e6409eca7be4f03332d5cbb2f344f80eca3a8896b9690d895f4` |
| 08 | 6656 | `52dc36e0d73704c6560c19bc0514495fa7bce58b1b828cf494eb3a8067d85970` |
| 09 | 5757 | `252d02590a7c84b64d32173384566c87b46daa46f0263e2257656d01397d26bc` |
| 10 | 6151 | `b7acfa7352fe1faabd0d25a43109be5a4bc1836f1aadf88a26daddf92d6c1ee9` |
| 11 | 6722 | `16686bf15a998d6101eff042d42951eba4cbfdde26ca9a76a47e4d4c8fe30f00` |

Ordered concatenation of all 25 exact world files independently reproduces:

```text
aggregate byte_length 581981
aggregate sha256      0816477c541f68dfd54a6454ee322e2b46db5d8f0b364103cc61e9ac870e00ff
manifest byte_length  4733
manifest sha256       cb9f6f4a7dc4df7f217b575c49991938d500684d2156905eb3a44fc7174e281c
```

## Disposition

No blocker was found. The exact finite write set, five-occurrence driver-only correction, 30-path affected set, 115-path complement, `85/145/17` accounting, `A/B/C=116/20/9`, commitments, all other worlds, canonical bytes and hashes, aggregate, and immutable v1/v2 boundaries pass.

```text
ADR0011_CALLHIERARCHY_LAYER1_GENERIC_V3_RAW_RESPONSE_CORPUS_BYTES_GO
```
