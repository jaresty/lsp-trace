# ADR-0011 call-hierarchy Layer-1 generic corpus v2 bytes review v6

## Disposition

```text
ADR0011_CALLHIERARCHY_LAYER1_GENERIC_CORPUS_V2_BYTES_GO
```

Reviewed independently in exact checkout `/Users/schwa/dev/lsp-trace-adr0011-c17` at repository HEAD `21d83d7f`, under the accepted opaque-Params amendment review-v7. This disposition accepts only the current v2 corpus bytes and manifest accounting. It grants no RED, implementation, runtime, production, activation, generated-output, or commit authority.

## Exact Params provenance and states

The accepted opacity-corrected fixture pin resolves exactly:

```text
commit       751b0276c46dbccc4899cfec32adf513c123b57f
fixture blob daea113100184745998cde6961d3164c0a3431ce
fixture SHA  e766074f72b2ff7d43a59d9532b52b3d5c150e4d30f3b61f431c9fab6a540dc2
```

The pinned fixture constructor preserves its supplied bytes and nilness with `Params: append(json.RawMessage(nil), params...)`. Its fake transport consumes ordinary `lspwire.Message` requests. Independent recursive pairing covered all 85 self-contained envelope occurrences. For every occurrence, the corrected v2 Params bytes are the exact generic request Params preimage mechanically obtained from the pinned fixture input bytes: present and available-empty inputs use the complete wrapper `{"payload_hex":"<lowercase hex of the fixture input bytes>"}`, including every wrapper byte; nil remains nil. No record uses a selected body/payload subspan as Params, and no Params value was reconstructed from an expected frame or implementation/runtime output.

The exact state accounting is:

```text
all envelope occurrences  85
present                    84
empty                       0
nil                         1
```

The 84 present values comprise 75 exact `{"payload_hex":"7b7d"}` wrappers plus one wrapper for each of the six fixed accepted failure-control inputs, one available-empty inner input, one `fixed-payload` input, and one `immutable-source` input. The sole nil occurrence has `params_state="nil"` and `params_hex=null`. Every present value is nonempty, lowercase, prefix-free, even-length hexadecimal encoding of the complete wrapper. Nil, available-empty input, and present nonempty input therefore remain distinct; no wrapper byte is dropped.

## Offline wire reproduction

For each of the 85 occurrences, an independent offline encoder combined only the fixed method, positive request ID, and decoded corrected Params bytes using the accepted generic JSON-RPC/LSP wire shape. It reproduced the committed complete request frame byte-for-byte, including the nil case encoded as an explicit `params:null` member. All 85 declared request-frame lengths and frame bytes remained unchanged.

This check is one-way validation of independently derived Params. It does not treat the frame as a Params source and does not authorize execution-time frame parsing.

## Request IDs and availability-aware invariants

Strict recursive traversal reproduced:

```text
available occurrences                 63
W25 unavailable/disabled occurrences  22
request-frame ID matches               85
response-frame ID matches              85
```

Every causal request ID is a positive JSON integer representable as `uint64`. For each available occurrence:

```text
driver.request.request_id
  == expected.identity.request_identity.request_key
  == complete request-frame JSON-RPC id
  == complete response-frame JSON-RPC id
```

For each of the 22 unavailable occurrences in the 11 W25 disabled pair records, `control.enabled`, `control.attachment_available`, and `control.observation_available` are all false. The disabled and enabled members have equal explicit request IDs and equal complete request and response frames; their common ID equals the enabled member's retained request key. The disabled member retains the even successor request-identity scalar as inert data only, and that scalar was not used as the causal equality criterion.

## Change isolation

Independent decoded comparison against the immediately prior committed v2 corpus recursively removed only `driver.request.params_state` and `driver.request.params_hex` from each envelope. All 25 resulting world objects were exactly equal. World 12 was already correct; the other 24 world files changed only those corrected Params fields, including nested W24/W25 envelopes.

Therefore no other driver field, decoded expected value, expected key, array order, request ID, transport event, or external-byte field changed. The expected denominator remains independent of all driver paths.

## Canonical manifest and corpus pins

The manifest and all 25 world files strictly decode and reproduce recursively key-sorted compact UTF-8 JSON with exactly one terminal LF. Manifest order is exactly `worlds/world-01.json` through `worlds/world-25.json`; the worlds directory has exactly those 25 regular nonsymlink files. Every world length and SHA-256 entry reproduces from complete file bytes.

Ordered concatenation of the 25 exact world files independently reproduces:

```text
aggregate byte_length 510478
aggregate sha256      abda056be300c0551c67d7e8393689f7b7a3974433283747d829487cf380e35f
schema                adr0011.callhierarchy.layer1.red-oracle-corpus/v2
source_head           ea049c2dad9be74e1b3e316fe042430be7f04c8e
```

The immutable source pins remain reachable and exact:

```text
v1 corpus tree at ea049c2  7d278859a92b2f4438cce3c1f80f1591cd9a8c1a
v1 corpus tree at 12d9af1  7d278859a92b2f4438cce3c1f80f1591cd9a8c1a
v1 manifest blob           80ebe165f3b92a9678a0dc6d891c46649e24885e
v1 worlds tree             2d11c2a58332d02c22ef27344a2f65a6c0165ca4
v1 boundary blobs          4c7bcfa1dc82401b65c58988102f75d97cefd971
                           e088a59c7e4203ab4db29102c8b2aa6129adc30c
                           e0c12dfc2daa0390f6ef9b76960a750fb0fac6dc
v1 RED fixture blob        daea113100184745998cde6961d3164c0a3431ce
```

Historical source-head commit `4f2b5504c1e4f4e559f48db7ab8e0c5b8ca70fa9` still contains no v1 corpus path.

## Inventory and cardinalities

The amendment table independently recounts exactly:

```text
v1 expected paths       162
preserve generic        133
preserve opaque          12
remove/defer             17
successor expected       145
```

Driver fields remain outside the 145-path expected denominator. The corpus independently reproduces:

```text
root envelopes                    25
world-24 inline worker envelopes  16
world-25 inline executions        22
world-25 ordered pairs            11
pair member envelopes             22
all envelope occurrences          85
```

The 17 removed/deferred paths remain exactly the four request-body fields, four request-payload fields, four response-body fields, four response-result fields, and `result.form`. No removed rich semantic field was restored through Params.

## Boundary

This review establishes exact opaque-Params fixture provenance, nil/present state preservation, one-way offline request-frame reproduction, change isolation, request-ID availability rules, canonical corpus/manifest pins, immutable predecessor reachability, expected-path accounting, and cardinalities only. It does not establish runtime execution, source completeness, implementation correctness, production behavior, feature identity, or authority beyond this corpus-byte gate.

```text
ADR0011_CALLHIERARCHY_LAYER1_GENERIC_CORPUS_V2_BYTES_GO
```
